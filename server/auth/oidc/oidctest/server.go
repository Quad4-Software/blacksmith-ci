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

// Package oidctest provides an in-process OpenID Connect provider for tests.
// It serves discovery, JWKS, token and userinfo endpoints, verifies PKCE, and
// signs ID tokens with a throwaway RSA key.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const (
	keyID         = "oidctest"
	rsaKeyBits    = 2048
	tokenLifetime = 5 * time.Minute
)

type pendingCode struct {
	nonce     string
	challenge string
	claims    map[string]any
}

// Server is a fake OpenID Connect provider.
type Server struct {
	*httptest.Server

	ClientID     string
	ClientSecret string
	// UserInfo is returned by the userinfo endpoint.
	UserInfo map[string]any
	// UserInfoStatus makes the userinfo endpoint respond with this HTTP status when non-zero.
	UserInfoStatus int
	// Audience overrides the aud claim of issued ID tokens when non-empty.
	Audience string

	key   *rsa.PrivateKey
	mu    sync.Mutex
	codes map[string]pendingCode
}

// New starts a fake provider that is closed when the test ends.
func New(t *testing.T) *Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	require.NoError(t, err)

	s := &Server{ClientID: "crow", ClientSecret: "crow-secret", key: key, codes: map[string]pendingCode{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", s.discovery)
	mux.HandleFunc("/keys", s.keys)
	mux.HandleFunc("/token", s.token)
	mux.HandleFunc("/userinfo", s.userinfo)
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// Issuer returns the issuer URL to configure in Crow.
func (s *Server) Issuer() string { return s.URL }

// IssueCode simulates the user signing in at the provider after being sent to
// authURL. It returns an authorization code whose ID token carries claims (merged
// over iss, aud, exp, iat and the nonce from authURL; a "nonce" claim overrides it).
func (s *Server) IssueCode(t *testing.T, authURL string, claims map[string]any) string {
	t.Helper()
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	q := u.Query()
	code := rand.Text()
	s.mu.Lock()
	s.codes[code] = pendingCode{nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), claims: claims}
	s.mu.Unlock()
	return code
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errchkjson // test helper, values are static JSON-safe maps
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) discovery(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                s.URL,
		"authorization_endpoint":                s.URL + "/authorize",
		"token_endpoint":                        s.URL + "/token",
		"jwks_uri":                              s.URL + "/keys",
		"userinfo_endpoint":                     s.URL + "/userinfo",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (s *Server) keys(w http.ResponseWriter, _ *http.Request) {
	pub := s.key.PublicKey
	s.writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"alg": "RS256",
		"use": "sig",
		"kid": keyID,
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	code := r.Form.Get("code")
	s.mu.Lock()
	pc, ok := s.codes[code]
	delete(s.codes, code)
	s.mu.Unlock()
	if !ok {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != pc.challenge {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant", "error_description": "PKCE verification failed"})
		return
	}

	aud := s.ClientID
	if s.Audience != "" {
		aud = s.Audience
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   s.URL,
		"aud":   aud,
		"iat":   now.Unix(),
		"exp":   now.Add(tokenLifetime).Unix(),
		"nonce": pc.nonce,
	}
	for k, v := range pc.claims {
		claims[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = keyID
	signed, err := tok.SignedString(s.key)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"expires_in":   int(tokenLifetime.Seconds()),
		"id_token":     signed,
	})
}

func (s *Server) userinfo(w http.ResponseWriter, _ *http.Request) {
	if s.UserInfoStatus != 0 {
		s.writeJSON(w, s.UserInfoStatus, map[string]string{"error": "server_error"})
		return
	}
	if s.UserInfo == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	s.writeJSON(w, http.StatusOK, s.UserInfo)
}
