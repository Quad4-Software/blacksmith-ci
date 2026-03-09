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

import "errors"

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
	ID        int64               `json:"id"         xorm:"pk autoincr 'id'"`
	RepoID    int64               `json:"repo_id"    xorm:"NOT NULL UNIQUE INDEX 'repo_id'"`
	IsEnabled bool                `json:"is_enabled" xorm:"DEFAULT true 'is_enabled'"`
	Events    []NotificationEvent `json:"events"     xorm:"json 'events'"`
	CreatedAt int64               `json:"created_at" xorm:"created"`
	UpdatedAt int64               `json:"updated_at" xorm:"updated"`
} //	@name	NotificationConfig

// TableName returns the database table name for xorm.
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
