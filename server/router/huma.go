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

package router

import (
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
)

// BearerSecurity is the OpenAPI security requirement for authenticated endpoints.
var BearerSecurity = []map[string][]string{{"BearerAuth": {}}}

// schemaNamer returns a unique schema name for each type, using the package
// name as a prefix to disambiguate types with the same short name (e.g.
// model.Repo vs metadata.Repo).
func schemaNamer(t reflect.Type, hint string) string {
	t = reflect.Indirect(reflect.New(t)).Type()
	name := t.Name()
	pkgPath := t.PkgPath()

	if name == "" {
		// For unnamed types (slices, maps, etc.), use the element type's
		// fully qualified name to avoid collisions between different slice
		// types that share the same generic hint (e.g. "Item").
		if elemName, elemPkg := elementTypeName(t); elemName != "" {
			name = elemName
			pkgPath = elemPkg
		} else {
			name = hint
		}
	}

	// Prefix with last package path segment (e.g. "model", "metadata").
	if pkgPath != "" {
		if i := strings.LastIndex(pkgPath, "/"); i >= 0 {
			name = pkgPath[i+1:] + "." + name
		} else {
			name = pkgPath + "." + name
		}
	}

	// Clean up generics and special chars, same as huma.DefaultSchemaNamer.
	name = strings.ReplaceAll(name, "[]", "List[")
	var result strings.Builder
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '[' || r == ']' || r == '*' || r == ','
	}) {
		if len(part) > 0 {
			r, size := utf8.DecodeRuneInString(part)
			if r < utf8.RuneSelf && 'a' <= r && r <= 'z' {
				result.WriteByte(byte(r - ('a' - 'A')))
				result.WriteString(part[size:])
			} else {
				result.WriteString(strings.ToUpper(string(r)))
				result.WriteString(part[size:])
			}
		}
	}
	return result.String()
}

// elementTypeName dereferences slices/arrays/pointers/maps to find the
// underlying named element type, returning its name and package path.
func elementTypeName(t reflect.Type) (string, string) {
	for {
		switch t.Kind() {
		case reflect.Slice, reflect.Array, reflect.Pointer:
			t = t.Elem()
		case reflect.Map:
			t = t.Elem()
		default:
			return t.Name(), t.PkgPath()
		}
	}
}

// newHumaAPI creates a huma API instance mounted on the given gin group.
func newHumaAPI(engine *gin.Engine, group *gin.RouterGroup, title, version string) huma.API {
	config := huma.DefaultConfig(title, version)
	// Allow extra JSON properties in request bodies so frontends can send
	// Partial<Model> objects that include fields beyond the body struct.
	config.AllowAdditionalPropertiesByDefault = true
	config.Servers = []*huma.Server{{URL: server.Config.Server.RootPath + "/api/v1"}}
	config.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", schemaNamer)
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"BearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			Description:  "Personal access token",
			BearerFormat: "JWT",
		},
	}
	config.DocsRenderer = huma.DocsRendererScalar
	if !server.Config.WebUI.EnableSwagger {
		config.OpenAPIPath = ""
		config.DocsPath = ""
	}
	config.SchemasPath = ""
	// Disable the SchemaLinkTransformer that huma registers by default.
	// It calls reflect.StructOf on response types, which panics for types
	// that embed a struct with methods (e.g. AutoscalerWithToken embeds
	// Autoscaler). Since SchemasPath is empty the feature is unused anyway.
	config.CreateHooks = nil

	humaAPI := humagin.NewWithGroup(engine, group, config)

	// Global middleware: inject *gin.Context into the Go context for huma handlers.
	humaAPI.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		ginCtx := humagin.Unwrap(ctx)
		next(huma.WithValue(ctx, api.GinContextKey(), ginCtx))
	})

	return humaAPI
}

// wrapGinMiddleware converts a gin.HandlerFunc into a huma per-operation middleware.
// If the gin middleware aborts the request, the huma handler is not called.
func wrapGinMiddleware(mw gin.HandlerFunc) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ginCtx := humagin.Unwrap(ctx)
		mw(ginCtx)
		if !ginCtx.IsAborted() {
			next(ctx)
		}
	}
}

// Huma middleware wrappers for session middleware.
var (
	hmMustUser       = wrapGinMiddleware(session.MustUser())
	hmMustAdmin      = wrapGinMiddleware(session.MustAdmin())
	hmMustPull       = wrapGinMiddleware(session.MustPull)
	hmMustPush       = wrapGinMiddleware(session.MustPush)
	hmMustRepoAdmin  = wrapGinMiddleware(session.MustRepoAdmin())
	hmSetRepo        = wrapGinMiddleware(session.SetRepo())
	hmSetPerm        = wrapGinMiddleware(session.SetPerm())
	hmSetOrg         = wrapGinMiddleware(session.SetOrg())
	hmMustOrg        = wrapGinMiddleware(session.MustOrg())
	hmMustAutoscaler = wrapGinMiddleware(session.MustAutoscaler())
)

// The hmMustPermission wrapper guards a huma operation by an instance permission.
func hmMustPermission(p model.Permission) func(ctx huma.Context, next func(huma.Context)) {
	return wrapGinMiddleware(session.MustPermission(p))
}

func hmMustOrgMember(admin bool) func(ctx huma.Context, next func(huma.Context)) {
	return wrapGinMiddleware(session.MustOrgMember(admin))
}
