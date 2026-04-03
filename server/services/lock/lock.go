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

package lock

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/store"
)

// Service provides distributed locking capabilities for HA deployments.
type Service struct {
	store      store.Store
	instanceID string
	locks      sync.Map // map[string]*lockInfo
	cleanup    chan struct{}
}

// lockInfo tracks local lock state.
type lockInfo struct {
	name      string
	acquired  bool
	expiresAt time.Time
	mutex     sync.Mutex
}

// NewService creates a new distributed lock service.
func NewService(store store.Store, instanceID string) *Service {
	service := &Service{
		store:      store,
		instanceID: instanceID,
		cleanup:    make(chan struct{}),
	}

	log.Info().Str("instanceID", instanceID).Msg("Starting distributed lock service")

	// Start background cleanup goroutine
	go service.cleanupLoop()

	return service
}

// AcquireLock attempts to acquire a distributed lock with specified TTL.
// Returns a context that will be canceled when the lock is lost.
func (s *Service) AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (context.Context, context.CancelFunc, error) {
	ttlSeconds := int64(ttl.Seconds())
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}

	log.Debug().Str("lock", lockName).Str("instanceID", s.instanceID).Dur("ttl", ttl).Msg("Attempting to acquire distributed lock")

	// Try to acquire the lock
	acquired, err := s.store.DistributedLockAcquire(lockName, s.instanceID, ttlSeconds)
	if err != nil {
		log.Error().Err(err).Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Failed to acquire distributed lock")
		return nil, nil, err
	}

	if !acquired {
		log.Debug().Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Distributed lock already held by another instance")
		return nil, nil, ErrLockNotAcquired
	}

	log.Info().Str("lock", lockName).Str("instanceID", s.instanceID).Dur("ttl", ttl).Msg("Successfully acquired distributed lock")

	// Use WithCancel instead of WithTimeout so the lock lives as long as
	// renewals succeed. The renewal goroutine cancels the context on lock loss.
	lockCtx, cancel := context.WithCancelCause(ctx)
	info := &lockInfo{
		name:      lockName,
		acquired:  true,
		expiresAt: time.Now().Add(ttl),
	}

	s.locks.Store(lockName, info)

	// Start lock renewal goroutine — it will call cancel() if the lock is lost
	cancelFn := func() { cancel(nil) }
	go s.renewLock(lockCtx, cancelFn, lockName, ttl)

	return lockCtx, func() {
		_ = s.releaseLock(lockName)
		cancel(nil)
	}, nil
}

// TryLock attempts to acquire a lock without blocking.
func (s *Service) TryLock(lockName string, ttl time.Duration) (bool, error) {
	ttlSeconds := int64(ttl.Seconds())
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}

	log.Debug().Str("lock", lockName).Str("instanceID", s.instanceID).Dur("ttl", ttl).Msg("Attempting to acquire lock (non-blocking)")

	acquired, err := s.store.DistributedLockAcquire(lockName, s.instanceID, ttlSeconds)
	if err != nil {
		log.Error().Err(err).Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Failed to try lock")
		return false, err
	}

	if acquired {
		log.Info().Str("lock", lockName).Str("instanceID", s.instanceID).Dur("ttl", ttl).Msg("Successfully acquired lock (non-blocking)")
		info := &lockInfo{
			name:      lockName,
			acquired:  true,
			expiresAt: time.Now().Add(ttl),
		}
		s.locks.Store(lockName, info)
	} else {
		log.Debug().Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Lock not available (non-blocking)")
	}

	return acquired, nil
}

// ReleaseLock explicitly releases a lock.
func (s *Service) ReleaseLock(lockName string) error {
	return s.releaseLock(lockName)
}

// IsLocked checks if a lock is currently held by any instance.
func (s *Service) IsLocked(lockName string) (bool, error) {
	locks, err := s.store.DistributedLockList()
	if err != nil {
		return false, err
	}

	now := time.Now().Unix()
	for _, lock := range locks {
		if lock.Name == lockName && lock.ExpiresAt > now {
			return true, nil
		}
	}

	return false, nil
}

