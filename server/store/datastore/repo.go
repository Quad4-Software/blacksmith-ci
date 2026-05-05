// Copyright 2021 Woodpecker Authors
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

package datastore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

func (s storage) GetRepo(id int64) (*model.Repo, error) {
	repo := new(model.Repo)
	return repo, wrapGet(s.db.NewSelect().Model(repo).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) GetRepoForgeID(remoteID model.ForgeRemoteID) (*model.Repo, error) {
	return s.getRepoForgeID(context.Background(), s.db, remoteID)
}

func (s storage) getRepoForgeID(ctx context.Context, db bun.IDB, remoteID model.ForgeRemoteID) (*model.Repo, error) {
	repo := new(model.Repo)
	return repo, wrapGet(db.NewSelect().Model(repo).Where("forge_remote_id = ?", remoteID).Scan(ctx))
}

func (s storage) GetRepoByForgeAndRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.Repo, error) {
	repo := new(model.Repo)
	return repo, wrapGet(s.db.NewSelect().Model(repo).
		Where("forge_id = ?", forgeID).
		Where("forge_remote_id = ?", remoteID).
		Scan(context.Background()))
}

func (s storage) GetRepoByForgeOwnerName(forgeID int64, owner, name string) (*model.Repo, error) {
	repo := new(model.Repo)
	return repo, wrapGet(s.db.NewSelect().Model(repo).
		Where("forge_id = ?", forgeID).
		Where("owner = ?", owner).
		Where("name = ?", name).
		Scan(context.Background()))
}

func (s storage) GetRepoNameFallback(remoteID model.ForgeRemoteID, fullName string) (*model.Repo, error) {
	return s.getRepoNameFallback(context.Background(), s.db, remoteID, fullName)
}

func (s storage) getRepoNameFallback(ctx context.Context, db bun.IDB, remoteID model.ForgeRemoteID, fullName string) (*model.Repo, error) {
	repo, err := s.getRepoForgeID(ctx, db, remoteID)
	if errors.Is(err, types.RecordNotExist) {
		return s.getRepoName(ctx, db, fullName)
	}
	return repo, err
}

func (s storage) GetRepoName(fullName string) (*model.Repo, error) {
	ctx := context.Background()
	repo, err := s.getRepoName(ctx, s.db, fullName)
	if errors.Is(err, types.RecordNotExist) {
		// the repository does not exist, so look for a redirection
		redirect, err := s.getRedirection(ctx, s.db, fullName)
		if err != nil {
			return nil, err
		}
		return s.GetRepo(redirect.RepoID)
	}
	return repo, err
}

func (s storage) getRepoName(ctx context.Context, db bun.IDB, fullName string) (*model.Repo, error) {
	repo := new(model.Repo)
	return repo, wrapGet(db.NewSelect().Model(repo).Where("LOWER(full_name) = ?", strings.ToLower(fullName)).Scan(ctx))
}

func (s storage) GetRepoCount() (int64, error) {
	n, err := s.db.NewSelect().Model((*model.Repo)(nil)).Where("active = ?", true).Count(context.Background())
	return int64(n), err
}

func (s storage) CreateRepo(repo *model.Repo) error {
	switch {
	case repo.Name == "":
		return fmt.Errorf("repo name is empty")
	case repo.Owner == "":
		return fmt.Errorf("repo owner is empty")
	case repo.FullName == "":
		return fmt.Errorf("repo full name is empty")
	}
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(repo).Exec(context.Background())
	return err
}

func (s storage) UpdateRepo(repo *model.Repo) error {
	res, err := s.db.NewUpdate().Model(repo).WherePK().Exec(context.Background())
	if err != nil {
		return fmt.Errorf("update repo %d failed: %w", repo.ID, err)
	}
	// MySQL without clientFoundRows=true returns 0 if no values actually changed,
	// even if the row exists. Verify the row exists before erroring.
	affected, _ := res.RowsAffected()
	if affected == 0 {
		exists, err := s.db.NewSelect().Model((*model.Repo)(nil)).Where("id = ?", repo.ID).Exists(context.Background())
		if err != nil {
			return fmt.Errorf("update repo %d failed to verify existence: %w", repo.ID, err)
		}
		if !exists {
			return fmt.Errorf("repo %d not found", repo.ID)
		}
	}
	return nil
}

