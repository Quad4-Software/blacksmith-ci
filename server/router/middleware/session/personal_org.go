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

package session

import (
	"strings"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// IsPersonalOrgOwner reports whether org is the personal namespace of user.
// That is the case when it is the user's own org, or when the user's linked
// account on the org's forge has the org's name. A bare match of org name and
// Crow login is not enough: logins of single sign-on users come from the
// identity provider and could otherwise claim any forge org.
func IsPersonalOrgOwner(_store store.Store, user *model.User, org *model.Org) bool {
	if user == nil || org == nil {
		return false
	}
	if user.OrgID != 0 && org.ID == user.OrgID {
		return true
	}
	if _store == nil {
		return false
	}
	userForge, err := _store.UserForgeGet(user.ID, org.ForgeID)
	if err != nil || userForge == nil || userForge.ForgeLogin == "" {
		return false
	}
	return strings.EqualFold(userForge.ForgeLogin, org.Name)
}
