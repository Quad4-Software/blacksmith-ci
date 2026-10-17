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

package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/forge"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// findOrCreateOrg returns the org named owner on the given forge.
// When Crow does not know the org yet, it is fetched from the forge and created.
// The second return value reports whether a new org was created.
func findOrCreateOrg(ctx context.Context, _store store.Store, _forge forge.Forge, u *model.User, owner string, forgeID int64) (*model.Org, bool, error) {
	org, err := _store.OrgFindByName(owner, forgeID)
	if err == nil {
		return org, false, nil
	}
	if !errors.Is(err, types.RecordNotExist) {
		return nil, false, fmt.Errorf("unable to find organization %s: %w", owner, err)
	}

	org, err = _forge.Org(ctx, u, owner)
	if err != nil {
		return nil, false, fmt.Errorf("organization %s not found on forge: %w", owner, err)
	}
	org.ForgeID = forgeID
	if err := _store.OrgCreate(org); err != nil {
		return nil, false, fmt.Errorf("failed to create organization %s: %w", owner, err)
	}
	return org, true, nil
}

// syncRepoOrg points repo.OrgID at the org that currently owns the repo.
// A repo transferred to another namespace on the forge would otherwise keep
// the OrgID of its previous owner, and org-level secrets and registries are
// resolved through OrgID, so its pipelines would keep receiving the previous
// org's credentials.
// When the owner change is a rename of the org itself, the org is renamed in
// place instead, so it keeps its secrets and registries; see isOrgRename.
// It only touches repo.OrgID and the org; the caller persists the repo.
func syncRepoOrg(ctx context.Context, _store store.Store, _forge forge.Forge, u *model.User, repo *model.Repo) error {
	var current *model.Org
	if repo.OrgID != 0 {
		org, err := _store.OrgGet(repo.OrgID)
		if err != nil && !errors.Is(err, types.RecordNotExist) {
			return fmt.Errorf("unable to get organization %d: %w", repo.OrgID, err)
		}
		if err == nil && org.ForgeID == repo.ForgeID {
			if strings.EqualFold(org.Name, repo.Owner) {
				return nil
			}
			current = org
		}
	}

	// An org Crow already knows under the new name is a different namespace,
	// so this is a transfer.
	known, err := _store.OrgFindByName(repo.Owner, repo.ForgeID)
	if err == nil {
		moveRepoToOrg(repo, known)
		return nil
	}
	if !errors.Is(err, types.RecordNotExist) {
		return fmt.Errorf("unable to find organization %s: %w", repo.Owner, err)
	}

	fromForge, err := _forge.Org(ctx, u, repo.Owner)
	if err != nil {
		return fmt.Errorf("organization %s not found on forge: %w", repo.Owner, err)
	}
	fromForge.ForgeID = repo.ForgeID

	if current != nil && isOrgRename(ctx, _store, _forge, u, repo, current, fromForge) {
		log.Info().
			Str("repo", repo.FullName).
			Int64("org_id", current.ID).
			Str("old_name", current.Name).
			Str("new_name", fromForge.Name).
			Msg("repo owner was renamed, renaming its organization")
		current.Name = fromForge.Name
		current.IsUser = fromForge.IsUser
		if err := _store.OrgUpdate(current); err != nil {
			return fmt.Errorf("failed to rename organization %d to %s: %w", current.ID, fromForge.Name, err)
		}
		return nil
	}

	if err := _store.OrgCreate(fromForge); err != nil {
		return fmt.Errorf("failed to create organization %s: %w", repo.Owner, err)
	}
	moveRepoToOrg(repo, fromForge)
	return nil
}

// maxRenameSiblings caps how many other repos of an org are fetched from the
// forge to tell a rename of the org from a transfer of one repo.
const maxRenameSiblings = 3

// isOrgRename reports whether the owner of repo changing from current to
// renamed is a rename of the org on the forge rather than a transfer of repo
// to another namespace.
// Org has no forge-side ID, so a rename needs positive evidence from the
// forge, and anything else is treated as a transfer: wrongly treating a
// transfer as a rename would hand the old org's secrets to the new owner,
// while the opposite only detaches them.
// The evidence is either the old name resolving to the new namespace (forges
// such as Gitea redirect a renamed org's old name), or, when the old name does
// not resolve, the other repos of the org that the forge returns all being
// owned by the new name.
// An error for the old name alone is no evidence, since it also happens when
// the user has no access to the old org.
// A rename of an org with no other repo in Crow therefore detaches its
// secrets unless the forge redirects the old name.
func isOrgRename(ctx context.Context, _store store.Store, _forge forge.Forge, u *model.User, repo *model.Repo, current, renamed *model.Org) bool {
	if old, err := _forge.Org(ctx, u, current.Name); err == nil {
		// The old name still resolves: it is a rename only if the forge
		// redirects it to the new namespace.
		return strings.EqualFold(old.Name, renamed.Name)
	}

	siblings, err := _store.OrgRepoList(current, &model.ListOptions{All: true})
	if err != nil {
		log.Warn().Err(err).Int64("org_id", current.ID).Msg("cannot list org repos to detect a rename")
		return false
	}
	checked, moved := 0, 0
	// OrgRepoList orders by ID, so walk backwards to check the newest repos.
	for i := len(siblings) - 1; i >= 0 && checked < maxRenameSiblings; i-- {
		sibling := siblings[i]
		if sibling.ID == repo.ID || sibling.ForgeID != repo.ForgeID || !sibling.ForgeRemoteID.IsValid() {
			continue
		}
		checked++
		siblingUser := u
		if sibling.UserID != u.ID {
			if siblingUser = forgeUserFor(ctx, _store, _forge, sibling); siblingUser == nil {
				continue
			}
		}
		fromForge, err := _forge.Repo(ctx, siblingUser, sibling.ForgeRemoteID, sibling.Owner, sibling.Name)
		if err != nil {
			log.Debug().Err(err).Str("repo", sibling.FullName).Msg("cannot fetch org repo to detect a rename")
			continue
		}
		if !strings.EqualFold(fromForge.Owner, renamed.Name) {
			// A repo still elsewhere means the org was not renamed.
			return false
		}
		moved++
	}
	return moved > 0
}

// forgeUserFor returns the Crow user of repo with its tokens for the repo's
// forge, or nil when that user has no tokens for that forge.
// It never falls back to a token issued by another forge.
func forgeUserFor(ctx context.Context, _store store.Store, _forge forge.Forge, repo *model.Repo) *model.User {
	if repo.UserID == 0 {
		return nil
	}
	user, err := _store.GetUser(repo.UserID)
	if err != nil {
		return nil
	}
	if userForge, err := _store.UserForgeGet(user.ID, repo.ForgeID); err == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	} else if user.ForgeID != repo.ForgeID {
		return nil
	}
	if err := forge.RefreshUserForge(ctx, _forge, _store, user, repo.ForgeID); err != nil {
		return nil
	}
	return user
}

func moveRepoToOrg(repo *model.Repo, org *model.Org) {
	if org.ID == repo.OrgID {
		return
	}
	log.Info().
		Str("repo", repo.FullName).
		Int64("old_org_id", repo.OrgID).
		Int64("new_org_id", org.ID).
		Msg("repo owner changed, moving repo to the owner's organization")
	repo.OrgID = org.ID
}
