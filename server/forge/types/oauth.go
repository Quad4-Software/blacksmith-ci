// Copyright 2024 Woodpecker Authors
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

import "golang.org/x/oauth2"

type OAuthRequest struct {
	Code  string
	State string
	// Prompt is the OAuth "prompt" parameter forwarded to the forge's authorize
	// endpoint (e.g. "login" to force re-authentication so a different account
	// can be chosen). Empty means the parameter is omitted.
	Prompt string
}

// AuthCodeOptions returns the extra AuthCodeURL options derived from the
// request, currently the OAuth "prompt" parameter when set. Forges pass the
// result to oauth2 config.AuthCodeURL so account-selection intent reaches the
// provider.
func (r *OAuthRequest) AuthCodeOptions() []oauth2.AuthCodeOption {
	if r.Prompt == "" {
		return nil
	}
	return []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("prompt", r.Prompt)}
}
