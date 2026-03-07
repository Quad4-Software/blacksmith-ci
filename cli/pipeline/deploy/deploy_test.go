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

package deploy

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/internal"
)

func TestDeployCmd_ParamFlagCommaHandling(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name: "simple param without commas",
			args: []string{"deploy", "--param", "FOO=bar"},
			expected: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name: "param value containing commas",
			args: []string{"deploy", "--param", "TARGETS=staging,production"},
			expected: map[string]string{
				"TARGETS": "staging,production",
			},
		},
		{
			name: "multiple params with commas",
			args: []string{"deploy", "--param", "A=x,y", "--param", "B=1,2,3"},
			expected: map[string]string{
				"A": "x,y",
				"B": "1,2,3",
			},
		},
		{
			name: "param with short alias and commas",
			args: []string{"deploy", "-p", "HOSTS=host1,host2,host3"},
			expected: map[string]string{
				"HOSTS": "host1,host2,host3",
			},
		},
		{
			name: "param with commas and special characters",
			args: []string{"deploy", "--param", "ARGS=--flag=a,--flag=b,--flag=c"},
			expected: map[string]string{
				"ARGS": "--flag=a,--flag=b,--flag=c",
			},
		},
		{
			name:     "no params provided",
			args:     []string{"deploy"},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured map[string]string

			command := &cli.Command{
				Writer:                    io.Discard,
				Name:                      "deploy",
				DisableSliceFlagSeparator: true,
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "param",
						Aliases: []string{"p"},
					},
				},
				Action: func(_ context.Context, c *cli.Command) error {
					captured = internal.ParseKeyPair(c.StringSlice("param"))
					return nil
				},
			}

			err := command.Run(t.Context(), tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, captured)
		})
	}
}

func TestDeployCmd_DisableSliceFlagSeparatorIsSet(t *testing.T) {
	assert.True(t, Command.DisableSliceFlagSeparator,
		"DisableSliceFlagSeparator must be true to allow commas in --param values")
}
