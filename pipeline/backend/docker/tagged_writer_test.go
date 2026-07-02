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

package docker

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaggedWriter(t *testing.T) {
	tests := []struct {
		name   string
		writes []string
		want   []byte
	}{
		{
			name:   "single terminated line",
			writes: []string{"hello\n"},
			want:   []byte{0x01, 'h', 'e', 'l', 'l', 'o', '\n'},
		},
		{
			name:   "unterminated line is flushed on close",
			writes: []string{"no newline"},
			want:   []byte{0x01, 'n', 'o', ' ', 'n', 'e', 'w', 'l', 'i', 'n', 'e'},
		},
		{
			name:   "trailing partial after terminated line",
			writes: []string{"first\nsecond"},
			want: []byte{
				0x01, 'f', 'i', 'r', 's', 't', '\n',
				0x01, 's', 'e', 'c', 'o', 'n', 'd',
			},
		},
		{
			name:   "partial across multiple writes",
			writes: []string{"par", "tial"},
			want:   []byte{0x01, 'p', 'a', 'r', 't', 'i', 'a', 'l'},
		},
		{
			name:   "no trailing data",
			writes: []string{"line\n"},
			want:   []byte{0x01, 'l', 'i', 'n', 'e', '\n'},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tw := &taggedWriter{w: &buf, tag: 0x01}
			for _, w := range tc.writes {
				n, err := tw.Write([]byte(w))
				assert.NoError(t, err)
				assert.Equal(t, len(w), n)
			}
			assert.NoError(t, tw.Flush())
			assert.Equal(t, tc.want, buf.Bytes())

			// Flush must be idempotent and not re-emit buffered data
			assert.NoError(t, tw.Flush())
			assert.Equal(t, tc.want, buf.Bytes())
		})
	}
}
