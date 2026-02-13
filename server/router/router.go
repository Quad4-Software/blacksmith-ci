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
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	openapi_files "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"codefloe.com/crowci/crow/v5/cmd/server/openapi"
	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/api"
	"codefloe.com/crowci/crow/v5/server/api/metrics"
	"codefloe.com/crowci/crow/v5/server/router/middleware/header"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/web"
)

// Load loads the router.
func Load(noRouteHandler http.HandlerFunc, middleware ...gin.HandlerFunc) http.Handler {
	e := gin.New()
	e.UseRawPath = true
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

	e.NoRoute(gin.WrapF(noRouteHandler))

	base := e.Group(server.Config.Server.RootPath)
	{
		base.GET("/web-config.js", web.Config)

		base.GET("/logout", api.GetLogout)
		auth := base.Group("/authorize")
		{
			auth.GET("", api.HandleAuth)
			auth.POST("", api.HandleAuth)
		}

		base.GET("/metrics", metrics.PromHandler())
		base.GET("/version", api.Version)

		// Health check endpoints for HA load balancer integration
		base.GET("/healthz", api.GetHealth)     // Comprehensive health check
		base.GET("/livez", api.GetHealthLive)   // Liveness probe
		base.GET("/readyz", api.GetHealthReady) // Readiness probe
	}

	apiRoutes(base)
	if server.Config.WebUI.EnableSwagger {
		setupSwaggerConfigAndRoutes(e)
	}

	return e
}

func setupSwaggerConfigAndRoutes(e *gin.Engine) {
	openapi.SwaggerInfo.Host = getHost(server.Config.Server.Host)
	openapi.SwaggerInfo.BasePath = server.Config.Server.RootPath + "/api"
	e.GET(server.Config.Server.RootPath+"/swagger/*any", ginSwagger.WrapHandler(openapi_files.Handler))
}

func getHost(s string) string {
	parse, _ := url.Parse(s)
	return parse.Host
}
