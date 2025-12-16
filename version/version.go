// Copyright 2019 Laszlo Fogas
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

// cSpell:ignore ldflags

package version

import (
	"os/exec"
	"strings"
)

// Version of Crow CI, set with ldflags, from Git tag.
var Version string

// BuildDate is the date when the binary was built, set with ldflags.
var BuildDate string

// commitHashLength defines the number of characters to use from git commit hash.
const commitHashLength = 10

// String returns the Version set at build time or "dev-<hash>" if in development.
func String() string {
	if Version == "" {
		// In development mode, try to get git commit hash
		if hash := getGitCommitHash(); hash != "" {
			return "dev-" + hash
		}
		return "dev"
	}

	return Version
}

// Date returns the BuildDate set at build time or the git commit date if in development.
func Date() string {
	if BuildDate == "" {
		return getGitCommitDate()
	}
	return BuildDate
}

// getGitCommitDate attempts to get the current git commit date for development builds.
func getGitCommitDate() string {
	cmd := exec.Command("git", "log", "-1", "--format=%cs")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getGitCommitHash attempts to get the current git commit hash for development builds.
func getGitCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	hash := strings.TrimSpace(string(output))
	if len(hash) >= commitHashLength {
		return hash[:commitHashLength] // Return first 10 characters like the Justfile does
	}

	return hash
}
