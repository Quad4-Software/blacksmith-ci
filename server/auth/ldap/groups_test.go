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

package ldap_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/auth/ldap"
)

func TestMatchGroup(t *testing.T) {
	dn := "cn=Crow-Admins,ou=groups,dc=corp,dc=example"
	for _, tc := range []struct {
		configured string
		want       bool
	}{
		{"crow-admins", true},
		{"CN=crow-admins, OU=Groups,DC=corp,DC=example", true},
		{"cn = crow-admins,ou=groups,dc=corp,dc=example", true},
		{"groups", false},
		{"cn=crow-admins,ou=other,dc=corp,dc=example", false},
		{"crow", false},
	} {
		assert.Equal(t, tc.want, ldap.MatchGroup(tc.configured, dn), tc.configured)
	}
}
