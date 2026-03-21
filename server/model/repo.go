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
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

type ApprovalMode string

const (
	RequireApprovalNone         ApprovalMode = "none"          // require approval for no events
	RequireApprovalForks        ApprovalMode = "forks"         // require approval for PRs from forks (default)
	RequireApprovalPullRequests ApprovalMode = "pull_requests" // require approval for all PRs
	RequireApprovalAllEvents    ApprovalMode = "all_events"    // require approval for all external events
)

func (mode ApprovalMode) Valid() bool {
	switch mode {
	case RequireApprovalNone,
		RequireApprovalForks,
		RequireApprovalPullRequests,
		RequireApprovalAllEvents:
		return true
	default:
		return false
	}
}

// Repo represents a repository.
type Repo struct {
	bun.BaseModel `bun:"table:repos"`

	ID      int64 `json:"id,omitempty"                    bun:"id,pk,autoincrement"`
	UserID  int64 `json:"-"                               bun:"user_id"`
	ForgeID int64 `json:"forge_id,omitempty"              bun:"forge_id,unique:forge"`
	// ForgeRemoteID is the unique identifier for the repository on the forge.
	ForgeRemoteID                ForgeRemoteID         `json:"forge_remote_id"                 bun:"forge_remote_id"`
	OrgID                        int64                 `json:"org_id"                          bun:"org_id"`
	Owner                        string                `json:"owner"                           bun:"owner,unique:forge"`
	Name                         string                `json:"name"                            bun:"name,unique:forge"`
	FullName                     string                `json:"full_name"                       bun:"full_name,unique:forge_full_name"`
	Avatar                       string                `json:"avatar_url,omitempty"            bun:"avatar,type:varchar(500)"`
	ForgeURL                     string                `json:"forge_url,omitempty"             bun:"forge_url,type:varchar(1000)"`
	Clone                        string                `json:"clone_url,omitempty"             bun:"clone,type:varchar(1000)"`
	CloneSSH                     string                `json:"clone_url_ssh"                   bun:"clone_ssh,type:varchar(1000)"`
	Branch                       string                `json:"default_branch,omitempty"        bun:"branch,type:varchar(500)"`
	PREnabled                    bool                  `json:"pr_enabled"                      bun:"pr_enabled,default:true"`
	Timeout                      int64                 `json:"timeout,omitempty"               bun:"timeout"`
	Visibility                   RepoVisibility        `json:"visibility"                      bun:"visibility,type:varchar(10)"`
	IsSCMPrivate                 bool                  `json:"private"                         bun:"private"`
	Trusted                      TrustedConfiguration  `json:"trusted"                         bun:"trusted,type:text"`
	RequireApproval              ApprovalMode          `json:"require_approval"                bun:"require_approval,type:varchar(50)"`
	IsActive                     bool                  `json:"active"                          bun:"active"`
	AllowPull                    bool                  `json:"allow_pr"                        bun:"allow_pr"`
	AllowDeploy                  bool                  `json:"allow_deploy"                    bun:"allow_deploy"`
	AllowManual                  bool                  `json:"allow_manual"                    bun:"allow_manual"`
	DeployTeam                   string                `json:"deploy_team"                     bun:"deploy_team,type:varchar(250)"`
	Config                       string                `json:"config_file"                     bun:"config_path,type:varchar(500)"`
	Hash                         string                `json:"-"                               bun:"hash,type:varchar(500)"`
	Perm                         *Perm                 `json:"-"                               bun:"-"`
	CancelPreviousPipelineEvents JSONSliceWebhookEvent `json:"cancel_previous_pipeline_events" bun:"cancel_previous_pipeline_events,type:text"`
	NetrcTrustedPlugins          JSONSliceString       `json:"netrc_trusted"                   bun:"netrc_trusted,type:text"`
	LogsPipelinesKeepMin         int64                 `json:"logs_keep_min,omitempty"         bun:"logs_keep_min"`
	LogsDurationKeep             string                `json:"logs_keep_duration"              bun:"logs_keep_duration,type:varchar(500)"`
} //	@name	Repo

// TableName returns the database table name.
func (Repo) TableName() string {
	return "repos"
}

func (r *Repo) ResetVisibility() {
	r.Visibility = VisibilityPublic
	if r.IsSCMPrivate {
		r.Visibility = VisibilityPrivate
	}
}

// ParseRepo parses the repository owner and name from a string.
func ParseRepo(str string) (user, repo string, err error) {
	before, after, _ := strings.Cut(str, "/")
	if before == "" || after == "" {
		err = fmt.Errorf("invalid or missing repository (e.g. octocat/hello-world)")
		return user, repo, err
	}
	user = before
	repo = after
	return user, repo, err
}

// Update updates the repository with values from the given Repo.
func (r *Repo) Update(from *Repo) {
	if from.ForgeRemoteID.IsValid() {
		r.ForgeRemoteID = from.ForgeRemoteID
	}
	r.Owner = from.Owner
	r.Name = from.Name
	r.FullName = from.FullName
	r.Avatar = from.Avatar
	r.ForgeURL = from.ForgeURL
	r.PREnabled = from.PREnabled
	if len(from.Clone) > 0 {
		r.Clone = from.Clone
	}
	if len(from.CloneSSH) > 0 {
		r.CloneSSH = from.CloneSSH
	}
	r.Branch = from.Branch
	if from.IsSCMPrivate != r.IsSCMPrivate {
		if from.IsSCMPrivate {
			r.Visibility = VisibilityPrivate
		} else {
			r.Visibility = VisibilityPublic
		}
	}
	r.IsSCMPrivate = from.IsSCMPrivate
}

// RepoPatch represents a repository patch object.
type RepoPatch struct {
	Config                       *string                    `json:"config_file,omitempty"`
	RequireApproval              *string                    `json:"require_approval,omitempty"`
	Timeout                      *int64                     `json:"timeout,omitempty"`
	Visibility                   *string                    `json:"visibility,omitempty"`
	AllowPull                    *bool                      `json:"allow_pr,omitempty"`
	AllowDeploy                  *bool                      `json:"allow_deploy,omitempty"`
	AllowManual                  *bool                      `json:"allow_manual,omitempty"`
	DeployTeam                   *string                    `json:"deploy_team,omitempty"`
	CancelPreviousPipelineEvents *[]WebhookEvent            `json:"cancel_previous_pipeline_events,omitempty"`
	LogsPipelinesKeepMin         int64                      `json:"logs_keep_min,omitempty"`
	LogsDurationKeep             string                     `json:"logs_keep_duration,omitempty"`
	NetrcTrusted                 *[]string                  `json:"netrc_trusted,omitempty"`
	Trusted                      *TrustedConfigurationPatch `json:"trusted,omitempty"`
} //	@name	RepoPatch

type ForgeRemoteID string

func (r ForgeRemoteID) IsValid() bool {
	return r != "" && r != "0"
}

type TrustedConfiguration struct {
	Network  bool `json:"network"`
	Volumes  bool `json:"volumes"`
	Security bool `json:"security"`
}

func (t TrustedConfiguration) Value() (driver.Value, error) { return jsonValue(t) }
func (t *TrustedConfiguration) Scan(src any) error          { return jsonScan(src, t) }

type TrustedConfigurationPatch struct {
	Network  *bool `json:"network,omitempty"`
	Volumes  *bool `json:"volumes,omitempty"`
	Security *bool `json:"security,omitempty"`
}

// RepoLastPipeline represents a repository with last pipeline execution information.
type RepoLastPipeline struct {
	*Repo
	LastPipeline *Pipeline `json:"last_pipeline,omitempty"`
} //	@name	RepoLastPipeline
