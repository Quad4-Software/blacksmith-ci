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

import "github.com/uptrace/bun"

type Redirection struct {
	bun.BaseModel `bun:"table:redirections"`

	ID       int64  `bun:"id,pk,autoincrement"`
	ForgeID  int64  `bun:"forge_id,unique:forge"`
	RepoID   int64  `bun:"repo_id"`
	FullName string `bun:"repo_full_name,unique:forge"`
}

func (r Redirection) TableName() string {
	return "redirections"
}
