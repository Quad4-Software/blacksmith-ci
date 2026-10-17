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

package pubsub

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

const (
	// Message cleanup interval.
	messageCleanupInterval = 1 * time.Minute

	// Message TTL - messages older than this are cleaned up.
	messageTTL = 5 * time.Minute

	// Polling interval for new messages.
	pollingInterval = 100 * time.Millisecond

	// Number of background goroutines started.
	backgroundRoutines = 2

	// Seconds before the newest seen created_at that each poll reads again.
	// The created_at column has one-second resolution and is set before the
	// row is committed, so a message can become visible after a newer one was
	// already polled. Re-reading a short window and deduplicating by ID
	// ensures such messages are still delivered exactly once.
	pollLookback int64 = 5
)

// messageCounter is used to generate unique message IDs.
var messageCounter atomic.Uint64

// DistributedPublisher implements a distributed pub/sub system using database storage.
type DistributedPublisher struct {
	store      store.Store
	instanceID string

	mu   sync.RWMutex
	subs map[*Receiver]context.Context

	// lastPolled is the newest created_at seen so far; it never moves backwards.
	// It and seen are only accessed by the poller goroutine.
	lastPolled int64
	// startedAt bounds the poll window so messages published before this
	// instance started are not delivered.
	startedAt int64
	// seen maps IDs of messages already handled to their created_at, so
	// overlapping polls do not deliver a message twice.
	seen map[string]int64

	ctx    context.Context
	cancel context.CancelCauseFunc
	wg     sync.WaitGroup
}

// NewDistributed creates a distributed publisher that works across multiple instances.
func NewDistributed(store store.Store, instanceID string) *DistributedPublisher {
	ctx, cancel := context.WithCancelCause(context.Background())

	p := &DistributedPublisher{
		store:      store,
		instanceID: instanceID,
		subs:       make(map[*Receiver]context.Context),
		ctx:        ctx,
		cancel:     cancel,
	}
	p.resetPollState(time.Now().Unix())

	log.Info().Str("instanceID", instanceID).Msg("Starting distributed pub/sub system")

	// Start background processes
	p.wg.Add(backgroundRoutines)
	go p.messagePoller()
	go p.messageCleanup()

	return p
}

// Publish publishes a message to all subscribers across all instances.
func (p *DistributedPublisher) Publish(message Message) {
	// Generate ID if not provided
	if message.ID == "" {
		message.ID = generateMessageID()
	}

	log.Debug().Str("messageID", message.ID).Str("instanceID", p.instanceID).Msg("Publishing distributed message")

	// Store message in database for distributed access
	dbMsg := &model.DistributedMessage{
		ID:         message.ID,
		Data:       message.Data,
		Labels:     message.Labels,
		CreatedAt:  time.Now().Unix(),
		InstanceID: p.instanceID,
	}

	if err := p.storeMessage(dbMsg); err != nil {
		log.Error().Err(err).Str("messageID", message.ID).Str("instanceID", p.instanceID).Msg("Failed to store pub/sub message")
		return
	}

	log.Debug().Str("messageID", message.ID).Str("instanceID", p.instanceID).Msg("Successfully stored distributed message")

	// Also publish locally for immediate delivery
	p.publishLocal(message)
}

// Subscribe subscribes to messages with the given receiver function.
func (p *DistributedPublisher) Subscribe(c context.Context, receiver Receiver) {
	p.mu.Lock()
	p.subs[&receiver] = c
	subCount := len(p.subs)
	p.mu.Unlock()

	log.Debug().Str("instanceID", p.instanceID).Int("subscriberCount", subCount).Msg("Added distributed pub/sub subscriber")

	// Wait for context cancellation
	<-c.Done()

	// Clean up subscription
	p.mu.Lock()
	delete(p.subs, &receiver)
	subCount = len(p.subs)
	p.mu.Unlock()

	log.Debug().Str("instanceID", p.instanceID).Int("subscriberCount", subCount).Msg("Removed distributed pub/sub subscriber")
}

// Close shuts down the distributed publisher.
func (p *DistributedPublisher) Close() error {
	p.cancel(nil)
	p.wg.Wait()
	return nil
}

