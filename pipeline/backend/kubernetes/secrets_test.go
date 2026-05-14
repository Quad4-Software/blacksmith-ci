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
	"encoding/json"
	"testing"

	"github.com/kinbiko/jsonassert"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"codefloe.com/crowci/crow/v5/pipeline"
	"codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

func TestNativeSecretsEnabled(t *testing.T) {
	nsp := newNativeSecretsProcessor(&config{
		NativeSecretsAllowFromStep: true,
	}, nil)
	assert.True(t, nsp.isEnabled())
}

func TestNativeSecretsDisabled(t *testing.T) {
	nsp := newNativeSecretsProcessor(&config{
		NativeSecretsAllowFromStep: false,
	}, []SecretRef{
		{
			Name: "env-simple",
		},
		{
			Name: "env-advanced",
			Key:  "key",
			Target: SecretTarget{
				Env: "ENV_VAR",
			},
		},
		{
			Name: "env-file",
			Key:  "cert",
			Target: SecretTarget{
				File: "/etc/ca/x3.cert",
			},
		},
	})
	assert.False(t, nsp.isEnabled())

	err := nsp.process()
	assert.NoError(t, err)
	assert.Empty(t, nsp.envFromSources)
	assert.Empty(t, nsp.envVars)
	assert.Empty(t, nsp.volumes)
	assert.Empty(t, nsp.mounts)
}

func TestSimpleSecret(t *testing.T) {
	nsp := newNativeSecretsProcessor(&config{
		NativeSecretsAllowFromStep: true,
	}, []SecretRef{
		{
			Name: "test-secret",
		},
	})

	err := nsp.process()
	assert.NoError(t, err)
	assert.Empty(t, nsp.envVars)
	assert.Empty(t, nsp.volumes)
	assert.Empty(t, nsp.mounts)
	assert.Equal(t, []v1.EnvFromSource{
		{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"},
			},
		},
	}, nsp.envFromSources)
}

func TestSecretWithKey(t *testing.T) {
	nsp := newNativeSecretsProcessor(&config{
		NativeSecretsAllowFromStep: true,
	}, []SecretRef{
		{
			Name: "test-secret",
			Key:  "access_key",
		},
	})

	err := nsp.process()
	assert.NoError(t, err)
	assert.Empty(t, nsp.envFromSources)
	assert.Empty(t, nsp.volumes)
	assert.Empty(t, nsp.mounts)
	assert.Equal(t, []v1.EnvVar{
		{
			Name: "ACCESS_KEY",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"},
					Key:                  "access_key",
				},
			},
		},
	}, nsp.envVars)
}

func TestSecretWithKeyMapping(t *testing.T) {
	nsp := newNativeSecretsProcessor(&config{
		NativeSecretsAllowFromStep: true,
	}, []SecretRef{
		{
			Name: "test-secret",
			Key:  "aws-secret",
			Target: SecretTarget{
				Env: "AWS_SECRET_ACCESS_KEY",
			},
		},
	})

	err := nsp.process()
	assert.NoError(t, err)
	assert.Empty(t, nsp.envFromSources)
	assert.Empty(t, nsp.volumes)
	assert.Empty(t, nsp.mounts)
	assert.Equal(t, []v1.EnvVar{
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"},
					Key:                  "aws-secret",
				},
			},
		},
	}, nsp.envVars)
}

func TestFileSecret(t *testing.T) {
	nsp := newNativeSecretsProcessor(&config{
		NativeSecretsAllowFromStep: true,
	}, []SecretRef{
		{
			Name: "reg-cred",
			Key:  ".dockerconfigjson",
			Target: SecretTarget{
				File: "~/.docker/config.json",
			},
		},
	})

	err := nsp.process()
	assert.NoError(t, err)
	assert.Empty(t, nsp.envFromSources)
	assert.Empty(t, nsp.envVars)
	assert.Equal(t, []v1.Volume{
		{
			Name: "reg-cred",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "reg-cred",
				},
			},
		},
	}, nsp.volumes)
	assert.Equal(t, []v1.VolumeMount{
		{
			Name:      "reg-cred",
			ReadOnly:  true,
			MountPath: "~/.docker/config.json",
			SubPath:   ".dockerconfigjson",
		},
	}, nsp.mounts)
}

func TestNoAuthNoSecret(t *testing.T) {
	assert.False(t, needsRegistrySecret(&types.Step{}))
}

func TestNoPasswordNoSecret(t *testing.T) {
	assert.False(t, needsRegistrySecret(&types.Step{
		AuthConfig: types.Auth{Username: "foo"},
	}))
}

func TestNoUsernameNoSecret(t *testing.T) {
	assert.False(t, needsRegistrySecret(&types.Step{
		AuthConfig: types.Auth{Password: "foo"},
	}))
}

