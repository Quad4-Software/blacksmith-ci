// Copyright 2023 Woodpecker Authors
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

package local

import "errors"

// workflowDirPrefix names every workflow's base directory inside the shared
// temp dir. The macOS sandbox profile denies reading it, so the two must agree.
const workflowDirPrefix = "crow-local-"

// notAllowedEnvVarOverwrites are all env vars that cannot be overwritten by step config.
var notAllowedEnvVarOverwrites = []string{
	"CI_NETRC_MACHINE",
	"CI_NETRC_USERNAME",
	"CI_NETRC_PASSWORD",
	"CI_SCRIPT",
	"HOME",
	// USERPROFILE is what Windows tools read instead of HOME; letting a step
	// override it would escape the per-workflow home.
	"USERPROFILE",
	"SHELL",
	"CI_WORKSPACE",
	// The backend points TMPDIR at a directory inside the workflow's own base
	// dir and the macOS sandbox profile is built around that. A step that moved
	// it elsewhere would write outside what the profile allows and fail in a
	// way that looks like a broken tool rather than a denied path.
	"TMPDIR",
	// TMP and TEMP are the Windows equivalents of TMPDIR.
	"TMP",
	"TEMP",
}

var (
	ErrUnsupportedStepType   = errors.New("unsupported step type")
	ErrWorkflowStateNotFound = errors.New("workflow state not found")
)
