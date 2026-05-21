// Copyright 2023 Woodpecker Authors
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

package core

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackendProbeTracker(t *testing.T) {
	probeErr := errors.New("ping failed")
	start := time.Unix(1_700_000_000, 0)

	t.Run("healthy probes never produce an error", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, nil))
		assert.NoError(t, tracker.record(start.Add(10*time.Minute), nil))
	})

	t.Run("failures under the threshold return nil and keep the start time", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, probeErr))
		assert.NoError(t, tracker.record(start.Add(30*time.Second), probeErr))
		assert.Equal(t, start, tracker.unhealthySince, "unhealthySince must be pinned to the first failure, not the latest")
	})

	t.Run("failure past the threshold returns ErrBackendUnavailable", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, probeErr))
		err := tracker.record(start.Add(61*time.Second), probeErr)
		assert.ErrorIs(t, err, ErrBackendUnavailable)
	})

	t.Run("a recovery resets the streak so later failures get a fresh budget", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, probeErr))
		assert.NoError(t, tracker.record(start.Add(30*time.Second), nil), "recovery must clear unhealthySince")
		assert.True(t, tracker.unhealthySince.IsZero())
		// New failure window starts here; threshold must not be measured from the original failure.
		assert.NoError(t, tracker.record(start.Add(45*time.Second), probeErr))
		assert.NoError(t, tracker.record(start.Add(90*time.Second), probeErr))
	})
}

func TestRunWithRecover(t *testing.T) {
	t.Run("returns nil for non-panicking function", func(t *testing.T) {
		fn := runWithRecover("test", func() error { return nil })
		assert.NoError(t, fn())
	})

	t.Run("returns error for non-panicking function that errors", func(t *testing.T) {
		want := errors.New("boom")
		fn := runWithRecover("test", func() error { return want })
		assert.ErrorIs(t, fn(), want)
	})

	t.Run("recovers from panic and returns it as error", func(t *testing.T) {
		fn := runWithRecover("test", func() error {
			panic("oh no") //nolint:forbidigo // testing recover()
		})
		err := fn()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic in test")
		assert.Contains(t, err.Error(), "oh no")
	})

	t.Run("recovers from panic with non-string value", func(t *testing.T) {
		fn := runWithRecover("test", func() error {
			panic(errors.New("typed panic")) //nolint:forbidigo // testing recover()
		})
		err := fn()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic in test")
		assert.Contains(t, err.Error(), "typed panic")
	})

	t.Run("recovers from runtime panic (out-of-range index)", func(t *testing.T) {
		fn := runWithRecover("test", func() error {
			s := []int{}
			i := len(s) // computed at runtime so nilness can't prove the panic
			_ = s[i]
			return nil
		})
		err := fn()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic in test")
	})
}

func TestStringSliceAddToMap(t *testing.T) {
	tests := []struct {
		name     string
		sl       []string
		m        map[string]string
		expected map[string]string
		err      bool
	}{
		{
			name: "add values to map",
			sl:   []string{"foo=bar", "baz=qux=nux"},
			m:    make(map[string]string),
			expected: map[string]string{
				"foo": "bar",
				"baz": "qux=nux",
			},
			err: false,
		},
		{
			name:     "empty slice",
			sl:       []string{},
			m:        make(map[string]string),
			expected: map[string]string{},
			err:      false,
		},
		{
			name:     "missing value",
			sl:       []string{"foo", "baz=qux"},
			m:        make(map[string]string),
			expected: map[string]string{},
			err:      true,
		},
		{
			name:     "empty string in slice",
			sl:       []string{"foo=bar", "", "baz=qux"},
			m:        make(map[string]string),
			expected: map[string]string{"foo": "bar", "baz": "qux"},
			err:      false,
		},
		{
			name: "multiple values for same key in different entries",
			sl:   []string{"org=value1", "org=value2"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org": "value1|value2",
			},
			err: false,
		},
		{
			name: "multiple values for same key in same entry",
			sl:   []string{"org=value1,org=value2"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org": "value1|value2",
			},
			err: false,
		},
		{
			name: "multiple key-value pairs in one entry",
			sl:   []string{"org=value1,repo=value2"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org":  "value1",
				"repo": "value2",
			},
			err: false,
		},
		{
			name: "complex example with multiple keys and values",
			sl:   []string{"org=value1,org=value2,repo=test"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org":  "value1|value2",
				"repo": "test",
			},
			err: false,
		},
		{
			name: "value containing equals sign",
			sl:   []string{"key=value=with=equals"},
			m:    make(map[string]string),
			expected: map[string]string{
				"key": "value=with=equals",
			},
			err: false,
		},
		{
			name: "appending to existing map",
			sl:   []string{"new=value"},
			m: map[string]string{
				"existing": "data",
			},
			expected: map[string]string{
				"existing": "data",
				"new":      "value",
			},
			err: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stringSliceAddToMap(tt.sl, tt.m)

			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tt.expected, tt.m)
			}
		})
	}
}
