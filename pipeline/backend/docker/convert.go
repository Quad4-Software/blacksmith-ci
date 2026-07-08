// Copyright 2022 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/docker/docker/api/types/container"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// Valid container volumes must have at least two components, source and destination.
const (
	minVolumeComponents    = 2
	maxContainerNameLength = 64
)

// Container label constants for workflow discovery and adoption.
// These are re-exported from common for backwards compatibility.
const (
	LabelTaskUUID = common.LabelTaskUUID
	LabelStepUUID = common.LabelStepUUID
	LabelStepName = common.LabelStepName
)

// returns a container configuration.
func (e *docker) toConfig(step *types.Step, options BackendOptions, taskUUID string) *container.Config {
	e.windowsPathPatch(step)

	config := &container.Config{
		Image: step.Image,
		Labels: map[string]string{
			// New labels for workflow discovery and adoption
			LabelTaskUUID: taskUUID,
			LabelStepUUID: step.UUID,
			LabelStepName: step.Name,
			// Legacy labels for backwards compatibility
			"wp_uuid": step.UUID,
			"wp_step": step.Name,
		},
		WorkingDir:   step.WorkingDir,
		AttachStdout: true,
		AttachStderr: true,
		Volumes:      toVol(step.Volumes),
		User:         options.User,
	}
	configEnv := make(map[string]string)
	maps.Copy(configEnv, step.Environment)

	if len(step.Commands) > 0 {
		env, entry := common.GenerateContainerConf(step.Commands, e.info.OSType, step.WorkingDir)
		for k, v := range env {
			configEnv[k] = v
		}
		config.Entrypoint = entry

		// step.WorkingDir will be respected by the generated script
		config.WorkingDir = step.WorkspaceBase
	}
	if len(step.Entrypoint) > 0 {
		config.Entrypoint = step.Entrypoint
	}

	if len(configEnv) != 0 {
		config.Env = toEnv(configEnv)
	}
	return config
}

func toContainerName(step *types.Step) string {
	scheme := step.ContainerNameScheme
	// Use descriptive naming by default unless explicitly set to "hash"
	if scheme != "hash" {
		owner := step.Environment["CI_REPO_OWNER"]
		repo := step.Environment["CI_REPO_NAME"]
		pipeline := step.Environment["CI_PIPELINE_NUMBER"]
		workflow := step.Environment["CI_WORKFLOW_NUMBER"]
		// Only use descriptive format if all required environment variables are present
		if owner != "" && repo != "" && pipeline != "" && step.Name != "" {
			var name string
			if workflow != "" {
				// Include workflow number (including "1" for first workflow)
				name = owner + "-" + repo + "-" + pipeline + "-" + workflow + "-" + step.Name
			} else {
				// Fallback naming when workflow number is not available
				name = owner + "-" + repo + "-" + pipeline + "-" + step.Name
			}
			return sanitizeAndTruncateName(name, maxContainerNameLength)
		}
	}
	return "crow_" + step.UUID
}

// sanitizeAndTruncateName makes a name safe for Docker/Kubernetes and truncates to maxLen.
func sanitizeAndTruncateName(name string, maxLen int) string {
	// Lowercase, replace invalid chars with '-', collapse multiple '-'.
	name = strings.ToLower(name)
	name = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(name, "-")
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	if utf8.RuneCountInString(name) > maxLen {
		runes := []rune(name)
		name = string(runes[:maxLen])
	}
	return strings.Trim(name, "-")
}

// returns a container host configuration.
func toHostConfig(step *types.Step, conf *config) *container.HostConfig {
	config := &container.HostConfig{
		Resources: container.Resources{
			CPUQuota:   conf.resourceLimit.CPUQuota,
			CPUShares:  conf.resourceLimit.CPUShares,
			CpusetCpus: conf.resourceLimit.CPUSet,
			Memory:     conf.resourceLimit.MemLimit,
			MemorySwap: conf.resourceLimit.MemSwapLimit,
		},
		ShmSize: conf.resourceLimit.ShmSize,
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		Privileged: step.Privileged,
	}

	if len(step.NetworkMode) != 0 {
		config.NetworkMode = container.NetworkMode(step.NetworkMode)
	}
	if len(step.DNS) != 0 {
		config.DNS = step.DNS
	}
	if len(step.DNSSearch) != 0 {
		config.DNSSearch = step.DNSSearch
	}
	extraHosts := []string{}
	for _, hostAlias := range step.ExtraHosts {
		extraHosts = append(extraHosts, hostAlias.Name+":"+hostAlias.IP)
	}
	if len(step.ExtraHosts) != 0 {
		config.ExtraHosts = extraHosts
	}
	if len(step.Devices) != 0 {
		config.Devices = toDev(step.Devices)
	}
	if len(step.Volumes) != 0 {
		config.Binds = step.Volumes
	}
	config.Tmpfs = parseTmpfs(step.Tmpfs, conf.resourceLimit.MemLimit)

	return config
}

