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
	"github.com/urfave/cli/v3"
)

var Flags = []cli.Flag{
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_NAMESPACE", "CROW_BACKEND_K8S_NAMESPACE"),
		Name:    "backend-k8s-namespace",
		Usage:   "backend k8s namespace, if used with WOODPECKER_BACKEND_K8S_NAMESPACE_PER_ORGANIZATION, this will be the prefix for the namespace appended with the organization name.",
		Value:   "woodpecker",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_NAMESPACE_PER_ORGANIZATION", "CROW_BACKEND_K8S_NAMESPACE_PER_ORGANIZATION"),
		Name:    "backend-k8s-namespace-per-org",
		Usage:   "Whether to enable namespace segregation per organization feature. When enabled, Crow will create the Kubernetes resources to separated Kubernetes namespaces per Crow organization.",
		Value:   false,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_VOLUME_SIZE", "CROW_BACKEND_K8S_VOLUME_SIZE"),
		Name:    "backend-k8s-volume-size",
		Usage:   "backend k8s volume size (default 10G)",
		Value:   "10G",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_STORAGE_CLASS", "CROW_BACKEND_K8S_STORAGE_CLASS"),
		Name:    "backend-k8s-storage-class",
		Usage:   "backend k8s storage class",
		Value:   "",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_STORAGE_RWX", "CROW_BACKEND_K8S_STORAGE_RWX"),
		Name:    "backend-k8s-storage-rwx",
		Usage:   "backend k8s storage access mode, should ReadWriteMany (RWX) instead of ReadWriteOnce (RWO) be used? (default: true)",
		Value:   true,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_POD_LABELS", "CROW_BACKEND_K8S_POD_LABELS"),
		Name:    "backend-k8s-pod-labels",
		Usage:   "backend k8s additional Agent-wide worker pod labels",
		Value:   "",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_POD_LABELS_ALLOW_FROM_STEP", "CROW_BACKEND_K8S_POD_LABELS_ALLOW_FROM_STEP"),
		Name:    "backend-k8s-pod-labels-allow-from-step",
		Usage:   "whether to allow using labels from step's backend options",
		Value:   false,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_POD_ANNOTATIONS", "CROW_BACKEND_K8S_POD_ANNOTATIONS"),
		Name:    "backend-k8s-pod-annotations",
		Usage:   "backend k8s additional Agent-wide worker pod annotations",
		Value:   "",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_POD_NODE_SELECTOR", "CROW_BACKEND_K8S_POD_NODE_SELECTOR"),
		Name:    "backend-k8s-pod-node-selector",
		Usage:   "backend k8s Agent-wide worker pod node selector",
		Value:   "",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_POD_TOLERATIONS", "CROW_BACKEND_K8S_POD_TOLERATIONS"),
		Name:    "backend-k8s-pod-tolerations",
		Usage:   "backend k8s Agent-wide worker pod tolerations",
		Value:   "",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_POD_TOLERATIONS_ALLOW_FROM_STEP", "CROW_BACKEND_K8S_POD_TOLERATIONS_ALLOW_FROM_STEP"),
		Name:    "backend-k8s-pod-tolerations-allow-from-step",
		Usage:   "whether to allow using tolerations from step's backend options",
		Value:   true,
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_POD_ANNOTATIONS_ALLOW_FROM_STEP", "CROW_BACKEND_K8S_POD_ANNOTATIONS_ALLOW_FROM_STEP"),
		Name:    "backend-k8s-pod-annotations-allow-from-step",
		Usage:   "whether to allow using annotations from step's backend options",
		Value:   false,
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_SECCTX_NONROOT", "CROW_BACKEND_K8S_SECCTX_NONROOT"), // cspell:words secctx nonroot
		Name:    "backend-k8s-secctx-nonroot",
		Usage:   "`run as non root` Kubernetes security context option",
	},
	&cli.StringSliceFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_PULL_SECRET_NAMES", "CROW_BACKEND_K8S_PULL_SECRET_NAMES"),
		Name:    "backend-k8s-pod-image-pull-secret-names",
		Usage:   "backend k8s pull secret names for private registries",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_ALLOW_NATIVE_SECRETS", "CROW_BACKEND_K8S_ALLOW_NATIVE_SECRETS"),
		Name:    "backend-k8s-allow-native-secrets",
		Usage:   "whether to allow existing Kubernetes secrets to be referenced from steps",
		Value:   false,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("WOODPECKER_BACKEND_K8S_PRIORITY_CLASS", "CROW_BACKEND_K8S_PRIORITY_CLASS"),
		Name:    "backend-k8s-priority-class",
		Usage:   "which kubernetes priority class to assign to created job pods",
		Value:   "",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_K8S_SERVICE_ACCOUNT_NAME"),
		Name:    "backend-k8s-service-account-name",
		Usage:   "backend k8s service account name for worker pods",
		Value:   "",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("CROW_BACKEND_K8S_SERVICE_ACCOUNT_NAME_ALLOW_FROM_STEP"),
		Name:    "backend-k8s-service-account-name-allow-from-step",
		Usage:   "whether to allow using serviceAccountName from step's backend options",
		Value:   false,
	},
}
