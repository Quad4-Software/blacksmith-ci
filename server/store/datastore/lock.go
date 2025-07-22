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
	"time"

	"xorm.io/builder"

	"codeberg.org/crowci/crow/v4/server/model"
)

const (
	// SecondsPerDay represents the number of seconds in 24 hours.
	SecondsPerDay = 24 * 60 * 60 // 86400 seconds
)

// DistributedLockAcquire attempts to acquire a distributed lock for HA coordination.
// Returns true if lock was acquired, false if already held by another instance.
func (s storage) DistributedLockAcquire(lockName string, instanceID string, ttlSeconds int64) (bool, error) {
	now := time.Now().Unix()
	expiresAt := now + ttlSeconds

	// Clean up this specific lock if it's expired
	// This ensures we don't wait for the periodic cleanup
	_, _ = s.engine.Where("name = ? AND expires_at < ?", lockName, now).
		Delete(&model.DistributedLock{})

	// Try to update the lock if it exists and is expired or owned by us
	affected, err := s.engine.Where("name = ?", lockName).
		And(builder.Or(
			builder.Lt{"expires_at": now},         // Lock is expired (shouldn't happen after cleanup above)
			builder.Eq{"instance_id": instanceID}, // Or it's our own lock
		)).
		Cols("instance_id", "acquired_at", "expires_at").
		Update(&model.DistributedLock{
			InstanceID: instanceID,
			AcquiredAt: now,
			ExpiresAt:  expiresAt,
		})
	if err != nil {
		return false, err
	}

	if affected > 0 {
		// Successfully acquired the lock
		return true, nil
	}

	// Check if lock exists
	exists, err := s.engine.Where("name = ?", lockName).Exist(&model.DistributedLock{})
	if err != nil {
		return false, err
	}

	if exists {
		// Lock exists and is held by another instance
		return false, nil
	}

	// Lock doesn't exist, create it
	lock := &model.DistributedLock{
		Name:       lockName,
		InstanceID: instanceID,
		AcquiredAt: now,
		ExpiresAt:  expiresAt,
	}

	_, err = s.engine.Insert(lock)
	if err != nil {
		// Another instance created it in the race, that's OK
		return false, nil
	}

	return true, nil
}

// DistributedLockRelease releases a distributed lock held by this instance.
func (s storage) DistributedLockRelease(lockName string, instanceID string) error {
	_, err := s.engine.Where("name = ? AND instance_id = ?", lockName, instanceID).
		Delete(&model.DistributedLock{})
	return err
}

// DistributedLockExtend extends the TTL of a lock held by this instance.
func (s storage) DistributedLockExtend(lockName string, instanceID string, ttlSeconds int64) (bool, error) {
	now := time.Now().Unix()
	expiresAt := now + ttlSeconds

	affected, err := s.engine.Where("name = ? AND instance_id = ?", lockName, instanceID).
		Cols("expires_at").Update(&model.DistributedLock{ExpiresAt: expiresAt})

	return affected > 0, err
}

// DistributedLockCleanup removes expired locks to prevent accumulation.
func (s storage) DistributedLockCleanup() error {
	now := time.Now().Unix()
	_, err := s.engine.Where(builder.Lt{"expires_at": now}).Delete(&model.DistributedLock{})
	return err
}

// DistributedLockList lists all currently held locks (for debugging/monitoring).
func (s storage) DistributedLockList() ([]*model.DistributedLock, error) {
	locks := make([]*model.DistributedLock, 0)
	err := s.engine.Find(&locks)
	return locks, err
}

// DistributedLockInit initializes known locks in the database to prevent INSERT race conditions.
// This should be called during migration or startup.
func (s storage) DistributedLockInit() error {
	knownLocks := []string{
		"queue:assignment:leader",
		"queue:processor:leader",
		"cron:scheduler:leader",
	}

	now := time.Now().Unix()

	for _, lockName := range knownLocks {
		// Check if lock already exists
		exists, err := s.engine.Where("name = ?", lockName).Exist(&model.DistributedLock{})
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

			if _, err := s.engine.Insert(lock); err != nil {
				// Ignore duplicate key errors - another instance may have created it
				continue
			}
		}
		// Don't touch existing locks - they may be legitimately held by other instances
	}

	return nil
}
