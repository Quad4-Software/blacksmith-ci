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

//go:build !windows

package main

import (
	"codefloe.com/crowci/crow/v6/pipeline/backend/docker"
	"codefloe.com/crowci/crow/v6/pipeline/backend/kubernetes"
	"codefloe.com/crowci/crow/v6/pipeline/backend/local"
	"codefloe.com/crowci/crow/v6/pipeline/backend/podman"
	"codefloe.com/crowci/crow/v6/pipeline/backend/tart"
	backendTypes "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

var backends = []backendTypes.Backend{
	kubernetes.New(),
	docker.New(),
	podman.New(),
	local.New(),
	// tart comes last on purpose. Auto-detect takes the first available
	// backend, and local.IsAvailable is true on any host that is not a
	// container, so tart is never auto-detected and has to be selected with
	// CROW_BACKEND=tart. Installing tart on a Mac that already runs a local
	// backend agent must not silently change how that agent runs pipelines.
	tart.New(),
}
