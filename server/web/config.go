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

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/shared/token"
	"codefloe.com/crowci/crow/v5/version"
)

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
		"csrf":                   csrf,
		"version":                version.String(),
		"build_date":             version.Date(),
		"skip_version_check":     server.Config.WebUI.SkipVersionCheck,
		"root_path":              server.Config.Server.RootPath,
		"enable_swagger":         server.Config.WebUI.EnableSwagger,
		"user_registered_agents": !server.Config.Agent.DisableUserRegisteredAgentRegistration,
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
window.CROW_CSRF = "{{ .csrf }}";
window.CROW_VERSION = "{{ .version }}";
window.CROW_BUILD_DATE = "{{ .build_date }}";
window.CROW_ROOT_PATH = "{{ .root_path }}";
window.CROW_ENABLE_SWAGGER = {{ .enable_swagger }};
window.CROW_SKIP_VERSION_CHECK = {{ .skip_version_check }}
window.CROW_USER_REGISTERED_AGENTS = {{ .user_registered_agents }}
`
