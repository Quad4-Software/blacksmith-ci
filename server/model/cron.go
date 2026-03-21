// Copyright 2022 Woodpecker Authors
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
	"fmt"

	"github.com/gdgvda/cron"
	"github.com/uptrace/bun"
)

type Cron struct {
	bun.BaseModel `bun:"table:crons"`

	ID        int64  `json:"id"         bun:"id,pk,autoincrement"`
	Name      string `json:"name"       bun:"name,unique:s"`
	RepoID    int64  `json:"repo_id"    bun:"repo_id,unique:s"`
	CreatorID int64  `json:"creator_id" bun:"creator_id"`
	NextExec  int64  `json:"next_exec"  bun:"next_exec"`
	Schedule  string `json:"schedule"   bun:"schedule,notnull"`
	Created   int64  `json:"created"    bun:"created,notnull,default:0"`
	Branch    string `json:"branch"     bun:"branch"`
	FailCount int    `json:"fail_count" bun:"fail_count,notnull,default:0"`
	FailMsg   string `json:"fail_msg"   bun:"fail_msg,type:text"`
	Disabled  bool   `json:"disabled"   bun:"disabled,notnull,default:false"`
} //	@name	Cron

// TableName returns the database table name.
func (Cron) TableName() string {
	return "crons"
}

// Validate ensures cron has a valid name and schedule.
func (c *Cron) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}

	if c.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}

	_, err := cron.ParseStandard(c.Schedule)
	if err != nil {
		return fmt.Errorf("can't parse schedule: %w", err)
	}

	return nil
}

// CronWithRepo extends Cron with repository context for user views.
type CronWithRepo struct {
	*Cron
	RepoName          string `json:"repo_name"`
	RepoOwner         string `json:"repo_owner"`
	RepoDefaultBranch string `json:"repo_default_branch"`
	ForgeID           int64  `json:"forge_id,omitempty"`   // forge ID for multi-forge display
	ForgeName         string `json:"forge_name,omitempty"` // forge hostname for multi-forge display
	ForgeIcon         string `json:"forge_icon,omitempty"` // forge icon name/URL for multi-forge display
	ForgeType         string `json:"forge_type,omitempty"` // forge type (github, gitlab, etc.) for icon fallback
}

// WithRepo creates a CronWithRepo from a Cron with repo details.
func (c *Cron) WithRepo(repoName, repoOwner, repoDefaultBranch string, forgeID int64, forgeName, forgeIcon, forgeType string) *CronWithRepo {
	return &CronWithRepo{
		Cron:              c,
		RepoName:          repoName,
		RepoOwner:         repoOwner,
		RepoDefaultBranch: repoDefaultBranch,
		ForgeID:           forgeID,
		ForgeName:         forgeName,
		ForgeIcon:         forgeIcon,
		ForgeType:         forgeType,
	}
}
