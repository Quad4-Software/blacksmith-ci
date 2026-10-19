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

package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/shared/token"
	"codefloe.com/crowci/crow/v6/version"
)

type webIdentityProviderConfig struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
}

type webAuthConfig struct {
	OIDC webIdentityProviderConfig `json:"oidc"`
	LDAP webIdentityProviderConfig `json:"ldap"`
	// ManagedRoles are set from identity provider groups and cannot be assigned in the UI.
	ManagedRoles []string `json:"managed_roles"`
	ForgeLogin   bool     `json:"forge_login"`
	// ForgeCount is the number of forges linked by the signed-in user, nil when signed out.
	ForgeCount *int `json:"forge_count"`
}

func authConfig(c *gin.Context, user *model.User) webAuthConfig {
	cfg := webAuthConfig{
		ManagedRoles: api.ManagedRoles(),
		ForgeLogin:   !server.Config.Server.ForgeLoginDisabled,
	}
	if p := server.Config.Services.OIDC; p != nil {
		cfg.OIDC = webIdentityProviderConfig{Enabled: true, Name: p.DisplayName()}
	}
	if p := server.Config.Services.LDAP; p != nil {
		cfg.LDAP = webIdentityProviderConfig{Enabled: true, Name: p.DisplayName()}
	}
	if user != nil {
		forges, err := store.FromContext(c).UserForgeList(user.ID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot count forges of %s", user.Login)
		} else {
			n := len(forges)
			cfg.ForgeCount = &n
		}
	}
	return cfg
}

func Config(c *gin.Context) {
	user := session.User(c)

	var csrf string
	if user != nil {
		t := token.New(token.CsrfToken)
		t.Set("user-id", strconv.FormatInt(user.ID, 10))
		csrf, _ = t.Sign(user.Hash)
	}

	configData := map[string]any{
		"user":                   user,
		"accounts":               api.SwitchAccounts(c, user),
		"auth":                   authConfig(c, user),
		"csrf":                   csrf,
		"version":                version.String(),
		"build_date":             version.Date(),
		"skip_version_check":     server.Config.WebUI.SkipVersionCheck,
		"root_path":              server.Config.Server.RootPath,
		"enable_swagger":         server.Config.WebUI.EnableSwagger,
		"user_registered_agents": !server.Config.Agent.DisableUserRegisteredAgentRegistration,
		"scheduling_strategy":    server.Config.Queue.SchedulingStrategy,
		"custom_themes":          server.Config.WebUI.CustomThemes,
		"agent_protocol": map[string]int32{
			"version":   proto.Version,
			"min_agent": proto.MinAgentVersion,
		},
		"agent_auto_restart": map[string]any{
			"policy":       server.Config.Agent.AutoRestart,
			"max_parallel": server.Config.Agent.AutoRestartMaxParallel,
		},
	}

	// default func map with json parser.
	funcMap := template.FuncMap{
		"json": func(v any) string {
			a, err := json.Marshal(v)
			if err != nil {
				log.Error().Err(err).Msg("could not marshal JSON")
				return ""
			}
			return string(a)
		},
	}

	c.Header("Content-Type", "text/javascript; charset=utf-8")
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(configTemplate))

	if err := tmpl.Execute(c.Writer, configData); err != nil {
		log.Error().Err(err).Msg("could not execute template")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

const configTemplate = `
window.CROW_USER = {{ json .user }};
window.CROW_ACCOUNTS = {{ json .accounts }};
window.CROW_CSRF = "{{ .csrf }}";
window.CROW_VERSION = "{{ .version }}";
window.CROW_BUILD_DATE = "{{ .build_date }}";
window.CROW_ROOT_PATH = "{{ .root_path }}";
window.CROW_ENABLE_SWAGGER = {{ .enable_swagger }};
window.CROW_SKIP_VERSION_CHECK = {{ .skip_version_check }}
window.CROW_USER_REGISTERED_AGENTS = {{ .user_registered_agents }}
window.CROW_SCHEDULING_STRATEGY = {{ json .scheduling_strategy }};
window.CROW_CUSTOM_THEMES = {{ json .custom_themes }};
window.CROW_AGENT_PROTOCOL = {{ json .agent_protocol }};
window.CROW_AGENT_AUTO_RESTART = {{ json .agent_auto_restart }};
window.CROW_AUTH = {{ json .auth }};
`
