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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/ldap"
	"codefloe.com/crowci/crow/v6/server/auth/ldap/ldaptest"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware"
	services_mocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	registry_mocks "codefloe.com/crowci/crow/v6/server/services/registry/mocks"
)

// ldapRuns gives every run of the test its own client IP, because the LDAP
// limiters live for the whole test process (e.g. with -count=2).
var ldapRuns atomic.Int32

func TestLoadTrustedProxiesKeyLDAPLimitOnPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := ldaptest.New(t, nil, nil)
	provider, err := ldap.New(ldap.Config{
		URL:          srv.URL,
		BindDN:       ldaptest.ServiceDN,
		BindPassword: ldaptest.ServicePassword,
		UserBaseDN:   "dc=example,dc=org",
	})
	require.NoError(t, err)
	server.Config.Services.LDAP = provider
	server.Config.Server.TrustedProxies = []string{"10.0.0.1"}
	t.Cleanup(func() {
		server.Config.Services.LDAP = nil
		server.Config.Server.TrustedProxies = nil
	})

	handler := Load(http.NotFound, middleware.Store(guardStore{}))
	peer := fmt.Sprintf("203.0.113.%d", 7+ldapRuns.Add(1)-1)
	post := func(i int) int {
		body := fmt.Sprintf(`{"username":"user%d","password":"wrong"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/authorize/ldap", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		req.RemoteAddr = peer + ":1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	// Rotating X-Forwarded-For values from an untrusted peer share the peer's bucket.
	for i := range 30 {
		require.Equal(t, http.StatusUnauthorized, post(i), "attempt %d", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, post(30))
}

func TestLoadUnknownAPIPathReturnsJSON404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	webUI := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html>"))
	}
	handler := Load(webUI, middleware.Store(guardStore{}))

	serve := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		return w
	}

	// An unencoded registry address with a path splits into an extra segment.
	w := serve(http.MethodDelete, "/api/v1/orgs/1/registries/rg.fr-par.scw.cloud/gudjam")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	w = serve(http.MethodGet, "/repos/1/settings")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "<!doctype html>", w.Body.String())
}

func TestEncodedRegistryAddressReachesHandler(t *testing.T) {
	regs := registry_mocks.NewMockService(t)
	regs.On("GlobalRegistryDelete", "rg.fr-par.scw.cloud/gudjam").Return(nil)
	mgr := services_mocks.NewMockManager(t)
	mgr.On("RegistryService").Return(regs)
	prev := server.Config.Services.Manager
	server.Config.Services.Manager = mgr
	t.Cleanup(func() { server.Config.Services.Manager = prev })

	e := newGuardEngine(t, &model.User{ID: 1, Login: "admin", Admin: true})
	// Load sets this so an encoded "/" stays inside one path segment.
	e.UseRawPath = true
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/registries/rg.fr-par.scw.cloud%2Fgudjam", nil))
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}
