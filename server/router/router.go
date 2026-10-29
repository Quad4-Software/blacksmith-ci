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

package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/api/metrics"
	"codefloe.com/crowci/crow/v6/server/router/middleware/header"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/web"
	"codefloe.com/crowci/crow/v6/version"
)

// Load loads the router.
func Load(noRouteHandler http.HandlerFunc, middleware ...gin.HandlerFunc) http.Handler {
	e := gin.New()
	e.UseRawPath = true
	// Without trusted proxies gin keeps trusting every peer's forwarding headers.
	if proxies := server.Config.Server.TrustedProxies; len(proxies) > 0 {
		// Setup validates the entries, so an error here is unexpected.
		if err := e.SetTrustedProxies(proxies); err != nil {
			log.Error().Err(err).Strs("proxies", proxies).Msg("cannot set trusted proxies")
		}
	}
	e.Use(gin.Recovery())

	e.Use(func(c *gin.Context) {
		log.Trace().Msgf("[%s] %s", c.Request.Method, c.Request.URL.String())
		c.Next()
	})

	e.Use(header.NoCache)
	e.Use(header.Options)
	e.Use(header.Secure)
	e.Use(middleware...)
	e.Use(session.SetUser())
	e.Use(session.EnforceAccessTokenScopes(server.Config.Server.RootPath, resolveAccessTokenScope))

	// Unknown API paths get a JSON 404 instead of the web UI, whose 200 a client
	// would otherwise read as success.
	apiPrefix := server.Config.Server.RootPath + "/api/"
	webUI := gin.WrapF(noRouteHandler)
	e.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, apiPrefix) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"title": "Not Found", "status": http.StatusNotFound, "detail": "no such API route"})
			return
		}
		webUI(c)
	})

	base := e.Group(server.Config.Server.RootPath)
	{
		base.GET("/web-config.js", web.Config)

		base.GET("/logout", api.GetLogout)
		base.GET("/switch-account", api.SwitchAccount)
		auth := base.Group("/authorize")
		{
			auth.GET("", api.HandleAuth)
			auth.POST("", api.HandleAuth)
			auth.GET("/oidc", api.HandleOIDCAuth)
			auth.GET("/oidc/callback", api.HandleOIDCCallback)
			auth.POST("/ldap", api.HandleLDAPLogin)
		}

		base.GET("/metrics", metrics.PromHandler())
		base.GET("/version", api.Version)

		// Health check endpoints for HA load balancer integration
		base.GET("/healthz", api.GetHealth)     // Comprehensive health check
		base.GET("/livez", api.GetHealthLive)   // Liveness probe
		base.GET("/readyz", api.GetHealthReady) // Readiness probe
	}

	apiBase := base.Group("/api/v1")
	humaAPI := newHumaAPI(e, apiBase, "Blacksmith CI API", version.String())
	apiRoutes(base, apiBase, humaAPI)

	return e
}
