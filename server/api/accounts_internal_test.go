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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// applyCookies copies Set-Cookie headers from a recorder onto a fresh request so
// the next helper call observes the previously written state, mimicking a browser.
func applyCookies(t *testing.T, rec *httptest.ResponseRecorder) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestRememberAccountRoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	rememberAccount(rec, req, 1, "token-1")

	req = applyCookies(t, rec)
	accounts := readAccounts(req)
	assert.Len(t, accounts, 1)
	assert.Equal(t, int64(1), accounts[0].ID)
	assert.Equal(t, "token-1", accounts[0].Token)
}

func TestRememberAccountDedupMovesToFront(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	rememberAccount(rec, req, 1, "token-1")
	req = applyCookies(t, rec)
	rec = httptest.NewRecorder()
	rememberAccount(rec, req, 2, "token-2")
	req = applyCookies(t, rec)
	rec = httptest.NewRecorder()

	// Re-remember account 1 with a refreshed token: it must move to the front
	// and not create a duplicate entry.
	rememberAccount(rec, req, 1, "token-1-new")
	req = applyCookies(t, rec)

	accounts := readAccounts(req)
	assert.Len(t, accounts, 2)
	assert.Equal(t, int64(1), accounts[0].ID)
	assert.Equal(t, "token-1-new", accounts[0].Token)
	assert.Equal(t, int64(2), accounts[1].ID)
}

func TestRememberAccountCaps(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for i := int64(1); i <= maxSwitchAccounts+3; i++ {
		rec := httptest.NewRecorder()
		rememberAccount(rec, req, i, "token")
		req = applyCookies(t, rec)
	}

	accounts := readAccounts(req)
	assert.Len(t, accounts, maxSwitchAccounts)
	// The most recently remembered account is always kept at the front.
	assert.Equal(t, int64(maxSwitchAccounts+3), accounts[0].ID)
}

func TestForgetAccount(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for i := int64(1); i <= 3; i++ {
		rec := httptest.NewRecorder()
		rememberAccount(rec, req, i, "token")
		req = applyCookies(t, rec)
	}

	rec := httptest.NewRecorder()
	remaining := forgetAccount(rec, req, 2)
	assert.Len(t, remaining, 2)
	for _, acc := range remaining {
		assert.NotEqual(t, int64(2), acc.ID)
	}

	// The cookie itself reflects the removal.
	req = applyCookies(t, rec)
	assert.Len(t, readAccounts(req), 2)
}

func TestReadAccountsMalformed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: accountsCookie, Value: "not-base64-or-json"})
	assert.Empty(t, readAccounts(req))
}
