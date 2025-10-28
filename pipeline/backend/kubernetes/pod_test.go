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

package kubernetes

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kinbiko/jsonassert"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"codeberg.org/crowci/crow/v4/pipeline/backend/types"
)

const taskUUID = "11301"

func TestPodName(t *testing.T) {
	name, err := podName(&types.Step{UUID: "01he8bebctabr3kgk0qj36d2me-0"})
	assert.NoError(t, err)
	assert.Equal(t, "crow-01he8bebctabr3kgk0qj36d2me-0", name)

	_, err = podName(&types.Step{UUID: "01he8bebctabr3kgk0qj36d2me\\0a"})
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)

	_, err = podName(&types.Step{UUID: "01he8bebctabr3kgk0qj36d2me-0-services-0..woodpecker-runtime.svc.cluster.local"})
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)
}

func TestStepToPodName(t *testing.T) {
	name, err := stepToPodName(&types.Step{UUID: "01he8bebctabr3kg", Name: "clone", Type: types.StepTypeClone})
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-01he8bebctabr3kg", name)
	name, err = stepToPodName(&types.Step{UUID: "01he8bebctabr3kg", Name: "cache", Type: types.StepTypeCache})
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-01he8bebctabr3kg", name)
	name, err = stepToPodName(&types.Step{UUID: "01he8bebctabr3kg", Name: "release", Type: types.StepTypePlugin})
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-01he8bebctabr3kg", name)
	name, err = stepToPodName(&types.Step{UUID: "01he8bebctabr3kg", Name: "prepare-env", Type: types.StepTypeCommands})
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-01he8bebctabr3kg", name)
	name, err = stepToPodName(&types.Step{UUID: "01he8bebctabr3kg", Name: "postgres", Type: types.StepTypeService})
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-svc-01he8bebctabr3kg-postgres", name)
	// Detached service
	name, err = stepToPodName(&types.Step{UUID: "01he8bebctabr3kg", Name: "postgres", Detached: true})
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-svc-01he8bebctabr3kg-postgres", name)
	// Detached long running container
	name, err = stepToPodName(&types.Step{UUID: "01he8bebctabr3kg", Name: "long running", Detached: true})
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-01he8bebctabr3kg", name)

	descriptiveStep := &types.Step{
		Name:                "build",
		UUID:                "irrelevant",
		ContainerNameScheme: "descriptive",
		Environment: map[string]string{
			"CI_REPO_OWNER":      "myowner",
			"CI_REPO_NAME":       "myrepo",
			"CI_PIPELINE_NUMBER": "123",
			"CI_WORKFLOW_NUMBER": "1",
		},
	}
	name, err = stepToPodName(descriptiveStep)
	assert.NoError(t, err)
	assert.EqualValues(t, "myowner-myrepo-123-1-build", name)

	// Test fallback to hash when environment variables are incomplete
	stepIncomplete := &types.Step{
		Name:                "build",
		UUID:                "abcdefghijk",
		ContainerNameScheme: "descriptive",
		Environment: map[string]string{
			"CI_REPO_OWNER": "myowner",
			"CI_REPO_NAME":  "myrepo",
			// Missing CI_PIPELINE_NUMBER
			"CI_WORKFLOW_NUMBER": "1",
		},
	}
	name, err = stepToPodName(stepIncomplete)
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-abcdefghijk", name)

	// Test truncation
	longRepo := &types.Step{
		Name:                "build",
		UUID:                "irrelevant",
		ContainerNameScheme: "descriptive",
		Environment: map[string]string{
			"CI_REPO_OWNER":      strings.Repeat("a", 35),
			"CI_REPO_NAME":       strings.Repeat("a", 35),
			"CI_PIPELINE_NUMBER": "123",
			"CI_WORKFLOW_NUMBER": "456",
		},
	}
	name, err = stepToPodName(longRepo)
	assert.NoError(t, err)
	if len(name) > 63 {
		t.Errorf("pod name too long: %d chars", len(name))
	}
}

