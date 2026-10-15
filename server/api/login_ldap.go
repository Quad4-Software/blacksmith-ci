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
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/ldap"
	"codefloe.com/crowci/crow/v6/server/auth/ratelimit"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
)

const (
	// Sign-in request bodies are limited to 4 KiB.
	maxLDAPLoginBody = 4096

	// Failed attempts allowed within ldapLimitWindow per lower-cased username
	// from one client IP, per username from any IP and per client IP; the
	// backoff after that is capped at ldapLimitWindow too. The strict limit is
	// per username and IP so that an attacker elsewhere cannot lock a user out
	// with a few wrong passwords; the per-username limit is the backstop against
	// guessing one password from many addresses.
	ldapUserIPMaxFailures = 5
	ldapUserMaxFailures   = 50
	ldapIPMaxFailures     = 30
	ldapLimitWindow       = 15 * time.Minute

	// IPv6 clients are counted per /64, the usual smallest allocation, so that
	// rotating addresses within one network does not escape the per-IP limit.
	ldapIPv6PrefixBits = 64
)

// Package-level limiters, replaceable in tests.
var (
	ldapUserIPLimiter = ratelimit.New(ldapUserIPMaxFailures, ldapLimitWindow, ldapLimitWindow)
	ldapUserLimiter   = ratelimit.New(ldapUserMaxFailures, ldapLimitWindow, ldapLimitWindow)
	ldapIPLimiter     = ratelimit.New(ldapIPMaxFailures, ldapLimitWindow, ldapLimitWindow)
)

// limiterKey is one key of one limiter.
type limiterKey struct {
	limiter *ratelimit.Limiter
	key     string
}

// clientIPKey returns the per-IP limiter key for ip: the address itself for
// IPv4 and the /64 network for IPv6.
func clientIPKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr, ldapIPv6PrefixBits).Masked().String()
}

// ldapFailure is a failed LDAP credential check: the HTTP status, the error
// code and, for a rate-limited attempt, the time to wait.
type ldapFailure struct {
	status     int
	code       string
	retryAfter time.Duration
}

// retryAfterSeconds formats d as a Retry-After header value.
func retryAfterSeconds(d time.Duration) string {
	return strconv.FormatInt(int64((d+time.Second-1)/time.Second), 10)
}

// verifyLDAPCredentials rate-limits the attempt, checks the credentials against
// the directory and checks the allowed groups. Failed attempts (wrong
// credentials, access denied) stay recorded in all limiters; directory errors
// are forgiven; verified credentials reset the username keys and forgive the
// IP.
func verifyLDAPCredentials(c *gin.Context, provider *ldap.Provider, username, password string) (*idpClaims, *ldapFailure) {
	userKey := strings.ToLower(strings.TrimSpace(username))
	ipKey := clientIPKey(c.ClientIP())
	userIP := limiterKey{ldapUserIPLimiter, userKey + "\x00" + ipKey}
	user := limiterKey{ldapUserLimiter, userKey}
	ip := limiterKey{ldapIPLimiter, ipKey}
	keys := []limiterKey{userIP, user, ip}

	now := time.Now()
	var recorded []limiterKey
	var wait time.Duration
	for _, k := range keys {
		ok, retryAfter := k.limiter.Attempt(k.key, now)
		if ok {
			recorded = append(recorded, k)
		}
		wait = max(wait, retryAfter)
	}
	if len(recorded) < len(keys) {
		for _, k := range recorded {
			k.limiter.Forgive(k.key)
		}
		log.Warn().Str("username", userKey).Str("ip", ipKey).Msg("LDAP sign-in rate limited")
		return nil, &ldapFailure{status: http.StatusTooManyRequests, code: "too_many_attempts", retryAfter: wait}
	}

	claims, err := provider.Authenticate(c.Request.Context(), username, password)
	switch {
	case errors.Is(err, ldap.ErrInvalidCredentials):
		log.Warn().Str("username", userKey).Str("ip", ipKey).Msg("LDAP sign-in with invalid credentials")
		return nil, &ldapFailure{status: http.StatusUnauthorized, code: "invalid_credentials"}
	case err != nil:
		log.Error().Err(err).Str("username", userKey).Msg("cannot check LDAP credentials")
		for _, k := range keys {
			k.limiter.Forgive(k.key)
		}
		return nil, &ldapFailure{status: http.StatusServiceUnavailable, code: "ldap_unavailable"}
	}

	if len(claims.Subject) > maxIdentitySubjectLen {
		log.Error().Int("length", len(claims.Subject)).Str("username", userKey).Msg("LDAP ID attribute is longer than 255 bytes")
		userIP.limiter.Reset(userIP.key)
		user.limiter.Reset(user.key)
		ip.limiter.Forgive(ip.key)
		return nil, &ldapFailure{status: http.StatusServiceUnavailable, code: "ldap_unavailable"}
	}
	if !provider.IsAllowed(claims.Groups) {
		log.Warn().Str("subject", claims.Subject).Strs("groups", claims.Groups).Msg("LDAP user is not in an allowed group")
		return nil, &ldapFailure{status: http.StatusForbidden, code: "ldap_access_denied"}
	}

	// Never reset the IP key: a valid account must not wipe the per-IP count.
	userIP.limiter.Reset(userIP.key)
	user.limiter.Reset(user.key)
	ip.limiter.Forgive(ip.key)
	return &idpClaims{Subject: claims.Subject, Email: claims.Email, Username: claims.Username, Groups: claims.Groups}, nil
}

