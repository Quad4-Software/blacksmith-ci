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

package datastore

import (
	"context"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

// watchKeyChunk bounds how many keys one WatchStateListForKeys query matches,
// keeping the statement under the bind-parameter limits of every driver.
const watchKeyChunk = 200

// WatchCreate inserts a watch. The (user, repo, event, target) key is unique,
// so a duplicate returns the database's constraint error.
func (s storage) WatchCreate(w *model.PipelineWatch) error {
	_, err := s.db.NewInsert().Model(w).Exec(context.Background())
	return err
}

func (s storage) WatchFind(id int64) (*model.PipelineWatch, error) {
	w := new(model.PipelineWatch)
	err := s.db.NewSelect().Model(w).Where("id = ?", id).Scan(context.Background())
	return w, wrapGet(err)
}

func (s storage) WatchFindByKey(userID, repoID int64, event model.WebhookEvent, target string) (*model.PipelineWatch, error) {
	w := new(model.PipelineWatch)
	err := s.db.NewSelect().Model(w).
		Where("user_id = ?", userID).
		Where("repo_id = ?", repoID).
		Where("watch_event = ?", event).
		Where("watch_target = ?", target).
		Scan(context.Background())
	return w, wrapGet(err)
}

func (s storage) WatchUpdate(w *model.PipelineWatch) error {
	// MySQL reports 0 affected rows for an update that changes nothing, so the
	// count cannot tell a missing row from an unchanged one. Callers check
	// existence with WatchFind.
	_, err := s.db.NewUpdate().Model(w).WherePK().Exec(context.Background())
	return err
}

// WatchDelete removes a watch. When it was the last watch of its key, the
// key's workflow states go with it: they are not updated while nobody watches
// the key and would be stale once someone watches it again.
func (s storage) WatchDelete(id int64) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		w := new(model.PipelineWatch)
		if err := tx.NewSelect().Model(w).Where("id = ?", id).Scan(ctx); err != nil {
			return wrapGet(err)
		}
		res, err := tx.NewDelete().Model((*model.PipelineWatch)(nil)).Where("id = ?", id).Exec(ctx)
		if err := wrapDelete(res, err); err != nil {
			return err
		}
		return deleteUnwatchedState(ctx, tx, model.WatchKey{RepoID: w.RepoID, Event: w.Event, Target: w.Target})
	})
}

// deleteUnwatchedState removes the workflow states of a key that no watch refers to anymore.
func deleteUnwatchedState(ctx context.Context, tx bun.Tx, key model.WatchKey) error {
	watched, err := tx.NewSelect().Model((*model.PipelineWatch)(nil)).
		Where("repo_id = ?", key.RepoID).
		Where("watch_event = ?", key.Event).
		Where("watch_target = ?", key.Target).
		Exists(ctx)
	if err != nil || watched {
		return err
	}
	_, err = tx.NewDelete().Model((*model.WatchWorkflowState)(nil)).
		Where("repo_id = ?", key.RepoID).
		Where("watch_event = ?", key.Event).
		Where("watch_target = ?", key.Target).
		Exec(ctx)
	return err
}

func (s storage) WatchListByUser(userID int64) ([]*model.PipelineWatch, error) {
	watches := make([]*model.PipelineWatch, 0)
	err := s.db.NewSelect().Model(&watches).
		Where("user_id = ?", userID).
		OrderExpr("id").
		Scan(context.Background())
	return watches, err
}

func (s storage) WatchListByUserRepo(userID, repoID int64) ([]*model.PipelineWatch, error) {
	watches := make([]*model.PipelineWatch, 0)
	err := s.db.NewSelect().Model(&watches).
		Where("user_id = ?", userID).
		Where("repo_id = ?", repoID).
		OrderExpr("id").
		Scan(context.Background())
	return watches, err
}

func (s storage) WatchListByKey(repoID int64, event model.WebhookEvent, target string) ([]*model.PipelineWatch, error) {
	watches := make([]*model.PipelineWatch, 0)
	err := s.db.NewSelect().Model(&watches).
		Where("repo_id = ?", repoID).
		Where("watch_event = ?", event).
		Where("watch_target = ?", target).
		OrderExpr("id").
		Scan(context.Background())
	return watches, err
}

func (s storage) WatchStateList(repoID int64, event model.WebhookEvent, target string) ([]*model.WatchWorkflowState, error) {
	states := make([]*model.WatchWorkflowState, 0)
	err := s.db.NewSelect().Model(&states).
		Where("repo_id = ?", repoID).
		Where("watch_event = ?", event).
		Where("watch_target = ?", target).
		OrderExpr("workflow_name").
		Scan(context.Background())
	return states, err
}

// WatchStateListForKeys returns the workflow states of all given keys.
func (s storage) WatchStateListForKeys(keys []model.WatchKey) ([]*model.WatchWorkflowState, error) {
	states := make([]*model.WatchWorkflowState, 0)
	for start := 0; start < len(keys); start += watchKeyChunk {
		chunk := keys[start:min(start+watchKeyChunk, len(keys))]
		var chunkStates []*model.WatchWorkflowState
		err := s.db.NewSelect().Model(&chunkStates).
			WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
				for _, k := range chunk {
					q = q.WhereOr("(repo_id = ? AND watch_event = ? AND watch_target = ?)", k.RepoID, k.Event, k.Target)
				}
				return q
			}).
			Scan(context.Background())
		if err != nil {
			return nil, err
		}
		states = append(states, chunkStates...)
	}
	return states, nil
}

// WatchStateReplace swaps the stored workflow states of one key for the given
// ones in a single transaction.
func (s storage) WatchStateReplace(repoID int64, event model.WebhookEvent, target string, states []*model.WatchWorkflowState) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*model.WatchWorkflowState)(nil)).
			Where("repo_id = ?", repoID).
			Where("watch_event = ?", event).
			Where("watch_target = ?", target).
			Exec(ctx); err != nil {
			return err
		}
		for _, st := range states {
			st.ID = 0
			st.RepoID, st.Event, st.Target = repoID, event, target
			if _, err := tx.NewInsert().Model(st).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}