func TestPodMeta(t *testing.T) {
	meta, err := podMeta(&types.Step{
		Name:        "postgres",
		UUID:        "01he8bebctabr3kg",
		Type:        types.StepTypeService,
		Image:       "postgres:16",
		WorkingDir:  "/woodpecker/src",
		Environment: map[string]string{"CI": "woodpecker"},
	}, &config{
		Namespace: "woodpecker",
	}, BackendOptions{}, "crow-01he8bebctabr3kg-0", taskUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-svc-01he8bebctabr3kg-postgres", meta.Labels[ServiceLabel])
	assert.EqualValues(t, taskUUID, meta.Labels[TaskUUIDLabel])

	// Detached service
	meta, err = podMeta(&types.Step{
		Name:        "postgres",
		UUID:        "01he8bebctabr3kg",
		Detached:    true,
		Image:       "postgres:16",
		WorkingDir:  "/woodpecker/src",
		Environment: map[string]string{"CI": "woodpecker"},
	}, &config{
		Namespace: "woodpecker",
	}, BackendOptions{}, "crow-01he8bebctabr3kg-0", taskUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "crow-svc-01he8bebctabr3kg-postgres", meta.Labels[ServiceLabel])

	// Detached long running container
	meta, err = podMeta(&types.Step{
		Name:        "long running",
		UUID:        "01he8bebctabr3kg",
		Detached:    true,
		Image:       "postgres:16",
		WorkingDir:  "/woodpecker/src",
		Environment: map[string]string{"CI": "woodpecker"},
	}, &config{
		Namespace: "woodpecker",
	}, BackendOptions{}, "crow-01he8bebctabr3kg-0", taskUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "", meta.Labels[ServiceLabel])
}

func TestStepLabel(t *testing.T) {
	name, err := stepLabel(&types.Step{Name: "Build image"})
	assert.NoError(t, err)
	assert.EqualValues(t, "build-image", name)

	_, err = stepLabel(&types.Step{Name: ".build.image"})
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)
}

func TestTinyPod(t *testing.T) {
	const expected = `
	{
		"metadata": {
			"name": "crow-01he8bebctabr3kgk0qj36d2me-0",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow",
				"crowci/step": "build-via-gradle",
				"crowci/task-uuid": "11301",
				"step": "build-via-gradle"
			}
		},
		"spec": {
			"volumes": [
				{
					"name": "workspace",
					"persistentVolumeClaim": {
						"claimName": "workspace"
					}
				}
			],
			"containers": [
				{
					"name": "crow-01he8bebctabr3kgk0qj36d2me-0",
					"image": "gradle:8.4.0-jdk21",
					"command": [
						"/bin/sh",
						"-c",
						"echo $CI_SCRIPT | base64 -d | /bin/sh -e"
					],
					"env": [
						"<<UNORDERED>>",
						{
							"name": "CI",
							"value": "woodpecker"
						},
						{
							"name": "SHELL",
							"value": "/bin/sh"
						},
						{
							"name": "CI_SCRIPT",
							"value": "CmlmIFsgLW4gIiRDSV9ORVRSQ19NQUNISU5FIiBdOyB0aGVuCmNhdCA8PEVPRiA+ICRIT01FLy5uZXRyYwptYWNoaW5lICRDSV9ORVRSQ19NQUNISU5FCmxvZ2luICRDSV9ORVRSQ19VU0VSTkFNRQpwYXNzd29yZCAkQ0lfTkVUUkNfUEFTU1dPUkQKRU9GCmNobW9kIDA2MDAgJEhPTUUvLm5ldHJjCmZpCnVuc2V0IENJX05FVFJDX1VTRVJOQU1FCnVuc2V0IENJX05FVFJDX1BBU1NXT1JECnVuc2V0IENJX1NDUklQVApta2RpciAtcCAiL3dvb2RwZWNrZXIvc3JjIgpjZCAiL3dvb2RwZWNrZXIvc3JjIgoKZWNobyArICdncmFkbGUgYnVpbGQnCmdyYWRsZSBidWlsZAo="
						}
					],
					"resources": {},
					"volumeMounts": [
						{
							"name": "workspace",
							"mountPath": "/woodpecker/src"
						}
					]
				}
			],
			"restartPolicy": "Never"
		},
		"status": {}
	}`

	pod, err := mkPod(&types.Step{
		Name:        "build-via-gradle",
		Image:       "gradle:8.4.0-jdk21",
		UUID:        "01he8bebctabr3kgk0qj36d2me-0",
		WorkingDir:  "/woodpecker/src",
		Pull:        false,
		Privileged:  false,
		Commands:    []string{"gradle build"},
		Volumes:     []string{"workspace:/woodpecker/src"},
		Environment: map[string]string{"CI": "woodpecker"},
	}, &config{
		Namespace: "woodpecker",
	}, "crow-01he8bebctabr3kgk0qj36d2me-0", "linux/amd64", BackendOptions{}, taskUUID)
	assert.NoError(t, err)

	podJSON, err := json.Marshal(pod)
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(podJSON), expected)
}

