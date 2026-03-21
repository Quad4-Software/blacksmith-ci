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
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	kilobyte            = 1000
	kibibyte            = 1024
	microsecondsPerCore = 100000
	millicoresPerCore   = 1000
)

var (
	// MemoryRegex matches memory values like "2Gi", "512Mi", "1G", "100M", "1024".
	memoryRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMGT]i?[bB]?)?$`)
	// CpuRegex matches CPU values like "2", "1000m", "0.5".
	cpuRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([m])?$`)
)

// ParseMemory parses a memory string with optional units and returns bytes.
// Supports: "2Gi", "512Mi", "1G", "100M", "1024" (plain numbers are bytes)
// Also accepts quoted strings like "'6Gi'" or '"512Mi"' from config files
// Returns 0 for empty string or "0".
func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(s)

	// Strip surrounding quotes (single or double) that might come from config files
	const minQuotedLen = 2 // minimum length for a quoted string (e.g., "x")
	if len(s) >= minQuotedLen {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			s = s[1 : len(s)-1]
			s = strings.TrimSpace(s)
		}
	}

	if s == "" || s == "0" {
		return 0, nil
	}

	matches := memoryRegex.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid memory format: %s (expected formats: 2Gi, 512Mi, 1G, 100M, or plain bytes)", s)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s", matches[1])
	}

	unit := strings.ToUpper(matches[2])
	var multiplier int64

	switch unit {
	case "", "B":
		multiplier = 1
	case "K", "KB":
		multiplier = kilobyte
	case "KI", "KIB":
		multiplier = kibibyte
	case "M", "MB":
		multiplier = kilobyte * kilobyte
	case "MI", "MIB":
		multiplier = kibibyte * kibibyte
	case "G", "GB":
		multiplier = kilobyte * kilobyte * kilobyte
	case "GI", "GIB":
		multiplier = kibibyte * kibibyte * kibibyte
	case "T", "TB":
		multiplier = kilobyte * kilobyte * kilobyte * kilobyte
	case "TI", "TIB":
		multiplier = kibibyte * kibibyte * kibibyte * kibibyte
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unit)
	}

	return int64(value * float64(multiplier)), nil
}

// ParseCPUQuota parses a CPU string and returns the quota in microseconds.
// Docker CPU quota is specified in microseconds per 100ms period.
// 100000 = 1 CPU core, 200000 = 2 cores, 50000 = 0.5 cores
// Supports: "2" (2 cores), "1000m" (1 core in millicores), "0.5" (0.5 cores)
// Also accepts quoted strings like "'3'" or '"3"' from config files
// Returns 0 for empty string or "0".
func ParseCPUQuota(s string) (int64, error) {
	s = strings.TrimSpace(s)

	// Strip surrounding quotes (single or double) that might come from config files
	const minQuotedLen = 2 // minimum length for a quoted string (e.g., "x")
	if len(s) >= minQuotedLen {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			s = s[1 : len(s)-1]
			s = strings.TrimSpace(s)
		}
	}

	if s == "" || s == "0" {
		return 0, nil
	}

	matches := cpuRegex.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid CPU format: %s (expected formats: 2, 1000m, or 0.5)", s)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value: %s", matches[1])
	}

	// Check if it's millicores (e.g., "1000m")
	if matches[2] == "m" {
		// Convert millicores to cores (1000m = 1 core)
		value /= millicoresPerCore
	}

	// Convert cores to Docker CPU quota (microseconds per 100ms period)
	// 1 core = 100000 microseconds
	quota := int64(value * microsecondsPerCore)

	return quota, nil
}
