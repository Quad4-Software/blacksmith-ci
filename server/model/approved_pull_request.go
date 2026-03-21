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

type ApprovedPullRequest struct {
	bun.BaseModel `bun:"table:approved_pull_requests"`

	ID         int64  `json:"id"          bun:"id,pk,autoincrement"`
	RepoID     int64  `json:"repo_id"     bun:"repo_id,unique:s"`
	Ref        string `json:"ref"         bun:"ref,unique:s"`
	ApprovedBy string `json:"approved_by" bun:"approved_by"`
	Created    int64  `json:"created"     bun:"created,notnull,default:0"`
}

func (ApprovedPullRequest) TableName() string {
	return "approved_pull_requests"
}