// publishLocal publishes a message to local subscribers only.
func (p *DistributedPublisher) publishLocal(message Message) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for receiver, ctx := range p.subs {
		// Check if subscriber context is still active
		select {
		case <-ctx.Done():
			continue
		default:
			go (*receiver)(message)
		}
	}
}

// resetPollState starts the poll cursor at now with an empty seen-set.
func (p *DistributedPublisher) resetPollState(now int64) {
	p.lastPolled = now
	p.startedAt = now
	p.seen = make(map[string]int64)
}

// messagePoller polls for new messages from other instances.
func (p *DistributedPublisher) messagePoller() {
	defer p.wg.Done()

	log.Debug().Str("instanceID", p.instanceID).Dur("interval", pollingInterval).Msg("Starting distributed pub/sub message poller")

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			log.Debug().Str("instanceID", p.instanceID).Msg("Stopping distributed pub/sub message poller")
			return
		case <-ticker.C:
			p.pollMessages()
		}
	}
}

// pollMessages retrieves and processes new messages from the database.
func (p *DistributedPublisher) pollMessages() {
	since := max(p.lastPolled-pollLookback, p.startedAt)
	messages, err := p.getMessagesSince(since)
	if err != nil {
		log.Error().Err(err).Str("instanceID", p.instanceID).Msg("Failed to poll pub/sub messages")
		return
	}

	processedCount := 0
	for _, dbMsg := range messages {
		if dbMsg.CreatedAt > p.lastPolled {
			p.lastPolled = dbMsg.CreatedAt
		}

		if _, ok := p.seen[dbMsg.ID]; ok {
			continue
		}
		p.seen[dbMsg.ID] = dbMsg.CreatedAt

		// Skip messages from our own instance (already published locally)
		if dbMsg.InstanceID == p.instanceID {
			continue
		}

		// Convert to pub/sub message and publish locally
		message := Message{
			ID:     dbMsg.ID,
			Data:   dbMsg.Data,
			Labels: dbMsg.Labels,
		}

		log.Debug().Str("messageID", message.ID).Str("fromInstance", dbMsg.InstanceID).Str("toInstance", p.instanceID).Msg("Processing distributed message")
		p.publishLocal(message)
		processedCount++
	}

	if processedCount > 0 {
		log.Debug().Str("instanceID", p.instanceID).Int("processedCount", processedCount).Msg("Processed distributed messages")
	}

	// Forget IDs that fall outside the next poll window; they cannot be
	// returned again.
	cutoff := max(p.lastPolled-pollLookback, p.startedAt)
	for id, createdAt := range p.seen {
		if createdAt < cutoff {
			delete(p.seen, id)
		}
	}
}

// messageCleanup removes old messages from the database.
func (p *DistributedPublisher) messageCleanup() {
	defer p.wg.Done()

	log.Debug().Str("instanceID", p.instanceID).Dur("interval", messageCleanupInterval).Dur("ttl", messageTTL).Msg("Starting distributed pub/sub message cleanup")

	ticker := time.NewTicker(messageCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			log.Debug().Str("instanceID", p.instanceID).Msg("Stopping distributed pub/sub message cleanup")
			return
		case <-ticker.C:
			log.Debug().Str("instanceID", p.instanceID).Msg("Running distributed pub/sub message cleanup")
			cutoff := time.Now().Add(-messageTTL).Unix()
			if err := p.cleanupMessagesOlderThan(cutoff); err != nil {
				log.Error().Err(err).Str("instanceID", p.instanceID).Msg("Failed to cleanup old pub/sub messages")
			} else {
				log.Debug().Str("instanceID", p.instanceID).Msg("Completed distributed pub/sub message cleanup")
			}
		}
	}
}

// storeMessage stores a message in the database.
func (p *DistributedPublisher) storeMessage(msg *model.DistributedMessage) error {
	return p.store.PubSubMessageInsert(msg)
}

// getMessagesSince retrieves messages created at or after the given timestamp.
func (p *DistributedPublisher) getMessagesSince(timestamp int64) ([]*model.DistributedMessage, error) {
	return p.store.PubSubMessageListSince(timestamp)
}

// cleanupMessagesOlderThan removes messages older than the given timestamp.
func (p *DistributedPublisher) cleanupMessagesOlderThan(timestamp int64) error {
	return p.store.PubSubMessageCleanupOlderThan(timestamp)
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), messageCounter.Add(1))
}
