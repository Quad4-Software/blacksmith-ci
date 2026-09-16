// Copyright 2024 Woodpecker Authors
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

package kubernetes

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestWorkflowNamespace(t *testing.T) {
	engine := kube{config: &config{Namespace: "crow"}}

	tests := []struct {
		name string
		conf *types.Config
		want string
	}{
		{
			name: "no stages falls back to default namespace",
			conf: &types.Config{},
			want: "crow",
		},
		{
			name: "stage without steps falls back to default namespace",
			conf: &types.Config{Stages: []*types.Stage{{}}},
			want: "crow",
		},
		{
			name: "resolves namespace from first step",
			conf: &types.Config{Stages: []*types.Stage{{Steps: []*types.Step{{OrgID: 42}}}}},
			want: "crow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, engine.workflowNamespace(tc.conf))
		})
	}
}

func TestWorkflowNamespacePerOrg(t *testing.T) {
	engine := kube{config: &config{Namespace: "crow", EnableNamespacePerOrg: true}}

	// no stages must not panic and must fall back to the base namespace
	assert.Equal(t, "crow", engine.workflowNamespace(&types.Config{}))

	// with a step the org id is appended
	conf := &types.Config{Stages: []*types.Stage{{Steps: []*types.Step{{OrgID: 42}}}}}
	assert.Equal(t, "crow-42", engine.workflowNamespace(conf))
}

func TestSetupWorkflowRejectsTempVolumes(t *testing.T) {
	engine := kube{config: &config{}}
	err := engine.SetupWorkflow(context.Background(), &types.Config{
		TempVolumes: []string{"crow_tmp_dind"},
	}, "task-uuid")
	assert.ErrorContains(t, err, "does not support `temp_volumes`")
}

func TestGettingConfig(t *testing.T) {
	engine := kube{
		config: &config{
			Namespace:            "default",
			StorageClass:         "hdd",
			VolumeSize:           "1G",
			StorageRwx:           false,
			PodLabels:            map[string]string{"l1": "v1"},
			PodAnnotations:       map[string]string{"a1": "v1"},
			ImagePullSecretNames: []string{"regcred"},
			SecurityContext:      SecurityContextConfig{RunAsNonRoot: false},
		},
	}
	config := engine.getConfig()
	config.Namespace = "wp"
	config.StorageClass = "ssd"
	config.StorageRwx = true
	config.PodLabels = nil
	config.PodAnnotations["a2"] = "v2"
	config.ImagePullSecretNames = append(config.ImagePullSecretNames, "docker.io")
	config.SecurityContext.RunAsNonRoot = true

	assert.Equal(t, "default", engine.config.Namespace)
	assert.Equal(t, "hdd", engine.config.StorageClass)
	assert.Equal(t, "1G", engine.config.VolumeSize)
	assert.False(t, engine.config.StorageRwx)
	assert.Len(t, engine.config.PodLabels, 1)
	assert.Len(t, engine.config.PodAnnotations, 1)
	assert.Len(t, engine.config.ImagePullSecretNames, 1)
	assert.False(t, engine.config.SecurityContext.RunAsNonRoot)
}

func TestDestroyWorkflowCleansSecrets(t *testing.T) {
	for _, perOrg := range []bool{false, true} {
		t.Run(fmt.Sprintf("perOrg=%t", perOrg), func(t *testing.T) {
			client := fake.NewClientset()
			engine := &kube{client: client, config: &config{Namespace: "crow", EnableNamespacePerOrg: perOrg}}
			step := &types.Step{UUID: "test", Name: "build", OrgID: 42, Image: "alpine", SecretMapping: map[string]string{"TOKEN": "test"}}
			step.AuthConfig.Username = "user"
			step.AuthConfig.Password = "password"
			ns := engine.config.GetNamespace(step.OrgID)
			require.NoError(t, startStepSecret(t.Context(), engine, step))
			require.NoError(t, startRegistrySecret(t.Context(), engine, step))
			before, err := client.CoreV1().Secrets(ns).List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, before.Items, 2)
			conf := &types.Config{Volume: "workspace", Stages: []*types.Stage{{Steps: []*types.Step{step}}}}
			// Teardown also covers a step whose pod was never created.
			require.NoError(t, engine.DestroyWorkflow(t.Context(), conf, "task"))
			after, err := client.CoreV1().Secrets(ns).List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Empty(t, after.Items)
			require.NoError(t, engine.DestroyWorkflow(t.Context(), conf, "task"))
		})
	}
}

func TestDestroyWorkflowContinuesAfterSecretDeleteError(t *testing.T) {
	client := fake.NewClientset(&v1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "workspace", Namespace: "crow"}})
	engine := &kube{client: client, config: &config{Namespace: "crow"}}
	step := &types.Step{UUID: "test", SecretMapping: map[string]string{"TOKEN": "test"}}
	require.NoError(t, startStepSecret(t.Context(), engine, step))
	client.PrependReactor("delete", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("secret deletion denied")
	})
	conf := &types.Config{Volume: "workspace", Stages: []*types.Stage{{Steps: []*types.Step{step}}}}
	require.ErrorContains(t, engine.DestroyWorkflow(t.Context(), conf, "task"), "secret deletion denied")
	pvcs, err := client.CoreV1().PersistentVolumeClaims("crow").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, pvcs.Items)
}
