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

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	"codefloe.com/crowci/crow/v6/server/store"
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

	// Active streaming sessions. A step can be watched by several clients at
	// once, so each Tail call gets its own session.
	mu       sync.RWMutex
	sessions map[int64]map[*streamSession]struct{}
}

// streamSession tracks one client tailing a step. Only the goroutine running
// Tail sends to handler, so the handler is never written to after Tail returns.
type streamSession struct {
	stepID         int64
	lastLogID      int64 // ID of last log entry we've seen
	handler        LogChan
	ctx            context.Context
	notify         chan struct{} // Signals that new logs may be available
	closed         chan struct{} // Closed when the step's log stream is closed
	closeOnce      sync.Once
	reloadSignaled atomic.Bool // Track if we've already sent reload signal
}

// markClosed tells the session that the step finished.
func (s *streamSession) markClosed() {
	s.closeOnce.Do(func() { close(s.closed) })
}

// NewDistributed returns a new distributed logger.
func NewDistributed(pubsub pubsub.Publisher, store store.Store, instanceID string) Log {
	logger.Debug().Str("instanceID", instanceID).Msg("creating distributed logger for HA mode")
	return &distributedLog{
		store:      store,
		instanceID: instanceID,
		sessions:   make(map[int64]map[*streamSession]struct{}),
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

	// Wake the sessions so they poll now instead of on the next tick. Each
	// session sends its own "live streaming started" reload signal when it
	// picks up the first new entry, so Write never touches their channels.
	d.notifyActiveSessions(stepID)

	return nil
}

func (d *distributedLog) Tail(ctx context.Context, stepID int64, handler LogChan) error {
	session := &streamSession{
		stepID:  stepID,
		handler: handler,
		ctx:     ctx,
		notify:  make(chan struct{}, 1), // Buffered to avoid blocking
		closed:  make(chan struct{}),
	}

	d.mu.Lock()
	if d.sessions[stepID] == nil {
		d.sessions[stepID] = make(map[*streamSession]struct{})
	}
	d.sessions[stepID][session] = struct{}{}
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.sessions[stepID], session)
		if len(d.sessions[stepID]) == 0 {
			delete(d.sessions, stepID)
		}
		d.mu.Unlock()
	}()

	logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("started database streaming session")

	// Stream on this goroutine and block until the client leaves or the step's
	// stream is closed, like the in-memory logger does.
	d.streamFromDatabase(session)

	logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Err(ctx.Err()).Msg("streaming session ended")
	return ctx.Err()
}

func (d *distributedLog) Close(_ context.Context, stepID int64) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Each session flushes its remaining logs and sends the completion reload
	// signal on its own goroutine, so closing never blocks other steps.
	for session := range d.sessions[stepID] {
		session.markClosed()
	}
	if len(d.sessions[stepID]) > 0 {
		logger.Debug().Int64("stepID", stepID).Str("instanceID", d.instanceID).Msg("closed database streaming sessions")
	}

	return nil
}

// streamFromDatabase polls the database for new log entries and streams them.
func (d *distributedLog) streamFromDatabase(session *streamSession) {
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
		case <-session.closed:
			d.finishSession(session)
			return
		}
	}
}

// finishSession sends the logs written since the last poll and then the reload
// signal that tells the UI the step has completed.
func (d *distributedLog) finishSession(session *streamSession) {
	if err := d.pollNewLogs(session); err != nil {
		logger.Error().Err(err).Int64("stepID", session.stepID).Msg("failed to poll final logs")
	}

	logger.Debug().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("workflow completed - sending reload signal")
	reloadEntry := &model.LogEntry{
		ID:     -1, // Special ID to indicate this is a reload signal
		StepID: session.stepID,
		Time:   time.Now().Unix(),
		Line:   -1,
		Data:   []byte(`{"reload":true,"reason":"workflow_completed"}`),
		Type:   model.LogEntryMetadata,
	}
	select {
	case session.handler <- []*model.LogEntry{reloadEntry}:
	case <-session.ctx.Done():
	case <-time.After(closeSignalTimeout):
		logger.Debug().Int64("stepID", session.stepID).Str("instanceID", d.instanceID).Msg("could not send reload signal - client is not reading")
	}
}

// closeSignalTimeout bounds how long a closing session waits for a slow client
// to accept the completion signal.
const closeSignalTimeout = time.Second

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
	defer d.mu.RUnlock()

	for session := range d.sessions[stepID] {
		// Non-blocking send: a pending notification already triggers a poll.
		select {
		case session.notify <- struct{}{}:
		default:
		}
	}
}
