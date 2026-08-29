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

package tart

import (
	"fmt"

	"github.com/go-viper/mapstructure/v2"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// BackendOptions defines the per-workflow options for the tart backend.
//
// There is one VM per workflow, so these are workflow-level values even though
// they are written on a step. The first step that sets one decides.
type BackendOptions struct {
	// Image is the OCI reference of the VM image. Honored only when the
	// agent sets CROW_BACKEND_TART_IMAGE_ALLOW_FROM_STEP.
	Image string `mapstructure:"image"`
	// CPU is the number of vCPUs, clamped to the agent's own setting.
	CPU int `mapstructure:"cpu"`
	// Memory is a size string such as "8Gi", clamped to the agent's setting.
	Memory string `mapstructure:"memory"`
}

func parseBackendOptions(step *types.Step) (BackendOptions, error) {
	var result BackendOptions
	if step == nil || step.BackendOptions == nil {
		return result, nil
	}
	err := mapstructure.WeakDecode(step.BackendOptions[EngineName], &result)
	return result, err
}

// vmSpec is what a workflow's VM is created with.
type vmSpec struct {
	Image    string
	CPU      int
	MemoryMB int
}

// resolveVMOptions decides the image, CPU and memory of this workflow's VM.
//
// Agent configuration supplies the defaults. A step may ask for less CPU or
// memory but never more, so the agent's values act as ceilings rather than
// suggestions. A step may only choose the image when the agent allows it.
func resolveVMOptions(cfg config, conf *types.Config) (vmSpec, error) {
	spec := vmSpec{Image: cfg.Image, CPU: cfg.CPU, MemoryMB: cfg.MemoryMB}
	if conf == nil {
		return spec, nil
	}

	var haveImage, haveCPU, haveMemory bool
	for _, stage := range conf.Stages {
		for _, step := range stage.Steps {
			opts, parseErr := parseBackendOptions(step)
			if parseErr != nil {
				return vmSpec{}, fmt.Errorf("step %q: parse tart backend options: %w", step.Name, parseErr)
			}

			if opts.Image != "" && !haveImage && cfg.AllowImageFromStep {
				spec.Image = opts.Image
				haveImage = true
			}

			if opts.CPU > 0 && !haveCPU {
				spec.CPU = clamp(opts.CPU, cfg.CPU)
				haveCPU = true
			}

			if opts.Memory != "" && !haveMemory {
				memBytes, memErr := common.ParseMemory(opts.Memory)
				if memErr != nil {
					return vmSpec{}, fmt.Errorf("step %q: tart memory: %w", step.Name, memErr)
				}
				spec.MemoryMB = clamp(int(memBytes/bytesPerMebibyte), cfg.MemoryMB)
				haveMemory = true
			}
		}
	}

	return spec, nil
}

// clamp limits a requested value to a ceiling, treating a zero ceiling as
// "the agent set none, so accept what was asked".
func clamp(requested, ceiling int) int {
	if ceiling > 0 && requested > ceiling {
		return ceiling
	}
	return requested
}
