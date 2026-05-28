// Copyright 2021 Woodpecker Authors
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

package proclog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingPushPopBelowCap(t *testing.T) {
	r := NewRing(4)
	r.Push(SinkLine{Line: "a"})
	r.Push(SinkLine{Line: "b"})

	out := r.DrainAll()
	require.Len(t, out, 2)
	assert.Equal(t, "a", out[0].Line)
	assert.Equal(t, "b", out[1].Line)
	assert.Equal(t, 0, r.Len())
}

func TestRingOverflowDropsOldest(t *testing.T) {
	r := NewRing(3)
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		r.Push(SinkLine{Line: s})
	}
	out := r.DrainAll()
	require.Len(t, out, 3)
	assert.Equal(t, []string{"c", "d", "e"}, []string{out[0].Line, out[1].Line, out[2].Line})
}

func TestRingDrainPartial(t *testing.T) {
	r := NewRing(10)
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		r.Push(SinkLine{Line: s})
	}
	out := r.DrainN(3)
	require.Len(t, out, 3)
	assert.Equal(t, []string{"a", "b", "c"}, []string{out[0].Line, out[1].Line, out[2].Line})
	assert.Equal(t, 2, r.Len())

	rest := r.DrainAll()
	require.Len(t, rest, 2)
	assert.Equal(t, []string{"d", "e"}, []string{rest[0].Line, rest[1].Line})
}
