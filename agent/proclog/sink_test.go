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

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSinkParsesZerologJSON(t *testing.T) {
	sink := NewSink(16)
	logger := zerolog.New(sink).With().Timestamp().Logger()

	logger.Info().Msg("hello world")
	logger.Warn().Str("foo", "bar").Msg("danger")

	got := drain(sink)
	require.Len(t, got, 2)

	assert.Equal(t, "info", got[0].Level)
	assert.NotZero(t, got[0].Time)
	assert.Contains(t, got[0].Line, "hello world")

	assert.Equal(t, "warn", got[1].Level)
	assert.Contains(t, got[1].Line, "danger")
}

func TestSinkOverflowDropsOldest(t *testing.T) {
	sink := NewSink(2)
	logger := zerolog.New(sink).With().Timestamp().Logger()
	logger.Info().Msg("a")
	logger.Info().Msg("b")
	logger.Info().Msg("c")

	got := drain(sink)
	require.Len(t, got, 2)
	assert.Contains(t, got[0].Line, "b")
	assert.Contains(t, got[1].Line, "c")
}

func TestSinkIgnoresBlankLines(t *testing.T) {
	sink := NewSink(16)
	_, err := sink.Write([]byte("\n"))
	require.NoError(t, err)
	_, err = sink.Write([]byte("   \n"))
	require.NoError(t, err)
	assert.Empty(t, drain(sink))
}

// drain reads everything currently in the sink's channel without blocking.
func drain(s *Sink) []SinkLine {
	var out []SinkLine
	for {
		select {
		case l, ok := <-s.Out():
			if !ok {
				return out
			}
			out = append(out, l)
		default:
			return out
		}
	}
}
