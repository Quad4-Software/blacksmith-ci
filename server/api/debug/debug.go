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

package debug

import (
	"net/http/pprof"

	"github.com/gin-gonic/gin"
)

// IndexHandler returns a handler that lists available pprof profiles.
func IndexHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Index(c.Writer, c.Request)
	}
}

// HeapHandler returns a handler for pprof heap dump.
func HeapHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Handler("heap").ServeHTTP(c.Writer, c.Request)
	}
}

// GoroutineHandler returns a handler for pprof goroutine stack traces.
func GoroutineHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Handler("goroutine").ServeHTTP(c.Writer, c.Request)
	}
}

// BlockHandler returns a handler for pprof block profile.
func BlockHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Handler("block").ServeHTTP(c.Writer, c.Request)
	}
}

// ThreadCreateHandler returns a handler for pprof threadcreate profile.
func ThreadCreateHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Handler("threadcreate").ServeHTTP(c.Writer, c.Request)
	}
}

// CmdlineHandler returns a handler for the command line invocation.
func CmdlineHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Cmdline(c.Writer, c.Request)
	}
}

// ProfileHandler returns a handler for pprof CPU profile.
func ProfileHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Profile(c.Writer, c.Request)
	}
}

// SymbolHandler returns a handler for pprof symbol lookup.
func SymbolHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Symbol(c.Writer, c.Request)
	}
}

// TraceHandler returns a handler for pprof execution trace.
func TraceHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pprof.Trace(c.Writer, c.Request)
	}
}
