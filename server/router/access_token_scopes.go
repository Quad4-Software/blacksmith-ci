// Copyright 2026 Crow Authors
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
	"strings"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
)

// Access token scope table.
//
// Every route served by the router must match one rule below, otherwise a
// request authenticated with an access token is denied (see
// session.EnforceAccessTokenScopes, and TestAccessTokenScopeCoverage which
// fails for routes without a rule). Rules are matched in order, the first one
// wins.
//
// The family of a route follows its guard:
//   - repository and organization routes need repo:*, where reads need
//     repo:read, changes need repo:write, and settings, secrets, registries,
//     org secrets/registries/agents and repo deletion, repair,
//     chown, move and import need repo:admin (also for reads);
//   - /user routes and instance-wide reads open to every signed-in user need user:*;
//   - routes guarded by an instance permission or admin need admin:*;
//   - public routes stay reachable, but run as an anonymous request.
//
// Watches of a repository are the exception: they only change the token
// owner's own subscriptions, so repo:read is enough also for changes.
//
// Tokens limited to an organization or a repository cannot use user or admin
// routes, which reach beyond that limit; reading the token owner's own profile
// is the only exception.
//
// For the user and admin families GET, HEAD and OPTIONS need the :read scope and
// all other methods the :write scope. A higher scope implies the lower ones of
// its family, see model.AccessTokenScope.Satisfies.

// The scopeRule maps the routes matching methods and path to a requirement.
type scopeRule struct {
	// The methods the rule applies to, nil for all methods.
	methods []string
	// The gin route pattern relative to the root path. A pattern ending in
	// "/*" matches the pattern without the suffix and everything below it.
	path string
	// The family scopes (read, write) picked by request method, used when
	// req.Kind is session.ScopeRequired and req.Scope is empty.
	read, write model.AccessTokenScope
	req         session.ScopeRequirement
	// allowLimited lets tokens limited to an org or a repo use a user or admin
	// family route, which is otherwise reserved for unlimited tokens.
	allowLimited bool
}

var (
	get      = []string{http.MethodGet}
	post     = []string{http.MethodPost}
	patchDel = []string{http.MethodPatch, http.MethodDelete}
	del      = []string{http.MethodDelete}
)

func publicRule(path string) scopeRule {
	return scopeRule{path: path, req: session.ScopeRequirement{Kind: session.ScopePublic}}
}

func fixedRule(methods []string, path string, scope model.AccessTokenScope) scopeRule {
	return scopeRule{methods: methods, path: path, req: session.ScopeRequirement{Kind: session.ScopeRequired, Scope: scope}}
}

func familyRule(path string, read, write model.AccessTokenScope) scopeRule {
	return scopeRule{path: path, read: read, write: write, req: session.ScopeRequirement{Kind: session.ScopeRequired}}
}

func repoRule(path string) scopeRule {
	return familyRule(path, model.ScopeRepoRead, model.ScopeRepoWrite)
}

func userRule(path string) scopeRule {
	return familyRule(path, model.ScopeUserRead, model.ScopeUserWrite)
}

func adminRule(path string) scopeRule {
	return familyRule(path, model.ScopeAdminRead, model.ScopeAdminWrite)
}

const v1 = "/api/v1"

