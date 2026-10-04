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

//go:build windows

package main

import (
	"codefloe.com/crowci/crow/v6/pipeline/backend/docker"
	"codefloe.com/crowci/crow/v6/pipeline/backend/hyperv"
	"codefloe.com/crowci/crow/v6/pipeline/backend/kubernetes"
	"codefloe.com/crowci/crow/v6/pipeline/backend/local"
	backendTypes "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

var backends = []backendTypes.Backend{
	kubernetes.New(),
	docker.New(),
	local.New(),
	// hyperv comes last on purpose, for the same reason tart does on Unix.
	// Auto-detect takes the first available backend, and local.IsAvailable is
	// true on any host that is not a container, so hyperv is never
	// auto-detected and has to be selected with CROW_BACKEND=hyperv. Enabling
	// the Hyper-V role on a Windows agent must not silently change how that
	// agent runs pipelines.
	hyperv.New(),
}
