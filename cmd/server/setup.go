// Copyright 2022 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

package main

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"
	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/cache"
	"codeberg.org/crowci/crow/v5/server/forge/setup"
	"codeberg.org/crowci/crow/v5/server/integrations"
	"codeberg.org/crowci/crow/v5/server/integrations/vault"
	"codeberg.org/crowci/crow/v5/server/logging"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/pubsub"
	"codeberg.org/crowci/crow/v5/server/queue"
	"codeberg.org/crowci/crow/v5/server/services"
	integrationService "codeberg.org/crowci/crow/v5/server/services/integration"
	"codeberg.org/crowci/crow/v5/server/services/lock"
	logService "codeberg.org/crowci/crow/v5/server/services/log"
	"codeberg.org/crowci/crow/v5/server/services/log/file"
	"codeberg.org/crowci/crow/v5/server/services/maintenance"
	"codeberg.org/crowci/crow/v5/server/services/permissions"
	"codeberg.org/crowci/crow/v5/server/store"
	"codeberg.org/crowci/crow/v5/server/store/datastore"
	"codeberg.org/crowci/crow/v5/server/store/types"
)

const (
	queueInfoRefreshInterval = 500 * time.Millisecond
	storeInfoRefreshInterval = 10 * time.Second
)

func setupStore(ctx context.Context, c *cli.Command) (store.Store, error) {
	datasource := c.String("db-datasource")
	driver := c.String("db-driver")
	schema := c.String("db-schema")
	xorm := store.XORM{
		Log:             c.Bool("db-log"),
		ShowSQL:         c.Bool("db-log-sql"),
		MaxOpenConns:    int(c.Int64("db-max-open-connections")),
		MaxIdleConns:    int(c.Int64("db-max-idle-connections")),
		ConnMaxLifetime: c.Duration("db-max-connection-timeout"),
	}

	if driver == "sqlite3" {
		if datastore.SupportedDriver("sqlite3") {
			log.Debug().Msg("server has sqlite3 support")
		} else {
			log.Debug().Msg("server was built without sqlite3 support!")
		}
	}

	if !datastore.SupportedDriver(driver) {
		return nil, fmt.Errorf("database driver '%s' not supported", driver)
	}

	if driver == "sqlite3" {
		if err := checkSqliteFileExist(datasource); err != nil {
			return nil, fmt.Errorf("check sqlite file: %w", err)
		}
	}

	if schema != "" && driver != "postgres" {
		return nil, fmt.Errorf("database schema is only supported for PostgreSQL")
	}

	opts := &store.Opts{
		Driver: driver,
		Config: datasource,
		Schema: schema,
		XORM:   xorm,
	}
	log.Debug().Str("driver", driver).Str("schema", schema).Any("xorm", xorm).Msg("setting up datastore")
	store, err := datastore.NewEngine(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open datastore: %w", err)
	}

	if err = store.Ping(); err != nil {
		return nil, err
	}

	if err := store.Migrate(ctx, c.Bool("migrations-allow-long")); err != nil {
		return nil, fmt.Errorf("could not migrate datastore: %w", err)
	}

	// Initialize distributed locks to prevent race conditions in HA mode
	if err := store.DistributedLockInit(); err != nil {
		log.Warn().Err(err).Msg("Failed to initialize distributed locks - they will be created on demand")
		// Non-fatal: locks will be created when first used
	}

	return store, nil
}

func checkSqliteFileExist(path string) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		log.Warn().Msgf("no sqlite3 file found, will create one at '%s'", path)
		return nil
	}
	return err
}

func setupQueue(ctx context.Context, s store.Store, lockService *lock.Service, c *cli.Command, haEnabled bool) (queue.Queue, error) {
	// Use database queue for HA deployments, memory queue for single instance
	backend := queue.TypeMemory
	if haEnabled {
		backend = queue.TypeDatabase
		log.Info().Bool("haEnabled", true).Msg("Using database queue for HA deployment")
	} else {
		log.Info().Bool("haEnabled", false).Msg("Using memory queue for single-instance deployment")
	}

	config := queue.Config{
		Backend:       backend,
		Store:         s,
		InstanceID:    server.Config.Server.InstanceID,
		LeaderLockTTL: c.Duration("queue-lock-ttl"),
	}

	if backend == queue.TypeDatabase {
		config.LockService = lockService
	}

	q, err := queue.New(ctx, config)
	if err != nil {
		return nil, err
	}

	// Clean up orphaned workflows on startup
	go func() {
		// Wait a bit for the server to fully start
		time.Sleep(5 * time.Second)

		// Use reflection to check if this is a fifo queue and get running workflow IDs
		if getter, ok := q.(interface{ GetRunningWorkflowIDs() []string }); ok {
			runningIDs := getter.GetRunningWorkflowIDs()
			if len(runningIDs) > 0 {
				log.Info().Strs("workflow_ids", runningIDs).Msg("checking for orphaned workflows on startup")
				if err := s.CleanupOrphanedWorkflows(runningIDs); err != nil {
					log.Error().Err(err).Msg("failed to cleanup orphaned workflows")
				}
			}
		}
	}()

	return q, nil
}

