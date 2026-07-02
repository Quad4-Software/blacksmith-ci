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
	"strconv"
	"time"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) GetPipeline(id int64) (*model.Pipeline, error) {
	pipeline := &model.Pipeline{}
	return pipeline, wrapGet(s.db.NewSelect().Model(pipeline).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) GetPipelineNumber(repo *model.Repo, num int64) (*model.Pipeline, error) {
	pipeline := new(model.Pipeline)
	return pipeline, wrapGet(s.db.NewSelect().Model(pipeline).
		Where("repo_id = ?", repo.ID).
		Where("number = ?", num).
		Scan(context.Background()))
}

func (s storage) GetPipelineBadge(repo *model.Repo, branch string) (*model.Pipeline, error) {
	pipeline := new(model.Pipeline)
	return pipeline, wrapGet(s.db.NewSelect().Model(pipeline).
		Where("repo_id = ?", repo.ID).
		Where("branch = ?", branch).
		Where("event = ?", model.EventPush).
		Where("status != ?", model.StatusBlocked).
		Order("number DESC").
		Limit(1).
		Scan(context.Background()))
}

func (s storage) GetPipelineLast(repo *model.Repo, branch string) (*model.Pipeline, error) {
	pipeline := new(model.Pipeline)
	return pipeline, wrapGet(s.db.NewSelect().Model(pipeline).
		Where("repo_id = ?", repo.ID).
		Where("branch = ?", branch).
		Where("event = ?", model.EventPush).
		Order("number DESC").
		Limit(1).
		Scan(context.Background()))
}

func (s storage) GetPipelineLastBefore(repo *model.Repo, branch string, num int64) (*model.Pipeline, error) {
	pipeline := new(model.Pipeline)
	return pipeline, wrapGet(s.db.NewSelect().Model(pipeline).
		Where("number < ?", num).
		Where("repo_id = ?", repo.ID).
		Where("branch = ?", branch).
		Order("number DESC").
		Limit(1).
		Scan(context.Background()))
}

func (s storage) GetPipelineList(repo *model.Repo, p *model.ListOptions, f *model.PipelineFilter) ([]*model.Pipeline, error) {
	pipelines := make([]*model.Pipeline, 0, 16)

	q := s.db.NewSelect().Model(&pipelines).Where("repo_id = ?", repo.ID)

	if f != nil {
		if f.After != 0 {
			q = q.Where("created > ?", f.After)
		}

		if f.Before != 0 {
			q = q.Where("created < ?", f.Before)
		}

		if f.Branch != "" {
			q = q.Where("branch = ?", f.Branch)
		}

		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}

		if len(f.Events) != 0 {
			q = q.Where("event IN (?)", bun.List(f.Events))
		}

		if f.RefContains != "" {
			q = q.Where("ref LIKE ?", "%"+f.RefContains+"%")
		}

		if len(f.WorkflowNames) != 0 {
			q = q.Where("EXISTS (SELECT 1 FROM workflows WHERE workflows.pipeline_id = pipeline.id AND workflows.name IN (?))", bun.List(f.WorkflowNames))
		}

		if f.Search != "" {
			searchPattern := "%" + f.Search + "%"
			searchExpr := "(message LIKE ? OR branch LIKE ? OR author LIKE ? OR event LIKE ?"
			args := []any{searchPattern, searchPattern, searchPattern, searchPattern}
			// Also search by pipeline number if the search term is numeric
			if _, err := strconv.ParseInt(f.Search, 10, 64); err == nil {
				searchExpr += " OR " + s.castAsText("number") + " LIKE ?"
				args = append(args, f.Search)
			}
			searchExpr += ")"
			q = q.Where(searchExpr, args...)
		}
	}

	return pipelines, s.paginate(q.Order("number DESC"), p).Scan(context.Background())
}

// GetRepoLatestPipelines get the latest pipeline for each repo.
func (s storage) GetRepoLatestPipelines(repoIDs []int64) ([]*model.Pipeline, error) {
	pipelines := make([]*model.Pipeline, 0, len(repoIDs))

	pipelineIDs := make([]int64, 0, len(repoIDs))
	err := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("MAX(id) AS id").
		Where("repo_id IN (?)", bun.List(repoIDs)).
		GroupExpr("repo_id").
		Scan(context.Background(), &pipelineIDs)
	if err != nil {
		return nil, err
	}

	return pipelines, s.db.NewSelect().Model(&pipelines).
		Where("id IN (?)", bun.List(pipelineIDs)).
		Scan(context.Background())
}

// GetActivePipelineList get all pipelines that are pending, running or blocked.
func (s storage) GetActivePipelineList(repo *model.Repo) ([]*model.Pipeline, error) {
	pipelines := make([]*model.Pipeline, 0)
	return pipelines, s.db.NewSelect().Model(&pipelines).
		Where("repo_id = ?", repo.ID).
		Where("status IN (?)", bun.List([]model.StatusValue{model.StatusPending, model.StatusRunning, model.StatusBlocked})).
		Order("number DESC").
		Scan(context.Background())
}