// helper function that converts a slice of volume paths to a set of
// unique volume names.
func toVol(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	set := make(map[string]struct{})
	for _, path := range paths {
		parts, err := splitVolumeParts(path)
		if err != nil {
			continue
		}
		if len(parts) < minVolumeComponents {
			continue
		}
		// Windows named pipes (e.g. \\.\pipe\docker_engine) are valid bind mounts
		// but cannot be declared as anonymous volumes: the Windows daemon requires
		// anonymous-volume mount points to be directories and rejects a pipe with
		// "Only directories can be mapped on this platform". The bind itself is
		// still forwarded via HostConfig.Binds.
		if isWindowsNamedPipe(parts[1]) {
			continue
		}
		set[parts[1]] = struct{}{}
	}
	return set
}

// helper function that converts a key value map of environment variables to a
// string slice in key=value format.
func toEnv(env map[string]string) []string {
	var envs []string
	for k, v := range env {
		if k != "" {
			envs = append(envs, k+"="+v)
		}
	}
	return envs
}

// toDev converts a slice of volume paths to a set of device mappings for
// use in a Docker container config. It handles splitting the volume paths
// into host and container paths, and setting default permissions.
func toDev(paths []string) []container.DeviceMapping {
	var devices []container.DeviceMapping

	for _, path := range paths {
		parts, err := splitVolumeParts(path)
		if err != nil {
			continue
		}
		if len(parts) < minVolumeComponents {
			continue
		}
		if strings.HasSuffix(parts[1], ":ro") || strings.HasSuffix(parts[1], ":rw") {
			parts[1] = parts[1][:len(parts[1])-1]
		}
		devices = append(devices, container.DeviceMapping{
			PathOnHost:        parts[0],
			PathInContainer:   parts[1],
			CgroupPermissions: "rwm",
		})
	}
	return devices
}

// helper function that serializes the auth configuration as JSON
// base64 payload.
func encodeAuthToBase64(authConfig types.Auth) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// parseTmpfs converts a list of tmpfs entries into Docker's tmpfs map,
// enforcing that tmpfs size does not exceed the agent's memory limit.
// If no size is specified and a memory limit is set, the size is capped to the limit.
// If a size exceeds the memory limit, it is capped to the limit.
func parseTmpfs(entries []string, memLimit int64) map[string]string {
	result := map[string]string{}
	for _, entry := range entries {
		if !strings.Contains(entry, ":") {
			result[entry] = enforceTmpfsSize("", memLimit)
			continue
		}
		parts, err := splitVolumeParts(entry)
		if err != nil {
			continue
		}
		result[parts[0]] = enforceTmpfsSize(parts[1], memLimit)
	}
	return result
}

// enforceTmpfsSize ensures the tmpfs options respect the agent's memory limit.
// Docker tmpfs options are comma-separated (e.g. "size=1G,mode=1770").
// If memLimit is 0 (no limit configured), options are passed through unchanged.
func enforceTmpfsSize(opts string, memLimit int64) string {
	if memLimit <= 0 {
		return opts
	}

	sizeLimit := formatBytes(memLimit)

	if opts == "" {
		return "size=" + sizeLimit
	}

	// Parse comma-separated options looking for size=
	parts := strings.Split(opts, ",")
	found := false
	for i, part := range parts {
		if strings.HasPrefix(part, "size=") {
			found = true
			sizeVal := strings.TrimPrefix(part, "size=")
			parsed, err := ParseMemory(sizeVal)
			if err == nil && parsed > memLimit {
				parts[i] = "size=" + sizeLimit
			}
			break
		}
	}
	if !found {
		parts = append(parts, "size="+sizeLimit)
	}
	return strings.Join(parts, ",")
}

// formatBytes formats a byte count as a human-readable string with the largest
// whole unit (e.g. 1073741824 -> "1024M", 536870912 -> "512M").
func formatBytes(b int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * mb
	)
	switch {
	case b >= gb && b%gb == 0:
		return fmt.Sprintf("%dG", b/gb)
	case b >= mb && b%mb == 0:
		return fmt.Sprintf("%dM", b/mb)
	default:
		return strconv.FormatInt(b, 10)
	}
}

// splitVolumeParts splits a volume string into its constituent parts.
//
// The parts are:
//
//  1. The path on the host machine
//  2. The path inside the container
//  3. The read/write mode
//
// It handles Windows and Linux style volume paths.
func splitVolumeParts(volumeParts string) ([]string, error) {
	// cspell:disable-next-line
	pattern := `^((?:[\w]\:)?[^\:]*)\:((?:[\w]\:)?[^\:]*)(?:\:([rwom]*))?`
	r, err := regexp.Compile(pattern)
	if err != nil {
		return []string{}, err
	}
	if r.MatchString(volumeParts) {
		results := r.FindStringSubmatch(volumeParts)[1:]
		var cleanResults []string
		for _, item := range results {
			if item != "" {
				cleanResults = append(cleanResults, item)
			}
		}
		return cleanResults, nil
	}
	return strings.Split(volumeParts, ":"), nil
}
