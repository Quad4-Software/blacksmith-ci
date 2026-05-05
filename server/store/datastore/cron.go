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

package datastore

import (
	"context"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) CronCreate(cron *model.Cron) error {
	if err := cron.Validate(); err != nil {
		return err
	}
	_, err := s.db.NewInsert().Model(cron).Exec(context.Background())
	return err
}

func (s storage) CronFind(repo *model.Repo, id int64) (*model.Cron, error) {
	cron := new(model.Cron)
	return cron, wrapGet(s.db.NewSelect().Model(cron).Where("id = ?", id).Where("repo_id = ?", repo.ID).Scan(context.Background()))
}

func (s storage) CronList(repo *model.Repo, p *model.ListOptions) ([]*model.Cron, error) {
	var crons []*model.Cron
	return crons, s.paginate(s.db.NewSelect().Model(&crons).Where("repo_id = ?", repo.ID).Order("name ASC"), p).Scan(context.Background())
}

func (s storage) CronUpdate(_ *model.Repo, cron *model.Cron) error {
	_, err := s.db.NewUpdate().Model(cron).WherePK().Exec(context.Background())
	return err
}

func (s storage) CronDelete(repo *model.Repo, id int64) error {
	return wrapDelete(s.db.NewDelete().Model((*model.Cron)(nil)).Where("id = ?", id).Where("repo_id = ?", repo.ID).Exec(context.Background()))
}

// CronListNextExecute returns limited number of jobs with NextExec being less or equal to the provided unix timestamp.
// Disabled crons are excluded.
func (s storage) CronListNextExecute(nextExec, limit int64) ([]*model.Cron, error) {
	crons := make([]*model.Cron, 0, limit)
	return crons, s.db.NewSelect().Model(&crons).
		Where("next_exec <= ?", nextExec).
		Where("disabled = ?", false).
		Limit(int(limit)).
		Scan(context.Background())
}

// CronGetLock try to get a lock by updating NextExec.
func (s storage) CronGetLock(cron *model.Cron, newNextExec int64) (bool, error) {
	res, err := s.db.NewUpdate().Model((*model.Cron)(nil)).
		Set("next_exec = ?", newNextExec).
		Where("id = ?", cron.ID).
		Where("next_exec = ?", cron.NextExec).
		Exec(context.Background())
	if err != nil {
		return false, err
	}

	n, _ := res.RowsAffected()
	gotLock := n != 0

	if gotLock {
		cron.NextExec = newNextExec
	}

	return gotLock, nil
}

// CronListOrphaned returns crons whose repo_id does not match any existing repo.
// These are dead — there is no execution target.
func (s storage) CronListOrphaned() ([]*model.Cron, error) {
	var crons []*model.Cron
	err := s.db.NewSelect().
		Model(&crons).
		Join("LEFT JOIN repos ON repos.id = ?TableAlias.repo_id").
		Where("repos.id IS NULL").
		Scan(context.Background())
	return crons, err
}

// CronListOrphanedOwner returns active (not-disabled) crons whose repo's owner
// has been deleted (repos.user_id = 0). Already-disabled rows are excluded so
// re-runs do not overwrite admin-set fail_msg values.
func (s storage) CronListOrphanedOwner() ([]*model.Cron, error) {
	var crons []*model.Cron
	err := s.db.NewSelect().
		Model(&crons).
		Join("INNER JOIN repos ON repos.id = ?TableAlias.repo_id").
		Where("repos.user_id = ?", 0).
		Where("?TableAlias.disabled = ?", false).
		Scan(context.Background())
	return crons, err
}

// CronDeleteByID deletes a cron row by its primary key without requiring a repo
// scope. Used by the orphaned-data-cleanup maintenance job, which encounters
// crons whose repo no longer exists.
func (s storage) CronDeleteByID(id int64) error {
	return wrapDelete(s.db.NewDelete().Model((*model.Cron)(nil)).Where("id = ?", id).Exec(context.Background()))
}

// CronDisableOrphaned marks a cron as disabled and sets fail_msg to the
// supplied reason. Used by the orphaned-data-cleanup maintenance job.
func (s storage) CronDisableOrphaned(id int64, msg string) error {
	_, err := s.db.NewUpdate().
		Model((*model.Cron)(nil)).
		Set("disabled = ?", true).
		Set("fail_msg = ?", msg).
		Where("id = ?", id).
		Exec(context.Background())
	return err
}

// UserCronList returns all crons from repos the user has access to.
func (s storage) UserCronList(repoIDs []int64, searchTerm string, p *model.ListOptions) ([]*model.Cron, error) {
	crons := make([]*model.Cron, 0)

	if len(repoIDs) == 0 {
		return crons, nil
	}

	q := s.db.NewSelect().Model(&crons).Where("repo_id IN (?)", bun.List(repoIDs))

	// Add search filter if provided
	if searchTerm != "" {
		q = q.Where("name LIKE ?", "%"+searchTerm+"%")
	}

	return crons, s.paginate(q.Order("name ASC"), p).Scan(context.Background())
}
