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

type ApprovedPullRequest struct {
	ID         int64  `json:"id"          xorm:"pk autoincr 'id'"`
	RepoID     int64  `json:"repo_id"     xorm:"UNIQUE(s) INDEX 'repo_id'"`
	Ref        string `json:"ref"         xorm:"UNIQUE(s) 'ref'"`
	ApprovedBy string `json:"approved_by" xorm:"'approved_by'"`
	Created    int64  `json:"created"     xorm:"'created' NOT NULL DEFAULT 0 created"`
}

func (ApprovedPullRequest) TableName() string {
	return "approved_pull_requests"
}