// GetLockHolder returns the instance ID that holds the lock, if any.
func (s *Service) GetLockHolder(lockName string) (string, error) {
	locks, err := s.store.DistributedLockList()
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	for _, lock := range locks {
		if lock.Name == lockName && lock.ExpiresAt > now {
			return lock.InstanceID, nil
		}
	}

	return "", nil
}

// GetInstanceID returns the instance ID for this lock service.
func (s *Service) GetInstanceID() string {
	return s.instanceID
}

// Close shuts down the lock service and releases all held locks.
func (s *Service) Close() error {
	close(s.cleanup)

	// Release all held locks
	s.locks.Range(func(key, value any) bool {
		if lockName, ok := key.(string); ok {
			_ = s.releaseLock(lockName)
		}
		return true
	})

	return nil
}

// releaseLock releases a specific lock.
func (s *Service) releaseLock(lockName string) error {
	log.Debug().Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Releasing distributed lock")
	s.locks.Delete(lockName)
	err := s.store.DistributedLockRelease(lockName, s.instanceID)
	if err != nil {
		log.Error().Err(err).Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Failed to release distributed lock")
	} else {
		log.Info().Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Successfully released distributed lock")
	}
	return err
}

// renewLock periodically renews a lock until context is canceled or lock is lost.
// On lock loss, it cancels the context to notify the holder.
func (s *Service) renewLock(ctx context.Context, cancelFn context.CancelFunc, lockName string, ttl time.Duration) {
	ticker := time.NewTicker(ttl / 3) // Renew at 1/3 of TTL
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.renewSingleLock(lockName, ttl); err != nil {
				log.Error().Err(err).Str("lock", lockName).Msg("Failed to renew lock — canceling lock context")
				cancelFn()
				return
			}
		}
	}
}

// renewSingleLock renews a single lock.
func (s *Service) renewSingleLock(lockName string, ttl time.Duration) error {
	ttlSeconds := int64(ttl.Seconds())
	log.Debug().Str("lock", lockName).Str("instanceID", s.instanceID).Dur("ttl", ttl).Msg("Renewing distributed lock")

	extended, err := s.store.DistributedLockExtend(lockName, s.instanceID, ttlSeconds)
	if err != nil {
		log.Error().Err(err).Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Failed to extend distributed lock")
		return err
	}

	if !extended {
		// Lock was lost, remove from local tracking
		log.Warn().Str("lock", lockName).Str("instanceID", s.instanceID).Msg("Distributed lock was lost during renewal")
		s.locks.Delete(lockName)
		return ErrLockLost
	}

	log.Debug().Str("lock", lockName).Str("instanceID", s.instanceID).Dur("ttl", ttl).Msg("Successfully renewed distributed lock")

	// Update local expiration time
	if value, ok := s.locks.Load(lockName); ok {
		if info, ok := value.(*lockInfo); ok {
			info.mutex.Lock()
			info.expiresAt = time.Now().Add(ttl)
			info.mutex.Unlock()
		}
	}

	return nil
}

// cleanupLoop periodically cleans up expired locks from the database.
func (s *Service) cleanupLoop() {
	log.Debug().Str("instanceID", s.instanceID).Msg("Starting distributed lock cleanup loop")
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanup:
			log.Debug().Str("instanceID", s.instanceID).Msg("Stopping distributed lock cleanup loop")
			return
		case <-ticker.C:
			log.Debug().Str("instanceID", s.instanceID).Msg("Running distributed lock cleanup")
			if err := s.store.DistributedLockCleanup(); err != nil {
				log.Error().Err(err).Str("instanceID", s.instanceID).Msg("Failed to cleanup expired locks")
			} else {
				log.Debug().Str("instanceID", s.instanceID).Msg("Completed distributed lock cleanup")
			}
		}
	}
}
