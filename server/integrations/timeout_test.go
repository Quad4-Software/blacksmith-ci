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

package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// timeoutNetError implements net.Error and reports Timeout()=true.
type timeoutNetError struct{}

func (*timeoutNetError) Error() string   { return "i/o timeout" }
func (*timeoutNetError) Timeout() bool   { return true }
func (*timeoutNetError) Temporary() bool { return true }

func TestIsTimeoutError(t *testing.T) {
	t.Run("context deadline exceeded", func(t *testing.T) {
		assert.True(t, IsTimeoutError(context.DeadlineExceeded))
	})

	t.Run("wrapped context deadline exceeded", func(t *testing.T) {
		wrapped := fmt.Errorf("doing thing: %w", context.DeadlineExceeded)
		assert.True(t, IsTimeoutError(wrapped))
	})

	t.Run("net.Error timeout via url.Error", func(t *testing.T) {
		urlErr := &url.Error{
			Op:  "Post",
			URL: "https://example.com/login",
			Err: &timeoutNetError{},
		}
		assert.True(t, IsTimeoutError(urlErr))
	})

	t.Run("non-timeout error", func(t *testing.T) {
		assert.False(t, IsTimeoutError(errors.New("connection refused")))
	})

	t.Run("nil", func(t *testing.T) {
		assert.False(t, IsTimeoutError(nil))
	})
}
