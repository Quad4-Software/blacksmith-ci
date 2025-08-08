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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestKubernetesCleanupConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   KubernetesCleanupConfig
		expected KubernetesCleanupConfig
	}{
		{
			name: "default config",
			config: KubernetesCleanupConfig{
				ResourceAgeThreshold: 24 * time.Hour,
				DryRun:               false,
			},
			expected: KubernetesCleanupConfig{
				ResourceAgeThreshold: 24 * time.Hour,
				DryRun:               false,
			},
		},
		{
			name: "dry run config",
			config: KubernetesCleanupConfig{
				ResourceAgeThreshold: 168 * time.Hour, // 1 week
				DryRun:               true,
			},
			expected: KubernetesCleanupConfig{
				ResourceAgeThreshold: 168 * time.Hour,
				DryRun:               true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected.ResourceAgeThreshold, tt.config.ResourceAgeThreshold)
			assert.Equal(t, tt.expected.DryRun, tt.config.DryRun)
		})
	}
}

func TestNewKubernetesCleanupFunc(t *testing.T) {
	config := KubernetesCleanupConfig{
		ResourceAgeThreshold: 24 * time.Hour,
		DryRun:               false,
	}

	cleanupFunc := NewKubernetesCleanupFunc(config)
	assert.NotNil(t, cleanupFunc)
}