func setupMembershipService(_ context.Context, _store store.Store, haEnabled bool) cache.MembershipService {
	// Use distributed cache for HA deployments, memory cache for single instance
	backend := cache.TypeMemory
	if haEnabled {
		backend = cache.TypeDistributed
		log.Info().Bool("haEnabled", true).Msg("Using distributed cache for HA deployment")
	} else {
		log.Info().Bool("haEnabled", false).Msg("Using memory cache for single-instance deployment")
	}

	config := cache.Config{
		Backend: backend,
		Store:   _store,
	}

	return cache.NewMembershipServiceWithConfig(config)
}

func setupPubSub(s store.Store, instanceID string, haEnabled bool) pubsub.Publisher {
	// Use distributed pub/sub for HA deployments, memory for single instance
	backend := pubsub.TypeMemory
	if haEnabled {
		backend = pubsub.TypeDistributed
		log.Info().Bool("haEnabled", true).Msg("Using distributed pub/sub for HA deployment")
	} else {
		log.Info().Bool("haEnabled", false).Msg("Using memory pub/sub for single-instance deployment")
	}

	return pubsub.NewPublisher(pubsub.Config{
		Backend:    backend,
		Store:      s,
		InstanceID: instanceID,
	})
}

func setupLogStore(c *cli.Command, s store.Store) (logService.Service, error) {
	switch c.String("log-store") {
	case "file":
		return file.NewLogStore(c.String("log-store-file-path"))
	default:
		return s, nil
	}
}

const jwtSecretID = "jwt-secret"

func setupJWTSecret(_store store.Store) (string, error) {
	jwtSecret, err := _store.ServerConfigGet(jwtSecretID)
	if errors.Is(err, types.RecordNotExist) {
		jwtSecret := base32.StdEncoding.EncodeToString(
			random.GetRandomBytes(32),
		)
		err = _store.ServerConfigSet(jwtSecretID, jwtSecret)
		if err != nil {
			return "", err
		}
		log.Debug().Msg("created jwt secret")
		return jwtSecret, nil
	}

	if err != nil {
		return "", err
	}

	return jwtSecret, nil
}