func TestUsernameAndPasswordNeedsSecret(t *testing.T) {
	assert.True(t, needsRegistrySecret(&types.Step{
		AuthConfig: types.Auth{Username: "foo", Password: "bar"},
	}))
}

func TestRegistrySecret(t *testing.T) {
	const expected = `{
		"metadata": {
			"name": "crow-01he8bebctabr3kgk0qj36d2me-0",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow",
				"crowci/step": "go-test",
				"step": "go-test"
			}
		},
		"type": "kubernetes.io/dockerconfigjson",
		"data": {
			".dockerconfigjson": "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsidXNlcm5hbWUiOiJmb28iLCJwYXNzd29yZCI6ImJhciJ9fX0="
		}
	}`

	secret, err := mkRegistrySecret(&types.Step{
		UUID:  "01he8bebctabr3kgk0qj36d2me-0",
		Name:  "go-test",
		Image: "meltwater/drone-cache",
		AuthConfig: types.Auth{
			Username: "foo",
			Password: "bar",
		},
	}, &config{
		Namespace: "woodpecker",
	})
	assert.NoError(t, err)

	secretJSON, err := json.Marshal(secret)
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(secretJSON), expected)
}

// TestRegistrySecretSanitizesInternalLabels ensures that crow's internal
// workflow labels (which may contain characters Kubernetes rejects in label
// values, such as '/' in repo full names or '{}' in some forge IDs) are
// sanitized before being attached to a registry pull secret. See issue #1033.
func TestRegistrySecretSanitizesInternalLabels(t *testing.T) {
	secret, err := mkRegistrySecret(&types.Step{
		UUID:  "01he8bebctabr3kgk0qj36d2me-0",
		Name:  "go-test",
		Image: "meltwater/drone-cache",
		AuthConfig: types.Auth{
			Username: "foo",
			Password: "bar",
		},
		WorkflowLabels: map[string]string{
			pipeline.LabelRepoFullName: "foo-team/baz-arch",
			pipeline.LabelRepoForgeID:  "{00000000-0000-0000-0000-000000000000}",
			pipeline.LabelBranch:       "feature/some-thing",
			pipeline.LabelRepoID:       "42",
		},
	}, &config{
		Namespace: "woodpecker",
	})
	assert.NoError(t, err)

	assert.Equal(t, "foo-team-baz-arch", secret.Labels[pipeline.LabelRepoFullName])
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", secret.Labels[pipeline.LabelRepoForgeID])
	assert.Equal(t, "feature-some-thing", secret.Labels[pipeline.LabelBranch])
	assert.Equal(t, "42", secret.Labels[pipeline.LabelRepoID])
}

// TestRegistrySecretFiltersUserLabelsWhenDisallowed ensures non-internal labels
// from the workflow are filtered out unless PodLabelsAllowFromStep is enabled,
// matching the behavior of podLabels.
func TestRegistrySecretFiltersUserLabelsWhenDisallowed(t *testing.T) {
	secret, err := mkRegistrySecret(&types.Step{
		UUID:  "01he8bebctabr3kgk0qj36d2me-0",
		Name:  "go-test",
		Image: "meltwater/drone-cache",
		AuthConfig: types.Auth{
			Username: "foo",
			Password: "bar",
		},
		WorkflowLabels: map[string]string{
			pipeline.LabelRepoFullName: "foo-team/baz-arch",
			"user-label":               "should-not-appear",
		},
	}, &config{
		Namespace:              "woodpecker",
		PodLabelsAllowFromStep: false,
	})
	assert.NoError(t, err)

	assert.Equal(t, "foo-team-baz-arch", secret.Labels[pipeline.LabelRepoFullName])
	_, hasUserLabel := secret.Labels["user-label"]
	assert.False(t, hasUserLabel, "user label should not be propagated when PodLabelsAllowFromStep is false")
}

// TestRegistrySecretSanitizesUserLabelsWhenAllowed ensures user-supplied labels
// also get sanitized when PodLabelsAllowFromStep is enabled.
func TestRegistrySecretSanitizesUserLabelsWhenAllowed(t *testing.T) {
	secret, err := mkRegistrySecret(&types.Step{
		UUID:  "01he8bebctabr3kgk0qj36d2me-0",
		Name:  "go-test",
		Image: "meltwater/drone-cache",
		AuthConfig: types.Auth{
			Username: "foo",
			Password: "bar",
		},
		WorkflowLabels: map[string]string{
			"team": "platform/infra",
		},
	}, &config{
		Namespace:              "woodpecker",
		PodLabelsAllowFromStep: true,
	})
	assert.NoError(t, err)

	assert.Equal(t, "platform-infra", secret.Labels["team"])
}
