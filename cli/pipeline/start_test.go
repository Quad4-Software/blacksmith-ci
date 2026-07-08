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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/internal"
)

func TestPipelineStartCmd_ParamFlagCommaHandling(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name: "simple param without commas",
			args: []string{"start", "--param", "FOO=bar"},
			expected: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name: "param value containing commas",
			args: []string{"start", "--param", "ARCH=some,var"},
			expected: map[string]string{
				"ARCH": "some,var",
			},
		},
		{
			name: "multiple params with commas",
			args: []string{"start", "--param", "A=x,y", "--param", "B=1,2,3"},
			expected: map[string]string{
				"A": "x,y",
				"B": "1,2,3",
			},
		},
		{
			name: "param with short alias and commas",
			args: []string{"start", "-p", "LIST=a,b,c"},
			expected: map[string]string{
				"LIST": "a,b,c",
			},
		},
		{
			name: "mixed short and long flags with commas",
			args: []string{"start", "-p", "X=1,2", "--param", "Y=3,4"},
			expected: map[string]string{
				"X": "1,2",
				"Y": "3,4",
			},
		},
		{
			name: "param with equals and commas in value",
			args: []string{"start", "--param", "QUERY=key=val1,key=val2"},
			expected: map[string]string{
				"QUERY": "key=val1,key=val2",
			},
		},
		{
			name:     "no params provided",
			args:     []string{"start"},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured map[string]string

			command := &cli.Command{
				Writer:                    io.Discard,
				Name:                      "start",
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

func TestPipelineStartCmd_DisableSliceFlagSeparatorIsSet(t *testing.T) {
	assert.True(t, pipelineStartCmd.DisableSliceFlagSeparator,
		"DisableSliceFlagSeparator must be true to allow commas in --param values")
}
