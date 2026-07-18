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

package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironCIWorkspace(t *testing.T) {
	testCases := []struct {
		name     string
		forgeURL string
		want     string
	}{
		{
			name:     "https forge url",
			forgeURL: "https://git.example.com/john-doe/my-repo",
			want:     "/crow/src/git.example.com/john-doe/my-repo",
		},
		{
			name:     "empty forge url falls back to src",
			forgeURL: "",
			want:     "/crow/src",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Metadata{Repo: Repo{ForgeURL: tc.forgeURL}}
			env := m.Environ()
			assert.Equal(t, tc.want, env["CI_WORKSPACE"])
		})
	}
}
