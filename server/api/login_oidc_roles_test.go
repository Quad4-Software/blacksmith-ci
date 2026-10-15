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
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/model"
)

func TestApplyProviderRoles(t *testing.T) {
	p := oidc.New(oidc.Config{Issuer: "https://idp.example", RoleGroups: map[string][]string{
		"operator": {"ops"},
		"auditor":  {"audit"},
	}})
	tests := []struct {
		name    string
		roles   model.JSONSliceString
		groups  []string
		want    model.JSONSliceString
		changed bool
	}{
		{"grants mapped role", nil, []string{"ops"}, model.JSONSliceString{"operator"}, true},
		{"revokes mapped role", model.JSONSliceString{"auditor", "operator"}, []string{"ops"}, model.JSONSliceString{"operator"}, true},
		{"keeps unmanaged role", model.JSONSliceString{"secrets-manager"}, nil, model.JSONSliceString{"secrets-manager"}, false},
		{"mixes", model.JSONSliceString{"secrets-manager"}, []string{"audit"}, model.JSONSliceString{"auditor", "secrets-manager"}, true},
		{"drops unknown stored role", model.JSONSliceString{"root"}, nil, model.JSONSliceString{}, true},
		{"no change", model.JSONSliceString{"operator"}, []string{"ops"}, model.JSONSliceString{"operator"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &model.User{Roles: tt.roles}
			assert.Equal(t, tt.changed, applyProviderRoles(u, p, tt.groups))
			assert.Equal(t, tt.want, u.Roles)
		})
	}
}
