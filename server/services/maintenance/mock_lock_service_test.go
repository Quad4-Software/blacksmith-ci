// Copyright 2025 Crow Authors
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

package maintenance

import (
	"context"
	"time"
)

// mockLockService is a simple mock that allows tests to pass.
type mockLockService struct{}

func (m *mockLockService) AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (context.Context, context.CancelFunc, error) {
	// For tests, just return the original context and a no-op cancel function
	return ctx, func() {}, nil
}
