// Copyright 2025 Crow CI Authors
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

package datastore

import (
	"codeberg.org/crowci/crow/v3/server/model"
)

// PubSubMessageInsert stores a message for distributed pub/sub.
func (s storage) PubSubMessageInsert(msg *model.DistributedMessage) error {
	_, err := s.engine.Insert(msg)
	return err
}

// PubSubMessageListAfter retrieves messages created after the given timestamp.
func (s storage) PubSubMessageListAfter(timestamp int64) ([]*model.DistributedMessage, error) {
	messages := make([]*model.DistributedMessage, 0)
	err := s.engine.Where("created_at > ?", timestamp).
		OrderBy("created_at ASC").
		Find(&messages)
	return messages, err
}

// PubSubMessageCleanupOlderThan removes messages older than the given timestamp.
func (s storage) PubSubMessageCleanupOlderThan(timestamp int64) error {
	_, err := s.engine.Where("created_at < ?", timestamp).Delete(&model.DistributedMessage{})
	return err
}
