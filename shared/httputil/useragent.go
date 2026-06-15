// Copyright 2024 Crow Authors
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

package httputil

import (
	"net/http"

	"codefloe.com/crowci/crow/v5/version"
)

// UserAgent returns the User-Agent header value identifying the Crow
// server in outgoing requests to git forges.
func UserAgent() string {
	return "crow-server/" + version.String()
}

// UserAgentTransport is an http.RoundTripper that sets the User-Agent
// header on outgoing requests when one is not already present.
type UserAgentTransport struct {
	Base      http.RoundTripper
	UserAgent string
}

func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if t.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", t.UserAgent)
	}
	return base.RoundTrip(req)
}

// WithUserAgent wraps the transport of the given http.Client so that the
// Crow User-Agent is sent with every request. A nil client yields a new
// one. The same client is returned for convenient chaining.
func WithUserAgent(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	client.Transport = &UserAgentTransport{Base: client.Transport, UserAgent: UserAgent()}
	return client
}