func TestFullPod(t *testing.T) {
	const expected = `
	{
		"metadata": {
			"name": "crow-01he8bebctabr3kgk0qj36d2me-0",
			"namespace": "woodpecker",
			"labels": {
				"app": "test",
				"app.kubernetes.io/managed-by": "crow",
				"part-of": "woodpecker-ci",
				"step": "go-test",
				"crowci/step": "go-test",
				"crowci/task-uuid": "11301"
			},
			"annotations": {
				"apps.kubernetes.io/pod-index": "0",
				"kubernetes.io/limit-ranger": "LimitRanger plugin set: cpu, memory request and limit for container"
			}
		},
		"spec": {
			"volumes": [
				{
					"name": "woodpecker-cache",
					"persistentVolumeClaim": {
						"claimName": "woodpecker-cache"
					}
				}
			],
			"containers": [
				{
					"name": "crow-01he8bebctabr3kgk0qj36d2me-0",
					"image": "meltwater/drone-cache",
					"command": [
						"/bin/sh",
						"-c"
					],
					"ports": [
						{
							"containerPort": 1234
						},
						{
							"containerPort": 2345,
							"protocol": "TCP"
						},
						{
							"containerPort": 3456,
							"protocol": "UDP"
						}
					],
					"env": [
						"<<UNORDERED>>",
						{
							"name": "CGO",
							"value": "0"
						},
						{
							"name": "CI_SCRIPT",
							"value": "CmlmIFsgLW4gIiRDSV9ORVRSQ19NQUNISU5FIiBdOyB0aGVuCmNhdCA8PEVPRiA+ICRIT01FLy5uZXRyYwptYWNoaW5lICRDSV9ORVRSQ19NQUNISU5FCmxvZ2luICRDSV9ORVRSQ19VU0VSTkFNRQpwYXNzd29yZCAkQ0lfTkVUUkNfUEFTU1dPUkQKRU9GCmNobW9kIDA2MDAgJEhPTUUvLm5ldHJjCmZpCnVuc2V0IENJX05FVFJDX1VTRVJOQU1FCnVuc2V0IENJX05FVFJDX1BBU1NXT1JECnVuc2V0IENJX1NDUklQVApta2RpciAtcCAiL3dvb2RwZWNrZXIvc3JjIgpjZCAiL3dvb2RwZWNrZXIvc3JjIgoKZWNobyArICdnbyBnZXQnCmdvIGdldAoKZWNobyArICdnbyB0ZXN0JwpnbyB0ZXN0Cg=="
						},
						{
							"name": "SHELL",
							"value": "/bin/sh"
						}
					],
					"resources": {
						"limits": {
							"cpu": "2",
							"memory": "256Mi"
						},
						"requests": {
							"cpu": "1",
							"memory": "128Mi"
						}
					},
					"volumeMounts": [
						{
							"name": "woodpecker-cache",
							"mountPath": "/woodpecker/src/cache"
						}
					],
					"imagePullPolicy": "Always",
					"securityContext": {
						"privileged": true
					}
				}
			],
			"restartPolicy": "Never",
			"nodeSelector": {
				"storage": "ssd",
				"topology.kubernetes.io/region": "eu-central-1"
			},
			"runtimeClassName": "runc",
			"serviceAccountName": "wp-svc-acc",
			"securityContext": {
				"runAsUser": 101,
				"runAsGroup": 101,
				"runAsNonRoot": true,
				"fsGroup": 101,
				"appArmorProfile": {
					"type": "Localhost",
					"localhostProfile": "k8s-apparmor-example-deny-write"
				},
				"seccompProfile": {
					"type": "Localhost",
					"localhostProfile": "profiles/audit.json"
				}
			},
			"imagePullSecrets": [
				{
					"name": "regcred"
				},
				{
					"name": "another-pull-secret"
				},
				{
					"name": "crow-01he8bebctabr3kgk0qj36d2me-0"
				}
			],
			"tolerations": [
				{
					"key": "net-port",
					"value": "100Mbit",
					"effect": "NoSchedule"
				}
			],
			"hostAliases": [
				{
					"ip": "1.1.1.1",
					"hostnames": [
						"cloudflare"
					]
				},
				{
					"ip": "2606:4700:4700::64",
					"hostnames": [
						"cf.v6"
					]
				}
			]
		},
		"status": {}
	}`

	runtimeClass := "runc"
	hostAliases := []types.HostAlias{
		{Name: "cloudflare", IP: "1.1.1.1"},
		{Name: "cf.v6", IP: "2606:4700:4700::64"},
	}
	ports := []types.Port{
		{Number: 1234},
		{Number: 2345, Protocol: "tcp"},
		{Number: 3456, Protocol: "udp"},
	}
	secCtx := SecurityContext{
		Privileged:   newBool(true),
		RunAsNonRoot: newBool(true),
		RunAsUser:    newInt64(101),
		RunAsGroup:   newInt64(101),
		FSGroup:      newInt64(101),
		SeccompProfile: &SecProfile{
			Type:             "Localhost",
			LocalhostProfile: "profiles/audit.json",
		},
		ApparmorProfile: &SecProfile{
			Type:             "Localhost",
			LocalhostProfile: "k8s-apparmor-example-deny-write",
		},
	}
	pod, err := mkPod(&types.Step{
		UUID:        "01he8bebctabr3kgk0qj36d2me-0",
		Name:        "go-test",
		Image:       "meltwater/drone-cache",
		WorkingDir:  "/woodpecker/src",
		Pull:        true,
		Privileged:  true,
		Commands:    []string{"go get", "go test"},
		Entrypoint:  []string{"/bin/sh", "-c"},
		Volumes:     []string{"woodpecker-cache:/woodpecker/src/cache"},
		Environment: map[string]string{"CGO": "0"},
		ExtraHosts:  hostAliases,
		Ports:       ports,
		AuthConfig: types.Auth{
			Username: "foo",
			Password: "bar",
		},
	}, &config{
		Namespace:                   "woodpecker",
		ImagePullSecretNames:        []string{"regcred", "another-pull-secret"},
		PodLabels:                   map[string]string{"app": "test"},
		PodLabelsAllowFromStep:      true,
		PodAnnotations:              map[string]string{"apps.kubernetes.io/pod-index": "0"},
		PodAnnotationsAllowFromStep: true,
		PodTolerationsAllowFromStep: true,
		PodNodeSelector:             map[string]string{"topology.kubernetes.io/region": "eu-central-1"},
		SecurityContext:             SecurityContextConfig{RunAsNonRoot: false},
	}, "crow-01he8bebctabr3kgk0qj36d2me-0", "linux/amd64", BackendOptions{
		Labels:             map[string]string{"part-of": "woodpecker-ci"},
		Annotations:        map[string]string{"kubernetes.io/limit-ranger": "LimitRanger plugin set: cpu, memory request and limit for container"},
		NodeSelector:       map[string]string{"storage": "ssd"},
		RuntimeClassName:   &runtimeClass,
		ServiceAccountName: "wp-svc-acc",
		Tolerations:        []Toleration{{Key: "net-port", Value: "100Mbit", Effect: TaintEffectNoSchedule}},
		Resources: Resources{
			Requests: map[string]string{"memory": "128Mi", "cpu": "1000m"},
			Limits:   map[string]string{"memory": "256Mi", "cpu": "2"},
		},
		SecurityContext: &secCtx,
	}, taskUUID)
	assert.NoError(t, err)

	podJSON, err := json.Marshal(pod)
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(podJSON), expected)
}

