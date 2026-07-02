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
	"sort"
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pubsub"
	"codefloe.com/crowci/crow/v5/server/store"
)

// distributedLog implements a database-streaming distributed logging system.
// Strategy:
// 1. All servers stream logs from database (not from agent connections)
// 2. Database polling for new log entries provides consistent live streaming
// 3. Works correctly in HA mode regardless of which server users connect to.
type distributedLog struct {
	// Database store for both historical and live log access
	store store.Store

	// Instance ID for this server
	instanceID string

	// Active streaming sessions
	mu       sync.RWMutex
	sessions map[int64]*streamSession
}

// streamSession tracks a live streaming session for a step.
type streamSession struct {
	stepID         int64
	lastLogID      int64 // ID of last log entry we've seen
	handler        LogChan
	ctx            context.Context
	cancel         context.CancelCauseFunc
	notify         chan struct{} // Channel to notify of new logs
	reloadSignaled atomic.Bool   // Track if we've already sent reload signal
}

// NewDistributed returns a new distributed logger.
func NewDistributed(pubsub pubsub.Publisher, store store.Store, instanceID string) Log {
	logger.Debug().Str("instanceID", instanceID).Msg("creating distributed logger for HA mode")
	return &distributedLog{
		store:      store,
		instanceID: instanceID,
		sessions:   make(map[int64]*streamSession),
	}
}

func (d *distributedLog) Open(ctx context.Context, stepID int64) error {
	// In database-streaming mode, opening just means we're ready to stream
	// The actual historical log loading happens when Tail is called
	logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("opened distributed log stream")
	return nil
}

// deduplicateLogEntries removes duplicate log entries based on line number.
// In distributed systems, the same log line might be stored multiple times by different servers.
func deduplicateLogEntries(entries []*model.LogEntry) []*model.LogEntry {
	if len(entries) == 0 {
		return entries
	}

	// Use a map to track unique entries by line number
	seen := make(map[int]bool)
	deduplicated := make([]*model.LogEntry, 0, len(entries))

	for _, entry := range entries {
		if entry == nil {
			continue
		}

		// Check if we've already seen this line number
		if !seen[entry.Line] {
			seen[entry.Line] = true
			deduplicated = append(deduplicated, entry)
		}
	}

	// Sort by line number to ensure proper order
	sort.Slice(deduplicated, func(i, j int) bool {
		return deduplicated[i].Line < deduplicated[j].Line
	})

	return deduplicated
}

func (d *distributedLog) Write(ctx context.Context, stepID int64, entries []*model.LogEntry) error {
	// In database-streaming mode, logs are stored to database by RPC layer
	// We only notify active sessions to trigger immediate polling
	// This reduces database load while still providing real-time streaming

	logger.Debug().Int64("stepID", stepID).Int("entryCount", len(entries)).Str("instanceID", d.instanceID).Msg("distributed logger Write() called")

	// Check if this is the first live log from an external agent for an active session
	d.mu.RLock()
	session, exists := d.sessions[stepID]
	d.mu.RUnlock()

	if exists && len(entries) > 0 && !session.reloadSignaled.Load() {
		// This appears to be the first live log for this session
		// Send a special reload signal to trigger page refresh
		logger.Info().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("first live log detected - sending page reload signal")

		reloadEntry := &model.LogEntry{
			ID:     -1, // Special ID to indicate this is a reload signal
			StepID: stepID,
			Time:   entries[0].Time,
			Line:   -1,
			Data:   []byte(`{"reload":true,"reason":"live_streaming_started"}`),
			Type:   model.LogEntryMetadata,
		}

		// Send reload signal immediately
		select {
		case session.handler <- []*model.LogEntry{reloadEntry}:
			logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("sent reload signal to UI")
			session.reloadSignaled.Store(true) // Mark that we've sent the reload signal
		default:
			// Channel full, skip reload signal
		}
	}

	// Notify active sessions that new logs are available
	// This triggers immediate polling instead of waiting for the next 100ms tick
	d.notifyActiveSessions(stepID)

	return nil
}

