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

package podman

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common"
)

type config struct {
	enableIPv6    bool
	network       string
	volumes       []string
	resourceLimit resourceLimit
}

type resourceLimit struct {
	MemSwapLimit int64
	MemLimit     int64
	ShmSize      int64
	CPUQuota     int64
	CPUShares    int64
	CPUSet       string
}

func configFromCli(c *cli.Command) (config, error) {
	// Parse memory and CPU limits with unit support
	memSwap, err := common.ParseMemory(c.String("backend-podman-limit-mem-swap"))
	if err != nil {
		return config{}, fmt.Errorf("invalid memory swap limit: %w", err)
	}

	memLimit, err := common.ParseMemory(c.String("backend-podman-limit-mem"))
	if err != nil {
		return config{}, fmt.Errorf("invalid memory limit: %w", err)
	}

	shmSize, err := common.ParseMemory(c.String("backend-podman-limit-shm-size"))
	if err != nil {
		return config{}, fmt.Errorf("invalid shm size: %w", err)
	}

	cpuQuota, err := common.ParseCPUQuota(c.String("backend-podman-limit-cpu-quota"))
	if err != nil {
		return config{}, fmt.Errorf("invalid CPU quota: %w", err)
	}

	conf := config{
		enableIPv6: c.Bool("backend-podman-ipv6"),
		network:    c.String("backend-podman-network"),
		resourceLimit: resourceLimit{
			MemSwapLimit: memSwap,
			MemLimit:     memLimit,
			ShmSize:      shmSize,
			CPUQuota:     cpuQuota,
			CPUShares:    c.Int64("backend-podman-limit-cpu-shares"),
			CPUSet:       c.String("backend-podman-limit-cpu-set"),
		},
	}

	volumes := strings.Split(c.String("backend-podman-volumes"), ",")
	conf.volumes = make([]string, 0, len(volumes))
	// Validate provided volume definitions
	for _, v := range volumes {
		if v == "" {
			continue
		}
		parts, err := common.SplitVolumeParts(v)
		if err != nil {
			log.Error().Err(err).Msgf("can not parse volume config")
			return conf, fmt.Errorf("invalid volume '%s' provided in CROW_BACKEND_PODMAN_VOLUMES: %w", v, err)
		}
		conf.volumes = append(conf.volumes, strings.Join(parts, ":"))
	}

	return conf, nil
}
