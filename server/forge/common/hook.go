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

package common

import (
	"net/url"
	"strings"
)

// hookPaths are the URL paths Crow has used for incoming forge webhooks,
// newest first. The older paths are kept so that repairing a repo still
// replaces webhooks registered by previous versions.
var hookPaths = []string{"/api/v1/hook", "/api/hook", "/hook"}

// IsCrowHookURL reports whether rawHookURL, the callback URL of a webhook
// registered on a forge, belongs to this Crow instance.
//
// The base is either the configured webhook host or a full Crow hook URL;
// both carry the optional path prefix under which Crow is served. Matching the
// host alone is not enough: several unrelated applications may share one
// public hostname and only differ by path, and deleting their webhooks on
// repair would be destructive.
func IsCrowHookURL(rawHookURL, base string) bool {
	hookURL, err := url.Parse(rawHookURL)
	if err != nil {
		return false
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return false
	}
	if !strings.EqualFold(hookURL.Host, baseURL.Host) {
		return false
	}

	prefix := hookBasePath(baseURL)
	hookPath := strings.TrimSuffix(hookURL.Path, "/")
	for _, p := range hookPaths {
		if hookPath == prefix+p {
			return true
		}
	}
	return false
}

// hookBasePath returns the path prefix Crow is served under, stripping the
// hook path when the base already is a full hook URL.
func hookBasePath(base *url.URL) string {
	path := strings.TrimSuffix(base.Path, "/")
	for _, p := range hookPaths {
		if strings.HasSuffix(path, p) {
			return strings.TrimSuffix(path, p)
		}
	}
	return path
}