func TestPodPrivilege(t *testing.T) {
	createTestPod := func(stepPrivileged, globalRunAsRoot bool, secCtx SecurityContext) (*v1.Pod, error) {
		return mkPod(&types.Step{
			Name:       "go-test",
			Image:      "golang:1.16",
			UUID:       "01he8bebctabr3kgk0qj36d2me-0",
			Privileged: stepPrivileged,
		}, &config{
			Namespace:       "woodpecker",
			SecurityContext: SecurityContextConfig{RunAsNonRoot: globalRunAsRoot},
		}, "crow-01he8bebctabr3kgk0qj36d2me-0", "linux/amd64", BackendOptions{
			SecurityContext: &secCtx,
		}, "")
	}

	// security context is requesting user and group 101 (non-root)
	secCtx := SecurityContext{
		RunAsUser:  newInt64(101),
		RunAsGroup: newInt64(101),
		FSGroup:    newInt64(101),
	}
	pod, err := createTestPod(false, false, secCtx)
	assert.NoError(t, err)
	assert.Equal(t, int64(101), *pod.Spec.SecurityContext.RunAsUser)
	assert.Equal(t, int64(101), *pod.Spec.SecurityContext.RunAsGroup)
	assert.Equal(t, int64(101), *pod.Spec.SecurityContext.FSGroup)

	// security context is requesting root, but step is not privileged
	secCtx = SecurityContext{
		RunAsUser:  newInt64(0),
		RunAsGroup: newInt64(0),
		FSGroup:    newInt64(0),
	}
	pod, err = createTestPod(false, false, secCtx)
	assert.NoError(t, err)
	assert.Equal(t, &v1.PodSecurityContext{
		SELinuxOptions:           (*v1.SELinuxOptions)(nil),
		WindowsOptions:           (*v1.WindowsSecurityContextOptions)(nil),
		RunAsUser:                (*int64)(nil),
		RunAsGroup:               (*int64)(nil),
		RunAsNonRoot:             (*bool)(nil),
		SupplementalGroups:       []int64(nil),
		SupplementalGroupsPolicy: (*v1.SupplementalGroupsPolicy)(nil),
		FSGroup:                  newInt64(0),
		Sysctls:                  []v1.Sysctl(nil),
		FSGroupChangePolicy:      (*v1.PodFSGroupChangePolicy)(nil),
		SeccompProfile:           (*v1.SeccompProfile)(nil),
		AppArmorProfile:          (*v1.AppArmorProfile)(nil),
	}, pod.Spec.SecurityContext)
	assert.Nil(t, pod.Spec.Containers[0].SecurityContext)

	// step is not privileged, but security context is requesting privileged
	secCtx = SecurityContext{
		Privileged: newBool(true),
	}
	pod, err = createTestPod(false, false, secCtx)
	assert.NoError(t, err)
	assert.Nil(t, pod.Spec.SecurityContext)
	assert.Equal(t, (*v1.PodSecurityContext)(nil), pod.Spec.SecurityContext)

	// step is privileged and security context is requesting privileged
	secCtx = SecurityContext{
		Privileged: newBool(true),
	}
	pod, err = createTestPod(true, false, secCtx)
	assert.NoError(t, err)
	assert.True(t, *pod.Spec.Containers[0].SecurityContext.Privileged)

	// step is privileged and no security context is provided
	secCtx = SecurityContext{}
	pod, err = createTestPod(true, false, secCtx)
	assert.NoError(t, err)
	assert.True(t, *pod.Spec.Containers[0].SecurityContext.Privileged)

	// global runAsNonRoot is true and override is requested value by security context
	secCtx = SecurityContext{
		RunAsNonRoot: newBool(false),
	}
	pod, err = createTestPod(false, true, secCtx)
	assert.NoError(t, err)
	assert.True(t, *pod.Spec.SecurityContext.RunAsNonRoot)
}

