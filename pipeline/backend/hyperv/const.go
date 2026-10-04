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

package hyperv

import "errors"

// EngineName is the backend's name and the key its per-step options live under
// in a step's BackendOptions map.
const EngineName = "hyperv"

var (
	ErrUnsupportedStepType    = errors.New("unsupported step type")
	ErrWorkflowStateNotFound  = errors.New("workflow state not found")
	ErrBaseDiskNotConfigured  = errors.New("no base VHDX configured, set CROW_BACKEND_HYPERV_BASE_VHDX")
	ErrGuestUserNotConfigured = errors.New("no guest credentials configured, set CROW_BACKEND_HYPERV_GUEST_USERNAME and CROW_BACKEND_HYPERV_GUEST_PASSWORD")
	ErrVMLimitReached         = errors.New("the configured Hyper-V VM limit is already in use")
)
