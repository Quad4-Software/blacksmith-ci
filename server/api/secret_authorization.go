// Copyright 2025 Crow Authors
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
	"context"

	"github.com/gin-gonic/gin"

	"codefloe.com/crowci/crow/v5/server/cache"
	"codefloe.com/crowci/crow/v5/server/forge"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

// SecretViewAuthorizer handles authorization for viewing abbreviated secret values.
// This is separate from the standard middleware permission checks because viewing
// secret values requires explicit permissions, not auto-granted admin permissions.
type SecretViewAuthorizer struct {
	store      store.Store
	membership cache.MembershipService
}

// NewSecretViewAuthorizer creates a new SecretViewAuthorizer instance.
func NewSecretViewAuthorizer(store store.Store, membership cache.MembershipService) *SecretViewAuthorizer {
	return &SecretViewAuthorizer{
		store:      store,
		membership: membership,
	}
}

// CanViewRepoSecret checks if user has EXPLICIT write access to view repo secrets.
// Instance admins without explicit forge-granted push access cannot view secret values.
// This ensures that instance admins can only view secrets for repos they have explicit
// access to via the forge (GitHub/GitLab/etc.), not just by virtue of being admins.
func (a *SecretViewAuthorizer) CanViewRepoSecret(user *model.User, repo *model.Repo) bool {
	if user == nil || repo == nil {
		return false
	}

	// Fetch the actual permission record from the database
	// This gives us the forge-synced permission, not the auto-granted admin permission
	perm, err := a.store.PermFind(user, repo)
	if err != nil || perm == nil {
		// No explicit permission record exists
		return false
	}

	// The permission must have been synced from the forge (Synced > 0)
	// and the user must have push access
	// This excludes cases where permission was only auto-granted by admin status
	return perm.Push && perm.Synced > 0
}

// CanViewOrgSecret checks if user is an org admin via forge membership.
// Org admin membership (as determined by the forge) is sufficient to view org secrets.
func (a *SecretViewAuthorizer) CanViewOrgSecret(c context.Context, user *model.User, org *model.Org, _forge forge.Forge) bool {
	if user == nil || org == nil || _forge == nil {
		return false
	}

	// Personal org: user can always view their own secrets
	if org.Name == user.Login {
		return true
	}

	// Check org membership via forge
	orgPerm, err := a.membership.Get(c, _forge, user, org.Name)
	if err != nil || orgPerm == nil {
		return false
	}

	return orgPerm.Admin
}

// CanViewGlobalSecret checks if user is an instance admin.
// Only instance admins can view global secret values.
func (a *SecretViewAuthorizer) CanViewGlobalSecret(user *model.User) bool {
	if user == nil {
		return false
	}
	return user.Admin
}

// NewSecretViewAuthorizerFromContext creates a SecretViewAuthorizer from gin context.
func NewSecretViewAuthorizerFromContext(c *gin.Context, membership cache.MembershipService) *SecretViewAuthorizer {
	return NewSecretViewAuthorizer(store.FromContext(c), membership)
}
