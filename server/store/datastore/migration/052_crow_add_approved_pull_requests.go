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

package migration

import (
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

var crowAddApprovedPullRequests = xormigrate.Migration{
	ID: "crow-add-approved-pull-requests",
	MigrateSession: func(sess *xorm.Session) error {
		type approved_pull_requests struct { //nolint:revive,stylecheck
			ID         int64  `xorm:"pk autoincr 'id'"`
			RepoID     int64  `xorm:"UNIQUE(s) INDEX 'repo_id'"`
			Ref        string `xorm:"UNIQUE(s) 'ref'"`
			ApprovedBy string `xorm:"'approved_by'"`
			Created    int64  `xorm:"'created' NOT NULL DEFAULT 0 created"`
		}

		return sess.Sync(new(approved_pull_requests))
	},
}
