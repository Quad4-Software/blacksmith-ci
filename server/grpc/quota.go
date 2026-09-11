// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package grpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
)

const (
	ciQuotaCheckAttempts = 3
	// The whole retry loop runs under this budget. It stays under the
	// 30 second queue poll cap so one Next RPC remains inside typical
	// reverse-proxy connection timeouts even while Forgejo is unreachable.
	ciQuotaCheckTimeout = 20 * time.Second
)

// checkCIQuota runs after the queue claims the task but before any executable
// workflow data reaches the agent. Retries keep the claim alive and never start
// work while the quota service is unavailable.
func (s *RPC) checkCIQuota(ctx context.Context, agent *model.Agent, task *model.Task) error {
	reporter := server.Config.Services.CIQuotaReporter
	// Organization-scoped names are tenant-controlled and cannot identify public runners.
	if reporter == nil || !agent.IsGlobal() {
		return nil
	}
	workflowID, err := strconv.ParseInt(task.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("CI quota: invalid workflow ID: %w", err)
	}
	workflow, err := s.store.WorkflowLoad(workflowID)
	if err != nil {
		return fmt.Errorf("CI quota: load workflow: %w", err)
	}
	repo, err := s.store.GetRepo(task.RepoID)
	if err != nil {
		return fmt.Errorf("CI quota: load repository: %w", err)
	}
	class := quotaClass(workflow.Platform)
	checkCtx, cancel := context.WithTimeout(ctx, ciQuotaCheckTimeout)
	defer cancel()
	for attempt := 0; attempt < ciQuotaCheckAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("CI quota check interrupted: %w", err)
		}
		if checkCtx.Err() != nil {
			break
		}
		if err := s.queue.Extend(ctx, agent.ID, task.ID); err != nil {
			return fmt.Errorf("CI quota: retain workflow claim: %w", err)
		}
		decision, checkErr := reporter.Check(checkCtx, repo.FullName, class, agent.Name)
		if checkErr == nil && decision == nil {
			checkErr = errors.New("CI quota check returned an empty decision")
		}
		if checkErr == nil {
			if decision.Allowed {
				return nil
			}
			reason := decision.Reason
			if reason == "" {
				reason = "the repository owner has no remaining allowance for this runner class"
			}
			return fmt.Errorf("CI quota denied (%s): %s", class, reason)
		}
		err = checkErr
		log.Warn().Err(err).Str("workflow_id", task.ID).Int("attempt", attempt+1).Msg("could not check CI quota before dispatch")
		if attempt < ciQuotaCheckAttempts-1 {
			timer := time.NewTimer(time.Second)
			select {
			case <-checkCtx.Done():
				timer.Stop()
				if ctx.Err() != nil {
					return fmt.Errorf("CI quota check interrupted: %w", ctx.Err())
				}
			case <-timer.C:
			}
		}
	}
	if err == nil {
		err = checkCtx.Err()
	}
	return fmt.Errorf("CI quota check unavailable; workflow not started: %w", err)
}
