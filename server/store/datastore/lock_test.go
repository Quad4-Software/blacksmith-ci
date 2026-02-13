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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
)

func TestDistributedLockAcquire(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	// Test acquiring a new lock
	acquired, err := store.DistributedLockAcquire("test-lock", "instance-1", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Test acquiring the same lock with different instance (should fail)
	acquired, err = store.DistributedLockAcquire("test-lock", "instance-2", 60)
	assert.NoError(t, err)
	assert.False(t, acquired)

	// Test acquiring the same lock with same instance (should succeed - extend)
	acquired, err = store.DistributedLockAcquire("test-lock", "instance-1", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Test acquiring a different lock
	acquired, err = store.DistributedLockAcquire("test-lock-2", "instance-2", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)
}

func TestDistributedLockAcquireExpired(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	// Create an expired lock
	now := time.Now().Unix()
	expiredLock := &model.DistributedLock{
		Name:       "expired-lock",
		InstanceID: "instance-1",
		AcquiredAt: now - 120,
		ExpiresAt:  now - 60, // Expired 60 seconds ago
	}
	_, err := store.engine.Insert(expiredLock)
	assert.NoError(t, err)

	// Another instance should be able to acquire the expired lock
	acquired, err := store.DistributedLockAcquire("expired-lock", "instance-2", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Verify the lock is now held by instance-2
	locks, err := store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "instance-2", locks[0].InstanceID)
}

func TestDistributedLockRelease(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	// Acquire a lock
	acquired, err := store.DistributedLockAcquire("release-test", "instance-1", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Release the lock
	err = store.DistributedLockRelease("release-test", "instance-1")
	assert.NoError(t, err)

	// Another instance should now be able to acquire it
	acquired, err = store.DistributedLockAcquire("release-test", "instance-2", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Try to release a lock held by different instance (should not error but do nothing)
	err = store.DistributedLockRelease("release-test", "instance-1")
	assert.NoError(t, err)

	// Verify lock is still held by instance-2
	locks, err := store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "instance-2", locks[0].InstanceID)
}

func TestDistributedLockExtend(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	// Acquire a lock
	acquired, err := store.DistributedLockAcquire("extend-test", "instance-1", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Get initial expiration
	locks, err := store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 1)
	initialExpiry := locks[0].ExpiresAt

	// Wait a moment and extend
	time.Sleep(2 * time.Second)
	extended, err := store.DistributedLockExtend("extend-test", "instance-1", 60)
	assert.NoError(t, err)
	assert.True(t, extended)

	// Verify expiration was extended
	locks, err = store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Greater(t, locks[0].ExpiresAt, initialExpiry)

	// Try to extend a lock held by different instance
	extended, err = store.DistributedLockExtend("extend-test", "instance-2", 60)
	assert.NoError(t, err)
	assert.False(t, extended)

	// Try to extend non-existent lock
	extended, err = store.DistributedLockExtend("non-existent", "instance-1", 60)
	assert.NoError(t, err)
	assert.False(t, extended)
}

func TestDistributedLockCleanup(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	now := time.Now().Unix()

	// Create some locks - mix of expired and active
	locks := []*model.DistributedLock{
		{
			Name:       "expired-1",
			InstanceID: "instance-1",
			AcquiredAt: now - 120,
			ExpiresAt:  now - 60, // Expired
		},
		{
			Name:       "active-1",
			InstanceID: "instance-2",
			AcquiredAt: now - 30,
			ExpiresAt:  now + 30, // Still active
		},
		{
			Name:       "expired-2",
			InstanceID: "instance-3",
			AcquiredAt: now - 90,
			ExpiresAt:  now - 30, // Expired
		},
	}

	for _, lock := range locks {
		_, err := store.engine.Insert(lock)
		assert.NoError(t, err)
	}

	// Run cleanup
	err := store.DistributedLockCleanup()
	assert.NoError(t, err)

	// Verify only active locks remain
	remainingLocks, err := store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, remainingLocks, 1)
	assert.Equal(t, "active-1", remainingLocks[0].Name)
}

func TestDistributedLockConcurrentAcquisition(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	// Test multiple sequential attempts to ensure no duplicate key errors
	const lockName = "concurrent-test"
	results := make([]bool, 5)

	// Simulate multiple instances trying to acquire the same lock
	for i := 0; i < 5; i++ {
		instanceName := "instance-" + string(rune('A'+i))
		acquired, err := store.DistributedLockAcquire(lockName, instanceName, 60)
		assert.NoError(t, err, "Should not get duplicate key error for instance %s", instanceName)
		results[i] = acquired
	}

	// Only the first should succeed
	assert.True(t, results[0], "First instance should acquire the lock")
	for i := 1; i < 5; i++ {
		assert.False(t, results[i], "Instance %d should not acquire the lock", i)
	}

	// Verify exactly one lock exists
	locks, err := store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, lockName, locks[0].Name)
	assert.Equal(t, "instance-A", locks[0].InstanceID)
}

func TestDistributedLockList(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	// Initially empty
	locks, err := store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 0)

	// Add some locks
	_, err = store.DistributedLockAcquire("lock-1", "instance-1", 60)
	assert.NoError(t, err)
	_, err = store.DistributedLockAcquire("lock-2", "instance-2", 60)
	assert.NoError(t, err)
	_, err = store.DistributedLockAcquire("lock-3", "instance-3", 60)
	assert.NoError(t, err)

	// List all locks
	locks, err = store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 3)

	// Verify all locks are present
	lockNames := make(map[string]bool)
	for _, lock := range locks {
		lockNames[lock.Name] = true
	}
	assert.True(t, lockNames["lock-1"])
	assert.True(t, lockNames["lock-2"])
	assert.True(t, lockNames["lock-3"])
}

func TestDistributedLockReacquisitionAfterExpiry(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedLock))
	defer closer()

	// Test that no duplicate key errors occur when reacquiring expired locks
	// This specifically tests the fix for the PostgreSQL duplicate key issue

	// Acquire a lock with very short TTL
	acquired, err := store.DistributedLockAcquire("short-ttl", "instance-1", 1)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Wait for it to expire
	time.Sleep(2 * time.Second)

	// Try to acquire from another instance - should succeed without errors
	acquired, err = store.DistributedLockAcquire("short-ttl", "instance-2", 60)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Verify the lock is now held by instance-2
	locks, err := store.DistributedLockList()
	assert.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "instance-2", locks[0].InstanceID)
}
