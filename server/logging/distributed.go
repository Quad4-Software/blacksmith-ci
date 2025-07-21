// Copyright 2025 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	logger "github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/pubsub"
	"codeberg.org/crowci/crow/v4/server/store"
)

// distributedLog implements a distributed logging system that synchronizes logs
// across multiple server instances using the pubsub system.
type distributedLog struct {
	// Local in-memory log store (same as the original)
	local Log

	// Pubsub system for distributing logs across instances
	pubsub pubsub.Publisher

	// Database store for accessing historical logs
	store store.Store

	// Instance ID to avoid echoing back our own messages
	instanceID string

	// Subscription management
	subscriptions map[int64]context.CancelCauseFunc
	subMu         sync.RWMutex
}

type logMessage struct {
	StepID     int64             `json:"step_id"`
	Entries    []*model.LogEntry `json:"entries"`
	InstanceID string            `json:"instance_id"`
	Type       string            `json:"type"` // "write" or "close"
}

// NewDistributed returns a new distributed logger that synchronizes across instances.
func NewDistributed(pubsub pubsub.Publisher, store store.Store, instanceID string) Log {
	return &distributedLog{
		local:         New(), // Use the existing in-memory implementation as local store
		pubsub:        pubsub,
		store:         store,
		instanceID:    instanceID,
		subscriptions: make(map[int64]context.CancelCauseFunc),
	}
}

func (d *distributedLog) Open(ctx context.Context, stepID int64) error {
	// Open locally
	err := d.local.Open(ctx, stepID)
	if err != nil {
		return err
	}

	// Populate local stream with historical logs from database
	step := &model.Step{ID: stepID}
	historicalLogs, err := d.store.LogFind(step)
	if err != nil {
		logger.Warn().Err(err).Int64("stepID", stepID).Msg("failed to load historical logs for distributed streaming")
	} else if len(historicalLogs) > 0 {
		// Write historical logs to local stream for immediate availability to new subscribers
		if err := d.local.Write(ctx, stepID, historicalLogs); err != nil {
			logger.Warn().Err(err).Int64("stepID", stepID).Msg("failed to populate local stream with historical logs")
		} else {
			logger.Debug().Int64("stepID", stepID).Int("count", len(historicalLogs)).Msg("populated local stream with historical logs")
		}
	}

	// Start listening for distributed log messages for this step
	d.subMu.Lock()
	defer d.subMu.Unlock()

	// Don't create multiple subscriptions for the same step
	if _, exists := d.subscriptions[stepID]; exists {
		return nil
	}

	// Create a context for this subscription derived from the provided context
	subCtx, cancel := context.WithCancelCause(ctx)
	d.subscriptions[stepID] = cancel

	// Subscribe to log messages for this step
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Msgf("distributed log subscription panic for step %d: %v", stepID, r)
			}
		}()

		d.pubsub.Subscribe(subCtx, func(m pubsub.Message) {
			// Only process log messages
			if m.Labels["type"] != "log" {
				return
			}

			var logMsg logMessage
			if err := json.Unmarshal(m.Data, &logMsg); err != nil {
				logger.Error().Err(err).Msg("failed to unmarshal distributed log message")
				return
			}

			// Ignore our own messages to avoid loops
			if logMsg.InstanceID == d.instanceID {
				return
			}

			// Only process messages for this step
			if logMsg.StepID != stepID {
				return
			}

			switch logMsg.Type {
			case "write":
				// Write the received logs to our local store
				if err := d.local.Write(subCtx, logMsg.StepID, logMsg.Entries); err != nil {
					logger.Error().Err(err).Int64("stepID", logMsg.StepID).Msg("failed to write distributed log entries locally")
				}
			case "close":
				// Close the local stream
				if err := d.local.Close(subCtx, logMsg.StepID); err != nil {
					logger.Error().Err(err).Int64("stepID", logMsg.StepID).Msg("failed to close distributed log stream locally")
				}
			}
		})
	}()

	return nil
}

func (d *distributedLog) Write(ctx context.Context, stepID int64, entries []*model.LogEntry) error {
	// Write locally first - this handles real-time streaming to clients connected to this instance
	err := d.local.Write(ctx, stepID, entries)
	if err != nil {
		return err
	}

	// Broadcast to other instances for their real-time streaming clients
	logMsg := logMessage{
		StepID:     stepID,
		Entries:    entries,
		InstanceID: d.instanceID,
		Type:       "write",
	}

	data, err := json.Marshal(logMsg)
	if err != nil {
		logger.Error().Err(err).Msg("failed to marshal distributed log message")
		return nil // Don't fail the write operation for this
	}

	// Publish using the step ID as the topic to allow efficient filtering
	d.pubsub.Publish(pubsub.Message{
		Data: data,
		Labels: map[string]string{
			"type":     "log",
			"step_id":  fmt.Sprintf("%d", stepID),
			"instance": d.instanceID,
		},
	})

	return nil
}

func (d *distributedLog) Tail(ctx context.Context, stepID int64, handler LogChan) error {
	// Ensure the stream is open and subscribed to distributed messages
	if err := d.Open(ctx, stepID); err != nil {
		return err
	}

	// Tail from local store (which now receives distributed messages)
	return d.local.Tail(ctx, stepID, handler)
}

func (d *distributedLog) Close(ctx context.Context, stepID int64) error {
	// Stop subscription for this step
	d.subMu.Lock()
	if cancel, exists := d.subscriptions[stepID]; exists {
		cancel(nil)
		delete(d.subscriptions, stepID)
	}
	d.subMu.Unlock()

	// Close locally
	err := d.local.Close(ctx, stepID)
	if err != nil {
		return err
	}

	// Broadcast close message to other instances
	logMsg := logMessage{
		StepID:     stepID,
		InstanceID: d.instanceID,
		Type:       "close",
	}

	data, err := json.Marshal(logMsg)
	if err != nil {
		logger.Error().Err(err).Msg("failed to marshal distributed log close message")
		return nil // Don't fail the close operation for this
	}

	d.pubsub.Publish(pubsub.Message{
		Data: data,
		Labels: map[string]string{
			"type":     "log",
			"step_id":  fmt.Sprintf("%d", stepID),
			"instance": d.instanceID,
		},
	})

	return nil
}
