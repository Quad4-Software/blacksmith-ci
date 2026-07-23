// Copyright 2026 Crow Authors
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

package kubernetes

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

const tmpfsInitContainerName = "crow-init-tmpfs"

// tmpfsModeRegex matches a 3-4 digit octal permission mode (e.g. "1777", "755").
var tmpfsModeRegex = regexp.MustCompile(`^[0-7]{3,4}$`)

// parsedTmpfs is a single `tmpfs` entry resolved to its kubernetes representation.
type parsedTmpfs struct {
	name      string             // deterministic name shared by the volume, its mount and the chmod
	path      string             // mount path inside the container
	sizeLimit *resource.Quantity // emptyDir size limit; nil means unbounded
	mode      string             // octal mode from `mode=`, empty if unset
}

// tmpfsVolumeName returns the deterministic volume name for the i-th tmpfs
// entry so the generated volume, its mount and its chmod always agree.
func tmpfsVolumeName(i int) string {
	return fmt.Sprintf("tmpfs-%d", i)
}

// parseTmpfsMounts resolves the raw `tmpfs` entries into their kubernetes
// representation. Supported options are `size=` and `mode=`; anything else is
// rejected with a clear error rather than silently ignored (mirroring the
// backend's existing `temp_volumes` guard).
//
// When an entry omits `size=`, the limit defaults to the step's memory limit
// (memLimit) if one is set, mirroring the docker backend which caps tmpfs size
// to the agent's memory limit. A memory-backed emptyDir is charged to the pod's
// memory budget, so bounding it avoids a single mount defaulting to 50% of node
// memory.
func parseTmpfsMounts(tmpfs []string, memLimit *resource.Quantity) ([]parsedTmpfs, error) {
	if len(tmpfs) == 0 {
		return nil, nil
	}

	parsed := make([]parsedTmpfs, 0, len(tmpfs))
	for i, entry := range tmpfs {
		path, opts, err := parseTmpfsEntry(entry)
		if err != nil {
			return nil, err
		}
		size, mode, err := tmpfsOptions(opts)
		if err != nil {
			return nil, fmt.Errorf("invalid tmpfs entry %q: %w", entry, err)
		}
		if size == nil {
			if memLimit != nil {
				size = memLimit
			} else {
				log.Warn().Msgf("tmpfs mount %q has no `size=` limit and the step has no memory limit; on kubernetes it defaults to 50%% of node memory and is charged to the pod's memory limit", path)
			}
		}
		parsed = append(parsed, parsedTmpfs{
			name:      tmpfsVolumeName(i),
			path:      path,
			sizeLimit: size,
			mode:      mode,
		})
	}
	return parsed, nil
}

// tmpfsVolumes maps the parsed entries to memory-backed emptyDir volumes, the
// closest kubernetes equivalent of a docker/podman tmpfs mount.
func tmpfsVolumes(parsed []parsedTmpfs) []v1.Volume {
	if len(parsed) == 0 {
		return nil
	}
	volumes := make([]v1.Volume, 0, len(parsed))
	for _, m := range parsed {
		emptyDir := &v1.EmptyDirVolumeSource{Medium: v1.StorageMediumMemory}
		if m.sizeLimit != nil {
			emptyDir.SizeLimit = m.sizeLimit
		}
		volumes = append(volumes, v1.Volume{
			Name:         m.name,
			VolumeSource: v1.VolumeSource{EmptyDir: emptyDir},
		})
	}
	return volumes
}

// tmpfsVolumeMounts returns the container volume mounts matching tmpfsVolumes.
func tmpfsVolumeMounts(parsed []parsedTmpfs) []v1.VolumeMount {
	if len(parsed) == 0 {
		return nil
	}
	mounts := make([]v1.VolumeMount, 0, len(parsed))
	for _, m := range parsed {
		mounts = append(mounts, v1.VolumeMount{Name: m.name, MountPath: m.path})
	}
	return mounts
}

