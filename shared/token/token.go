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

package token

import (
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

type SecretFunc func(*Token) (string, error)

type Type string

const (
	UserToken        Type = "user"       // user token (exp cli) - deprecated, use AccessToken
	SessToken        Type = "sess"       // session token (ui token requires csrf check)
	HookToken        Type = "hook"       // repo hook token
	CsrfToken        Type = "csrf"       // CSRF protection token
	AgentToken       Type = "agent"      // agent authentication token
	AutoscalerToken  Type = "autoscaler" // autoscaler authentication token
	OAuthStateToken  Type = "oauth-state"
	OIDCStateToken   Type = "oidc-state"  // short-lived OIDC sign-in state, nonce and PKCE verifier
	AccessToken      Type = "access"      // personal access token with scopes
	UnsubscribeToken Type = "unsubscribe" // email unsubscribe token
)

// SignerAlgo id default algorithm used to sign JWT tokens.
const SignerAlgo = "HS256"

type Token struct {
	Type   Type
	claims jwt.MapClaims
}

func Parse(allowedTypes []Type, raw string, fn SecretFunc) (*Token, error) {
	token := &Token{
		claims: jwt.MapClaims{},
	}
	parsed, err := jwt.Parse(raw, keyFunc(token, fn))
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, jwt.ErrTokenUnverifiable
	}

	hasAllowedType := false
	for _, k := range allowedTypes {
		if k == token.Type {
			hasAllowedType = true
			break
		}
	}

	if !hasAllowedType {
		return nil, jwt.ErrInvalidType
	}

	return token, nil
}

// ExtractToken returns the raw token string from the request without parsing
// it (Authorization bearer, X-Gitlab-Token, access_token query, or session
// cookie), in that order of precedence.
func ExtractToken(r *http.Request) (string, error) {
	token := r.Header.Get("Authorization")
	if len(token) != 0 {
		log.Trace().Msg("token.ExtractToken: found token in Authorization header")
		bearer := token
		if _, err := fmt.Sscanf(token, "Bearer %s", &bearer); err != nil {
			return "", err
		}
		return bearer, nil
	}

	token = r.Header.Get("X-Gitlab-Token")
	if len(token) != 0 {
		return token, nil
	}

	// Note: Use r.URL.Query().Get() instead of r.FormValue() to avoid
	// consuming the request body when Content-Type is form-urlencoded.
	token = r.URL.Query().Get("access_token")
	if len(token) != 0 {
		return token, nil
	}

	cookie, err := r.Cookie("user_sess")
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func ParseRequest(allowedTypes []Type, r *http.Request, fn SecretFunc) (*Token, error) {
	raw, err := ExtractToken(r)
	if err != nil {
		return nil, err
	}
	return Parse(allowedTypes, raw, fn)
}

func CheckCsrf(r *http.Request, fn SecretFunc) error {
	// get and options requests are always
	// enabled, without CSRF checks.
	switch r.Method {
	case http.MethodGet, http.MethodOptions:
		return nil
	}

	// parse the raw CSRF token value and validate
	raw := r.Header.Get("X-CSRF-TOKEN")
	_, err := Parse([]Type{CsrfToken}, raw, fn)
	return err
}

func New(tokenType Type) *Token {
	return &Token{Type: tokenType, claims: jwt.MapClaims{}}
}

// Sign signs the token using the given secret hash
// and returns the string value.
func (t *Token) Sign(secret string) (string, error) {
	return t.SignExpires(secret, 0)
}

// Sign signs the token using the given secret hash
// with an expiration date.
func (t *Token) SignExpires(secret string, exp int64) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("token claim is not a MapClaims")
	}

	for k, v := range t.claims {
		claims[k] = v
	}

	claims["type"] = t.Type
	if exp > 0 {
		claims["exp"] = float64(exp)
	}

	return token.SignedString([]byte(secret))
}

func (t *Token) Set(key, value string) {
	t.claims[key] = value
}

func (t *Token) Get(key string) string {
	claim, ok := t.claims[key].(string)
	if !ok {
		return ""
	}

	return claim
}

func keyFunc(token *Token, fn SecretFunc) jwt.Keyfunc {
	return func(t *jwt.Token) (any, error) {
		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("token claim is not a MapClaims")
		}

		// validate the correct algorithm is being used
		if t.Method.Alg() != SignerAlgo {
			return nil, jwt.ErrSignatureInvalid
		}

		// extract the token type and cast to the expected type
		tokenType, ok := claims["type"].(string)
		if !ok {
			return nil, jwt.ErrInvalidType
		}
		token.Type = Type(tokenType)

		// copy custom claims
		for k, v := range claims {
			// skip the reserved claims https://datatracker.ietf.org/doc/html/rfc7519#section-4.1
			if k == "iss" || k == "sub" || k == "aud" || k == "exp" || k == "nbf" || k == "iat" || k == "jti" {
				continue
			}

			if k == "type" {
				continue
			}

			token.claims[k] = v
		}

		// invoke the callback function to retrieve
		// the secret key used to verify
		secret, err := fn(token)
		return []byte(secret), err
	}
}
