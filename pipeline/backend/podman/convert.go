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

//go:build !windows

package podman

import (
	"maps"
	"net"
	"strings"

	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	nettypes "go.podman.io/common/libnetwork/types"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common"
	backendTypes "codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// volumeOptionsIndex is the index where volume options start (source:dest:options).
const volumeOptionsIndex = 2

// toSpecGenerator converts a Step to a specgen.SpecGenerator for Podman.
func (e *podman) toSpecGenerator(step *backendTypes.Step, options BackendOptions, taskUUID string) *specgen.SpecGenerator {
	s := specgen.NewSpecGenerator(step.Image, false)

	// Basic configuration
	s.Labels = common.MakeLabels(step, taskUUID)
	s.WorkDir = step.WorkingDir
	s.User = options.User

	// Environment
	configEnv := make(map[string]string)
	maps.Copy(configEnv, step.Environment)

	if len(step.Commands) > 0 {
		env, entry := common.GenerateContainerConf(step.Commands, e.osType, step.WorkingDir)
		for k, v := range env {
			configEnv[k] = v
		}
		s.Entrypoint = entry
		// step.WorkingDir will be respected by the generated script
		s.WorkDir = step.WorkspaceBase
	}
	if len(step.Entrypoint) > 0 {
		s.Entrypoint = step.Entrypoint
	}

	s.Env = configEnv

	// Privileged mode
	s.Privileged = &step.Privileged

	// Network mode
	if len(step.NetworkMode) != 0 {
		s.NetNS = specgen.Namespace{
			NSMode: specgen.NamespaceMode(step.NetworkMode),
		}
	}

	// DNS configuration
	if len(step.DNS) != 0 {
		s.DNSServers = parseIPAddresses(step.DNS)
	}
	if len(step.DNSSearch) != 0 {
		s.DNSSearch = step.DNSSearch
	}

	// Extra hosts
	if len(step.ExtraHosts) != 0 {
		s.HostAdd = make([]string, 0, len(step.ExtraHosts))
		for _, hostAlias := range step.ExtraHosts {
			s.HostAdd = append(s.HostAdd, hostAlias.Name+":"+hostAlias.IP)
		}
	}

	// Devices
	if len(step.Devices) != 0 {
		s.Devices = toLinuxDevices(step.Devices)
	}

	// Volumes
	if len(step.Volumes) != 0 {
		s.Volumes = toNamedVolumes(step.Volumes)
	}

	// Tmpfs mounts
	if len(step.Tmpfs) != 0 {
		s.Mounts = toTmpfsMounts(step.Tmpfs)
	}

	// Resource limits
	s.ResourceLimits = toResourceLimits(&e.config)

	return s
}

// newVolumeCreateOptions creates volume options for Podman.
func newVolumeCreateOptions(name string) types.VolumeCreateOptions {
	return types.VolumeCreateOptions{
		Name:   name,
		Driver: "local",
	}
}

// newNetworkCreateOptions creates network options for Podman.
func newNetworkCreateOptions(name string, enableIPv6 bool) *nettypes.Network {
	return &nettypes.Network{
		Name:        name,
		Driver:      networkDriverBridge,
		IPv6Enabled: enableIPv6,
	}
}

// toNamedVolumes converts a slice of volume strings to Podman NamedVolume.
func toNamedVolumes(volumes []string) []*specgen.NamedVolume {
	if len(volumes) == 0 {
		return nil
	}

	result := make([]*specgen.NamedVolume, 0, len(volumes))
	for _, vol := range volumes {
		parts, err := common.SplitVolumeParts(vol)
		if err != nil || len(parts) < common.MinVolumeComponents {
			continue
		}

		nv := &specgen.NamedVolume{
			Name: parts[0],
			Dest: parts[1],
		}

		// Handle options (ro, rw, etc.)
		if len(parts) > volumeOptionsIndex {
			nv.Options = strings.Split(parts[volumeOptionsIndex], ",")
		}

		result = append(result, nv)
	}
	return result
}

// toLinuxDevices converts a slice of device paths to spec.LinuxDevice.
func toLinuxDevices(paths []string) []spec.LinuxDevice {
	var devices []spec.LinuxDevice

	for _, path := range paths {
		parts, err := common.SplitVolumeParts(path)
		if err != nil || len(parts) < common.MinVolumeComponents {
			continue
		}

		containerPath := parts[1]
		if strings.HasSuffix(containerPath, ":ro") || strings.HasSuffix(containerPath, ":rw") {
			containerPath = containerPath[:len(containerPath)-3]
		}

		devices = append(devices, spec.LinuxDevice{
			Path: containerPath,
		})
	}
	return devices
}

// toTmpfsMounts converts a slice of tmpfs paths to spec.Mount.
func toTmpfsMounts(tmpfs []string) []spec.Mount {
	if len(tmpfs) == 0 {
		return nil
	}

	mounts := make([]spec.Mount, 0, len(tmpfs))
	for _, path := range tmpfs {
		mount := spec.Mount{
			Type:   "tmpfs",
			Source: "tmpfs",
		}

		if !strings.Contains(path, ":") {
			mount.Destination = path
		} else {
			parts, err := common.SplitVolumeParts(path)
			if err != nil {
				continue
			}
			mount.Destination = parts[0]
			if len(parts) > 1 {
				mount.Options = strings.Split(parts[1], ",")
			}
		}

		mounts = append(mounts, mount)
	}
	return mounts
}

// toResourceLimits converts config resource limits to spec.LinuxResources.
func toResourceLimits(conf *config) *spec.LinuxResources {
	resources := &spec.LinuxResources{}
	hasLimits := false

	// Memory limits
	if conf.resourceLimit.MemLimit > 0 || conf.resourceLimit.MemSwapLimit > 0 {
		resources.Memory = &spec.LinuxMemory{}
		hasLimits = true
		if conf.resourceLimit.MemLimit > 0 {
			resources.Memory.Limit = &conf.resourceLimit.MemLimit
		}
		if conf.resourceLimit.MemSwapLimit > 0 {
			resources.Memory.Swap = &conf.resourceLimit.MemSwapLimit
		}
	}

	// CPU limits
	if conf.resourceLimit.CPUQuota > 0 || conf.resourceLimit.CPUShares > 0 || conf.resourceLimit.CPUSet != "" {
		resources.CPU = &spec.LinuxCPU{}
		hasLimits = true
		if conf.resourceLimit.CPUQuota > 0 {
			resources.CPU.Quota = &conf.resourceLimit.CPUQuota
		}
		if conf.resourceLimit.CPUShares > 0 {
			shares := uint64(conf.resourceLimit.CPUShares)
			resources.CPU.Shares = &shares
		}
		if conf.resourceLimit.CPUSet != "" {
			resources.CPU.Cpus = conf.resourceLimit.CPUSet
		}
	}

	if !hasLimits {
		return nil
	}
	return resources
}

// parseIPAddresses converts a slice of IP strings to net.IP.
func parseIPAddresses(ips []string) []net.IP {
	result := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if parsed := net.ParseIP(ip); parsed != nil {
			result = append(result, parsed)
		}
	}
	return result
}
