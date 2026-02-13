// Copyright 2018 Drone.IO Inc.
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

package main

import (
	"context"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/cmd/agent/core"
	"codefloe.com/crowci/crow/v5/pipeline"
	"codefloe.com/crowci/crow/v5/pipeline/backend/docker"
	"codefloe.com/crowci/crow/v5/pipeline/backend/kubernetes"
	"codefloe.com/crowci/crow/v5/pipeline/backend/local"
	backendTypes "codefloe.com/crowci/crow/v5/pipeline/backend/types"
	"codefloe.com/crowci/crow/v5/shared/utils"
)

var backends = []backendTypes.Backend{
	kubernetes.New(),
	docker.New(),
	local.New(),
}

func main() {
	ctx := utils.WithContextSigtermCallback(context.Background(), func() {
		// Set preserve flag BEFORE context cancellation propagates to pipeline goroutines.
		// This ensures workflows are not destroyed when agent receives SIGTERM/SIGINT.
		pipeline.SetPreserveWorkflows(true)
		log.Info().Msg("termination signal received, workflows will be preserved for adoption")
	})
	core.RunAgent(ctx, backends)
}
