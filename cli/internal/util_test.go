// Copyright 2023 Woodpecker Authors
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

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKeyPair(t *testing.T) {
	s := []string{"FOO=bar", "BAR=", "BAZ=qux=quux", "INVALID"}
	p := ParseKeyPair(s)
	assert.Equal(t, "bar", p["FOO"])
	assert.Equal(t, "qux=quux", p["BAZ"])
	val, exists := p["BAR"]
	assert.Empty(t, val)
	assert.True(t, exists, "missing a key with no value, keys with empty values are also valid")
	_, exists = p["INVALID"]
	assert.False(t, exists, "keys without an equal sign suffix are invalid")
}

func TestParseKeyPairWithCommas(t *testing.T) {
	s := []string{
		"SIMPLE=value",
		"COMMA=a,b,c",
		"MIXED=key=val1,key=val2",
		`JSON={"a":1,"b":2}`,
		"ONLY_COMMAS=,,,",
	}
	p := ParseKeyPair(s)
	assert.Equal(t, "value", p["SIMPLE"])
	assert.Equal(t, "a,b,c", p["COMMA"], "commas in values must be preserved")
	assert.Equal(t, "key=val1,key=val2", p["MIXED"], "commas and equals in values must be preserved")
	assert.Equal(t, `{"a":1,"b":2}`, p["JSON"], "JSON-like values with commas must be preserved")
	assert.Equal(t, ",,,", p["ONLY_COMMAS"], "comma-only values must be preserved")
}
