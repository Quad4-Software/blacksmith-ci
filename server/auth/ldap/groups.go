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

package ldap

import (
	"regexp"
	"strings"

	goldap "github.com/go-ldap/ldap/v3"
)

var (
	reSpaceAroundEquals = regexp.MustCompile(`\s*=\s*`)
	reSpaceAfterComma   = regexp.MustCompile(`,\s+`)
)

// MatchGroup reports whether a configured group (a DN or a bare name) matches a
// group DN. A DN matches when it equals groupDN case-insensitively after
// normalizing spaces; a bare name matches the value of groupDN's first RDN.
func MatchGroup(configured, groupDN string) bool {
	c, g := normalizeGroup(configured), normalizeGroup(groupDN)
	if c == "" {
		return false
	}
	if c == g {
		return true
	}
	if strings.Contains(c, "=") {
		return false
	}
	dn, err := goldap.ParseDN(groupDN)
	if err != nil || len(dn.RDNs) == 0 || len(dn.RDNs[0].Attributes) == 0 {
		return false
	}
	return strings.EqualFold(c, strings.TrimSpace(dn.RDNs[0].Attributes[0].Value))
}

// normalizeGroup lower-cases s and removes the spaces after commas and around
// equals signs, so "CN=a, OU=b" and "cn = a,ou=b" compare equal.
func normalizeGroup(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reSpaceAroundEquals.ReplaceAllString(s, "=")
	return reSpaceAfterComma.ReplaceAllString(s, ",")
}
