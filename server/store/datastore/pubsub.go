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
	"context"

	"codefloe.com/crowci/crow/v6/server/model"
)

// PubSubMessageInsert stores a message for distributed pub/sub.
func (s storage) PubSubMessageInsert(msg *model.DistributedMessage) error {
	_, err := s.db.NewInsert().Model(msg).Exec(context.Background())
	return err
}

// PubSubMessageListSince retrieves messages created at or after the given
// timestamp. The bound is inclusive because created_at has one-second
// resolution, so callers must deduplicate by message ID.
func (s storage) PubSubMessageListSince(timestamp int64) ([]*model.DistributedMessage, error) {
	messages := make([]*model.DistributedMessage, 0)
	err := s.db.NewSelect().Model(&messages).Where("created_at >= ?", timestamp).
		Order("created_at ASC").
		Scan(context.Background())
	return messages, err
}

// PubSubMessageCleanupOlderThan removes messages older than the given timestamp.
func (s storage) PubSubMessageCleanupOlderThan(timestamp int64) error {
	_, err := s.db.NewDelete().Model((*model.DistributedMessage)(nil)).Where("created_at < ?", timestamp).Exec(context.Background())
	return err
}