func TestScratchPod(t *testing.T) {
	const expected = `
	{
		"metadata": {
			"name": "crow-01he8bebctabr3kgk0qj36d2me-0",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow",
				"crowci/step": "curl-google",
				"crowci/task-uuid": "11301",
				"step": "curl-google"
			}
		},
		"spec": {
			"containers": [
				{
					"name": "crow-01he8bebctabr3kgk0qj36d2me-0",
					"image": "quay.io/curl/curl",
					"command": [
						"/usr/bin/curl",
						"-v",
						"google.com"
					],
					"resources": {}
				}
			],
			"restartPolicy": "Never"
		},
		"status": {}
	}`

	pod, err := mkPod(&types.Step{
		Name:       "curl-google",
		Image:      "quay.io/curl/curl",
		UUID:       "01he8bebctabr3kgk0qj36d2me-0",
		Entrypoint: []string{"/usr/bin/curl", "-v", "google.com"},
	}, &config{
		Namespace: "woodpecker",
	}, "crow-01he8bebctabr3kgk0qj36d2me-0", "linux/amd64", BackendOptions{}, taskUUID)
	assert.NoError(t, err)

	podJSON, err := json.Marshal(pod)
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(podJSON), expected)
}

func TestSecrets(t *testing.T) {
	const expected = `
	{
		"metadata": {
			"name": "crow-3kgk0qj36d2me01he8bebctabr-0",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow",
				"crowci/step": "test-secrets",
				"crowci/task-uuid": "11301",
				"step": "test-secrets"
			}
		},
		"spec": {
			"volumes": [
				{
					"name": "workspace",
					"persistentVolumeClaim": {
						"claimName": "workspace"
					}
				},
				{
					"name": "reg-cred",
					"secret": {
						"secretName": "reg-cred"
					}
				}
			],
			"containers": [
				{
					"name": "crow-3kgk0qj36d2me01he8bebctabr-0",
					"image": "alpine",
					"envFrom": [
						{
							"secretRef": {
								"name": "ghcr-push-secret"
							}
						}
					],
					"env": [
						{
							"name": "CGO",
							"value": "0"
						},
						{
							"name": "AWS_ACCESS_KEY_ID",
							"valueFrom": {
								"secretKeyRef": {
									"name": "aws-ecr",
									"key": "AWS_ACCESS_KEY_ID"
								}
							}
						},
						{
							"name": "AWS_SECRET_ACCESS_KEY",
							"valueFrom": {
								"secretKeyRef": {
									"name": "aws-ecr",
									"key": "access-key"
								}
							}
						}
					],
					"resources": {},
					"volumeMounts": [
						{
							"name": "workspace",
							"mountPath": "/woodpecker/src"
						},
						{
							"name": "reg-cred",
							"mountPath": "~/.docker/config.json",
							"subPath": ".dockerconfigjson",
							"readOnly": true
						}
					]
				}
			],
			"restartPolicy": "Never"
		},
		"status": {}
	}`

	pod, err := mkPod(&types.Step{
		Name:        "test-secrets",
		Image:       "alpine",
		UUID:        "01he8bebctabr3kgk0qj36d2me-0",
		Environment: map[string]string{"CGO": "0"},
		Volumes:     []string{"workspace:/woodpecker/src"},
	}, &config{
		Namespace:                  "woodpecker",
		NativeSecretsAllowFromStep: true,
	}, "crow-3kgk0qj36d2me01he8bebctabr-0", "linux/amd64", BackendOptions{
		Secrets: []SecretRef{
			{
				Name: "ghcr-push-secret",
			},
			{
				Name: "aws-ecr",
				Key:  "AWS_ACCESS_KEY_ID",
			},
			{
				Name:   "aws-ecr",
				Key:    "access-key",
				Target: SecretTarget{Env: "AWS_SECRET_ACCESS_KEY"},
			},
			{
				Name:   "reg-cred",
				Key:    ".dockerconfigjson",
				Target: SecretTarget{File: "~/.docker/config.json"},
			},
		},
	}, taskUUID)
	assert.NoError(t, err)

	podJSON, err := json.Marshal(pod)
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(podJSON), expected)
}

