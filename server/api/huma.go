// Copyright 2025 Crow Authors
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
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"

	"codefloe.com/crowci/crow/v6/server/store/types"
)

// ginCtxKeyType is the context key type for storing *gin.Context in context.Context.
type ginCtxKeyType struct{}

// ginContextKey is the key used to store gin.Context in the Go context.
// Exported via GinContextKey() for use by the router middleware.
var ginContextKey = ginCtxKeyType{}

// GinContextKey returns the key used to store *gin.Context in context.Context.
// Used by the router middleware to inject the gin context.
func GinContextKey() any {
	return ginContextKey
}

// ginCtx extracts the *gin.Context from a huma handler's context.Context.
func ginCtx(ctx context.Context) *gin.Context {
	c, _ := ctx.Value(ginContextKey).(*gin.Context)
	return c
}

// PaginationParams are reusable query parameters for paginated endpoints.
type PaginationParams struct {
	Page    int `query:"page" default:"1" minimum:"1" doc:"for response pagination, page offset number"`
	PerPage int `query:"perPage" default:"50" minimum:"1" maximum:"50" doc:"for response pagination, max items per page"`
}

// dbErr converts a database error to an appropriate huma error.
func dbErr(err error) error {
	if errors.Is(err, types.RecordNotExist) {
		return huma.Error404NotFound("not found")
	}
	return huma.Error500InternalServerError(err.Error())
}