func (d *distributedLog) Tail(ctx context.Context, stepID int64, handler LogChan) error {
	d.mu.Lock()

	// Check if we already have an active session for this step
	if session, exists := d.sessions[stepID]; exists {
		// Cancel existing session
		session.cancel(nil)
		delete(d.sessions, stepID)
	}

	// Create a new streaming session
	sessionCtx, cancel := context.WithCancelCause(ctx)
	session := &streamSession{
		stepID:    stepID,
		lastLogID: 0,
		handler:   handler,
		ctx:       sessionCtx,
		cancel:    cancel,
		notify:    make(chan struct{}, 1), // Buffered to avoid blocking
	}
	d.sessions[stepID] = session

	// Release the mutex before starting the goroutine and blocking
	d.mu.Unlock()

	// Start the streaming goroutine
	go d.streamFromDatabase(session)

	logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("started database streaming session")

	// Block until the context is canceled (like the original logger does)
	// This prevents the API handler from closing the connection immediately
	<-sessionCtx.Done()

	logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Err(sessionCtx.Err()).Msg("streaming session ended")
	return sessionCtx.Err()
}

func (d *distributedLog) Close(ctx context.Context, stepID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Send reload signal before closing to notify UI that workflow has ended
	if session, exists := d.sessions[stepID]; exists {
		// Send reload signal to trigger UI update for workflow completion
		{ // Always send on close to update final status
			logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("workflow completed - sending reload signal")

			reloadEntry := &model.LogEntry{
				ID:     -1, // Special ID to indicate this is a reload signal
				StepID: stepID,
				Time:   time.Now().Unix(),
				Line:   -1,
				Data:   []byte(`{"reload":true,"reason":"workflow_completed"}`),
				Type:   model.LogEntryMetadata,
			}

			// Try to send reload signal (non-blocking since we're closing anyway)
			select {
			case session.handler <- []*model.LogEntry{reloadEntry}:
				logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("sent reload signal for workflow completion")
			default:
				logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("could not send reload signal - channel full or closed")
			}

			// Give a brief moment for the reload signal to be processed
			time.Sleep(100 * time.Millisecond)
		}

		// Now cancel and remove the streaming session
		session.cancel(nil)
		delete(d.sessions, stepID)
		logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("closed database streaming session")
	}

	return nil
}

// streamFromDatabase polls the database for new log entries and streams them.
func (d *distributedLog) streamFromDatabase(session *streamSession) {
	defer func() {
		// Clean up session when goroutine exits
		d.mu.Lock()
		delete(d.sessions, session.stepID)
		d.mu.Unlock()
	}()

	ticker := time.NewTicker(1 * time.Second) // Poll every 1 second (reduced from 100ms to lower DB load)
	defer ticker.Stop()

	// First, send all historical logs
	historicalLogCount, err := d.sendHistoricalLogs(session)
	if err != nil {
		logger.Error().Err(err).Int64("stepID", session.stepID).Msg("failed to send historical logs")
		return
	}

	// If there were historical logs, send reload signal to ensure UI updates
	if historicalLogCount > 0 && !session.reloadSignaled.Load() {
		logger.Info().Int64("stepID", session.stepID).Int("historicalLogs", historicalLogCount).Str("instanceID", d.instanceID).Msg("step has logs - sending page reload signal")

		reloadEntry := &model.LogEntry{
			ID:     -1, // Special ID to indicate this is a reload signal
			StepID: session.stepID,
			Time:   time.Now().Unix(),
			Line:   -1,
			Data:   []byte(`{"reload":true,"reason":"step_has_logs"}`),
			Type:   model.LogEntryMetadata,
		}

		// Send reload signal - make it blocking to ensure it's sent
		select {
		case <-session.ctx.Done():
			return
		case session.handler <- []*model.LogEntry{reloadEntry}:
			logger.Debug().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("sent reload signal for existing logs")
			session.reloadSignaled.Store(true) // Mark that we've sent the reload signal
		}
	}

	logger.Debug().Int64("stepID", session.stepID).Int("historicalLogs", historicalLogCount).Str("instanceID", d.instanceID).Msg("sent historical logs, starting live polling")

	// Then poll for new logs
	firstPoll := true
	for {
		select {
		case <-session.ctx.Done():
			logger.Debug().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Err(session.ctx.Err()).Msg("database streaming session canceled")
			return
		case <-ticker.C:
			// On first poll after historical logs, ensure reload signal was sent if needed
			if firstPoll && historicalLogCount > 0 && !session.reloadSignaled.Load() {
				logger.Warn().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("reload signal was not sent after historical logs, sending now")

				reloadEntry := &model.LogEntry{
					ID:     -1, // Special ID to indicate this is a reload signal
					StepID: session.stepID,
					Time:   time.Now().Unix(),
					Line:   -1,
					Data:   []byte(`{"reload":true,"reason":"step_has_logs_retry"}`),
					Type:   model.LogEntryMetadata,
				}

				select {
				case session.handler <- []*model.LogEntry{reloadEntry}:
					logger.Info().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("sent delayed reload signal")
					session.reloadSignaled.Store(true)
				default:
					logger.Error().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("failed to send delayed reload signal - channel full")
				}
			}
			firstPoll = false

			if err := d.pollNewLogs(session); err != nil {
				logger.Error().Err(err).Int64("stepID", session.stepID).Msg("failed to poll new logs")
				// Continue polling despite errors
			}
		case <-session.notify:
			// New logs available, poll immediately
			if err := d.pollNewLogs(session); err != nil {
				logger.Error().Err(err).Int64("stepID", session.stepID).Msg("failed to poll new logs after notification")
			}
			// Drain any extra notifications
			select {
			case <-session.notify:
			default:
			}
		}
	}
}

