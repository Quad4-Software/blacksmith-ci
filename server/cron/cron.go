// Copyright 2022 Woodpecker Authors
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

package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/gdgvda/cron"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v3/server"
	"codeberg.org/crowci/crow/v3/server/forge"
	"codeberg.org/crowci/crow/v3/server/model"
	"codeberg.org/crowci/crow/v3/server/pipeline"
	"codeberg.org/crowci/crow/v3/server/services/lock"
	"codeberg.org/crowci/crow/v3/server/store"
)

const (
	// Specifies the interval Crow checks for new crons to exec.
	checkTime = time.Minute

	// Specifies the batch size of crons to retrieve per check from database.
	checkItems = 10

	// Cron scheduler leader election lock name.
	cronLeaderLock = "cron:scheduler:leader"
)

// Run starts the cron scheduler with persistent leadership.
func Run(ctx context.Context, store store.Store, lockService *lock.Service, leaderLockTTL time.Duration) error {
	// Get instance ID from lock service for logging
	instanceID := lockService.GetInstanceID()

	// Try to acquire leader lock - this will block a leader got elected
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Try to acquire leader lock with automatic renewal
			lockCtx, releaseLock, err := lockService.AcquireLock(ctx, cronLeaderLock, leaderLockTTL)
			if err != nil {
				log.Debug().Err(err).Str("instanceID", instanceID).Msg("Failed to acquire cron leader lock, retrying...")
				time.Sleep(checkTime)
				continue
			}

			log.Info().Str("lock", cronLeaderLock).Str("leaderInstance", instanceID).Msg("This instance is now the cron scheduler leader")

			// Leader determined - run scheduler until context is canceled or leadership is lost
			leaderErr := runAsLeader(ctx, lockCtx, store, instanceID)

			// Release lock and log stepping down
			releaseLock()
			log.Info().Str("lock", cronLeaderLock).Str("leaderInstance", instanceID).Msg("Stepping down as cron scheduler leader")

			if leaderErr != nil {
				return leaderErr
			}

			// Otherwise, leadership was lost - try to become leader again
			log.Debug().Str("instanceID", instanceID).Msg("Lost cron leadership, attempting to re-acquire...")
		}
	}
}

// runAsLeader runs the cron scheduler as the leader until leadership is lost or context is canceled.
func runAsLeader(ctx context.Context, lockCtx context.Context, store store.Store, instanceID string) error {
	ticker := time.NewTicker(checkTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-lockCtx.Done():
			// Lost leadership (lock renewal failed)
			log.Debug().Str("instanceID", instanceID).Msg("Lost cron leader lock, stepping down")
			return nil
		case <-ticker.C:
			// Time to check for cron jobs
			now := time.Now()
			log.Trace().Msg("cron: fetch next crons")

			crons, err := store.CronListNextExecute(now.Unix(), checkItems)
			if err != nil {
				log.Error().Err(err).Int64("now", now.Unix()).Msg("obtain cron list")
				continue
			}

			for _, cron := range crons {
				// Check if we still hold leadership before processing each cron
				select {
				case <-lockCtx.Done():
					log.Debug().Str("instanceID", instanceID).Msg("Lost cron leader lock during processing")
					return nil
				default:
					if err := runCron(ctx, store, cron, now); err != nil {
						log.Error().Err(err).Int64("cronID", cron.ID).Msg("run cron failed")
					}
				}
			}
		}
	}
}

// CalcNewNext parses a cron string and calculates the next exec time based on it.
func CalcNewNext(schedule string, now time.Time) (time.Time, error) {
	// remove local timezone
	now = now.UTC()

	// TODO: allow the users / the admin to set a specific timezone

	c, err := cron.ParseStandard(schedule)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron parse schedule: %w", err)
	}
	return c.Next(now), nil
}

func runCron(ctx context.Context, store store.Store, cron *model.Cron, now time.Time) error {
	log.Trace().Msgf("cron: run id[%d]", cron.ID)

	newNext, err := CalcNewNext(cron.Schedule, now)
	if err != nil {
		return err
	}

	// try to get lock on cron
	gotLock, err := store.CronGetLock(cron, newNext.Unix())
	if err != nil {
		return err
	}
	if !gotLock {
		// another go routine caught it
		return nil
	}

	repo, newPipeline, err := CreatePipeline(ctx, store, cron)
	if err != nil {
		return err
	}

	_, err = pipeline.Create(ctx, store, repo, newPipeline)
	return err
}

func CreatePipeline(ctx context.Context, store store.Store, cron *model.Cron) (*model.Repo, *model.Pipeline, error) {
	repo, err := store.GetRepo(cron.RepoID)
	if err != nil {
		return nil, nil, err
	}

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	if cron.Branch == "" {
		// fallback to the repos default branch
		cron.Branch = repo.Branch
	}

	creator, err := store.GetUser(cron.CreatorID)
	if err != nil {
		return nil, nil, err
	}

	// If the forge has a refresh token, the current access token
	// may be stale. Therefore, we should refresh prior to dispatching
	// the pipeline.
	if err := forge.Refresh(ctx, _forge, store, creator); err != nil {
		log.Error().Err(err).Msg("Failed to refresh user auth token")
		return nil, nil, fmt.Errorf("failed to refresh authentication token: %w", err)
	}

	commit, err := _forge.BranchHead(ctx, creator, repo, cron.Branch)
	if err != nil {
		return nil, nil, err
	}

	return repo, &model.Pipeline{
		Event:     model.EventCron,
		Commit:    commit.SHA,
		Ref:       "refs/heads/" + cron.Branch,
		Branch:    cron.Branch,
		Message:   cron.Name,
		Timestamp: cron.NextExec,
		Sender:    cron.Name,
		ForgeURL:  commit.ForgeURL,
	}, nil
}
