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

package common

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var dns1123Label = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func TestSanitizeAndTruncateName(t *testing.T) {
	longPrefix := strings.Repeat("owner", 8) + "-" + strings.Repeat("repo", 6) + "-123-1-"

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short name unchanged", input: "owner-repo-1-1-build", maxLen: 64, want: "owner-repo-1-1-build"},
		{name: "sanitized", input: "Owner/Repo_1.1 Build", maxLen: 64, want: "owner-repo-1-1-build"},
		{name: "exactly max unchanged", input: strings.Repeat("a", 63), maxLen: 63, want: strings.Repeat("a", 63)},
		{name: "surrounding dashes trimmed before length check", input: "-" + strings.Repeat("a", 63) + "-", maxLen: 63, want: strings.Repeat("a", 63)},
		{name: "one over max is hashed", input: strings.Repeat("a", 64), maxLen: 63, want: strings.Repeat("a", 54) + "-ffe054fe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SanitizeAndTruncateName(tt.input, tt.maxLen))
		})
	}

	for _, maxLen := range []int{63, 64} {
		a := SanitizeAndTruncateName(longPrefix+"build-frontend-assets", maxLen)
		b := SanitizeAndTruncateName(longPrefix+"build-frontend-tests", maxLen)
		assert.NotEqual(t, a, b, "names sharing a long prefix must not collide")
		for _, n := range []string{a, b} {
			assert.LessOrEqual(t, len(n), maxLen)
			assert.Regexp(t, dns1123Label, n)
		}
		// Deterministic, so create/wait/tail/destroy all resolve the same name.
		assert.Equal(t, a, SanitizeAndTruncateName(longPrefix+"build-frontend-assets", maxLen))
	}

	// A cut that lands on a '-' must not leave a doubled or trailing dash.
	cutOnDash := strings.Repeat("a", 53) + "-" + strings.Repeat("b", 20)
	got := SanitizeAndTruncateName(cutOnDash, 63)
	assert.Regexp(t, dns1123Label, got)
	assert.NotContains(t, got, "--")
	assert.LessOrEqual(t, len(got), 63)
}
