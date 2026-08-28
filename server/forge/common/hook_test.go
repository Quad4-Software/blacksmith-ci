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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCrowHookURL(t *testing.T) {
	tests := []struct {
		name    string
		hookURL string
		base    string
		want    bool
	}{
		{
			name:    "current hook path against webhook host",
			hookURL: "https://hooks.example.com/api/v1/hook?access_token=abc",
			base:    "https://hooks.example.com",
			want:    true,
		},
		{
			name:    "current hook path against full hook url",
			hookURL: "https://hooks.example.com/api/v1/hook?access_token=abc",
			base:    "https://hooks.example.com/api/v1/hook?access_token=xyz",
			want:    true,
		},
		{
			name:    "legacy api hook path",
			hookURL: "https://hooks.example.com/api/hook?access_token=abc",
			base:    "https://hooks.example.com",
			want:    true,
		},
		{
			name:    "legacy hook path",
			hookURL: "https://hooks.example.com/hook",
			base:    "https://hooks.example.com/",
			want:    true,
		},
		{
			name:    "unrelated app sharing the hostname",
			hookURL: "https://hooks.example.com/somethingelse",
			base:    "https://hooks.example.com",
			want:    false,
		},
		{
			name:    "unrelated app with a hook-like path",
			hookURL: "https://hooks.example.com/somethingelse/api/v1/hook",
			base:    "https://hooks.example.com",
			want:    false,
		},
		{
			name:    "different host",
			hookURL: "https://other.example.com/api/v1/hook",
			base:    "https://hooks.example.com",
			want:    false,
		},
		{
			name:    "crow served under a path prefix",
			hookURL: "https://hooks.example.com/crow/api/v1/hook?access_token=abc",
			base:    "https://hooks.example.com/crow",
			want:    true,
		},
		{
			name:    "crow served under a path prefix, full hook url as base",
			hookURL: "https://hooks.example.com/crow/api/v1/hook?access_token=abc",
			base:    "https://hooks.example.com/crow/api/v1/hook?access_token=xyz",
			want:    true,
		},
		{
			name:    "path prefix mismatch",
			hookURL: "https://hooks.example.com/api/v1/hook",
			base:    "https://hooks.example.com/crow",
			want:    false,
		},
		{
			name:    "trailing slash on hook url",
			hookURL: "https://hooks.example.com/api/v1/hook/",
			base:    "https://hooks.example.com",
			want:    true,
		},
		{
			name:    "invalid hook url",
			hookURL: "%gh&%ij",
			base:    "https://hooks.example.com",
			want:    false,
		},
		{
			name:    "invalid base url",
			hookURL: "https://hooks.example.com/api/v1/hook",
			base:    "%gh&%ij",
			want:    false,
		},
		{
			name:    "empty hook url",
			hookURL: "",
			base:    "https://hooks.example.com",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsCrowHookURL(tt.hookURL, tt.base))
		})
	}
}
