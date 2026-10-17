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

package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeRedirectPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "root", in: "/", want: true},
		{name: "local path", in: "/repos/1/pipeline/2", want: true},
		{name: "local path with query and fragment", in: "/user?tab=forges#top", want: true},
		{name: "encoded query value", in: "/repos?next=%2F%2Fevil.com", want: true},
		{name: "empty", in: "", want: false},
		{name: "relative path", in: "repos/1", want: false},
		{name: "protocol relative", in: "//evil.com", want: false},
		{name: "protocol relative with path", in: "//evil.com/login", want: false},
		{name: "backslash", in: "/\\evil.com", want: false},
		{name: "backslash later in path", in: "/repos\\..\\x", want: false},
		{name: "encoded slashes", in: "/%2F%2Fevil.com", want: false},
		{name: "encoded slash after slash", in: "/%2Fevil.com", want: false},
		{name: "encoded backslash", in: "/%5Cevil.com", want: false},
		{name: "absolute https", in: "https://evil.com", want: false},
		{name: "absolute http", in: "http://evil.com/", want: false},
		{name: "javascript scheme", in: "javascript:alert(1)", want: false},
		{name: "data scheme", in: "data:text/html,x", want: false},
		{name: "tab", in: "/\t/evil.com", want: false},
		{name: "newline", in: "/repos\n/x", want: false},
		{name: "carriage return header injection", in: "/\r\nLocation: https://evil.com", want: false},
		{name: "encoded newline", in: "/%0a/evil.com", want: false},
		{name: "delete char", in: "/\x7f", want: false},
		{name: "invalid escape", in: "/%zz", want: false},
		{name: "too long", in: "/" + strings.Repeat("a", maxRedirectPathLen), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, safeRedirectPath(tt.in), tt.in)
		})
	}
}

func TestRedirectPathOr(t *testing.T) {
	assert.Equal(t, "/repos/1", redirectPathOr("/repos/1", "/user"))
	assert.Equal(t, "/user", redirectPathOr("//evil.com", "/user"))
	assert.Equal(t, "/user", redirectPathOr("", "/user"))
}
