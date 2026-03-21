// Copyright 2025 Crow Authors
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

package common

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// MinVolumeComponents is the minimum number of components for a valid volume path.
const MinVolumeComponents = 2

// SplitVolumeParts splits a volume string into its constituent parts.
//
// The parts are:
//  1. The path on the host machine
//  2. The path inside the container
//  3. The read/write mode (optional)
//
// It handles Windows and Linux style volume paths.
func SplitVolumeParts(volumeParts string) ([]string, error) {
	// cspell:disable-next-line
	pattern := `^((?:[\w]\:)?[^\:]*)\:((?:[\w]\:)?[^\:]*)(?:\:([rwom]*))?`
	r, err := regexp.Compile(pattern)
	if err != nil {
		return []string{}, err
	}
	if r.MatchString(volumeParts) {
		results := r.FindStringSubmatch(volumeParts)[1:]
		var cleanResults []string
		for _, item := range results {
			if item != "" {
				cleanResults = append(cleanResults, item)
			}
		}
		return cleanResults, nil
	}
	return strings.Split(volumeParts, ":"), nil
}

// ToVol converts a slice of volume paths to a set of unique volume names.
func ToVol(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	set := make(map[string]struct{})
	for _, path := range paths {
		parts, err := SplitVolumeParts(path)
		if err != nil {
			continue
		}
		if len(parts) < MinVolumeComponents {
			continue
		}
		set[parts[1]] = struct{}{}
	}
	return set
}

// ToEnv converts a key-value map of environment variables to a
// string slice in key=value format.
func ToEnv(env map[string]string) []string {
	var envs []string
	for k, v := range env {
		if k != "" {
			envs = append(envs, k+"="+v)
		}
	}
	return envs
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload.
func EncodeAuthToBase64(authConfig types.Auth) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}
