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
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesCleanupConfig represents the configuration for Kubernetes cleanup.
type KubernetesCleanupConfig struct {
	// ResourceAgeThreshold is the age threshold for resources to be considered stale
	ResourceAgeThreshold time.Duration
	// DryRun if true, will only log what would be deleted
	DryRun bool
	// Namespaces is the list of namespaces to clean up. If empty, uses current namespace
	Namespaces []string
}

// KubernetesCleanupStats tracks statistics about the cleanup operation.
type KubernetesCleanupStats struct {
	ServicesFound   int
	ServicesDeleted int
	PVCsFound       int
	PVCsDeleted     int
	SecretsFound    int
	SecretsDeleted  int
}

// NewKubernetesCleanupFunc creates a function to clean up stale Kubernetes resources.
// It cleans up Services (crow-svc-*), PVCs (wp-*), and Secrets (wp-*, crow-*) that are older than the configured threshold.
//
// Permission Requirements:
// - If config.Namespaces is specified, requires permissions in those namespaces only
// - If config.Namespaces is empty, only requires permissions in the current namespace
// - No cluster-wide permissions required (no ClusterRole needed)
//
// This would allow more precise filtering and avoid potential false positives.
func NewKubernetesCleanupFunc(config KubernetesCleanupConfig) func(ctx context.Context, dryRun bool) error {
	return func(ctx context.Context, dryRun bool) error {
		dynamicConfig := config
		dynamicConfig.DryRun = dryRun
		client, err := getKubernetesClient()
		if err != nil {
			return fmt.Errorf("failed to get Kubernetes client: %w", err)
		}

		namespaces := getNamespacesToCleanup(dynamicConfig)

		var stats *KubernetesCleanupStats
		if statsFromCtx := ctx.Value(cleanupStatsKey); statsFromCtx != nil {
			if s, ok := statsFromCtx.(*KubernetesCleanupStats); ok {
				stats = s
			}
		}

		// Clean up services
		servicesFound, servicesDeleted := cleanupServices(ctx, client, dynamicConfig, namespaces)
		if stats != nil {
			stats.ServicesFound = servicesFound
			stats.ServicesDeleted = servicesDeleted
		}

		// Clean up PVCs
		pvcsFound, pvcsDeleted := cleanupPVCs(ctx, client, dynamicConfig, namespaces)
		if stats != nil {
			stats.PVCsFound = pvcsFound
			stats.PVCsDeleted = pvcsDeleted
		}

		// Clean up secrets
		secretsFound, secretsDeleted := cleanupSecrets(ctx, client, dynamicConfig, namespaces)
		if stats != nil {
			stats.SecretsFound = secretsFound
			stats.SecretsDeleted = secretsDeleted
		}
		return nil
	}
}

// getKubernetesClient returns a Kubernetes client.
func getKubernetesClient() (kubernetes.Interface, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeConfigPath := os.Getenv("KUBECONFIG")
		if kubeConfigPath == "" {
			kubeConfigPath = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, err
		}
	}

	return kubernetes.NewForConfig(config)
}

// getNamespacesToCleanup returns the list of namespaces to clean up.
// If config.Namespaces is specified, use those. Otherwise, try to get current namespace.
func getNamespacesToCleanup(config KubernetesCleanupConfig) []string {
	if len(config.Namespaces) > 0 {
		log.Info().Strs("namespaces", config.Namespaces).Msg("Using configured namespaces for cleanup")
		return config.Namespaces
	}

	// Try to get current namespace from service account
	currentNamespace, err := getCurrentNamespace()
	if err != nil {
		log.Warn().Err(err).Msg("Could not determine current namespace, defaulting to 'default'")
		return []string{"default"}
	}

	log.Info().Str("namespace", currentNamespace).Msg("Using current namespace for cleanup")
	return []string{currentNamespace}
}

// getCurrentNamespace returns the current namespace from the service account token.
func getCurrentNamespace() (string, error) {
	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	if data, err := os.ReadFile(namespaceFile); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	if ns := os.Getenv("KUBERNETES_NAMESPACE"); ns != "" {
		return ns, nil
	}

	return "", fmt.Errorf("could not determine current namespace")
}

