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
	"net/url"
	"strings"
)

// maxRedirectPathLen bounds a post-login redirect path, which travels inside
// the OAuth state token.
const maxRedirectPathLen = 2048

// safeRedirectPath reports whether raw is a local path below the server's
// root path that is safe to redirect to after a sign-in. It accepts only an
// absolute path such as "/repos/1?tab=x": no scheme, no host, no
// protocol-relative "//" prefix, no backslashes (which browsers treat as "/")
// and no control characters. Anything else could send the user off-site.
func safeRedirectPath(raw string) bool {
	if raw == "" || len(raw) > maxRedirectPathLen {
		return false
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return false
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return false
		}
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.Opaque != "" {
		return false
	}
	// u.Path is percent-decoded, so this also catches "/%2F%2Fevil.com" and
	// "/%5Cevil.com", in case anything downstream decodes the path again.
	if strings.HasPrefix(u.Path, "//") || strings.ContainsRune(u.Path, '\\') {
		return false
	}
	for _, r := range u.Path {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// redirectPathOr returns raw when it is a safe local redirect path, and
// fallback otherwise.
func redirectPathOr(raw, fallback string) string {
	if safeRedirectPath(raw) {
		return raw
	}
	return fallback
}
