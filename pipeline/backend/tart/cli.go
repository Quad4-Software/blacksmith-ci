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
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// vmNamePrefix names every VM this backend creates. Stale-resource cleanup
// keys off it, so nothing outside this prefix is ever touched.
const vmNamePrefix = "crow-"

// vmNameUnsafe matches everything a tart VM name should not carry. Task UUIDs
// are ULIDs and never trip it, but a caller is not required to supply one.
var vmNameUnsafe = regexp.MustCompile(`[^a-z0-9._-]+`)

// vmName derives this workflow's VM name from its task UUID. The prefix is
// what stale-resource cleanup keys off.
func vmName(taskUUID string) string {
	name := vmNameUnsafe.ReplaceAllString(strings.ToLower(taskUUID), "-")
	return vmNamePrefix + strings.Trim(name, "-")
}

func cloneArgs(image, name string) []string {
	return []string{"clone", image, name}
}

// setArgs returns nil when there is nothing to set, so the caller can skip the
// invocation entirely and leave the image's own CPU and memory in place.
func setArgs(name string, cpu, memoryMB int) []string {
	if cpu <= 0 && memoryMB <= 0 {
		return nil
	}
	args := []string{"set", name}
	if cpu > 0 {
		args = append(args, "--cpu", strconv.Itoa(cpu))
	}
	if memoryMB > 0 {
		args = append(args, "--memory", strconv.Itoa(memoryMB))
	}
	return args
}

func runArgs(name string, network networkMode) []string {
	args := []string{"run", "--no-graphics"}
	switch network {
	case networkSoftnet:
		args = append(args, "--net-softnet")
	case networkHost:
		args = append(args, "--net-host")
	case networkShared:
		// tart's own default, no flag.
	}
	return append(args, name)
}

// execArgs runs the guest shell reading its script from stdin. -s makes the
// shell read commands from stdin, and -i attaches the host's stdin to it.
func execArgs(name, shell string) []string {
	return []string{"exec", "-i", name, shell, "-s"}
}

func stopArgs(name string, timeout time.Duration) []string {
	return []string{"stop", name, "--timeout", strconv.Itoa(int(timeout.Seconds()))}
}

func deleteArgs(name string) []string {
	return []string{"delete", name}
}

func listArgs() []string {
	return []string{"list", "--format", "json", "--source", "local"}
}

// vmEntry is one row of 'tart list --format json'.
type vmEntry struct {
	Source string
	Name   string
	State  string
}

func parseVMList(raw []byte) ([]vmEntry, error) {
	var entries []vmEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse tart list output: %w", err)
	}
	return entries, nil
}
