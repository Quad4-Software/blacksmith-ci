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

	"codeberg.org/crowci/crow/v3/server/cron"
	"codeberg.org/crowci/crow/v3/server/model"
	"codeberg.org/crowci/crow/v3/server/store"
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
	config := s.service.GetMaintenanceConfig()
	if config == nil || !config.Enabled {
		return
	}

	now := time.Now().UTC()

	// Parse the cron schedule to get next execution time
	nextExec, err := cron.CalcNewNext(config.Schedule, now)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse maintenance schedule")
		return
	}

	// Check if it's time to run maintenance (within the last minute)
	if now.After(nextExec) && now.Sub(nextExec) < time.Minute {
		// Get the last maintenance log to check if we've already run recently
		logs, err := s.store.MaintenanceLogList(1)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get maintenance logs")
			return
		}

		// If we have a recent log entry (within the last 2 minutes), skip
		if len(logs) > 0 {
			lastRun := time.Unix(logs[0].Created, 0)
			if now.Sub(lastRun) < 2*time.Minute {
				return
			}
		}

		log.Info().Msg("Running scheduled maintenance")
		if err := s.service.RunMaintenance(s.ctx); err != nil {
			log.Error().Err(err).Msg("Scheduled maintenance failed")
		}
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
