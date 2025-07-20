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

// DistributedLockAcquire attempts to acquire a distributed lock for HA coordination.
// Returns true if lock was acquired, false if already held by another instance.
func (s storage) DistributedLockAcquire(lockName string, instanceID string, ttlSeconds int64) (bool, error) {
	now := time.Now().Unix()
	expiresAt := now + ttlSeconds

	lock := &model.DistributedLock{
		Name:       lockName,
		InstanceID: instanceID,
		AcquiredAt: now,
		ExpiresAt:  expiresAt,
	}

	// Try to insert a new lock (will fail if lock already exists and not expired)
	affected, err := s.engine.Insert(lock)
	if err != nil {
		// If insert failed, try to acquire expired lock or extend our own lock
		return s.distributedLockTryExpiredOrExtend(lockName, instanceID, expiresAt)
	}

	return affected > 0, nil
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

// distributedLockTryExpiredOrExtend attempts to acquire expired locks or extend our own.
func (s storage) distributedLockTryExpiredOrExtend(lockName string, instanceID string, expiresAt int64) (bool, error) {
	now := time.Now().Unix()

	// Try to update expired locks or extend our own lock
	affected, err := s.engine.Where("name = ?", lockName).
		And(builder.Or(
			builder.Lt{"expires_at": now},         // Lock is expired
			builder.Eq{"instance_id": instanceID}, // Or it's our own lock
		)).
		Cols("instance_id", "acquired_at", "expires_at").
		Update(&model.DistributedLock{
			InstanceID: instanceID,
			AcquiredAt: now,
			ExpiresAt:  expiresAt,
		})

	return affected > 0, err
}