// sameOrigin reports whether origin has the scheme, host and port of
// CROW_HOST, ignoring case and treating an omitted port as the default one.
func sameOrigin(origin string) bool {
	want, ok := normalizeOrigin(server.Config.Server.Host)
	if !ok {
		return false
	}
	got, ok := normalizeOrigin(origin)
	return ok && got == want
}

// normalizeOrigin returns scheme://host:port of an http or https URL in lower
// case, with the default port filled in.
func normalizeOrigin(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	switch {
	case port != "":
	case scheme == "https":
		port = "443"
	case scheme == "http":
		port = "80"
	default:
		return "", false
	}
	return scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port), true
}

// HandleLDAPLogin signs a user in with directory credentials. POST /authorize/ldap.
func HandleLDAPLogin(c *gin.Context) {
	provider := server.Config.Services.LDAP
	if provider == nil {
		c.Status(http.StatusNotFound)
		c.Writer.WriteHeaderNow()
		return
	}

	// A JSON content type and a matching origin block cross-site login CSRF:
	// a plain HTML form cannot send either.
	if mediaType, _, _ := mime.ParseMediaType(c.ContentType()); mediaType != "application/json" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "unsupported_media_type"})
		return
	}
	if origin := c.GetHeader("Origin"); origin != "" && !sameOrigin(origin) {
		log.Warn().Str("origin", origin).Msg("LDAP sign-in from a foreign origin")
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid_origin"})
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxLDAPLoginBody)).Decode(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	claims, failure := verifyLDAPCredentials(c, provider, body.Username, body.Password)
	if failure != nil {
		if failure.retryAfter > 0 {
			c.Header("Retry-After", retryAfterSeconds(failure.retryAfter))
		}
		c.JSON(failure.status, gin.H{"error": failure.code})
		return
	}

	_store := store.FromContext(c)
	user, err := signInIdentity(_store, provider, claims)
	if err != nil {
		code := identityErrorCode(err)
		if code == "internal_error" {
			log.Error().Err(err).Msg("cannot complete LDAP sign-in")
			c.JSON(http.StatusInternalServerError, gin.H{"error": code})
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": code})
		return
	}

	if err := createSession(c, _store, user); err != nil {
		log.Error().Err(err).Msg("cannot create session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.Status(http.StatusNoContent)
	c.Writer.WriteHeaderNow()
}

type LinkLDAPIdentityInput struct {
	Body struct {
		Username string `json:"username" maxLength:"256"`
		Password string `json:"password" maxLength:"1024"`
	}
}

type LinkLDAPIdentityOutput struct{ Body *model.UserIdentity }

// LinkLDAPIdentity links the directory account to the signed-in user.
func LinkLDAPIdentity(ctx context.Context, input *LinkLDAPIdentityInput) (*LinkLDAPIdentityOutput, error) {
	provider := server.Config.Services.LDAP
	if provider == nil {
		return nil, huma.Error404NotFound("LDAP sign-in is not configured")
	}
	c := ginCtx(ctx)

	claims, failure := verifyLDAPCredentials(c, provider, input.Body.Username, input.Body.Password)
	if failure != nil {
		err := huma.NewError(failure.status, failure.code)
		if failure.retryAfter > 0 {
			return nil, huma.ErrorWithHeaders(err, http.Header{"Retry-After": {retryAfterSeconds(failure.retryAfter)}})
		}
		return nil, err
	}

	user := session.User(c)
	identity, err := linkIdentity(store.FromContext(c), provider, user, claims)
	if err != nil {
		code := identityErrorCode(err)
		if code == "internal_error" {
			return nil, huma.Error500InternalServerError("Error linking identity", err)
		}
		return nil, huma.Error409Conflict(code)
	}
	return &LinkLDAPIdentityOutput{Body: identity}, nil
}
