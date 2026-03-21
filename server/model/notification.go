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

import (
	"database/sql/driver"
	"errors"

	"github.com/uptrace/bun"
)

var ErrNotificationConfigNotFound = errors.New("notification config not found")

// NotificationEvent represents when a notification should be sent.
type NotificationEvent string

const (
	NotificationEventSuccess      NotificationEvent = "success"
	NotificationEventFailure      NotificationEvent = "failure"
	NotificationEventStatusChange NotificationEvent = "status_change"
)

// NotificationConfig stores per-repo email notification settings.
type NotificationConfig struct {
	bun.BaseModel `bun:"table:notification_configs"`

	ID        int64                      `json:"id"         bun:"id,pk,autoincrement"`
	RepoID    int64                      `json:"repo_id"    bun:"repo_id,notnull,unique"`
	IsEnabled bool                       `json:"is_enabled" bun:"is_enabled,default:true"`
	Events    JSONSliceNotificationEvent `json:"events"     bun:"events,type:text"`
	CreatedAt int64                      `json:"created_at" bun:",nullzero"`
	UpdatedAt int64                      `json:"updated_at" bun:",nullzero"`
} //	@name	NotificationConfig

// TableName returns the database table name.
func (NotificationConfig) TableName() string {
	return "notification_configs"
}

// NotificationConfigStore persists notification configuration to storage.
type NotificationConfigStore interface {
	NotificationConfigFind(repoID int64) (*NotificationConfig, error)
	NotificationConfigUpsert(*NotificationConfig) error
	NotificationConfigDelete(repoID int64) error
}

// NotifyYAMLConfig represents per-pipeline notification overrides from .crow.yml.
type NotifyYAMLConfig struct {
	Disabled bool                `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	Events   []NotificationEvent `json:"events,omitempty"   yaml:"events,omitempty"`
}

func (n NotifyYAMLConfig) Value() (driver.Value, error) { return jsonValue(n) }
func (n *NotifyYAMLConfig) Scan(src any) error          { return jsonScan(src, n) }

// EmailUnsubscribe records that a user has opted out of email notifications for a repo.
type EmailUnsubscribe struct {
	bun.BaseModel `bun:"table:email_unsubscribes"`

	ID        int64  `json:"id"         bun:"id,pk,autoincrement"`
	UserID    int64  `json:"user_id"    bun:"user_id,notnull"`
	RepoID    int64  `json:"repo_id"    bun:"repo_id,notnull"`
	Email     string `json:"email"      bun:"email,notnull"`
	CreatedAt int64  `json:"created_at" bun:",nullzero"`
}

// EmailUnsubscribeStore persists email unsubscribe records.
type EmailUnsubscribeStore interface {
	EmailUnsubscribeFind(userID, repoID int64) (*EmailUnsubscribe, error)
	EmailUnsubscribeCreate(*EmailUnsubscribe) error
	EmailUnsubscribeDelete(userID, repoID int64) error
}
