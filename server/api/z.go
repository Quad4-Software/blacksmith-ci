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

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/version"
)

// Version serves the server version (registered on the base route, not the API).
func Version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"source":  "https://codeberg.org/crowci/crow",
		"version": version.String(),
	})
}

// LogLevel serves the current log level (registered on the base route, not the API).
func LogLevel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"log-level": zerolog.GlobalLevel().String(),
	})
}

// SetLogLevel sets the current log level (registered on the base route, not the API).
func SetLogLevel(c *gin.Context) {
	logLevel := struct {
		LogLevel string `json:"log-level"`
	}{}
	if err := c.Bind(&logLevel); err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	lvl, err := zerolog.ParseLevel(logLevel.LogLevel)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	log.Log().Msgf("log level set to %s", lvl.String())
	zerolog.SetGlobalLevel(lvl)
	c.JSON(http.StatusOK, logLevel)
}