// tmpfsInitContainer builds an init container that applies each requested
// `mode`, since a memory-backed emptyDir cannot set a mount mode itself. It
// returns (nil, nil) when no entry requests a mode.
//
// The init container reuses the step image (already pulled for the pod) and
// runs as root so it can chmod mounts owned by root. That is only possible when
// running as root is permitted (rootAllowed): when the agent or step enforces
// runAsNonRoot, honoring a `mode=` is rejected with a clear error rather than
// emitting a pod the PodSecurity admission would deny. It also requires the
// image to provide a shell and `chmod`.
func tmpfsInitContainer(parsed []parsedTmpfs, image string, rootAllowed bool) (*v1.Container, error) {
	var cmds []string
	var mounts []v1.VolumeMount
	for _, m := range parsed {
		if m.mode == "" {
			continue
		}
		// m.mode is validated against tmpfsModeRegex; m.path is single-quoted.
		cmds = append(cmds, fmt.Sprintf("chmod %s %s", m.mode, shellSingleQuote(m.path)))
		mounts = append(mounts, v1.VolumeMount{Name: m.name, MountPath: m.path})
	}
	if len(cmds) == 0 {
		return nil, nil
	}
	if !rootAllowed {
		return nil, fmt.Errorf("the kubernetes backend can only honor a tmpfs `mode=` option through a root init container, which is disallowed while runAsNonRoot is enforced; make the step privileged or drop the `mode=` option")
	}
	return &v1.Container{
		Name:         tmpfsInitContainerName,
		Image:        image,
		Command:      []string{"/bin/sh", "-c", strings.Join(cmds, " && ")},
		VolumeMounts: mounts,
		SecurityContext: &v1.SecurityContext{
			RunAsUser: newInt64(0),
		},
	}, nil
}

// tmpfsModeAllowed reports whether a root init container (needed to honor a
// tmpfs `mode=`) may run for this step. Privileged steps already run as root;
// otherwise a runAsNonRoot policy from the agent config or the step's backend
// options forbids it.
func tmpfsModeAllowed(step *types.Step, config *config, options BackendOptions) bool {
	if step.Privileged {
		return true
	}
	if config.SecurityContext.RunAsNonRoot {
		return false
	}
	if options.SecurityContext != nil && options.SecurityContext.RunAsNonRoot != nil && *options.SecurityContext.RunAsNonRoot {
		return false
	}
	return true
}

// parseTmpfsEntry splits a `tmpfs` entry into its mount path and its raw,
// comma-separated option string (e.g. "size=64m,mode=1777"). Entries without a
// colon are bare paths with no options.
func parseTmpfsEntry(entry string) (path, opts string, err error) {
	if !strings.Contains(entry, ":") {
		return entry, "", nil
	}
	parts, err := common.SplitVolumeParts(entry)
	if err != nil {
		return "", "", err
	}
	if len(parts) == 0 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid tmpfs entry %q", entry)
	}
	if len(parts) > 1 {
		opts = parts[1]
	}
	return parts[0], opts, nil
}

// tmpfsOptions parses the supported tmpfs options into an emptyDir size limit
// and an octal mode. Options a memory-backed emptyDir cannot honor return an
// error instead of being silently dropped.
func tmpfsOptions(opts string) (size *resource.Quantity, mode string, err error) {
	if opts == "" {
		return nil, "", nil
	}

	for _, opt := range strings.Split(opts, ",") {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		key, val, found := strings.Cut(opt, "=")
		if !found {
			return nil, "", fmt.Errorf("unsupported tmpfs option %q", opt)
		}
		switch key {
		case "size":
			// Docker/Podman accept lowercase units (e.g. "size=64m"), but
			// common.ParseMemory only matches uppercase units, so normalize.
			bytes, err := common.ParseMemory(strings.ToUpper(val))
			if err != nil {
				return nil, "", err
			}
			size = resource.NewQuantity(bytes, resource.BinarySI)
		case "mode":
			if !tmpfsModeRegex.MatchString(val) {
				return nil, "", fmt.Errorf("invalid tmpfs mode %q: expected 3-4 octal digits", val)
			}
			mode = val
		default:
			return nil, "", fmt.Errorf("the kubernetes backend does not support the tmpfs option %q", opt)
		}
	}
	return size, mode, nil
}

// shellSingleQuote wraps s in single quotes for safe use in a /bin/sh command,
// escaping any embedded single quotes.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
