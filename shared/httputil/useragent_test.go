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
	"net/http/httptest"
	"testing"
)

func TestUserAgentTransport(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		preset    string
		want      string
	}{
		{name: "sets user agent", userAgent: "crow-server/test", want: "crow-server/test"},
		{name: "keeps existing user agent", userAgent: "crow-server/test", preset: "custom/1.0", want: "custom/1.0"},
		{name: "empty user agent is a no-op", userAgent: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("User-Agent")
			}))
			defer srv.Close()

			client := &http.Client{Transport: &UserAgentTransport{UserAgent: tt.userAgent}}
			req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tt.preset != "" {
				req.Header.Set("User-Agent", tt.preset)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			if tt.want == "" {
				// net/http sets its own default when none is provided.
				if got == "" {
					t.Fatalf("expected the net/http default user agent, got empty")
				}
				return
			}
			if got != tt.want {
				t.Fatalf("User-Agent = %q, want %q", got, tt.want)
			}
		})
	}
}
