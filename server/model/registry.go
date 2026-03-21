// Copyright 2021 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

package model

import (
	"errors"
	"net/url"

	"github.com/uptrace/bun"
)

var (
	errRegistryAddressInvalid  = errors.New("invalid registry address")
	errRegistryUsernameInvalid = errors.New("invalid registry username")
	errRegistryPasswordInvalid = errors.New("invalid registry password")
)

// Registry represents a docker registry with credentials.
type Registry struct {
	bun.BaseModel `bun:"table:registries"`

	ID        int64  `json:"id"         bun:"id,pk,autoincrement"`
	OrgID     int64  `json:"org_id"     bun:"org_id,notnull,default:0,unique:s"`
	RepoID    int64  `json:"repo_id"    bun:"repo_id,notnull,default:0,unique:s"`
	Address   string `json:"address"    bun:"address,notnull,unique:s"`
	Username  string `json:"username"   bun:"username,type:varchar(2000)"`
	Password  string `json:"password"   bun:"password,type:text"`
	ReadOnly  bool   `json:"readonly"   bun:"-"`
	CreatedAt int64  `json:"created_at" bun:",nullzero"`
	UpdatedAt int64  `json:"updated_at" bun:",nullzero"`
} //	@name	Registry

func (r Registry) TableName() string {
	return "registries"
}

// Global registry.
func (r Registry) IsGlobal() bool {
	return r.RepoID == 0 && r.OrgID == 0
}

// Organization registry.
func (r Registry) IsOrganization() bool {
	return r.RepoID == 0 && r.OrgID != 0
}

// Repository registry.
func (r Registry) IsRepository() bool {
	return r.RepoID != 0 && r.OrgID == 0
}

// Validate validates the registry information.
func (r *Registry) Validate() error {
	switch {
	case len(r.Address) == 0:
		return errRegistryAddressInvalid
	case len(r.Username) == 0:
		return errRegistryUsernameInvalid
	case len(r.Password) == 0:
		return errRegistryPasswordInvalid
	}

	_, err := url.Parse(r.Address)
	return err
}

// Copy makes a copy of the registry without the password.
func (r *Registry) Copy() *Registry {
	return &Registry{
		ID:       r.ID,
		OrgID:    r.OrgID,
		RepoID:   r.RepoID,
		Address:  r.Address,
		Username: r.Username,
		ReadOnly: r.ReadOnly,
	}
}

// RegistryWithOrg extends Registry with organization context for user views.
type RegistryWithOrg struct {
	*Registry
	OrgName   string `json:"org_name"`
	ScopeType string `json:"scope_type"`           // "user", "org", or "global"
	ForgeID   int64  `json:"forge_id,omitempty"`   // forge ID for multi-forge display
	ForgeName string `json:"forge_name,omitempty"` // forge hostname for multi-forge display
	ForgeIcon string `json:"forge_icon,omitempty"` // forge icon name/URL for multi-forge display
	ForgeType string `json:"forge_type,omitempty"` // forge type (github, gitlab, etc.) for icon fallback
} //	@name	RegistryWithOrg

// WithOrg creates a RegistryWithOrg from a Registry with the given org name.
func (r *Registry) WithOrg(orgName string, isPersonalOrg bool, forgeID int64, forgeName, forgeIcon, forgeType string) *RegistryWithOrg {
	scopeType := "org"
	if r.IsGlobal() {
		scopeType = "global"
	} else if isPersonalOrg {
		scopeType = "user"
	}
	return &RegistryWithOrg{
		Registry:  r,
		OrgName:   orgName,
		ScopeType: scopeType,
		ForgeID:   forgeID,
		ForgeName: forgeName,
		ForgeIcon: forgeIcon,
		ForgeType: forgeType,
	}
}