var scopeRules = []scopeRule{
	// Routes outside the API, served without the token's identity.
	publicRule(""), // The web UI (NoRoute).
	publicRule("/web-config.js"),
	publicRule("/logout"),
	publicRule("/switch-account"),
	publicRule("/authorize/*"),
	publicRule("/metrics"),
	publicRule("/version"),
	publicRule("/healthz"),
	publicRule("/livez"),
	publicRule("/readyz"),
	publicRule("/api/hook"),

	// Public API routes.
	publicRule(v1 + "/openapi*"),
	publicRule(v1 + "/docs"),
	publicRule(v1 + "/hook"),
	publicRule(v1 + "/timezones"),
	publicRule(v1 + "/integrations/types"),
	publicRule(v1 + "/notifications/unsubscribe"),
	publicRule(v1 + "/badges/*"),
	publicRule(v1 + "/repos/public"),
	// Admins see forge secrets, so the user is kept only with admin:read.
	{methods: get, path: v1 + "/forges/*", req: session.ScopeRequirement{Kind: session.ScopePublic, Scope: model.ScopeAdminRead, Unrestricted: true}},
	// Authenticated with the autoscaler token, not with the user.
	publicRule(v1 + "/autoscaler"),
	publicRule(v1 + "/autoscalers/:autoscaler_id/heartbeat"),
	{methods: post, path: v1 + "/autoscalers/:autoscaler_id/logs", req: session.ScopeRequirement{Kind: session.ScopePublic}},
	// Authenticated with the agent token the host being provisioned was given.
	publicRule(v1 + "/agent/provisioning"),
	// Authenticated with the cache token of a running workflow, not with the user.
	publicRule(v1 + "/cache/*"),

	// User routes. The legacy API token is unscoped, so a scoped token must
	// not mint one.
	{path: v1 + "/user/token", req: session.ScopeRequirement{Kind: session.ScopeDenied}},
	{methods: get, path: v1 + "/user", req: session.ScopeRequirement{Kind: session.ScopeRequired, Scope: model.ScopeUserRead}, allowLimited: true},
	userRule(v1 + "/user/*"),

	// Instance-wide reads open to every signed-in user.
	fixedRule(get, v1+"/secrets", model.ScopeUserRead),
	fixedRule(get, v1+"/secrets/:secret", model.ScopeUserRead),
	fixedRule(get, v1+"/registries", model.ScopeUserRead),
	fixedRule(get, v1+"/registries/:registry", model.ScopeUserRead),
	fixedRule(get, v1+"/agents", model.ScopeUserRead),
	fixedRule(get, v1+"/agents/:agent_id", model.ScopeUserRead),
	fixedRule(get, v1+"/agents/:agent_id/tasks", model.ScopeUserRead),
	fixedRule(get, v1+"/agents/:agent_id/workflows", model.ScopeUserRead),
	fixedRule(get, v1+"/autoscalers", model.ScopeUserRead),
	fixedRule(get, v1+"/autoscalers/:autoscaler_id", model.ScopeUserRead),
	fixedRule(get, v1+"/signature/public-key", model.ScopeUserRead),
	// Instance-wide metrics additionally need admin:read, see api.GetMetrics.
	fixedRule(get, v1+"/metrics/*", model.ScopeUserRead),

	// Organizations.
	fixedRule(get, v1+"/orgs", model.ScopeAdminRead),
	fixedRule(del, v1+"/orgs/:org_id", model.ScopeAdminWrite),
	fixedRule(get, v1+"/orgs/lookup/*", model.ScopeRepoRead),
	fixedRule(get, v1+"/orgs/:org_id", model.ScopeRepoRead),
	fixedRule(get, v1+"/orgs/:org_id/permissions", model.ScopeRepoRead),
	fixedRule(nil, v1+"/orgs/:org_id/*", model.ScopeRepoAdmin),

	// Repositories.
	fixedRule(get, v1+"/repos", model.ScopeAdminRead),
	fixedRule(post, v1+"/repos/repair", model.ScopeAdminWrite),
	{methods: post, path: v1 + "/repos", req: session.ScopeRequirement{Kind: session.ScopeRequired, Scope: model.ScopeRepoAdmin, Unrestricted: true}},
	fixedRule(get, v1+"/repos/lookup/*", model.ScopeRepoRead),
	fixedRule(patchDel, v1+"/repos/:repo_id", model.ScopeRepoAdmin),
	fixedRule(post, v1+"/repos/:repo_id/chown", model.ScopeRepoAdmin),
	fixedRule(post, v1+"/repos/:repo_id/repair", model.ScopeRepoAdmin),
	fixedRule(post, v1+"/repos/:repo_id/move", model.ScopeRepoAdmin),
	fixedRule(del, v1+"/repos/:repo_id/pipelines/:number", model.ScopeRepoAdmin),
	fixedRule(nil, v1+"/repos/:repo_id/secrets/*", model.ScopeRepoAdmin),
	fixedRule(nil, v1+"/repos/:repo_id/registries/*", model.ScopeRepoAdmin),
	fixedRule(nil, v1+"/repos/:repo_id/caches/*", model.ScopeRepoAdmin),
	// Watches are the token owner's own subscriptions, so repo:read lets a
	// reader manage them; they change no repository state.
	fixedRule(nil, v1+"/repos/:repo_id/watches", model.ScopeRepoRead),
	fixedRule(nil, v1+"/repos/:repo_id/watches/*", model.ScopeRepoRead),
	fixedRule(nil, v1+"/repos/:repo_id/importable", model.ScopeRepoAdmin),
	fixedRule(nil, v1+"/repos/:repo_id/import-preview/*", model.ScopeRepoAdmin),
	fixedRule(nil, v1+"/repos/:repo_id/import-settings", model.ScopeRepoAdmin),
	repoRule(v1 + "/repos/:repo_id/*"),
	fixedRule(get, v1+"/stream/logs/:repo_id/:pipeline/:stepId", model.ScopeRepoRead),
	// Events are limited to the repositories the token admits, see api.EventStreamSSE.
	fixedRule(get, v1+"/stream/events", model.ScopeRepoRead),
	// Secret targets need admin:write on top for global secrets, see api.authorizeSecretAccess.
	fixedRule(nil, v1+"/secret-targets/*", model.ScopeRepoAdmin),

	// Routes guarded by an instance permission or admin.
	adminRule(v1 + "/users/*"),
	adminRule(v1 + "/entities"),
	adminRule(v1 + "/pipelines"),
	adminRule(v1 + "/queue/*"),
	adminRule(v1 + "/secrets/*"),
	adminRule(v1 + "/registries/*"),
	adminRule(v1 + "/agents/*"),
	adminRule(v1 + "/autoscalers/*"),
	adminRule(v1 + "/forges/*"),
	adminRule(v1 + "/log-level"),
	adminRule(v1 + "/debug/*"),
	adminRule(v1 + "/admin/*"),
}