func TestPodTolerations(t *testing.T) {
	const expected = `
	{
		"metadata": {
			"name": "wp-01he8bebctabr3kgk0qj36d2me-0",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow",
				"crowci/step": "toleration-test",
				"crowci/task-uuid": "11301",
				"step": "toleration-test"
			}
		},
		"spec": {
			"containers": [
				{
					"name": "wp-01he8bebctabr3kgk0qj36d2me-0",
					"image": "alpine",
					"resources": {}
				}
			],
			"restartPolicy": "Never",
			"tolerations": [
				{
					"key": "foo",
					"value": "bar",
					"effect": "NoSchedule"
				},
				{
					"key": "baz",
					"value": "qux",
					"effect": "NoExecute"
				}
			]
		},
		"status": {}
	}`

	globalTolerations := []Toleration{
		{Key: "foo", Value: "bar", Effect: TaintEffectNoSchedule},
		{Key: "baz", Value: "qux", Effect: TaintEffectNoExecute},
	}

	pod, err := mkPod(&types.Step{
		Name:  "toleration-test",
		Image: "alpine",
		UUID:  "01he8bebctabr3kgk0qj36d2me-0",
	}, &config{
		Namespace:                   "woodpecker",
		PodTolerations:              globalTolerations,
		PodTolerationsAllowFromStep: false,
	}, "wp-01he8bebctabr3kgk0qj36d2me-0", "linux/amd64", BackendOptions{}, taskUUID)
	assert.NoError(t, err)

	podJSON, err := json.Marshal(pod)
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(podJSON), expected)
}

