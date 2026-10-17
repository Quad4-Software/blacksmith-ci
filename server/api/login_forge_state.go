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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/shared/httputil"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// forgeOAuthCookie binds a forge OAuth flow to the browser that started it.
// It holds a random nonce whose hash is carried in the state token, so a state
// minted in another browser (login CSRF) is rejected on the callback.
const forgeOAuthCookie = "forge_oauth"

// forgeOAuthNonceClaim is the state token claim holding the nonce hash.
const forgeOAuthNonceClaim = "nonce-hash"

// newForgeOAuthNonce returns a fresh random nonce for the forge OAuth cookie.
func newForgeOAuthNonce() string {
	return base64.RawURLEncoding.EncodeToString(random.GetRandomBytes(32))
}

// forgeOAuthNonceHash returns the value stored in the state token for nonce.
func forgeOAuthNonceHash(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

// forgeOAuthCookiePath scopes the cookie to the OAuth endpoint, below the
// server's root path.
func forgeOAuthCookiePath() string {
	return server.Config.Server.RootPath + "/authorize"
}

func forgeOAuthCookieSecure(r *http.Request) bool {
	return httputil.IsHTTPS(r) || strings.HasPrefix(server.Config.Server.Host, "https://")
}

// setForgeOAuthCookie stores nonce for the callback leg of the flow. SameSite
// Lax keeps the cookie on the top-level redirect back from the forge.
func setForgeOAuthCookie(w http.ResponseWriter, r *http.Request, nonce string) {
	http.SetCookie(w, &http.Cookie{
		Name:     forgeOAuthCookie,
		Value:    nonce,
		Path:     forgeOAuthCookiePath(),
		MaxAge:   int(stateTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   forgeOAuthCookieSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func delForgeOAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     forgeOAuthCookie,
		Value:    "",
		Path:     forgeOAuthCookiePath(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   forgeOAuthCookieSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// forgeOAuthCookieMatches reports whether the request carries the nonce
// cookie that belongs to stateToken.
func forgeOAuthCookieMatches(r *http.Request, stateToken *token.Token) bool {
	want := stateToken.Get(forgeOAuthNonceClaim)
	if want == "" {
		return false
	}
	cookie, err := r.Cookie(forgeOAuthCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(forgeOAuthNonceHash(cookie.Value)), []byte(want)) == 1
}