func (s storage) GetPipelineCount() (int64, error) {
	n, err := s.db.NewSelect().Model((*model.Pipeline)(nil)).Count(context.Background())
	return int64(n), err
}

func (s storage) GetPipelineCountByRepo(repoID int64) (int64, error) {
	n, err := s.db.NewSelect().Model((*model.Pipeline)(nil)).Where("repo_id = ?", repoID).Count(context.Background())
	return int64(n), err
}

func (s storage) CreatePipeline(pipeline *model.Pipeline, stepList ...*model.Step) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		repoExist, err := tx.NewSelect().Model((*model.Repo)(nil)).Where("id = ?", pipeline.RepoID).Exists(ctx)
		if err != nil {
			return err
		}

		if !repoExist {
			return ErrorRepoNotExist{RepoID: pipeline.RepoID}
		}

		// calc pipeline number
		var number int64
		err = tx.NewSelect().
			TableExpr("pipelines").
			ColumnExpr("COALESCE(MAX(number), 0)").
			Where("repo_id = ?", pipeline.RepoID).
			Scan(ctx, &number)
		if err != nil {
			return err
		}
		pipeline.Number = number + 1

		pipeline.Created = time.Now().UTC().Unix()
		// only Insert set auto created ID back to object
		if _, err := tx.NewInsert().Model(pipeline).Exec(ctx); err != nil {
			return err
		}

		for i := range stepList {
			stepList[i].PipelineID = pipeline.ID
			// only Insert set auto created ID back to object
			if _, err := tx.NewInsert().Model(stepList[i]).Exec(ctx); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s storage) UpdatePipeline(pipeline *model.Pipeline) error {
	_, err := s.db.NewUpdate().Model(pipeline).WherePK().Exec(context.Background())
	return err
}

func (s storage) DeletePipeline(pipeline *model.Pipeline) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		return s.deletePipeline(ctx, tx, pipeline.ID)
	})
}

func (s storage) deletePipeline(ctx context.Context, db bun.IDB, pipelineID int64) error {
	if err := s.workflowsDelete(ctx, db, pipelineID); err != nil {
		return err
	}

	// Delete tasks associated with this pipeline to prevent orphaned tasks
	if _, err := db.NewDelete().Model((*model.Task)(nil)).Where("pipeline_id = ?", pipelineID).Exec(ctx); err != nil {
		return err
	}

	var confIDs []int64
	if err := db.NewSelect().TableExpr("pipeline_configs").ColumnExpr("config_id").Where("pipeline_id = ?", pipelineID).Scan(ctx, &confIDs); err != nil {
		return err
	}
	for _, confID := range confIDs {
		exist, err := db.NewSelect().Model((*model.PipelineConfig)(nil)).
			Where("config_id = ?", confID).
			Where("pipeline_id != ?", pipelineID).
			Exists(ctx)
		if err != nil {
			return err
		}
		if !exist {
			// this config is only used for this pipeline. so delete it
			if _, err := db.NewDelete().Model((*model.Config)(nil)).Where("id = ?", confID).Exec(ctx); err != nil {
				return err
			}
		}
	}

	if _, err := db.NewDelete().Model((*model.PipelineConfig)(nil)).Where("pipeline_id = ?", pipelineID).Exec(ctx); err != nil {
		return err
	}
	return wrapDelete(db.NewDelete().Model((*model.Pipeline)(nil)).Where("id = ?", pipelineID).Exec(ctx))
}

func (s storage) SetPipelineLogPurged(pipeline *model.Pipeline) error {
	pipeline.LogsDeleted = true
	_, err := s.db.NewUpdate().Model(pipeline).Column("logs_deleted").Where("id = ?", pipeline.ID).Exec(context.Background())
	return err
}

func (s storage) GetPipelineLogPurged(pipeline *model.Pipeline) (bool, error) {
	var logsPurged bool
	err := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("logs_deleted").
		Where("id = ?", pipeline.ID).
		Scan(context.Background(), &logsPurged)
	if err != nil {
		return false, err
	}
	return logsPurged, nil
}

func (s storage) GetPipelineListKeyset(repo *model.Repo, lastID int64, beforeDate int64, limit int) ([]*model.Pipeline, error) {
	pipelines := make([]*model.Pipeline, 0)
	return pipelines, s.db.NewSelect().Model(&pipelines).
		Where("id < ?", lastID).
		Where("created < ?", beforeDate).
		Where("repo_id = ?", repo.ID).
		Order("id DESC").
		Limit(limit).
		Scan(context.Background())
}

func (s storage) GetRepoSpecificPipelineCount(repo *model.Repo) (int64, error) {
	n, err := s.db.NewSelect().Model((*model.Pipeline)(nil)).Where("repo_id = ?", repo.ID).Count(context.Background())
	return int64(n), err
}
