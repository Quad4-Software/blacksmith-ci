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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gdgvda/cron"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/forge"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pipeline"
	"codefloe.com/crowci/crow/v5/server/services/lock"
	"codefloe.com/crowci/crow/v5/server/store"
)

const (
	// Specifies the interval Crow checks for new crons to exec.
	checkTime = time.Minute

	// Specifies the batch size of crons to retrieve per check from database.
	checkItems = 10

	// Cron scheduler leader election lock name.
	cronLeaderLock = "cron:scheduler:leader"

	maxAuthFailures = 3 // consecutive auth failures before a cron is disabled
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
		return trackCronFailure(store, cron, err)
	}

	_, err = pipeline.Create(ctx, store, repo, newPipeline)
	if err != nil {
		return trackCronFailure(store, cron, err)
	}

	// Success: reset failure state if previously failing
	if cron.FailCount > 0 {
		cron.FailCount = 0
		cron.FailMsg = ""
		if updateErr := store.CronUpdate(&model.Repo{ID: cron.RepoID}, cron); updateErr != nil {
			log.Error().Err(updateErr).Int64("cronID", cron.ID).Msg("failed to reset cron failure state")
		}
	}

	return nil
}

// isAuthError returns true if the error is related to authentication/token issues.
// This covers both the explicit ErrTokenInvalidated (token was cleared from DB)
// and raw OAuth errors (e.g., "token expired and refresh token is not set").
func isAuthError(err error) bool {
	if errors.Is(err, forge.ErrTokenInvalidated) {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "token expired") ||
		strings.Contains(errStr, "refresh token is not set") ||
		strings.Contains(errStr, "invalid_grant") ||
		strings.Contains(errStr, "token has been revoked")
}

// trackCronFailure records a cron execution failure. For auth errors, it increments
// the fail counter and disables the cron after maxAuthFailures consecutive failures.
func trackCronFailure(store store.Store, cron *model.Cron, err error) error {
	if !isAuthError(err) {
		return err
	}

	cron.FailCount++
	cron.FailMsg = err.Error()

	if cron.FailCount >= maxAuthFailures {
		cron.Disabled = true
		log.Warn().
			Int64("cronID", cron.ID).
			Int64("repoID", cron.RepoID).
			Str("name", cron.Name).
			Int("failCount", cron.FailCount).
			Msg("cron disabled after repeated auth failures")
	}

	if updateErr := store.CronUpdate(&model.Repo{ID: cron.RepoID}, cron); updateErr != nil {
		log.Error().Err(updateErr).Int64("cronID", cron.ID).Msg("failed to update cron failure state")
	}

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

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	// This ensures we use the correct OAuth tokens for the forge the repo belongs to.
	if userForge, ufErr := store.UserForgeGet(creator.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		creator.AccessToken = userForge.AccessToken
		creator.RefreshToken = userForge.RefreshToken
		creator.Expiry = userForge.Expiry
		creator.ForgeID = userForge.ForgeID
		creator.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// If the forge has a refresh token, the current access token
	// may be stale. Therefore, we should refresh prior to dispatching
	// the pipeline.
	if refreshErr := forge.RefreshUserForge(ctx, _forge, store, creator, repo.ForgeID); refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
		return nil, nil, fmt.Errorf("failed to refresh authentication token: %w", refreshErr)
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
