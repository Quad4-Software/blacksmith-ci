// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package local

import (
	"os"
	"strconv"
	"strings"
)

// procMeminfo is a variable so tests can point it at a fixture.
var procMeminfo = "/proc/meminfo"

// bytesPerKiB converts the kB unit used by /proc/meminfo into bytes.
const bytesPerKiB = 1024

// hostMemTotal returns the total physical memory of the host in bytes.
// It returns 0 when the value cannot be determined.
func hostMemTotal() int64 {
	data, err := os.ReadFile(procMeminfo)
	if err != nil {
		return 0
	}
	return parseMemTotal(string(data))
}

// parseMemTotal extracts the `MemTotal` value from /proc/meminfo contents and
// returns it in bytes. It returns 0 when the entry is missing or malformed.
func parseMemTotal(meminfo string) int64 {
	for line := range strings.Lines(meminfo) {
		key, value, found := strings.Cut(line, ":")
		if !found || key != "MemTotal" {
			continue
		}

		fields := strings.Fields(value)
		if len(fields) == 0 {
			return 0
		}

		amount, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0
		}

		// The kernel always reports kB, but only trust an explicit unit.
		if len(fields) < 2 || !strings.EqualFold(fields[1], "kB") {
			return 0
		}

		return amount * bytesPerKiB
	}

	return 0
}
