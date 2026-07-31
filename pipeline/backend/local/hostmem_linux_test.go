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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMemTotal(t *testing.T) {
	tests := []struct {
		name    string
		meminfo string
		want    int64
	}{
		{
			name:    "reads MemTotal in kB",
			meminfo: "MemTotal:       65805744 kB\nMemFree:         1234 kB\n",
			want:    65805744 * 1024,
		},
		{
			name:    "ignores lines before MemTotal",
			meminfo: "Committed_AS:   123 kB\nMemTotal:  1024 kB\n",
			want:    1024 * 1024,
		},
		{
			name:    "does not match MemTotal prefixed keys",
			meminfo: "MemTotalHuge:   999 kB\nMemTotal:       2048 kB\n",
			want:    2048 * 1024,
		},
		{
			name:    "missing entry",
			meminfo: "MemFree:         1234 kB\n",
			want:    0,
		},
		{
			name:    "non numeric value",
			meminfo: "MemTotal:       lots kB\n",
			want:    0,
		},
		{
			name:    "missing value",
			meminfo: "MemTotal:\n",
			want:    0,
		},
		{
			name:    "unexpected unit",
			meminfo: "MemTotal:       64 GB\n",
			want:    0,
		},
		{
			name:    "empty",
			meminfo: "",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseMemTotal(tt.meminfo))
		})
	}
}

func TestHostMemTotal(t *testing.T) {
	orig := procMeminfo
	t.Cleanup(func() { procMeminfo = orig })

	path := filepath.Join(t.TempDir(), "meminfo")
	require := assert.New(t)
	require.NoError(os.WriteFile(path, []byte("MemTotal:       4096 kB\n"), 0o600))

	procMeminfo = path
	assert.Equal(t, int64(4096*1024), hostMemTotal())

	procMeminfo = filepath.Join(t.TempDir(), "does-not-exist")
	assert.Equal(t, int64(0), hostMemTotal())
}
