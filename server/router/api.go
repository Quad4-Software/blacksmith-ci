// Copyright 2021 Woodpecker Authors
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

package router

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/api"
	"codefloe.com/crowci/crow/v5/server/api/debug"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/router/middleware/token"
)

func apiRoutes(e *gin.RouterGroup) {
	// Legacy hook endpoint for backwards compatibility with existing webhooks
	// New repos will use /api/v1/hook (see repo.go)
	e.POST("/api/hook", api.PostHook)

	apiBase := e.Group("/api/v1")
	apiBase.Use(token.Refresh)
	apiBase.Use(session.SetAutoscaler())
	{
		user := apiBase.Group("/user")
		{
			user.Use(session.MustUser())
			user.GET("", api.GetSelf)
			user.GET("/feed", api.GetFeed)
			user.GET("/repos", api.GetRepos)
			user.GET("/orgs", api.GetUserOrgList)
			user.GET("/secrets", api.GetUserSecretList)
			user.GET("/registries", api.GetUserRegistryList)
			user.GET("/crons", api.GetUserCronList)

			// User forge connections (multi-forge support)
			user.GET("/forges", api.GetUserForges)
			user.GET("/forges/available", api.GetAvailableForges)
			user.GET("/forges/:forge_id", api.GetUserForge)
			user.GET("/forges/:forge_id/affected-repos", api.GetUserForgeAffectedRepos)
			user.DELETE("/forges/:forge_id", api.DeleteUserForge)

			// Integrations (external secret stores)
			user.GET("/integrations", api.GetIntegrationList)
			user.GET("/integrations/accessible", api.GetAccessibleIntegrations)
			user.POST("/integrations", api.PostIntegration)
			user.GET("/integrations/:integration_id", api.GetIntegration)
			user.PATCH("/integrations/:integration_id", api.PatchIntegration)
			user.DELETE("/integrations/:integration_id", api.DeleteIntegration)
			user.POST("/integrations/:integration_id/test", api.TestIntegration)

			// Access tokens (new API)
			user.GET("/access-tokens", api.GetAccessTokenList)
			user.POST("/access-tokens", api.PostAccessToken)
			user.DELETE("/access-tokens/batch", api.DeleteAccessTokenBatch)
			user.GET("/access-tokens/scopes", api.GetAvailableScopes)
			user.GET("/access-tokens/:token_id", api.GetAccessToken)
			user.PATCH("/access-tokens/:token_id", api.PatchAccessToken)
			user.DELETE("/access-tokens/:token_id", api.DeleteAccessToken)

			// Legacy token endpoints (deprecated - use access-tokens instead)
			user.POST("/token", api.PostToken)
			user.DELETE("/token", api.DeleteToken)
		}

		users := apiBase.Group("/users")
		{
			users.Use(session.MustAdmin())
			users.GET("", api.GetUsers)
			users.POST("", api.PostUser)
			users.GET("/:login", api.GetUser)
			users.PATCH("/:login", api.PatchUser)
			users.DELETE("/:login", api.DeleteUser)
		}

		// Timezones endpoint (public)
		apiBase.GET("/timezones", api.GetTimezones)

		// Integration types (public, for UI forms)
		apiBase.GET("/integrations/types", api.GetIntegrationTypes)

		orgs := apiBase.Group("/orgs")
		{
			orgs.GET("", session.MustAdmin(), api.GetOrgs)
			orgs.GET("/lookup/*org_full_name", api.LookupOrg)
			orgBase := orgs.Group("/:org_id")
			{
				orgBase.Use(session.SetOrg())
				orgBase.Use(session.MustOrg())
				orgBase.GET("/permissions", api.GetOrgPermissions)
				orgBase.GET("", session.MustOrgMember(false), api.GetOrg)

				org := orgBase.Group("")
				{
					org.Use(session.MustOrgMember(true))
					org.DELETE("", session.MustAdmin(), api.DeleteOrg)

					org.GET("/secrets", api.GetOrgSecretList)
					org.POST("/secrets", api.PostOrgSecret)
					org.GET("/secrets/:secret", api.GetOrgSecret)
					org.GET("/secrets/:secret/abbreviated", api.GetOrgSecretAbbreviated)
					org.PATCH("/secrets/:secret", api.PatchOrgSecret)
					org.DELETE("/secrets/:secret", api.DeleteOrgSecret)

					org.GET("/registries", api.GetOrgRegistryList)
					org.POST("/registries", api.PostOrgRegistry)
					org.GET("/registries/:registry", api.GetOrgRegistry)
					org.PATCH("/registries/:registry", api.PatchOrgRegistry)
					org.DELETE("/registries/:registry", api.DeleteOrgRegistry)

					if !server.Config.Agent.DisableUserRegisteredAgentRegistration {
						org.GET("/agents", api.GetOrgAgents)
						org.POST("/agents", api.PostOrgAgent)
						org.PATCH("/agents/:agent_id", api.PatchOrgAgent)
						org.DELETE("/agents/:agent_id", api.DeleteOrgAgent)
					}
				}
			}
		}

		// Admin entities endpoint (unified users and orgs)
		apiBase.GET("/entities", session.MustAdmin(), api.GetEntities)

		// User-accessible metrics endpoint (admins can see all, users see their repos)
		apiBase.GET("/metrics", session.MustUser(), api.GetMetrics)

		repo := apiBase.Group("/repos")
		{
			// Public endpoint - no authentication required
			repo.GET("/public", api.GetPublicRepos)
			repo.GET("/lookup/*repo_full_name", session.SetRepo(), session.SetPerm(), session.MustPull, api.LookupRepo)
			repo.POST("", session.MustUser(), api.PostRepo)
			repo.GET("", session.MustAdmin(), api.GetAllRepos)
			repo.POST("/repair", session.MustAdmin(), api.RepairAllRepos)
			repoBase := repo.Group("/:repo_id")
			{
				repoBase.Use(session.SetRepo())
				repoBase.Use(session.SetPerm())

				repoBase.GET("/permissions", api.GetRepoPermissions)

				repo := repoBase.Group("")
				{
					repo.Use(session.MustPull)

					repo.GET("", api.GetRepo)

					repo.GET("/branches", api.GetRepoBranches)
					repo.GET("/pull_requests", api.GetRepoPullRequests)
					repo.GET("/configs", api.GetRepoConfigs)

					repo.GET("/pipelines", api.GetPipelines)
					repo.POST("/pipelines", session.MustPush, api.CreatePipeline)
					repo.DELETE("/pipelines/:number", session.MustRepoAdmin(), api.DeletePipeline)
					repo.GET("/pipelines/:number", api.GetPipeline)
					repo.GET("/pipelines/:number/config", api.GetPipelineConfig)
					repo.GET("/pipelines/:number/metadata", session.MustPush, api.GetPipelineMetadata)

					// requires push permissions
					repo.POST("/pipelines/:number", session.MustPush, api.PostPipeline)
					repo.POST("/pipelines/:number/cancel", session.MustPush, api.CancelPipeline)
					repo.POST("/pipelines/:number/workflows/:workflow_id/cancel", session.MustPush, api.CancelWorkflow)
					repo.POST("/pipelines/:number/approve", session.MustPush, api.PostApproval)
					repo.POST("/pipelines/:number/decline", session.MustPush, api.PostDecline)

					repo.GET("/logs/:number/:stepId", api.GetStepLogs)
					repo.DELETE("/logs/:number/:stepId", session.MustPush, api.DeleteStepLogs)

					// requires push permissions
					repo.DELETE("/logs/:number", session.MustPush, api.DeletePipelineLogs)

					// requires push permissions
					repo.GET("/secrets", session.MustPush, api.GetSecretList)
					repo.POST("/secrets", session.MustPush, api.PostSecret)
					repo.GET("/secrets/:secret", session.MustPush, api.GetSecret)
					repo.GET("/secrets/:secret/abbreviated", session.MustPush, api.GetSecretAbbreviated)
					repo.PATCH("/secrets/:secret", session.MustPush, api.PatchSecret)
					repo.DELETE("/secrets/:secret", session.MustPush, api.DeleteSecret)

					// requires push permissions
					repo.GET("/registries", session.MustPush, api.GetRegistryList)
					repo.POST("/registries", session.MustPush, api.PostRegistry)
					repo.GET("/registries/:registry", session.MustPush, api.GetRegistry)
					repo.PATCH("/registries/:registry", session.MustPush, api.PatchRegistry)
					repo.DELETE("/registries/:registry", session.MustPush, api.DeleteRegistry)

					// requires push permissions
					repo.GET("/cron", session.MustPush, api.GetCronList)
					repo.POST("/cron", session.MustPush, api.PostCron)
					repo.GET("/cron/:cron", session.MustPush, api.GetCron)
					repo.POST("/cron/:cron", session.MustPush, api.RunCron)
					repo.PATCH("/cron/:cron", session.MustPush, api.PatchCron)
					repo.DELETE("/cron/:cron", session.MustPush, api.DeleteCron)

					// requires admin permissions
					repo.PATCH("", session.MustRepoAdmin(), api.PatchRepo)
					repo.DELETE("", session.MustRepoAdmin(), api.DeleteRepo)
					repo.POST("/chown", session.MustRepoAdmin(), api.ChownRepo)
					repo.POST("/repair", session.MustRepoAdmin(), api.RepairRepo)
					repo.POST("/move", session.MustRepoAdmin(), api.MoveRepo)

					// import settings from another repo
					repo.GET("/importable", session.MustRepoAdmin(), api.GetImportableRepos)
					repo.GET("/import-preview/:source_repo_id", session.MustRepoAdmin(), api.GetRepoForImport)
					repo.POST("/import-settings", session.MustRepoAdmin(), api.ImportSettings)
				}
			}
		}

		badges := apiBase.Group("/badges/:repo_id_or_owner")
		{
			badges.GET("/status.svg", api.GetBadge)
			badges.GET("/cc.xml", api.GetCC)
		}

		_badges := apiBase.Group("/badges/:repo_id_or_owner/:repo_name")
		{
			_badges.GET("/status.svg", api.GetBadge)
			_badges.GET("/cc.xml", api.GetCC)
		}

		pipelines := apiBase.Group("/pipelines")
		{
			pipelines.Use(session.MustAdmin())
			pipelines.GET("", api.GetPipelineQueue)
		}

		queue := apiBase.Group("/queue")
		{
			queue.Use(session.MustAdmin())
			queue.GET("/info", api.GetQueueInfo)
			queue.POST("/pause", api.PauseQueue)
			queue.POST("/resume", api.ResumeQueue)
			queue.GET("/norunningpipelines", api.BlockTilQueueHasRunningItem)
		}

		// global secrets can be read without actual values by any user
		readGlobalSecrets := apiBase.Group("/secrets")
		{
			readGlobalSecrets.Use(session.MustUser())
			readGlobalSecrets.GET("", api.GetGlobalSecretList)
			readGlobalSecrets.GET("/:secret", api.GetGlobalSecret)
		}
		secrets := apiBase.Group("/secrets")
		{
			secrets.Use(session.MustAdmin())
			secrets.GET("/:secret/abbreviated", api.GetGlobalSecretAbbreviated)
			secrets.POST("", api.PostGlobalSecret)
			secrets.PATCH("/:secret", api.PatchGlobalSecret)
			secrets.DELETE("/:secret", api.DeleteGlobalSecret)
		}

		// global registries can be read without actual values by any user
		readGlobalRegistries := apiBase.Group("/registries")
		{
			readGlobalRegistries.Use(session.MustUser())
			readGlobalRegistries.GET("", api.GetGlobalRegistryList)
			readGlobalRegistries.GET("/:registry", api.GetGlobalRegistry)
		}
		registries := apiBase.Group("/registries")
		{
			registries.Use(session.MustAdmin())
			registries.POST("", api.PostGlobalRegistry)
			registries.PATCH("/:registry", api.PatchGlobalRegistry)
			registries.DELETE("/:registry", api.DeleteGlobalRegistry)
		}

		logLevel := apiBase.Group("/log-level")
		{
			logLevel.Use(session.MustAdmin())
			logLevel.GET("", api.LogLevel)
			logLevel.POST("", api.SetLogLevel)
		}

		agentBase := apiBase.Group("/agents")
		{
			// Allow all authenticated users to view agents
			agentBase.GET("", session.MustUser(), api.GetAgents)
			agentBase.GET("/:agent_id", session.MustUser(), api.GetAgent)
			agentBase.GET("/:agent_id/tasks", session.MustUser(), api.GetAgentTasks)

			// Require admin privileges for agent modifications
			agentBase.POST("", session.MustAdmin(), api.PostAgent)
			agentBase.PATCH("/:agent_id", session.MustAdmin(), api.PatchAgent)
			agentBase.DELETE("/:agent_id", session.MustAdmin(), api.DeleteAgent)
		}

		// Autoscaler self endpoint - requires autoscaler token authentication
		apiBase.GET("/autoscaler", session.MustAutoscaler(), api.GetAutoscalerSelf)

		autoscalerBase := apiBase.Group("/autoscalers")
		{
			// Allow all authenticated users to view autoscalers
			autoscalerBase.GET("", session.MustUser(), api.GetAutoscalers)
			autoscalerBase.GET("/:autoscaler_id", session.MustUser(), api.GetAutoscaler)

			// Require admin privileges for autoscaler modifications
			autoscalerBase.POST("", session.MustAdmin(), api.PostAutoscaler)
			autoscalerBase.PATCH("/:autoscaler_id", session.MustAdmin(), api.PatchAutoscaler)
			autoscalerBase.DELETE("/:autoscaler_id", session.MustAdmin(), api.DeleteAutoscaler)
			autoscalerBase.POST("/:autoscaler_id/token", session.MustAdmin(), api.PostAutoscalerToken)

			// Heartbeat endpoint - TODO: add autoscaler token authentication middleware
			autoscalerBase.POST("/:autoscaler_id/heartbeat", api.PostAutoscalerHeartbeat)
		}

		apiBase.GET("/forges", api.GetForges)
		apiBase.GET("/forges/:forgeId", api.GetForge)
		forgeBase := apiBase.Group("/forges")
		{
			forgeBase.Use(session.MustAdmin())
			forgeBase.POST("", api.PostForge)
			forgeBase.PATCH("/:forgeId", api.PatchForge)
			forgeBase.DELETE("/:forgeId", api.DeleteForge)
		}

		apiBase.GET("/signature/public-key", session.MustUser(), api.GetSignaturePublicKey)

		// Webhook endpoint - new repos use this path
		apiBase.POST("/hook", api.PostHook)

		stream := apiBase.Group("/stream")
		{
			stream.GET("/logs/:repo_id/:pipeline/:stepId",
				session.SetRepo(),
				session.SetPerm(),
				session.MustPull,
				api.LogStreamSSE)
			stream.GET("/events", api.EventStreamSSE)
		}

		admin := apiBase.Group("/admin")
		{
			admin.Use(session.MustAdmin())

			// Maintenance endpoints
			maintenance := admin.Group("/maintenance")
			{
				maintenance.GET("/config", api.GetMaintenanceConfig)
				maintenance.PUT("/config", api.UpdateMaintenanceConfig)
				maintenance.GET("/config/:action", api.GetMaintenanceConfigByAction)
				maintenance.PUT("/config/:action", api.UpdateMaintenanceConfigByAction)
				maintenance.GET("/stats", api.GetMaintenanceStats)
				maintenance.GET("/logs", api.GetMaintenanceLogs)
				maintenance.POST("/run", api.RunMaintenanceNow)
			}
		}

		if zerolog.GlobalLevel() <= zerolog.DebugLevel {
			debugger := apiBase.Group("/debug")
			{
				debugger.Use(session.MustAdmin())
				debugger.GET("/pprof/", debug.IndexHandler())
				debugger.GET("/pprof/heap", debug.HeapHandler())
				debugger.GET("/pprof/goroutine", debug.GoroutineHandler())
				debugger.GET("/pprof/block", debug.BlockHandler())
				debugger.GET("/pprof/threadcreate", debug.ThreadCreateHandler())
				debugger.GET("/pprof/cmdline", debug.CmdlineHandler())
				debugger.GET("/pprof/profile", debug.ProfileHandler())
				debugger.GET("/pprof/symbol", debug.SymbolHandler())
				debugger.POST("/pprof/symbol", debug.SymbolHandler())
				debugger.GET("/pprof/trace", debug.TraceHandler())
			}
		}
	}
}
