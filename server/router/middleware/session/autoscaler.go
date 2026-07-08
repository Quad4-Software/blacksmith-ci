// Copyright 2025 Crow Authors
// Copyright 2022 Woodpecker Authors
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

package session

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

func Autoscaler(c *gin.Context) *model.Autoscaler {
	v, ok := c.Get("autoscaler")
	if !ok {
		return nil
	}
	u, ok := v.(*model.Autoscaler)
	if !ok {
		return nil
	}
	return u
}

func SetAutoscaler() gin.HandlerFunc {
	return func(c *gin.Context) {
		_store := store.FromContext(c)

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// Extract bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]

		// Find autoscaler by token
		autoscaler, err := _store.AutoscalerFindByToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		c.Set("autoscaler", autoscaler)
		c.Next()
	}
}

func MustAutoscaler() gin.HandlerFunc {
	return func(c *gin.Context) {
		autoscaler := Autoscaler(c)
		if autoscaler == nil {
			c.String(http.StatusUnauthorized, "Invalid or missing autoscaler token. Please provide a valid token in the Authorization header (format: 'Authorization: Bearer <token>').")
			c.Abort()
			return
		}
		c.Next()
	}
}
