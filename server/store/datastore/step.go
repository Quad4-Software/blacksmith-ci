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

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) StepLoad(id int64) (*model.Step, error) {
	step := new(model.Step)
	return step, wrapGet(s.db.NewSelect().Model(step).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) StepFind(pipeline *model.Pipeline, pid int) (*model.Step, error) {
	step := new(model.Step)
	return step, wrapGet(s.db.NewSelect().Model(step).
		Where("pipeline_id = ? AND pid = ?", pipeline.ID, pid).
		Scan(context.Background()))
}

func (s storage) StepByUUID(uuid string) (*model.Step, error) {
	step := new(model.Step)
	return step, wrapGet(s.db.NewSelect().Model(step).
		Where("uuid = ?", uuid).
		Scan(context.Background()))
}

func (s storage) StepChild(pipeline *model.Pipeline, ppid int, child string) (*model.Step, error) {
	step := new(model.Step)
	return step, wrapGet(s.db.NewSelect().Model(step).
		Where("pipeline_id = ? AND ppid = ? AND name = ?", pipeline.ID, ppid, child).
		Scan(context.Background()))
}

func (s storage) StepList(pipeline *model.Pipeline) ([]*model.Step, error) {
	stepList := make([]*model.Step, 0)
	return stepList, s.db.NewSelect().Model(&stepList).
		Where("pipeline_id = ?", pipeline.ID).
		Order("pid ASC").
		Scan(context.Background())
}

func (s storage) StepListFromWorkflowFind(workflow *model.Workflow) ([]*model.Step, error) {
	return s.stepListWorkflow(context.Background(), s.db, workflow)
}

func (s storage) stepListWorkflow(ctx context.Context, db bun.IDB, workflow *model.Workflow) ([]*model.Step, error) {
	stepList := make([]*model.Step, 0)
	return stepList, db.NewSelect().Model(&stepList).
		Where("pipeline_id = ?", workflow.PipelineID).
		Where("ppid = ?", workflow.PID).
		Order("pid ASC").
		Scan(ctx)
}

func (s storage) stepCreate(ctx context.Context, db bun.IDB, steps []*model.Step) error {
	for i := range steps {
		// only Insert on single object ref set auto created ID back to object
		if _, err := db.NewInsert().Model(steps[i]).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s storage) StepUpdate(step *model.Step) error {
	_, err := s.db.NewUpdate().Model(step).WherePK().Exec(context.Background())
	return err
}

func deleteStep(ctx context.Context, db bun.IDB, stepID int64) error {
	if err := logDelete(ctx, db, stepID); err != nil {
		return err
	}
	return wrapDelete(db.NewDelete().Model((*model.Step)(nil)).Where("id = ?", stepID).Exec(ctx))
}
