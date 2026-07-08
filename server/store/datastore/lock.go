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
	"time"

	"codefloe.com/crowci/crow/v6/server/model"
)

const (
	// SecondsPerDay represents the number of seconds in 24 hours.
	SecondsPerDay = 24 * 60 * 60 // 86400 seconds
)

// DistributedLockAcquire attempts to acquire a distributed lock for HA coordination.
// Returns true if lock was acquired, false if already held by another instance.
// Uses a two-step approach to minimize TOCTOU race windows:
// 1. UPDATE existing row if expired or owned by us
// 2. INSERT with conflict suppression if row doesn't exist.
func (s storage) DistributedLockAcquire(lockName string, instanceID string, ttlSeconds int64) (bool, error) {
	ctx := context.Background()
	now := time.Now().Unix()
	expiresAt := now + ttlSeconds

	// Step 1: Try to claim via UPDATE — succeeds when the row is expired OR already ours.
	res, err := s.db.NewUpdate().Model((*model.DistributedLock)(nil)).
		Set("instance_id = ?", instanceID).
		Set("acquired_at = ?", now).
		Set("expires_at = ?", expiresAt).
		Where("name = ?", lockName).
		Where("(expires_at < ? OR instance_id = ?)", now, instanceID).
		Exec(ctx)
	if err != nil {
		return false, err
	}

	if affected, _ := res.RowsAffected(); affected > 0 {
		return true, nil
	}

	// Step 2: Row doesn't exist yet — INSERT, suppressing duplicate key.
	lock := &model.DistributedLock{
		Name:       lockName,
		InstanceID: instanceID,
		AcquiredAt: now,
		ExpiresAt:  expiresAt,
	}

	var insertRes interface{ RowsAffected() (int64, error) }
	if s.driver == DriverMysql {
		insertRes, err = s.db.NewInsert().Model(lock).
			On("DUPLICATE KEY UPDATE acquired_at = acquired_at"). // no-op = DO NOTHING
			Exec(ctx)
	} else {
		insertRes, err = s.db.NewInsert().Model(lock).
			On("CONFLICT (name) DO NOTHING").
			Exec(ctx)
	}
	if err != nil {
		return false, nil //nolint:nilerr // duplicate key race is expected
	}

	affected, _ := insertRes.RowsAffected()
	return affected == 1, nil
}

// DistributedLockRelease releases a distributed lock held by this instance.
func (s storage) DistributedLockRelease(lockName string, instanceID string) error {
	_, err := s.db.NewDelete().Model((*model.DistributedLock)(nil)).
		Where("name = ?", lockName).
		Where("instance_id = ?", instanceID).
		Exec(context.Background())
	return err
}

// DistributedLockExtend extends the TTL of a lock held by this instance.
func (s storage) DistributedLockExtend(lockName string, instanceID string, ttlSeconds int64) (bool, error) {
	now := time.Now().Unix()
	expiresAt := now + ttlSeconds

	res, err := s.db.NewUpdate().Model((*model.DistributedLock)(nil)).
		Set("expires_at = ?", expiresAt).
		Where("name = ?", lockName).
		Where("instance_id = ?", instanceID).
		Exec(context.Background())
	if err != nil {
		return false, err
	}

	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

// DistributedLockCleanup removes expired locks to prevent accumulation.
func (s storage) DistributedLockCleanup() error {
	now := time.Now().Unix()
	_, err := s.db.NewDelete().Model((*model.DistributedLock)(nil)).
		Where("expires_at < ?", now).
		Exec(context.Background())
	return err
}

// DistributedLockList lists all currently held locks (for debugging/monitoring).
func (s storage) DistributedLockList() ([]*model.DistributedLock, error) {
	locks := make([]*model.DistributedLock, 0)
	err := s.db.NewSelect().Model(&locks).Scan(context.Background())
	return locks, err
}

// DistributedLockInit initializes known locks in the database to prevent INSERT race conditions.
// This should be called during migration or startup.
func (s storage) DistributedLockInit() error {
	ctx := context.Background()
	knownLocks := []string{
		"queue:assignment:leader",
		"queue:processor:leader",
		"cron:scheduler:leader",
	}

	now := time.Now().Unix()

	for _, lockName := range knownLocks {
		// Check if lock already exists
		exists, err := s.db.NewSelect().Model((*model.DistributedLock)(nil)).
			Where("name = ?", lockName).
			Exists(ctx)
		if err != nil {
			return err
		}

		if !exists {
			// Create expired lock that can be immediately acquired
			// Use a timestamp in the past to ensure it's expired
			lock := &model.DistributedLock{
				Name:       lockName,
				InstanceID: "init-placeholder",
				AcquiredAt: now - SecondsPerDay, // 24 hours ago
				ExpiresAt:  now - SecondsPerDay, // Already expired 24 hours ago
			}

			if _, err := s.db.NewInsert().Model(lock).Exec(ctx); err != nil {
				// Ignore duplicate key errors - another instance may have created it
				continue
			}
		}
		// Don't touch existing locks - they may be legitimately held by other instances
	}

	return nil
}
