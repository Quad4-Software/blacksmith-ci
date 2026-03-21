// Copyright 2024 Woodpecker Authors
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

type ForgeType string

const (
	ForgeTypeGithub              ForgeType = "github"
	ForgeTypeGitlab              ForgeType = "gitlab"
	ForgeTypeGitea               ForgeType = "gitea"
	ForgeTypeForgejo             ForgeType = "forgejo"
	ForgeTypeBitbucket           ForgeType = "bitbucket"
	ForgeTypeBitbucketDatacenter ForgeType = "bitbucket-dc"
	ForgeTypeAddon               ForgeType = "addon"
)

type Forge struct {
	bun.BaseModel `bun:"table:forges"`

	ID                int64            `json:"id"                           bun:"id,pk,autoincrement"`
	Type              ForgeType        `json:"type"                         bun:"type,type:varchar(250)"`
	URL               string           `json:"url"                          bun:"url,type:varchar(500)"`
	Client            string           `json:"client,omitempty"             bun:"client,type:varchar(250)"`
	ClientSecret      string           `json:"-"                            bun:"client_secret,type:varchar(250)"`
	SkipVerify        bool             `json:"skip_verify,omitempty"        bun:"skip_verify"`
	OAuthHost         string           `json:"oauth_host,omitempty"         bun:"oauth_host,type:varchar(250)"`
	Icon              string           `json:"icon,omitempty"               bun:"icon,type:varchar(250)"`
	AdditionalOptions JSONMapStringAny `json:"additional_options,omitempty" bun:"additional_options,type:text"`
} //	@name	Forge

// TableName returns the database table name.
func (Forge) TableName() string {
	return "forges"
}

// PublicCopy returns a copy of the forge without sensitive information and technical details.
func (f *Forge) PublicCopy() *Forge {
	forge := &Forge{
		ID:   f.ID,
		Type: f.Type,
		URL:  f.URL,
		Icon: f.Icon,
	}

	return forge
}
