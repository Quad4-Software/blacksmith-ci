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
	"codefloe.com/crowci/crow/v6/server/router/middleware"
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
