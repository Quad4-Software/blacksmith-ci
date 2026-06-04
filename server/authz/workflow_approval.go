// Copyright 2026 Crow Authors
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

// Package authz contains authorization helpers that decide whether a Crow
// user may perform a specific action on a specific resource.
package authz

import (
	"context"
	"strings"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/services"
	"codefloe.com/crowci/crow/v5/server/store"
)

// CanApproveWorkflow returns (allowed, denial-reason, error).
//
// The denial-reason string is for server-side logging only — callers should
// NOT echo it back to the client. Doing so would leak the workflow's team
// allow-list and the caller's forge linkage state.
//
// Resolution algorithm:
//
//  1. If both workflow.ApprovalUsers and workflow.ApprovalTeams are empty,
//     fall back to the repo approve permission: caller must hold Push or
//     Admin on the repo.
//  2. Otherwise, resolve the caller's identity on the repo's forge via
//     user_forges. No row ⇒ deny.
//  3. Direct username match against ApprovalUsers ⇒ allow.
//  4. For each "org/team" entry in ApprovalTeams, list the caller's teams
//     (orgs) on the forge and check membership ⇒ allow on match.
//  5. Else deny.
//
// NOTE: forge.Forge.Teams currently returns the caller's organizations
// (model.Team.Login == org login). Membership in the named org therefore
// satisfies an "org/team" entry; finer-grained team-within-org checks are
// not yet supported across all forges.
func CanApproveWorkflow(
	ctx context.Context,
	s store.Store,
	fmgr services.Manager,
	user *model.User,
	wf *model.Workflow,
	repo *model.Repo,
) (bool, string, error) {
	emptyList := len(wf.ApprovalUsers) == 0 && len(wf.ApprovalTeams) == 0

	if emptyList {
		perm, err := s.PermFind(user, repo)
		if err != nil {
			return false, "permission lookup failed", err
		}
		if perm == nil || (!perm.Push && !perm.Admin) {
			return false, "lacking repo approve permission", nil
		}
		return true, "", nil
	}

	uf, err := s.UserForgeGet(user.ID, repo.ForgeID)
	if err != nil || uf == nil {
		return false, "no linked identity on forge", nil
	}

	for _, name := range wf.ApprovalUsers {
		if name == uf.ForgeLogin {
			return true, "", nil
		}
	}

	if len(wf.ApprovalTeams) > 0 {
		f, err := fmgr.ForgeFromRepo(repo)
		if err != nil {
			return false, "forge lookup failed", err
		}
		teams, err := f.Teams(ctx, user)
		if err != nil {
			return false, "forge team lookup failed", err
		}
		for _, want := range wf.ApprovalTeams {
			orgName, _, ok := splitOrgTeam(want)
			if !ok {
				continue
			}
			for _, t := range teams {
				if t.Login == orgName {
					return true, "", nil
				}
			}
		}
	}

	return false, "not in approvers list", nil
}

// splitOrgTeam splits an "org/team" string into its parts. Empty halves
// or a missing separator returns ok=false.
func splitOrgTeam(s string) (org, team string, ok bool) {
	i := strings.IndexByte(s, '/')
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}