func setupEvilGlobals(ctx context.Context, c *cli.Command, s store.Store) (err error) {
	haEnabled := c.Bool("enable-ha")
	log.Info().Bool("haEnabled", haEnabled).Str("envVar", os.Getenv("CROW_ENABLE_HA")).Msg("HA configuration debug")

	// Set instance ID BEFORE initializing services that need it
	if haEnabled {
		if hostname, err := os.Hostname(); err == nil {
			server.Config.Server.InstanceID = "crow-server-" + hostname
			log.Info().Str("instanceID", server.Config.Server.InstanceID).Bool("haEnabled", true).Msg("HA mode enabled - using automatic instance ID")
		} else {
			server.Config.Server.InstanceID = "crow-server-default"
			log.Warn().Err(err).Str("instanceID", server.Config.Server.InstanceID).Bool("haEnabled", true).Msg("HA mode enabled - failed to get hostname, using default instance ID")
		}
	} else {
		server.Config.Server.InstanceID = ""
		log.Info().Bool("haEnabled", false).Msg("HA mode disabled - using single-instance configuration")
	}

	// services (now with correct instance ID)
	server.Config.Services.Membership = setupMembershipService(ctx, s, haEnabled)
	// Setup lock service for distributed coordination
	lockService := lock.NewService(s, server.Config.Server.InstanceID)

	// Setup pub/sub system - use distributed for HA deployments
	server.Config.Services.Pubsub = setupPubSub(s, server.Config.Server.InstanceID, haEnabled)

	// Setup logging service - use distributed for HA deployments
	if haEnabled {
		server.Config.Services.Logs = logging.NewDistributed(server.Config.Services.Pubsub, s, server.Config.Server.InstanceID)
		log.Info().Bool("haEnabled", true).Msg("Using distributed logging for HA deployment")
	} else {
		server.Config.Services.Logs = logging.New()
		log.Info().Bool("haEnabled", false).Msg("Using memory logging for single-instance deployment")
	}

	server.Config.Services.Queue, err = setupQueue(ctx, s, lockService, c, haEnabled)
	if err != nil {
		return fmt.Errorf("could not setup queue: %w", err)
	}
	server.Config.Services.Manager, err = services.NewManager(c, s, setup.Forge)
	if err != nil {
		return fmt.Errorf("could not setup service manager: %w", err)
	}
	server.Config.Services.LogStore, err = setupLogStore(c, s)
	if err != nil {
		return fmt.Errorf("could not setup log store: %w", err)
	}

	// maintenance service
	server.Config.Services.Store = s
	server.Config.Services.MaintenanceService = maintenance.NewService(s)

	// integration service (external secret stores)
	providerFactory := integrations.NewDefaultFactory(vault.NewProviderFunc())
	integrations.SetGlobalFactory(providerFactory)
	server.Config.Services.IntegrationService = integrationService.NewService(s, providerFactory)

	// Check if vacuum-specific maintenance environment variables are set
	vacuumEnvVarsSet := os.Getenv("CROW_MAINTENANCE_VACUUM_LOG_TABLE_ENABLED") != "" ||
		os.Getenv("CROW_MAINTENANCE_VACUUM_LOG_TABLE_SCHEDULE") != ""

	// Check if Kubernetes-specific maintenance environment variables are set
	k8sEnvVarsSet := os.Getenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_ENABLED") != "" ||
		os.Getenv("WOODPECKER_MAINTENANCE_KUBERNETES_CLEANUP_ENABLED") != "" ||
		os.Getenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_AGE") != "" ||
		os.Getenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_NAMESPACES") != ""

	server.Config.Services.MaintenanceService.SetKubernetesEnvVarsConfigured(k8sEnvVarsSet)
	server.Config.Services.MaintenanceService.SetVacuumEnvVarsConfigured(vacuumEnvVarsSet)

	// Handle vacuum configuration from environment variables
	if vacuumEnvVarsSet {
		vacuumEnabled := c.Bool("maintenance-log-vacuum-enabled")
		vacuumSchedule := c.String("maintenance-log-vacuum-schedule")

		// Update vacuum action config in database
		vacuumConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("vacuum")
		if err != nil {
			log.Error().Err(err).Msg("Failed to get vacuum action config")
		} else {
			needsUpdate := false
			if vacuumConfig.Enabled != vacuumEnabled {
				vacuumConfig.Enabled = vacuumEnabled
				needsUpdate = true
			}
			if vacuumConfig.Schedule != vacuumSchedule {
				vacuumConfig.Schedule = vacuumSchedule
				needsUpdate = true
			}

			if needsUpdate {
				log.Info().
					Bool("setting_enabled", vacuumConfig.Enabled).
					Str("setting_schedule", vacuumConfig.Schedule).
					Msg("Updating vacuum action configuration from environment variables")

				if err := server.Config.Services.MaintenanceService.UpdateMaintenanceConfigByAction("vacuum", vacuumConfig); err != nil {
					log.Error().Err(err).Msg("Failed to update vacuum action config")
				}
			}
		}
	}

	k8sCleanupAge := c.String("maintenance-kubernetes-cleanup-age")

	// Auto-enable if using Kubernetes backend (detect via CROW_BACKEND environment variable)
	backendType := os.Getenv("CROW_BACKEND")
	if backendType == "" {
		backendType = os.Getenv("WOODPECKER_BACKEND")
	}
	usingKubernetesBackend := backendType == "kubernetes"

	if k8sCleanupAge != "" || usingKubernetesBackend {
		if k8sCleanupAge == "" {
			k8sCleanupAge = "168h" // Default to 7 days (168h)
		}
		ageDuration, err := time.ParseDuration(k8sCleanupAge)
		if err != nil {
			log.Error().Err(err).Str("age", k8sCleanupAge).Msg("Invalid Kubernetes cleanup age duration")
		} else {
			// Parse namespace configuration
			var namespaces []string
			namespacesStr := c.String("maintenance-kubernetes-cleanup-namespaces")
			if namespacesStr != "" {
				namespaces = strings.Split(namespacesStr, ",")
				// Trim whitespace from each namespace
				for i, ns := range namespaces {
					namespaces[i] = strings.TrimSpace(ns)
				}
			}

			k8sCleanupConfig := maintenance.KubernetesCleanupConfig{
				ResourceAgeThreshold: ageDuration,
				DryRun:               false, // Will be set per-request in the UI
				Namespaces:           namespaces,
			}
			server.Config.Services.MaintenanceService.SetKubernetesCleanupFunc(
				maintenance.NewKubernetesCleanupFunc(k8sCleanupConfig),
			)

			server.Config.Services.MaintenanceService.SetKubernetesCleanupEnabled(true)

			enabled := c.Bool("maintenance-kubernetes-cleanup-enabled")

			envEnabledVar := os.Getenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_ENABLED")
			envEnabledVarWP := os.Getenv("WOODPECKER_MAINTENANCE_KUBERNETES_CLEANUP_ENABLED")
			envVarExplicitlySet := envEnabledVar != "" || envEnabledVarWP != ""

			// If using Kubernetes backend and no explicit env var is set, enable by default
			if !envVarExplicitlySet && usingKubernetesBackend {
				enabled = true
			}
			if envVarExplicitlySet {
				enabled = c.Bool("maintenance-kubernetes-cleanup-enabled")
			}

			k8sActionConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("kubernetes_cleanup")
			if err != nil {
				log.Error().Err(err).Msg("Failed to get kubernetes_cleanup action config")
			} else {
				log.Info().
					Bool("current_k8s_enabled", k8sActionConfig.Enabled).
					Str("current_k8s_schedule", k8sActionConfig.Schedule).
					Bool("env_enabled", enabled).
					Bool("auto_enabled", usingKubernetesBackend).
					Msg("Checking Kubernetes cleanup action configuration")

				needsUpdate := false

				// Only update if schedule is empty (first time setup) or if explicitly enabled via env var
				if k8sActionConfig.Schedule == "" {
					k8sActionConfig.Schedule = "0 5 * * *" // Daily at 5 AM
					needsUpdate = true
				}

				if k8sActionConfig.Enabled != enabled {
					k8sActionConfig.Enabled = enabled
					needsUpdate = true
				}

				if needsUpdate {
					log.Info().
						Bool("setting_enabled", k8sActionConfig.Enabled).
						Str("setting_schedule", k8sActionConfig.Schedule).
						Msg("Updating Kubernetes cleanup action configuration")

					if err := server.Config.Services.MaintenanceService.UpdateMaintenanceConfigByAction("kubernetes_cleanup", k8sActionConfig); err != nil {
						log.Error().Err(err).Msg("Failed to update Kubernetes cleanup action config")
					} else {
						log.Info().Msg("Updated Kubernetes cleanup action configuration in database")
					}
				} else {
					log.Info().Msg("Kubernetes cleanup action configuration already set correctly, skipping")
				}
			}
		}
	}

	server.Config.Services.MaintenanceScheduler = maintenance.NewScheduler(server.Config.Services.MaintenanceService, s, lockService)
	if err := server.Config.Services.MaintenanceScheduler.Start(); err != nil {
		log.Error().Err(err).Msg("failed to start maintenance scheduler")
	}

	// agents
	server.Config.Agent.DisableUserRegisteredAgentRegistration = c.Bool("disable-user-agent-registration")

	// authentication
	server.Config.Pipeline.AuthenticatePublicRepos = c.Bool("authenticate-public-repos")

	// Pull requests
	server.Config.Pipeline.DefaultAllowPullRequests = c.Bool("default-allow-pull-requests")

	// Approval mode
	approvalMode := model.ApprovalMode(c.String("default-approval-mode"))
	if !approvalMode.Valid() {
		return fmt.Errorf("approval mode %s is not valid", approvalMode)
	}
	server.Config.Pipeline.DefaultApprovalMode = approvalMode

	// Cloning
	server.Config.Pipeline.DefaultClonePlugin = c.String("default-clone-plugin")
	server.Config.Pipeline.TrustedClonePlugins = c.StringSlice("plugins-trusted-clone")
	server.Config.Pipeline.TrustedClonePlugins = append(server.Config.Pipeline.TrustedClonePlugins, server.Config.Pipeline.DefaultClonePlugin)

	// Logs
	server.Config.Pipeline.LogsDurationKeep = c.String("logs-duration-keep")
	server.Config.Pipeline.LogsPipelinesKeepMin = c.Int64("logs-pipelines-keep-min")

	// Execution
	_events := c.StringSlice("default-cancel-previous-pipeline-events")
	events := make([]model.WebhookEvent, 0, len(_events))
	for _, v := range _events {
		e := model.WebhookEvent(v)
		if err := e.Validate(); err != nil {
			return err
		}
		events = append(events, e)
	}
	server.Config.Pipeline.DefaultCancelPreviousPipelineEvents = events
	server.Config.Pipeline.DefaultTimeout = c.Int64("default-pipeline-timeout")
	server.Config.Pipeline.MaxTimeout = c.Int64("max-pipeline-timeout")

	_labels := c.StringSlice("default-workflow-labels")
	labels := make(map[string]string, len(_labels))
	for _, v := range _labels {
		name, value, ok := strings.Cut(v, "=")
		if !ok {
			return fmt.Errorf("invalid label filter: %s", v)
		}
		labels[name] = value
	}
	server.Config.Pipeline.DefaultWorkflowLabels = labels

	// backend options for pipeline compiler
	server.Config.Pipeline.Proxy.No = c.String("backend-no-proxy")
	server.Config.Pipeline.Proxy.HTTP = c.String("backend-http-proxy")
	server.Config.Pipeline.Proxy.HTTPS = c.String("backend-https-proxy")
	server.Config.Pipeline.ContainerNameScheme = c.String("container-name-scheme")

	// server configuration
	server.Config.Server.JWTSecret, err = setupJWTSecret(s)
	if err != nil {
		return fmt.Errorf("could not setup jwt secret: %w", err)
	}
	server.Config.Server.Cert = c.String("server-cert")
	server.Config.Server.Key = c.String("server-key")
	server.Config.Server.AgentToken = c.String("agent-secret")
	serverHost := strings.TrimSuffix(c.String("server-host"), "/")
	server.Config.Server.Host = serverHost
	if c.IsSet("server-webhook-host") {
		server.Config.Server.WebhookHost = c.String("server-webhook-host")
	} else {
		server.Config.Server.WebhookHost = serverHost
	}
	server.Config.Server.OAuthHost = serverHost
	server.Config.Server.Port = c.String("server-addr")
	server.Config.Server.PortTLS = c.String("server-addr-tls")
	server.Config.Server.StatusContext = c.String("status-context")
	server.Config.Server.StatusContextFormat = c.String("status-context-format")
	server.Config.Server.SessionExpires = c.Duration("session-expires")
	u, _ := url.Parse(server.Config.Server.Host)
	rootPath := strings.TrimSuffix(u.Path, "/")

	// Instance ID is now configured at the beginning of setupEvilGlobals
	if rootPath != "" && !strings.HasPrefix(rootPath, "/") {
		rootPath = "/" + rootPath
	}
	server.Config.Server.RootPath = rootPath
	server.Config.Server.CustomCSSFile = strings.TrimSpace(c.String("custom-css-file"))
	server.Config.Server.CustomJsFile = strings.TrimSpace(c.String("custom-js-file"))
	server.Config.Pipeline.Networks = c.StringSlice("network")
	server.Config.Pipeline.Volumes = c.StringSlice("volume")
	server.Config.WebUI.EnableSwagger = c.Bool("enable-swagger")
	server.Config.WebUI.SkipVersionCheck = c.Bool("skip-version-check")
	server.Config.Pipeline.PrivilegedPlugins = c.StringSlice("plugins-privileged")
	server.Config.Pipeline.PrivilegedPluginsMatchType = c.String("plugins-privileged-match-type")

	// prometheus
	server.Config.Prometheus.AuthToken = c.String("prometheus-auth-token")

	// permissions
	server.Config.Permissions.Open = c.Bool("open")
	server.Config.Permissions.Admins = permissions.NewAdmins(c.StringSlice("admin"))
	server.Config.Permissions.Orgs = permissions.NewOrgs(c.StringSlice("orgs"))
	server.Config.Permissions.OwnersAllowlist = permissions.NewOwnersAllowlist(c.StringSlice("repo-owners"))
	return nil
}