func TestPodTolerationsAllowFromStep(t *testing.T) {
	const expectedDisallow = `
	{
		"metadata": {
			"name": "wp-01he8bebctabr3kgk0qj36d2me-0",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow",
				"crowci/step": "toleration-test",
				"crowci/task-uuid": "11301",
				"step": "toleration-test"
			}
		},
		"spec": {
			"containers": [
				{
					"name": "wp-01he8bebctabr3kgk0qj36d2me-0",
					"image": "alpine",
					"resources": {}
				}
			],
			"restartPolicy": "Never"
		},
		"status": {}
	}`
	const expectedAllow = `
	{
		"metadata": {
			"name": "wp-01he8bebctabr3kgk0qj36d2me-0",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow",
				"crowci/step": "toleration-test",
				"crowci/task-uuid": "11301",
				"step": "toleration-test"
			}
		},
		"spec": {
			"containers": [
				{
					"name": "wp-01he8bebctabr3kgk0qj36d2me-0",
					"image": "alpine",
					"resources": {}
				}
			],
			"restartPolicy": "Never",
			"tolerations": [
				{
					"key": "custom",
					"value": "value",
					"effect": "NoSchedule"
				}
			]
		},
		"status": {}
	}`

	stepTolerations := []Toleration{
		{Key: "custom", Value: "value", Effect: TaintEffectNoSchedule},
	}

	step := &types.Step{
		Name:  "toleration-test",
		Image: "alpine",
		UUID:  "01he8bebctabr3kgk0qj36d2me-0",
	}

	pod, err := mkPod(step, &config{
		Namespace:                   "woodpecker",
		PodTolerationsAllowFromStep: false,
	}, "wp-01he8bebctabr3kgk0qj36d2me-0", "linux/amd64", BackendOptions{
		Tolerations: stepTolerations,
	}, taskUUID)
	assert.NoError(t, err)

	podJSON, err := json.Marshal(pod)
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(podJSON), expectedDisallow)

	pod, err = mkPod(step, &config{
		Namespace:                   "woodpecker",
		PodTolerationsAllowFromStep: true,
	}, "wp-01he8bebctabr3kgk0qj36d2me-0", "linux/amd64", BackendOptions{
		Tolerations: stepTolerations,
	}, taskUUID)
	assert.NoError(t, err)

	podJSON, err = json.Marshal(pod)
	assert.NoError(t, err)

	ja = jsonassert.New(t)
	ja.Assertf(string(podJSON), expectedAllow)
}

func TestStepSecret(t *testing.T) {
	const expected = `{
		"metadata": {
			"name": "crow-01he8bebctabr3kgk0qj36d2me-0-step-secret",
			"namespace": "woodpecker",
			"labels": {
				"app.kubernetes.io/managed-by": "crow"
			}
		},
		"type": "Opaque",
		"stringData": {
			"VERY_SECRET": "secret_value"
		}
	}`

	secret, err := mkStepSecret(&types.Step{
		UUID:  "01he8bebctabr3kgk0qj36d2me-0",
		Name:  "go-test",
		Image: "meltwater/drone-cache",
		SecretMapping: map[string]string{
			"VERY_SECRET": "secret_value",
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
