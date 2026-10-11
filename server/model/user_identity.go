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

package model

import "github.com/uptrace/bun"

// UserIdentity links a Crow user to an account at an external identity provider
// (for example an OpenID Connect issuer). Unlike UserForge it carries no tokens:
// it only proves who the user is at sign-in.
type UserIdentity struct {
	bun.BaseModel `bun:"table:user_identities"`

	ID int64 `json:"id" bun:"id,pk,autoincrement"`

	// UserID references the Crow user this identity belongs to.
	UserID int64 `json:"-" bun:"user_id,notnull"`

	// Provider names the identity provider kind, e.g. "oidc".
	Provider string `json:"provider" bun:"provider,type:varchar(50),notnull,unique:provider_subject"`

	// Issuer is the identity provider's issuer URL. A subject is only unique
	// within its issuer, so identities are keyed by provider, issuer and subject.
	Issuer string `json:"issuer" bun:"issuer,type:varchar(255),notnull,default:'',unique:provider_subject"`

	// Subject is the stable user identifier at the provider (the OIDC "sub" claim).
	Subject string `json:"subject" bun:"subject,type:varchar(255),notnull,unique:provider_subject"`

	// Email, Username and Groups are the values last seen at sign-in.
	Email    string          `json:"email,omitempty" bun:"email,type:varchar(500)"`
	Username string          `json:"username,omitempty" bun:"username,type:varchar(250)"`
	Groups   JSONSliceString `json:"groups" bun:"idp_groups,type:text"`

	// LinkedAt is the unix timestamp when this identity was linked.
	LinkedAt int64 `json:"linked_at" bun:"linked_at,notnull"`

	// LastLogin is the unix timestamp of the last sign-in through this identity.
	LastLogin int64 `json:"last_login,omitempty" bun:"last_login"`
} //	@name	UserIdentity
