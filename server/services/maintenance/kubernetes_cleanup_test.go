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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
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

func TestCleanupSecretsSelection(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprintf("dryRun=%t", dryRun), func(t *testing.T) {
			old := metav1.NewTime(time.Now().Add(-48 * time.Hour))
			owned := map[string]string{"app.kubernetes.io/managed-by": "crow"}
			client := fake.NewClientset(
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "jobs", Labels: owned, CreationTimestamp: old}},
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "recent", Namespace: "jobs", Labels: owned, CreationTimestamp: metav1.Now()}},
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "legacy-step-secret", Namespace: "jobs", CreationTimestamp: old}},
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "other", Labels: owned, CreationTimestamp: old}},
			)
			found, deleted, err := cleanupSecrets(t.Context(), client, KubernetesCleanupConfig{ResourceAgeThreshold: 24 * time.Hour, DryRun: dryRun}, []string{"jobs"})
			require.NoError(t, err)
			require.Equal(t, 1, found)
			wantDeleted := 1
			if dryRun {
				wantDeleted = 0
			}
			require.Equal(t, wantDeleted, deleted)
			remaining, err := client.CoreV1().Secrets("").List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, remaining.Items, 4-wantDeleted)
		})
	}
}

func TestCleanupKubernetesResourcesReportsErrorsAndContinues(t *testing.T) {
	for _, resource := range []string{"services", "persistentvolumeclaims", "secrets"} {
		for _, verb := range []string{"list", "delete"} {
			t.Run(resource+"/"+verb, func(t *testing.T) {
				metadata := metav1.ObjectMeta{Name: "stale", Namespace: "jobs", Labels: map[string]string{"app.kubernetes.io/managed-by": "crow"}, CreationTimestamp: metav1.NewTime(time.Now().Add(-48 * time.Hour))}
				client := fake.NewClientset(&v1.Service{ObjectMeta: metadata}, &v1.PersistentVolumeClaim{ObjectMeta: metadata}, &v1.Secret{ObjectMeta: metadata}, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "stale", Namespace: "other", Labels: metadata.Labels, CreationTimestamp: metadata.CreationTimestamp}})
				failure := fmt.Errorf("permission denied")
				client.PrependReactor(verb, resource, func(action ktesting.Action) (bool, runtime.Object, error) {
					if action.GetNamespace() == "jobs" {
						return true, nil, failure
					}
					return false, nil, nil
				})
				stats := &KubernetesCleanupStats{}
				ctx := context.WithValue(t.Context(), cleanupStatsKey, stats)
				err := cleanupKubernetesResources(ctx, client, KubernetesCleanupConfig{ResourceAgeThreshold: 24 * time.Hour}, []string{"jobs", "other"})
				require.ErrorIs(t, err, failure)
				require.Contains(t, err.Error(), "jobs")
				require.Equal(t, 3, stats.ServicesDeleted+stats.PVCsDeleted+stats.SecretsDeleted)
				remaining, err := client.CoreV1().Secrets("other").List(t.Context(), metav1.ListOptions{})
				require.NoError(t, err)
				require.Empty(t, remaining.Items)
			})
		}
	}
}

func TestCleanupNamespaces(t *testing.T) {
	namespaces, err := getNamespacesToCleanup(KubernetesCleanupConfig{Namespaces: []string{"jobs", "crow-42"}})
	require.NoError(t, err)
	require.Equal(t, []string{"jobs", "crow-42"}, namespaces)
	for _, namespaces := range [][]string{{""}, {"jobs", " "}} {
		_, err := getNamespacesToCleanup(KubernetesCleanupConfig{Namespaces: namespaces})
		require.ErrorContains(t, err, "must not be empty")
	}
}