func (r scopeRule) matches(method, path string) bool {
	if r.methods != nil && !containsMethod(r.methods, method) {
		return false
	}
	switch {
	case strings.HasSuffix(r.path, "/*"):
		prefix := strings.TrimSuffix(r.path, "/*")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	case strings.HasSuffix(r.path, "*"):
		return strings.HasPrefix(path, strings.TrimSuffix(r.path, "*"))
	default:
		return path == r.path
	}
}

func containsMethod(methods []string, method string) bool {
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// resolveAccessTokenScope returns the scope requirement of a route, see
// session.ScopeResolver.
func resolveAccessTokenScope(method, path string) (session.ScopeRequirement, bool) {
	for _, r := range scopeRules {
		if !r.matches(method, path) {
			continue
		}
		req := r.req
		if req.Kind == session.ScopeRequired && req.Scope == "" {
			req.Scope = r.write
			if isReadMethod(method) {
				req.Scope = r.read
			}
		}
		if req.Kind == session.ScopeRequired && !r.allowLimited && reachesBeyondLimit(req.Scope) {
			req.Unrestricted = true
		}
		return req, true
	}
	return session.ScopeRequirement{Kind: session.ScopeDenied}, false
}

// reachesBeyondLimit reports whether routes needing scope act on the whole
// user account or instance rather than on a single org or repo.
func reachesBeyondLimit(scope model.AccessTokenScope) bool {
	return scope.IsAdminScope() || scope == model.ScopeUserRead || scope == model.ScopeUserWrite
}