// cleanupServices cleans up stale Crow services.
func cleanupServices(ctx context.Context, client kubernetes.Interface, config KubernetesCleanupConfig, namespaces []string) (int, int) {
	log.Info().Msg("Starting Kubernetes service cleanup")

	totalDeleted := 0
	totalFound := 0

	for _, namespaceName := range namespaces {
		// Use label selector to find all Crow-managed services
		services, err := client.CoreV1().Services(namespaceName).List(ctx, meta_v1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=crow",
		})
		if err != nil {
			log.Warn().Err(err).Str("namespace", namespaceName).Msg("Failed to list services")
			continue
		}

		for _, svc := range services.Items {
			age := time.Since(svc.CreationTimestamp.Time)
			if age >= config.ResourceAgeThreshold {
				totalFound++

				if config.DryRun {
					log.Info().
						Str("service", svc.Name).
						Str("namespace", namespaceName).
						Str("age", age.String()).
						Msg("Would delete stale service (dry run)")
				} else {
					err := client.CoreV1().Services(namespaceName).Delete(ctx, svc.Name, meta_v1.DeleteOptions{})
					if err != nil && !errors.IsNotFound(err) {
						log.Error().Err(err).
							Str("service", svc.Name).
							Str("namespace", namespaceName).
							Msg("Failed to delete service")
					} else {
						totalDeleted++
						log.Info().
							Str("service", svc.Name).
							Str("namespace", namespaceName).
							Str("age", age.String()).
							Msg("Deleted stale service")
					}
				}
			}
		}
	}

	log.Info().
		Int("total_found", totalFound).
		Int("total_deleted", totalDeleted).
		Bool("dry_run", config.DryRun).
		Msg("Kubernetes service cleanup completed")

	return totalFound, totalDeleted
}

// cleanupPVCs cleans up stale Crow PVCs.
func cleanupPVCs(ctx context.Context, client kubernetes.Interface, config KubernetesCleanupConfig, namespaces []string) (int, int) {
	log.Info().Msg("Starting Kubernetes PVC cleanup")

	totalDeleted := 0
	totalFound := 0

	for _, namespaceName := range namespaces {
		pvcs, err := client.CoreV1().PersistentVolumeClaims(namespaceName).List(ctx, meta_v1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=crow",
		})
		if err != nil {
			log.Warn().Err(err).Str("namespace", namespaceName).Msg("Failed to list PVCs")
			continue
		}

		for _, pvc := range pvcs.Items {
			// Check if PVC is old enough to be cleaned up
			age := time.Since(pvc.CreationTimestamp.Time)
			if age >= config.ResourceAgeThreshold {
				totalFound++

				if config.DryRun {
					log.Info().
						Str("pvc", pvc.Name).
						Str("namespace", namespaceName).
						Str("age", age.String()).
						Msg("Would delete stale PVC (dry run)")
				} else {
					err := client.CoreV1().PersistentVolumeClaims(namespaceName).Delete(ctx, pvc.Name, meta_v1.DeleteOptions{})
					if err != nil && !errors.IsNotFound(err) {
						log.Error().Err(err).
							Str("pvc", pvc.Name).
							Str("namespace", namespaceName).
							Msg("Failed to delete PVC")
					} else {
						totalDeleted++
						log.Info().
							Str("pvc", pvc.Name).
							Str("namespace", namespaceName).
							Str("age", age.String()).
							Msg("Deleted stale PVC")
					}
				}
			}
		}
	}

	log.Info().
		Int("total_found", totalFound).
		Int("total_deleted", totalDeleted).
		Bool("dry_run", config.DryRun).
		Msg("Kubernetes PVC cleanup completed")

	return totalFound, totalDeleted
}

// cleanupSecrets cleans up stale Crow secrets.
func cleanupSecrets(ctx context.Context, client kubernetes.Interface, config KubernetesCleanupConfig, namespaces []string) (int, int) {
	log.Info().Msg("Starting Kubernetes Secret cleanup")

	totalDeleted := 0
	totalFound := 0

	for _, namespaceName := range namespaces {
		secrets, err := client.CoreV1().Secrets(namespaceName).List(ctx, meta_v1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=crow",
		})
		if err != nil {
			log.Warn().Err(err).Str("namespace", namespaceName).Msg("Failed to list secrets")
			continue
		}

		for _, secret := range secrets.Items {
			// Check if secret is old enough to be cleaned up
			age := time.Since(secret.CreationTimestamp.Time)
			if age >= config.ResourceAgeThreshold {
				totalFound++

				if config.DryRun {
					log.Info().
						Str("secret", secret.Name).
						Str("namespace", namespaceName).
						Str("age", age.String()).
						Msg("Would delete stale secret (dry run)")
				} else {
					err := client.CoreV1().Secrets(namespaceName).Delete(ctx, secret.Name, meta_v1.DeleteOptions{})
					if err != nil && !errors.IsNotFound(err) {
						log.Error().Err(err).
							Str("secret", secret.Name).
							Str("namespace", namespaceName).
							Msg("Failed to delete secret")
					} else {
						totalDeleted++
						log.Info().
							Str("secret", secret.Name).
							Str("namespace", namespaceName).
							Str("age", age.String()).
							Msg("Deleted stale secret")
					}
				}
			}
		}
	}

	log.Info().
		Int("total_found", totalFound).
		Int("total_deleted", totalDeleted).
		Bool("dry_run", config.DryRun).
		Msg("Kubernetes Secret cleanup completed")

	return totalFound, totalDeleted
}
