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

package types

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestOAuthRequestAuthCodeOptions(t *testing.T) {
	cfg := &oauth2.Config{
		ClientID:    "client-id",
		RedirectURL: "https://crow.example/authorize",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://forge.example/login/oauth/authorize"},
	}

	// Without a prompt, no "prompt" parameter is added to the authorize URL.
	req := &OAuthRequest{State: "state-token"}
	parsed, err := url.Parse(cfg.AuthCodeURL(req.State, req.AuthCodeOptions()...))
	assert.NoError(t, err)
	assert.Empty(t, parsed.Query().Get("prompt"))
	assert.Equal(t, "state-token", parsed.Query().Get("state"))

	// With a prompt, it is forwarded so the forge can show its account picker.
	req.Prompt = "select_account"
	parsed, err = url.Parse(cfg.AuthCodeURL(req.State, req.AuthCodeOptions()...))
	assert.NoError(t, err)
	assert.Equal(t, "select_account", parsed.Query().Get("prompt"))
	assert.Equal(t, "state-token", parsed.Query().Get("state"))
}