// sendHistoricalLogs sends all existing logs for the step.
// Returns the number of logs sent and any error.
func (d *distributedLog) sendHistoricalLogs(session *streamSession) (int, error) {
	step := &model.Step{ID: session.stepID}
	logs, err := d.store.LogFind(step)
	if err != nil {
		logger.Error().Err(err).Int64("stepID", session.stepID).Msg("failed to query historical logs from database")
		return 0, err
	}

	logger.Debug().Int64("stepID", session.stepID).Int("historicalLogs", len(logs)).Str("instanceID", d.instanceID).Msg("queried historical logs from database")

	if len(logs) == 0 {
		logger.Debug().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("no historical logs found in database")
		return 0, nil
	}

	// Deduplicate and sort logs
	deduplicatedLogs := deduplicateLogEntries(logs)

	// Send logs via handler (LogChan expects slices, so we send individual entries as single-item slices)
	for _, logEntry := range deduplicatedLogs {
		select {
		case <-session.ctx.Done():
			return len(deduplicatedLogs), nil
		case session.handler <- []*model.LogEntry{logEntry}:
			// Update last seen log ID
			if logEntry.ID > session.lastLogID {
				session.lastLogID = logEntry.ID
			}
		}
	}

	logger.Debug().Int64("stepID", session.stepID).Int("count", len(deduplicatedLogs)).Str("instanceID", d.instanceID).Msg("sent historical logs")
	return len(deduplicatedLogs), nil
}

// pollNewLogs checks for new log entries since the last poll.
func (d *distributedLog) pollNewLogs(session *streamSession) error {
	// Query for logs newer than our last seen log ID
	logs, err := d.store.LogFindAfter(session.stepID, session.lastLogID)
	if err != nil {
		logger.Error().Err(err).Int64("stepID", session.stepID).Int64("afterLogID", session.lastLogID).Msg("failed to query database for new logs")
		return err
	}

	if len(logs) == 0 {
		// No new logs - this is normal, don't log every empty poll
		return nil
	}

	// Check if this is the first live log and send reload signal if needed
	if !session.reloadSignaled.Load() {
		logger.Info().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("first live log detected via polling - sending page reload signal")

		reloadEntry := &model.LogEntry{
			ID:     -1, // Special ID to indicate this is a reload signal
			StepID: session.stepID,
			Time:   logs[0].Time,
			Line:   -1,
			Data:   []byte(`{"reload":true,"reason":"live_streaming_started"}`),
			Type:   model.LogEntryMetadata,
		}

		// Send reload signal immediately
		select {
		case <-session.ctx.Done():
			return nil
		case session.handler <- []*model.LogEntry{reloadEntry}:
			logger.Debug().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("sent reload signal to UI via polling")
			session.reloadSignaled.Store(true) // Mark that we've sent the reload signal
		default:
			// Channel full, skip reload signal but continue with logs
		}
	}

	// Sort by ID to ensure correct order
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].ID < logs[j].ID
	})

	// Send new logs via handler (LogChan expects slices, so we send individual entries as single-item slices)
	for _, logEntry := range logs {
		select {
		case <-session.ctx.Done():
			return nil
		case session.handler <- []*model.LogEntry{logEntry}:
			// Update last seen log ID
			session.lastLogID = logEntry.ID
		}
	}

	logger.Debug().Int64("stepID", session.stepID).Int("newLogs", len(logs)).Str("instanceID", d.instanceID).Msg("sent new logs from database polling")
	return nil
}

// notifyActiveSessions notifies active streaming sessions that new logs may be available.
func (d *distributedLog) notifyActiveSessions(stepID int64) {
	d.mu.RLock()
	session, exists := d.sessions[stepID]
	d.mu.RUnlock()

	if exists {
		// Non-blocking send to notify channel
		select {
		case session.notify <- struct{}{}:
			logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("notified active streaming session of new logs")
		default:
			logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("notification channel full, already pending")
		}
	} else {
		logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("no active streaming session found for step")
	}
}