func (s storage) DeleteRepo(repo *model.Repo) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		return s.deleteRepo(ctx, tx, repo)
	})
}

func (s storage) deleteRepo(ctx context.Context, db bun.IDB, repo *model.Repo) error {
	const batchSize = perPage
	if _, err := db.NewDelete().Model((*model.Config)(nil)).Where("repo_id = ?", repo.ID).Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*model.Perm)(nil)).Where("repo_id = ?", repo.ID).Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*model.Registry)(nil)).Where("repo_id = ?", repo.ID).Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*model.Secret)(nil)).Where("repo_id = ?", repo.ID).Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*model.Redirection)(nil)).Where("repo_id = ?", repo.ID).Exec(ctx); err != nil {
		return err
	}

	// delete related pipelines
	for startPipelines := 0; ; startPipelines += batchSize {
		pipelineIDs := make([]int64, 0, batchSize)
		err := db.NewSelect().
			TableExpr("pipelines").
			ColumnExpr("id").
			Where("repo_id = ?", repo.ID).
			Limit(batchSize).
			Offset(startPipelines).
			Scan(ctx, &pipelineIDs)
		if err != nil {
			return err
		}
		if len(pipelineIDs) == 0 {
			break
		}

		for i := range pipelineIDs {
			if err := s.deletePipeline(ctx, db, pipelineIDs[i]); err != nil {
				return err
			}
		}
	}

	return wrapDelete(db.NewDelete().Model((*model.Repo)(nil)).Where("id = ?", repo.ID).Exec(ctx))
}

// RepoList list all repos where permissions for specific user are stored
// TODO: paginate
func (s storage) RepoList(user *model.User, owned, active bool) ([]*model.Repo, error) {
	repos := make([]*model.Repo, 0)
	q := s.db.NewSelect().Model(&repos).
		Join("INNER JOIN perms ON perms.repo_id = repo.id").
		Where("perms.user_id = ?", user.ID)
	if owned {
		q = q.Where("(perms.push = ? OR perms.admin = ?)", true, true)
	}
	if active {
		q = q.Where("repo.active = ?", true)
	}
	return repos, q.
		Order("full_name ASC").
		Scan(context.Background())
}

// RepoListByUserAndForge returns all active repos owned by a user from a specific forge.
func (s storage) RepoListByUserAndForge(userID, forgeID int64) ([]*model.Repo, error) {
	repos := make([]*model.Repo, 0)
	return repos, s.db.NewSelect().Model(&repos).
		Where("user_id = ?", userID).
		Where("forge_id = ?", forgeID).
		Where("active = ?", true).
		Order("full_name ASC").
		Scan(context.Background())
}

// RepoListAll list all repos.
func (s storage) RepoListAll(active bool, p *model.ListOptions) ([]*model.Repo, error) {
	repos := make([]*model.Repo, 0)
	q := s.db.NewSelect().Model(&repos)
	if active {
		q = q.Where("active = ?", true)
	}
	return repos, s.paginate(q.Order("full_name ASC"), p).Scan(context.Background())
}

// RepoListOrphanedOwner returns repos whose owner user has been deleted.
// Repos.user_id = 0 is the zero-default left behind because there is no FK
// constraint and no ON DELETE SET NULL behavior (issue #1010).
func (s storage) RepoListOrphanedOwner() ([]*model.Repo, error) {
	var repos []*model.Repo
	err := s.db.NewSelect().Model(&repos).Where("user_id = ?", 0).Scan(context.Background())
	return repos, err
}

// RepoListPublic returns all active public repos for anonymous browsing.
func (s storage) RepoListPublic(p *model.ListOptions) ([]*model.Repo, error) {
	repos := make([]*model.Repo, 0)
	q := s.db.NewSelect().Model(&repos).
		Where("active = ?", true).
		Where("visibility = ?", model.VisibilityPublic)
	return repos, s.paginate(q.Order("full_name ASC"), p).Scan(context.Background())
}
