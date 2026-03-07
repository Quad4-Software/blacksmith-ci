// Copyright 2026 Crow CI Authors
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

package pipeline

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestPipelineCreateCmd_VarFlagCommaHandling(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name: "simple key=value without commas",
			args: []string{"create", "--branch", "main", "--var", "FOO=bar"},
			expected: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name: "value containing commas is preserved",
			args: []string{"create", "--branch", "main", "--var", "ARCH=some,var"},
			expected: map[string]string{
				"ARCH": "some,var",
			},
		},
		{
			name: "multiple vars each with commas",
			args: []string{"create", "--branch", "main", "--var", "A=x,y,z", "--var", "B=1,2,3"},
			expected: map[string]string{
				"A": "x,y,z",
				"B": "1,2,3",
			},
		},
		{
			name: "value with commas and equals signs",
			args: []string{"create", "--branch", "main", "--var", "OPTS=a=1,b=2,c=3"},
			expected: map[string]string{
				"OPTS": "a=1,b=2,c=3",
			},
		},
		{
			name: "multiple vars without commas",
			args: []string{"create", "--branch", "main", "--var", "FOO=bar", "--var", "BAZ=qux"},
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name: "value with leading and trailing commas",
			args: []string{"create", "--branch", "main", "--var", "CSV=,a,b,c,"},
			expected: map[string]string{
				"CSV": ",a,b,c,",
			},
		},
		{
			name: "value that is only commas",
			args: []string{"create", "--branch", "main", "--var", "SEP=,,,"},
			expected: map[string]string{
				"SEP": ",,,",
			},
		},
		{
			name: "JSON-like value with commas",
			args: []string{"create", "--branch", "main", "--var", `CONFIG={"a":1,"b":2}`},
			expected: map[string]string{
				"CONFIG": `{"a":1,"b":2}`,
			},
		},
		{
			name:     "no vars provided",
			args:     []string{"create", "--branch", "main"},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured map[string]string

			command := &cli.Command{
				Writer:                    io.Discard,
				Name:                      "create",
				DisableSliceFlagSeparator: true,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "branch",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "key=value",
					},
				},
				Action: func(_ context.Context, c *cli.Command) error {
					captured = make(map[string]string)
					for _, vaz := range c.StringSlice("var") {
						before, after, _ := strings.Cut(vaz, "=")
						if before != "" && after != "" {
							captured[before] = after
						}
					}
					return nil
				},
			}

			err := command.Run(t.Context(), tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, captured)
		})
	}
}

func TestPipelineCreateCmd_DisableSliceFlagSeparatorIsSet(t *testing.T) {
	assert.True(t, pipelineCreateCmd.DisableSliceFlagSeparator,
		"DisableSliceFlagSeparator must be true to allow commas in --var values")
}
