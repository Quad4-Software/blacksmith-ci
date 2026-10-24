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
	"github.com/uptrace/bun"
)

type Pipeline struct {
	bun.BaseModel `bun:"table:pipelines"`

	ID     int64 `json:"id"                      bun:"id,pk,autoincrement"`
	RepoID int64 `json:"-"                       bun:"repo_id,unique:s"`
	// OrgID, OrgName and RepoFullName are denormalized from the owning repo at
	// creation time so historic metrics survive repo/org renames and deletion
	// (mirroring the agent identity denormalized onto workflows).
	OrgID               int64                  `json:"-"                       bun:"org_id"`
	OrgName             string                 `json:"-"                       bun:"org_name,type:text"`
	RepoFullName        string                 `json:"-"                       bun:"repo_full_name,type:text"`
	Number              int64                  `json:"number"                  bun:"number,unique:s"`
	Author              string                 `json:"author"                  bun:"author"`
	Parent              int64                  `json:"parent"                  bun:"parent"`
	Event               WebhookEvent           `json:"event"                   bun:"event"`
	Status              StatusValue            `json:"status"                  bun:"status"`
	Errors              JSONSlicePipelineError `json:"errors"                  bun:"errors,type:text"`
	Created             int64                  `json:"created"                 bun:"created,notnull,default:0"`
	Updated             int64                  `json:"updated"                 bun:"updated,notnull,default:0"`
	Started             int64                  `json:"started"                 bun:"started"`
	Finished            int64                  `json:"finished"                bun:"finished"`
	DeployTo            string                 `json:"deploy_to"               bun:"deploy"`
	DeployTask          string                 `json:"deploy_task"             bun:"deploy_task"`
	Commit              string                 `json:"commit"                  bun:"commit"`
	Branch              string                 `json:"branch"                  bun:"branch"`
	Ref                 string                 `json:"ref"                     bun:"ref"`
	Refspec             string                 `json:"refspec"                 bun:"refspec"`
	Title               string                 `json:"title"                   bun:"title"`
	Message             string                 `json:"message"                 bun:"message,type:text"`
	Timestamp           int64                  `json:"timestamp"               bun:"timestamp"`
	Sender              string                 `json:"sender"                  bun:"sender"`
	Avatar              string                 `json:"author_avatar"           bun:"avatar,type:varchar(500)"`
	Email               string                 `json:"author_email"            bun:"email,type:varchar(500)"`
	ForgeURL            string                 `json:"forge_url"               bun:"forge_url"`
	Reviewer            string                 `json:"reviewed_by"             bun:"reviewer"`
	Reviewed            int64                  `json:"reviewed"                bun:"reviewed"`
	Workflows           []*Workflow            `json:"workflows,omitempty"     bun:"-"`
	ChangedFiles        JSONSliceString        `json:"changed_files,omitempty" bun:"changed_files,type:text"`
	AdditionalVariables JSONMapStringString    `json:"variables,omitempty"     bun:"additional_variables,type:text"`
	PullRequestLabels   JSONSliceString        `json:"pr_labels,omitempty"     bun:"pr_labels,type:text"`
	IsPrerelease        bool                   `json:"is_prerelease,omitempty" bun:"is_prerelease"`
	FromFork            bool                   `json:"from_fork,omitempty"     bun:"from_fork"`
	LogsDeleted         bool                   `json:"logs_deleted,omitempty"  bun:"logs_deleted"`
	IsAlwaysApproved    bool                   `json:"is_always_approved,omitempty" bun:"-"`
} //	@name	Pipeline

// TableName returns the database table name.
func (Pipeline) TableName() string {
	return "pipelines"
}

type PipelineFilter struct {
	Before        int64
	After         int64
	Branch        string
	Events        []WebhookEvent
	RefContains   string
	Status        StatusValue
	Search        string   // Search query for number, message, branch, author
	WorkflowNames []string // Filter by workflow names (matches any)
}

// IsMultiPipeline checks if step list contain more than one parent step.
func (p Pipeline) IsMultiPipeline() bool {
	return len(p.Workflows) > 1
}

type PipelineOptions struct {
	Branch    string            `json:"branch"`
	Ref       string            `json:"ref,omitempty"` // defaults to Branch; set to refs/tags/<tag> for a run against a tag
	Variables map[string]string `json:"variables"`
	Workflows []string          `json:"workflows,omitempty"` // specific workflows to run (if empty, all matching workflows are run)
} //	@name	PipelineOptions
