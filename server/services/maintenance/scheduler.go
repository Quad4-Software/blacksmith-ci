// Copyright 2025 Crow Authors
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

package maintenance

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v4/server/cron"
	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/store"
)

// Scheduler manages database maintenance scheduling.
type Scheduler struct {
	service Service
	store   store.Store
	ctx     context.Context
	cancel  context.CancelCauseFunc
	ticker  *time.Ticker
	done    chan struct{}
}

// NewScheduler creates a new maintenance scheduler.
func NewScheduler(service Service, store store.Store) *Scheduler {
	return &Scheduler{
		service: service,
		store:   store,
		done:    make(chan struct{}),
	}
}

// Start begins the maintenance scheduler.
func (s *Scheduler) Start() error {
	s.ctx, s.cancel = context.WithCancelCause(context.Background())

	// Start the scheduler loop
	go s.run()

	log.Info().Msg("Maintenance scheduler started")
	return nil
}

// Stop stops the maintenance scheduler.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel(nil)
	}

	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.done)
}

// run is the main scheduler loop.
func (s *Scheduler) run() {
	// Check every minute for maintenance tasks
	s.ticker = time.NewTicker(time.Minute)
	defer s.ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.done:
			return
		case <-s.ticker.C:
			s.checkMaintenance()
		}
	}
}

// checkMaintenance checks if maintenance should be run.
func (s *Scheduler) checkMaintenance() {
	now := time.Now().UTC()

	s.checkAction("vacuum", now)

	s.checkAction("kubernetes_cleanup", now)
}

// checkAction checks if a specific maintenance action should be run.
func (s *Scheduler) checkAction(actionType string, now time.Time) {
	log.Debug().Str("action", actionType).Time("now", now).Msg("Checking maintenance action")

	config, err := s.service.GetMaintenanceConfigByAction(actionType)
	if err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("SCHEDULER ERROR: Failed to get action config")
		return
	}

	if config == nil {
		log.Error().Str("action", actionType).Msg("SCHEDULER ERROR: Action config is nil")
		return
	}

	log.Debug().
		Str("action", actionType).
		Bool("enabled", config.Enabled).
		Str("schedule", config.Schedule).
		Time("last_run", config.LastRun).
		Int64("config_id", config.ID).
		Msg("SCHEDULER: Loaded action config from database")

	if !config.Enabled {
		log.Debug().Str("action", actionType).Bool("enabled", config.Enabled).Msg("Action is disabled, skipping")
		return
	}

	// Special check for kubernetes_cleanup - ensure it's available
	if actionType == "kubernetes_cleanup" {
		available := s.service.IsKubernetesCleanupAvailable()
		log.Debug().
			Str("action", actionType).
			Bool("available", available).
			Msg("Checking if Kubernetes cleanup is available")
		if !available {
			log.Debug().Str("action", actionType).Msg("Kubernetes cleanup not available, skipping")
			return
		}
	}

	// Use the last run time from the action's config
	var lastRun time.Time
	if !config.LastRun.IsZero() {
		lastRun = config.LastRun
	} else {
		// If no previous run, use a time far in the past to ensure first run
		lastRun = time.Unix(0, 0)
	}

	// Calculate the next execution time based on the last run
	nextExec, err := cron.CalcNewNext(config.Schedule, lastRun)
	if err != nil {
		log.Error().Err(err).Str("action", actionType).Str("schedule", config.Schedule).Msg("Failed to parse action schedule")
		return
	}

	log.Debug().
		Str("action", actionType).
		Str("schedule", config.Schedule).
		Time("last_run", lastRun).
		Time("next_exec", nextExec).
		Time("now", now).
		Bool("should_run", now.After(nextExec) || now.Equal(nextExec)).
		Msg("Calculated next execution time")

	// Check if it's time to run the action (if current time is past the scheduled time)
	if now.After(nextExec) || now.Equal(nextExec) {
		// Ensure we don't run too frequently (at least 30 seconds apart)
		timeSinceLastRun := now.Sub(lastRun)
		if timeSinceLastRun >= 30*time.Second {
			log.Debug().
				Str("action", actionType).
				Str("schedule", config.Schedule).
				Time("next_exec", nextExec).
				Time("last_run", lastRun).
				Dur("time_since_last_run", timeSinceLastRun).
				Msg("Running scheduled maintenance action")

			switch actionType {
			case "vacuum":
				if err := s.service.RunMaintenance(s.ctx); err != nil {
					log.Error().Err(err).Msg("Scheduled vacuum maintenance failed")
				}
			case "kubernetes_cleanup":
				if err := s.service.RunKubernetesCleanup(s.ctx, false); err != nil {
					log.Error().Err(err).Msg("Scheduled kubernetes cleanup failed")
				}
			default:
				log.Warn().Str("action", actionType).Msg("Unknown maintenance action type")
			}
		} else {
			log.Debug().
				Str("action", actionType).
				Dur("time_since_last_run", timeSinceLastRun).
				Msg("Skipping action - too soon since last run (< 30 seconds)")
		}
	} else {
		log.Debug().
			Str("action", actionType).
			Time("next_exec", nextExec).
			Time("now", now).
			Msg("Not time to run action yet")
	}
}

// UpdateSchedule updates the maintenance schedule.
func (s *Scheduler) UpdateSchedule(schedule string, enabled bool) error {
	config := &model.MaintenanceConfig{
		Enabled:  enabled,
		Schedule: schedule,
	}

	return s.service.UpdateMaintenanceConfig(config)
}

// RunManual runs maintenance manually (outside of schedule).
func (s *Scheduler) RunManual() error {
	return s.service.RunMaintenance(context.Background())
}
