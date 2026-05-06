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
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/api"
	"codefloe.com/crowci/crow/v5/server/api/debug"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/router/middleware/token"
)

func apiRoutes(e *gin.RouterGroup, apiBase *gin.RouterGroup, humaAPI huma.API) {
	// Legacy hook endpoint for backwards compatibility with existing webhooks
	// New repos will use /api/v1/hook (see below)
	e.POST("/api/hook", api.PostHook)

	apiBase.Use(token.Refresh)
	apiBase.Use(session.SetAutoscaler())

	// ── User routes ──────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-self", Method: http.MethodGet, Path: "/user",
		Summary: "Get the currently authenticated user", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetSelf)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-feed", Method: http.MethodGet, Path: "/user/feed",
		Summary: "Get the currently authenticated user's pipeline feed", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetFeed)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-repos", Method: http.MethodGet, Path: "/user/repos",
		Summary: "Get the currently authenticated user's repositories",
		Description: "Returns active repos by default, or active+inactive when ?all=true. " +
			"The response shape changes based on the query parameter, so the OpenAPI schema " +
			"is untyped (Body any). Prefer GET /user/repos/active or GET /user/repos/available " +
			"for typed responses suitable for client codegen.",
		Tags: []string{"User"}, Deprecated: true,
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetRepos)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-repos-active", Method: http.MethodGet, Path: "/user/repos/active",
		Summary: "Get the user's active repositories with last pipeline info", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetActiveRepos)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-repos-available", Method: http.MethodGet, Path: "/user/repos/available",
		Summary: "Get repositories visible from linked forges (enabled and enableable)",
		Description: "Returns all repos the authenticated user can see across linked forges. " +
			"Already-enabled repos are flagged with active=true. Use ?forge_id= to scope to one " +
			"forge and avoid fanning out to others, or ?only_unenabled=true to omit already-enabled repos.",
		Tags:     []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAvailableRepos)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-orgs", Method: http.MethodGet, Path: "/user/orgs",
		Summary: "Get the currently authenticated user's organizations", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetUserOrgList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-secrets", Method: http.MethodGet, Path: "/user/secrets",
		Summary: "Get the currently authenticated user's secrets", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetUserSecretList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-registries", Method: http.MethodGet, Path: "/user/registries",
		Summary: "Get the currently authenticated user's registries", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetUserRegistryList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-crons", Method: http.MethodGet, Path: "/user/crons",
		Summary: "Get the currently authenticated user's cron jobs", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetUserCronList)

	// User forge connections (multi-forge support)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-forges", Method: http.MethodGet, Path: "/user/forges",
		Summary: "Get the currently authenticated user's forge connections", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetUserForges)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-available-forges", Method: http.MethodGet, Path: "/user/forges/available",
		Summary: "Get available forges for the current user", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAvailableForges)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-forge", Method: http.MethodGet, Path: "/user/forges/{forge_id}",
		Summary: "Get a specific user forge connection", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetUserForge)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user-forge-affected-repos", Method: http.MethodGet, Path: "/user/forges/{forge_id}/affected-repos",
		Summary: "Get repos affected by a user forge connection", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetUserForgeAffectedRepos)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-user-forge", Method: http.MethodDelete, Path: "/user/forges/{forge_id}",
		Summary: "Delete a user forge connection", Tags: []string{"User"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.DeleteUserForge)

	// Integrations (external secret stores)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-integrations", Method: http.MethodGet, Path: "/user/integrations",
		Summary: "Get integration list", Tags: []string{"Integrations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetIntegrationList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-accessible-integrations", Method: http.MethodGet, Path: "/user/integrations/accessible",
		Summary: "Get accessible integrations", Tags: []string{"Integrations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAccessibleIntegrations)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-integration", Method: http.MethodPost, Path: "/user/integrations",
		Summary: "Create a new integration", Tags: []string{"Integrations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PostIntegration)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-integration", Method: http.MethodGet, Path: "/user/integrations/{integration_id}",
		Summary: "Get an integration", Tags: []string{"Integrations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetIntegration)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-integration", Method: http.MethodPatch, Path: "/user/integrations/{integration_id}",
		Summary: "Update an integration", Tags: []string{"Integrations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PatchIntegration)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-integration", Method: http.MethodDelete, Path: "/user/integrations/{integration_id}",
		Summary: "Delete an integration", Tags: []string{"Integrations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.DeleteIntegration)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "test-integration", Method: http.MethodPost, Path: "/user/integrations/{integration_id}/test",
		Summary: "Test an integration", Tags: []string{"Integrations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.TestIntegration)

	// Access tokens (new API)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-access-tokens", Method: http.MethodGet, Path: "/user/access-tokens",
		Summary: "List access tokens", Tags: []string{"Access Tokens"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAccessTokenList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-access-token", Method: http.MethodPost, Path: "/user/access-tokens",
		Summary: "Create an access token", Tags: []string{"Access Tokens"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PostAccessToken)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-access-tokens-batch", Method: http.MethodDelete, Path: "/user/access-tokens/batch",
		Summary: "Delete multiple access tokens", Tags: []string{"Access Tokens"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.DeleteAccessTokenBatch)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-available-scopes", Method: http.MethodGet, Path: "/user/access-tokens/scopes",
		Summary: "Get available token scopes", Tags: []string{"Access Tokens"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAvailableScopes)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-access-token", Method: http.MethodGet, Path: "/user/access-tokens/{token_id}",
		Summary: "Get an access token", Tags: []string{"Access Tokens"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAccessToken)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-access-token", Method: http.MethodPatch, Path: "/user/access-tokens/{token_id}",
		Summary: "Update an access token", Tags: []string{"Access Tokens"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PatchAccessToken)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-access-token", Method: http.MethodDelete, Path: "/user/access-tokens/{token_id}",
		Summary: "Delete an access token", Tags: []string{"Access Tokens"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.DeleteAccessToken)

	// Legacy token endpoints (deprecated - use access-tokens instead)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-token", Method: http.MethodPost, Path: "/user/token",
		Summary: "Create a legacy token", Tags: []string{"User"}, Deprecated: true,
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PostToken)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-token", Method: http.MethodDelete, Path: "/user/token",
		Summary: "Delete a legacy token", Tags: []string{"User"}, Deprecated: true,
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.DeleteToken)

	// ── Admin user management ────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-users", Method: http.MethodGet, Path: "/users",
		Summary: "List users", Tags: []string{"Users"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetUsers)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-user", Method: http.MethodPost, Path: "/users",
		Summary: "Create a user", Tags: []string{"Users"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostUser)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-user", Method: http.MethodGet, Path: "/users/{login}",
		Summary: "Get a user", Tags: []string{"Users"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetUser)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-user", Method: http.MethodPatch, Path: "/users/{login}",
		Summary: "Update a user", Tags: []string{"Users"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PatchUser)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-user", Method: http.MethodDelete, Path: "/users/{login}",
		Summary: "Delete a user", Tags: []string{"Users"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.DeleteUser)

	// ── Public endpoints ─────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-timezones", Method: http.MethodGet, Path: "/timezones",
		Summary: "List available timezones", Tags: []string{"System"},
	}, api.GetTimezones)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-integration-types", Method: http.MethodGet, Path: "/integrations/types",
		Summary: "List available integration types", Tags: []string{"Integrations"},
	}, api.GetIntegrationTypes)

	// ── Organizations ────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-orgs", Method: http.MethodGet, Path: "/orgs",
		Summary: "List organizations", Tags: []string{"Organizations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetOrgs)
	// LookupOrg uses a wildcard path param (org name may contain slashes)
	apiBase.GET("/orgs/lookup/*org_full_name", api.LookupOrgGin)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-org-permissions", Method: http.MethodGet, Path: "/orgs/{org_id}/permissions",
		Summary: "Get organization permissions for the current user", Tags: []string{"Organizations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg},
	}, api.GetOrgPermissions)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-org", Method: http.MethodGet, Path: "/orgs/{org_id}",
		Summary: "Get an organization", Tags: []string{"Organizations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(false)},
	}, api.GetOrg)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-org", Method: http.MethodDelete, Path: "/orgs/{org_id}",
		Summary: "Delete an organization", Tags: []string{"Organizations"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin, hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.DeleteOrg)

	// Org secrets
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-org-secrets", Method: http.MethodGet, Path: "/orgs/{org_id}/secrets",
		Summary: "List organization secrets", Tags: []string{"Organization Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.GetOrgSecretList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-org-secret", Method: http.MethodPost, Path: "/orgs/{org_id}/secrets",
		Summary: "Create an organization secret", Tags: []string{"Organization Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.PostOrgSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-org-secret", Method: http.MethodGet, Path: "/orgs/{org_id}/secrets/{secret}",
		Summary: "Get an organization secret", Tags: []string{"Organization Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.GetOrgSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-org-secret-abbreviated", Method: http.MethodGet, Path: "/orgs/{org_id}/secrets/{secret}/abbreviated",
		Summary: "Get an abbreviated organization secret", Tags: []string{"Organization Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.GetOrgSecretAbbreviated)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-org-secret", Method: http.MethodPatch, Path: "/orgs/{org_id}/secrets/{secret}",
		Summary: "Update an organization secret", Tags: []string{"Organization Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.PatchOrgSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-org-secret", Method: http.MethodDelete, Path: "/orgs/{org_id}/secrets/{secret}",
		Summary: "Delete an organization secret", Tags: []string{"Organization Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.DeleteOrgSecret)

	// Org registries
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-org-registries", Method: http.MethodGet, Path: "/orgs/{org_id}/registries",
		Summary: "List organization registries", Tags: []string{"Organization Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.GetOrgRegistryList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-org-registry", Method: http.MethodPost, Path: "/orgs/{org_id}/registries",
		Summary: "Create an organization registry", Tags: []string{"Organization Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.PostOrgRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-org-registry", Method: http.MethodGet, Path: "/orgs/{org_id}/registries/{registry}",
		Summary: "Get an organization registry", Tags: []string{"Organization Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.GetOrgRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-org-registry", Method: http.MethodPatch, Path: "/orgs/{org_id}/registries/{registry}",
		Summary: "Update an organization registry", Tags: []string{"Organization Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.PatchOrgRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-org-registry", Method: http.MethodDelete, Path: "/orgs/{org_id}/registries/{registry}",
		Summary: "Delete an organization registry", Tags: []string{"Organization Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
	}, api.DeleteOrgRegistry)

	// Org agents
	if !server.Config.Agent.DisableUserRegisteredAgentRegistration {
		huma.Register(humaAPI, huma.Operation{
			OperationID: "get-org-agents", Method: http.MethodGet, Path: "/orgs/{org_id}/agents",
			Summary: "List agents for an organization", Tags: []string{"Agents"},
			Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser, hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
		}, api.GetOrgAgents)
		huma.Register(humaAPI, huma.Operation{
			OperationID: "post-org-agent", Method: http.MethodPost, Path: "/orgs/{org_id}/agents",
			Summary: "Create a new organization-scoped agent", Tags: []string{"Agents"},
			Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser, hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
		}, api.PostOrgAgent)
		huma.Register(humaAPI, huma.Operation{
			OperationID: "patch-org-agent", Method: http.MethodPatch, Path: "/orgs/{org_id}/agents/{agent_id}",
			Summary: "Update an organization-scoped agent", Tags: []string{"Agents"},
			Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser, hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
		}, api.PatchOrgAgent)
		huma.Register(humaAPI, huma.Operation{
			OperationID: "delete-org-agent", Method: http.MethodDelete, Path: "/orgs/{org_id}/agents/{agent_id}",
			Summary: "Delete an organization-scoped agent", Tags: []string{"Agents"},
			Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser, hmSetOrg, hmMustOrg, hmMustOrgMember(true)},
		}, api.DeleteOrgAgent)
	}

	// ── Admin entities (unified users and orgs) ──────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-entities", Method: http.MethodGet, Path: "/entities",
		Summary: "List users and organizations", Tags: []string{"Admin"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetEntities)

	// ── Metrics ──────────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-metrics", Method: http.MethodGet, Path: "/metrics",
		Summary: "Get pipeline metrics", Tags: []string{"System"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetMetrics)

	// ── Repositories ─────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-public-repos", Method: http.MethodGet, Path: "/repos/public",
		Summary: "List public repositories", Tags: []string{"Repositories"},
	}, api.GetPublicRepos)
	// LookupRepo uses a wildcard path param (owner/repo contains a slash)
	apiBase.GET("/repos/lookup/*repo_full_name", session.SetRepo(), session.SetPerm(), session.MustPull, api.LookupRepoGin)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-repo", Method: http.MethodPost, Path: "/repos",
		Summary: "Activate a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PostRepo)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-all-repos", Method: http.MethodGet, Path: "/repos",
		Summary: "List all repositories (admin)", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetAllRepos)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "repair-all-repos", Method: http.MethodPost, Path: "/repos/repair",
		Summary: "Repair all repositories (admin)", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.RepairAllRepos)

	// Repo detail routes (require SetRepo + SetPerm)
	repoMW := huma.Middlewares{hmSetRepo, hmSetPerm}
	repoPullMW := huma.Middlewares{hmSetRepo, hmSetPerm, hmMustPull}
	repoPushMW := huma.Middlewares{hmSetRepo, hmSetPerm, hmMustPull, hmMustPush}
	repoAdminMW := huma.Middlewares{hmSetRepo, hmSetPerm, hmMustPull, hmMustRepoAdmin}

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-permissions", Method: http.MethodGet, Path: "/repos/{repo_id}/permissions",
		Summary: "Get repo permissions for the current user", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoMW,
	}, api.GetRepoPermissions)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo", Method: http.MethodGet, Path: "/repos/{repo_id}",
		Summary: "Get a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetRepo)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-branches", Method: http.MethodGet, Path: "/repos/{repo_id}/branches",
		Summary: "List branches for a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetRepoBranches)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-pull-requests", Method: http.MethodGet, Path: "/repos/{repo_id}/pull_requests",
		Summary: "List pull requests for a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetRepoPullRequests)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-configs", Method: http.MethodGet, Path: "/repos/{repo_id}/configs",
		Summary: "List pipeline configs for a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetRepoConfigs)

	// Pipelines
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-pipelines", Method: http.MethodGet, Path: "/repos/{repo_id}/pipelines",
		Summary: "List pipelines for a repository", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetPipelines)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-workflow-names", Method: http.MethodGet, Path: "/repos/{repo_id}/workflows/names",
		Summary: "List distinct workflow names for a repository", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetWorkflowNames)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "create-pipeline", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines",
		Summary: "Create a new pipeline", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.CreatePipeline)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-pipeline", Method: http.MethodDelete, Path: "/repos/{repo_id}/pipelines/{number}",
		Summary: "Delete a pipeline", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.DeletePipeline)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-pipeline", Method: http.MethodGet, Path: "/repos/{repo_id}/pipelines/{number}",
		Summary: "Get a pipeline", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetPipeline)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-pipeline-config", Method: http.MethodGet, Path: "/repos/{repo_id}/pipelines/{number}/config",
		Summary: "Get pipeline configuration", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetPipelineConfig)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-pipeline-metadata", Method: http.MethodGet, Path: "/repos/{repo_id}/pipelines/{number}/metadata",
		Summary: "Get pipeline metadata", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetPipelineMetadata)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-pipeline", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}",
		Summary: "Restart a pipeline", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PostPipeline)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "cancel-pipeline", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}/cancel",
		Summary: "Cancel a pipeline", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.CancelPipeline)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "cancel-workflow", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}/workflows/{workflow_id}/cancel",
		Summary: "Cancel a workflow", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.CancelWorkflow)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "rerun-workflow", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}/workflows/{workflow_id}/rerun",
		Summary: "Rerun a workflow", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.RerunWorkflow)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "rerun-step", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}/workflows/{workflow_id}/steps/{step_id}/rerun",
		Summary: "Rerun a step", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.RerunStep)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-approval", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}/approve",
		Summary: "Approve a pipeline", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PostApproval)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-approval-always", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}/approve-always",
		Summary: "Approve a pipeline and always approve future pipelines from this context", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PostApprovalAlways)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-approval-always", Method: http.MethodDelete, Path: "/repos/{repo_id}/pipelines/{number}/approve-always",
		Summary: "Delete an always-approve rule", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.DeleteApprovalAlways)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-decline", Method: http.MethodPost, Path: "/repos/{repo_id}/pipelines/{number}/decline",
		Summary: "Decline a pipeline", Tags: []string{"Pipelines"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PostDecline)

	// Logs
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-step-logs", Method: http.MethodGet, Path: "/repos/{repo_id}/logs/{number}/{stepId}",
		Summary: "Get step logs", Tags: []string{"Pipeline Logs"},
		Security: BearerSecurity, Middlewares: repoPullMW,
	}, api.GetStepLogs)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-step-logs", Method: http.MethodDelete, Path: "/repos/{repo_id}/logs/{number}/{stepId}",
		Summary: "Delete step logs", Tags: []string{"Pipeline Logs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.DeleteStepLogs)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-pipeline-logs", Method: http.MethodDelete, Path: "/repos/{repo_id}/logs/{number}",
		Summary: "Delete all logs for a pipeline", Tags: []string{"Pipeline Logs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.DeletePipelineLogs)

	// Repo secrets
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-secrets", Method: http.MethodGet, Path: "/repos/{repo_id}/secrets",
		Summary: "List repository secrets", Tags: []string{"Repository Secrets"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetSecretList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-repo-secret", Method: http.MethodPost, Path: "/repos/{repo_id}/secrets",
		Summary: "Create a repository secret", Tags: []string{"Repository Secrets"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PostSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-secret", Method: http.MethodGet, Path: "/repos/{repo_id}/secrets/{secret}",
		Summary: "Get a repository secret", Tags: []string{"Repository Secrets"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-secret-abbreviated", Method: http.MethodGet, Path: "/repos/{repo_id}/secrets/{secret}/abbreviated",
		Summary: "Get an abbreviated repository secret", Tags: []string{"Repository Secrets"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetSecretAbbreviated)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-repo-secret", Method: http.MethodPatch, Path: "/repos/{repo_id}/secrets/{secret}",
		Summary: "Update a repository secret", Tags: []string{"Repository Secrets"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PatchSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-repo-secret", Method: http.MethodDelete, Path: "/repos/{repo_id}/secrets/{secret}",
		Summary: "Delete a repository secret", Tags: []string{"Repository Secrets"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.DeleteSecret)

	// Repo registries
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-registries", Method: http.MethodGet, Path: "/repos/{repo_id}/registries",
		Summary: "List repository registries", Tags: []string{"Repository Registries"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetRegistryList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-repo-registry", Method: http.MethodPost, Path: "/repos/{repo_id}/registries",
		Summary: "Create a repository registry", Tags: []string{"Repository Registries"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PostRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-registry", Method: http.MethodGet, Path: "/repos/{repo_id}/registries/{registry}",
		Summary: "Get a repository registry", Tags: []string{"Repository Registries"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-repo-registry", Method: http.MethodPatch, Path: "/repos/{repo_id}/registries/{registry}",
		Summary: "Update a repository registry", Tags: []string{"Repository Registries"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PatchRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-repo-registry", Method: http.MethodDelete, Path: "/repos/{repo_id}/registries/{registry}",
		Summary: "Delete a repository registry", Tags: []string{"Repository Registries"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.DeleteRegistry)

	// Repo cron jobs
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-crons", Method: http.MethodGet, Path: "/repos/{repo_id}/cron",
		Summary: "List repository cron jobs", Tags: []string{"Repository Cron Jobs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetCronList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-repo-cron", Method: http.MethodPost, Path: "/repos/{repo_id}/cron",
		Summary: "Create a repository cron job", Tags: []string{"Repository Cron Jobs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PostCron)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-cron", Method: http.MethodGet, Path: "/repos/{repo_id}/cron/{cron}",
		Summary: "Get a repository cron job", Tags: []string{"Repository Cron Jobs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.GetCron)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "run-repo-cron", Method: http.MethodPost, Path: "/repos/{repo_id}/cron/{cron}",
		Summary: "Run a repository cron job", Tags: []string{"Repository Cron Jobs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.RunCron)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-repo-cron", Method: http.MethodPatch, Path: "/repos/{repo_id}/cron/{cron}",
		Summary: "Update a repository cron job", Tags: []string{"Repository Cron Jobs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.PatchCron)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-repo-cron", Method: http.MethodDelete, Path: "/repos/{repo_id}/cron/{cron}",
		Summary: "Delete a repository cron job", Tags: []string{"Repository Cron Jobs"},
		Security: BearerSecurity, Middlewares: repoPushMW,
	}, api.DeleteCron)

	// Repo notification config
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-notification-config", Method: http.MethodGet, Path: "/repos/{repo_id}/notification",
		Summary: "Get repository notification config", Tags: []string{"Repository Notifications"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.GetRepoNotificationConfig)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "put-repo-notification-config", Method: http.MethodPut, Path: "/repos/{repo_id}/notification",
		Summary: "Create or update repository notification config", Tags: []string{"Repository Notifications"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.PutRepoNotificationConfig)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-repo-notification-config", Method: http.MethodDelete, Path: "/repos/{repo_id}/notification",
		Summary: "Delete repository notification config", Tags: []string{"Repository Notifications"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.DeleteRepoNotificationConfig)

	// Repo admin operations
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-repo", Method: http.MethodPatch, Path: "/repos/{repo_id}",
		Summary: "Update a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.PatchRepo)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-repo", Method: http.MethodDelete, Path: "/repos/{repo_id}",
		Summary: "Delete / deactivate a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.DeleteRepo)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "chown-repo", Method: http.MethodPost, Path: "/repos/{repo_id}/chown",
		Summary: "Change repository owner", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.ChownRepo)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "repair-repo", Method: http.MethodPost, Path: "/repos/{repo_id}/repair",
		Summary: "Repair a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.RepairRepo)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "move-repo", Method: http.MethodPost, Path: "/repos/{repo_id}/move",
		Summary: "Move a repository", Tags: []string{"Repositories"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.MoveRepo)

	// Import settings from another repo
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-importable-repos", Method: http.MethodGet, Path: "/repos/{repo_id}/importable",
		Summary: "List importable repos", Tags: []string{"Repository Import"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.GetImportableRepos)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-repo-for-import", Method: http.MethodGet, Path: "/repos/{repo_id}/import-preview/{source_repo_id}",
		Summary: "Preview settings to import from another repo", Tags: []string{"Repository Import"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.GetRepoForImport)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "import-settings", Method: http.MethodPost, Path: "/repos/{repo_id}/import-settings",
		Summary: "Import settings from another repository", Tags: []string{"Repository Import"},
		Security: BearerSecurity, Middlewares: repoAdminMW,
	}, api.ImportSettings)

	// ── Badges ───────────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-badge", Method: http.MethodGet, Path: "/badges/{repo_id_or_owner}/status.svg",
		Summary: "Get a pipeline status badge", Tags: []string{"Badges"},
	}, api.GetBadge)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-cc", Method: http.MethodGet, Path: "/badges/{repo_id_or_owner}/cc.xml",
		Summary: "Get CCMenu/CCTray XML", Tags: []string{"Badges"},
	}, api.GetCC)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-badge-by-name", Method: http.MethodGet, Path: "/badges/{repo_id_or_owner}/{repo_name}/status.svg",
		Summary: "Get a pipeline status badge by owner/name", Tags: []string{"Badges"},
	}, api.GetBadgeByName)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-cc-by-name", Method: http.MethodGet, Path: "/badges/{repo_id_or_owner}/{repo_name}/cc.xml",
		Summary: "Get CCMenu/CCTray XML by owner/name", Tags: []string{"Badges"},
	}, api.GetCCByName)

	// ── Email unsubscribe (public, token-verified) ─────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-unsubscribe", Method: http.MethodGet, Path: "/notifications/unsubscribe",
		Summary: "Unsubscribe from email notifications", Tags: []string{"Notifications"},
	}, api.GetUnsubscribe)

	// ── Pipeline queue (admin) ───────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-pipeline-queue", Method: http.MethodGet, Path: "/pipelines",
		Summary: "Get the pipeline queue", Tags: []string{"Pipeline Queue"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetPipelineQueue)

	// Queue management (gin - uses blocking/streaming patterns)
	queue := apiBase.Group("/queue")
	{
		queue.Use(session.MustAdmin())
		queue.GET("/info", api.GetQueueInfo)
		queue.POST("/pause", api.PauseQueue)
		queue.POST("/resume", api.ResumeQueue)
		queue.GET("/norunningpipelines", api.BlockTilQueueHasRunningItem)
	}

	// ── Global secrets ───────────────────────────────────────────────────
	// Readable by any authenticated user (without actual values)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-global-secrets", Method: http.MethodGet, Path: "/secrets",
		Summary: "List global secrets", Tags: []string{"Global Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetGlobalSecretList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-global-secret", Method: http.MethodGet, Path: "/secrets/{secret}",
		Summary: "Get a global secret", Tags: []string{"Global Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetGlobalSecret)
	// Admin-only
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-global-secret-abbreviated", Method: http.MethodGet, Path: "/secrets/{secret}/abbreviated",
		Summary: "Get an abbreviated global secret", Tags: []string{"Global Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetGlobalSecretAbbreviated)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-global-secret", Method: http.MethodPost, Path: "/secrets",
		Summary: "Create a global secret", Tags: []string{"Global Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostGlobalSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-global-secret", Method: http.MethodPatch, Path: "/secrets/{secret}",
		Summary: "Update a global secret", Tags: []string{"Global Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PatchGlobalSecret)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-global-secret", Method: http.MethodDelete, Path: "/secrets/{secret}",
		Summary: "Delete a global secret", Tags: []string{"Global Secrets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.DeleteGlobalSecret)

	// ── Secret targets ───────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-secret-targets", Method: http.MethodGet, Path: "/secret-targets/{secret_id}/targets",
		Summary: "List secret targets", Tags: []string{"Secret Targets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetSecretTargetList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-secret-target", Method: http.MethodPost, Path: "/secret-targets/{secret_id}/targets",
		Summary: "Add a secret target", Tags: []string{"Secret Targets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PostSecretTarget)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-secret-target", Method: http.MethodDelete, Path: "/secret-targets/{secret_id}/targets/{target_id}",
		Summary: "Remove a secret target", Tags: []string{"Secret Targets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.DeleteSecretTarget)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-secret-owner", Method: http.MethodPatch, Path: "/secret-targets/{secret_id}/owner",
		Summary: "Transfer secret ownership", Tags: []string{"Secret Targets"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.PatchSecretOwner)

	// ── Global registries ────────────────────────────────────────────────
	// Readable by any authenticated user (without actual values)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-global-registries", Method: http.MethodGet, Path: "/registries",
		Summary: "List global registries", Tags: []string{"Global Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetGlobalRegistryList)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-global-registry", Method: http.MethodGet, Path: "/registries/{registry}",
		Summary: "Get a global registry", Tags: []string{"Global Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetGlobalRegistry)
	// Admin-only
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-global-registry", Method: http.MethodPost, Path: "/registries",
		Summary: "Create a global registry", Tags: []string{"Global Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostGlobalRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-global-registry", Method: http.MethodPatch, Path: "/registries/{registry}",
		Summary: "Update a global registry", Tags: []string{"Global Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PatchGlobalRegistry)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-global-registry", Method: http.MethodDelete, Path: "/registries/{registry}",
		Summary: "Delete a global registry", Tags: []string{"Global Registries"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.DeleteGlobalRegistry)

	// ── Log level (admin) ────────────────────────────────────────────────
	logLevel := apiBase.Group("/log-level")
	{
		logLevel.Use(session.MustAdmin())
		logLevel.GET("", api.LogLevel)
		logLevel.POST("", api.SetLogLevel)
	}

	// ── Agents ───────────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-agents", Method: http.MethodGet, Path: "/agents",
		Summary: "List agents", Tags: []string{"Agents"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAgents)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-agent", Method: http.MethodGet, Path: "/agents/{agent_id}",
		Summary: "Get an agent", Tags: []string{"Agents"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAgent)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-agent-tasks", Method: http.MethodGet, Path: "/agents/{agent_id}/tasks",
		Summary: "List agent tasks", Tags: []string{"Agents"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAgentTasks)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-agent", Method: http.MethodPost, Path: "/agents",
		Summary: "Create a new agent", Tags: []string{"Agents"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostAgent)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-agent", Method: http.MethodPatch, Path: "/agents/{agent_id}",
		Summary: "Update an agent", Tags: []string{"Agents"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PatchAgent)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-agent", Method: http.MethodDelete, Path: "/agents/{agent_id}",
		Summary: "Delete an agent", Tags: []string{"Agents"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.DeleteAgent)

	// ── Autoscalers ──────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-autoscaler-self", Method: http.MethodGet, Path: "/autoscaler",
		Summary: "Get the currently authenticated autoscaler", Tags: []string{"Autoscalers"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAutoscaler},
	}, api.GetAutoscalerSelf)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-autoscalers", Method: http.MethodGet, Path: "/autoscalers",
		Summary: "List autoscalers", Tags: []string{"Autoscalers"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAutoscalers)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-autoscaler", Method: http.MethodGet, Path: "/autoscalers/{autoscaler_id}",
		Summary: "Get an autoscaler", Tags: []string{"Autoscalers"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetAutoscaler)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-autoscaler", Method: http.MethodPost, Path: "/autoscalers",
		Summary: "Create an autoscaler", Tags: []string{"Autoscalers"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostAutoscaler)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-autoscaler", Method: http.MethodPatch, Path: "/autoscalers/{autoscaler_id}",
		Summary: "Update an autoscaler", Tags: []string{"Autoscalers"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PatchAutoscaler)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-autoscaler", Method: http.MethodDelete, Path: "/autoscalers/{autoscaler_id}",
		Summary: "Delete an autoscaler", Tags: []string{"Autoscalers"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.DeleteAutoscaler)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-autoscaler-token", Method: http.MethodPost, Path: "/autoscalers/{autoscaler_id}/token",
		Summary: "Generate a new autoscaler token", Tags: []string{"Autoscalers"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostAutoscalerToken)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-autoscaler-heartbeat", Method: http.MethodPost, Path: "/autoscalers/{autoscaler_id}/heartbeat",
		Summary: "Send an autoscaler heartbeat", Tags: []string{"Autoscalers"},
	}, api.PostAutoscalerHeartbeat)

	// ── Forges ───────────────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-forges", Method: http.MethodGet, Path: "/forges",
		Summary: "List forges", Tags: []string{"Forges"},
	}, api.GetForges)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-forge", Method: http.MethodGet, Path: "/forges/{forgeId}",
		Summary: "Get a forge", Tags: []string{"Forges"},
	}, api.GetForge)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-forge", Method: http.MethodPost, Path: "/forges",
		Summary: "Create a forge", Tags: []string{"Forges"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostForge)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "patch-forge", Method: http.MethodPatch, Path: "/forges/{forgeId}",
		Summary: "Update a forge", Tags: []string{"Forges"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PatchForge)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "delete-forge", Method: http.MethodDelete, Path: "/forges/{forgeId}",
		Summary: "Delete a forge", Tags: []string{"Forges"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.DeleteForge)

	// ── Signature public key ─────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-signature-public-key", Method: http.MethodGet, Path: "/signature/public-key",
		Summary: "Get the pipeline signature public key", Tags: []string{"System"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustUser},
	}, api.GetSignaturePublicKey)

	// ── Webhook ──────────────────────────────────────────────────────────
	apiBase.POST("/hook", api.PostHook)

	// ── SSE streams (gin - not convertible to huma) ──────────────────────
	stream := apiBase.Group("/stream")
	{
		stream.GET("/logs/:repo_id/:pipeline/:stepId",
			session.SetRepo(),
			session.SetPerm(),
			session.MustPull,
			api.LogStreamSSE)
		stream.GET("/events", api.EventStreamSSE)
	}

	// ── Admin maintenance ────────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-maintenance-config", Method: http.MethodGet, Path: "/admin/maintenance/config",
		Summary: "Get maintenance configuration", Tags: []string{"Admin Maintenance"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetMaintenanceConfig)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "update-maintenance-config", Method: http.MethodPut, Path: "/admin/maintenance/config",
		Summary: "Update maintenance configuration", Tags: []string{"Admin Maintenance"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.UpdateMaintenanceConfig)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-maintenance-config-by-action", Method: http.MethodGet, Path: "/admin/maintenance/config/{action}",
		Summary: "Get maintenance configuration for a specific action", Tags: []string{"Admin Maintenance"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetMaintenanceConfigByAction)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "update-maintenance-config-by-action", Method: http.MethodPut, Path: "/admin/maintenance/config/{action}",
		Summary: "Update maintenance configuration for a specific action", Tags: []string{"Admin Maintenance"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.UpdateMaintenanceConfigByAction)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-maintenance-stats", Method: http.MethodGet, Path: "/admin/maintenance/stats",
		Summary: "Get maintenance statistics", Tags: []string{"Admin Maintenance"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetMaintenanceStats)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-maintenance-logs", Method: http.MethodGet, Path: "/admin/maintenance/logs",
		Summary: "Get maintenance logs", Tags: []string{"Admin Maintenance"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetMaintenanceLogs)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "run-maintenance-now", Method: http.MethodPost, Path: "/admin/maintenance/run",
		Summary: "Run maintenance now", Tags: []string{"Admin Maintenance"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.RunMaintenanceNow)

	// ── Admin Notifications ──────────────────────────────────────────────
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-admin-smtp-status", Method: http.MethodGet, Path: "/admin/notifications/smtp",
		Summary: "Get SMTP notification configuration status", Tags: []string{"Admin Notifications"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.GetAdminSMTPStatus)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-admin-smtp-test", Method: http.MethodPost, Path: "/admin/notifications/test",
		Summary: "Send a test notification email", Tags: []string{"Admin Notifications"},
		Security: BearerSecurity, Middlewares: huma.Middlewares{hmMustAdmin},
	}, api.PostAdminTestSMTP)

	// ── Debug (only at debug log level) ──────────────────────────────────
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
