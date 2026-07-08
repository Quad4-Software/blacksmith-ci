package datastore

import (
	"context"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) SecretTargetCreate(target *model.SecretTarget) error {
	// Left as a plain INSERT: the table already carries a
	// UNIQUE(secret_id, org_id, repo_id) constraint, so this is atomic (no
	// SELECT-then-INSERT race) and callers rely on a duplicate returning an
	// error rather than being silently upserted.
	_, err := s.db.NewInsert().Model(target).Exec(context.Background())
	return err
}

func (s storage) SecretTargetDelete(target *model.SecretTarget) error {
	return wrapDelete(s.db.NewDelete().Model((*model.SecretTarget)(nil)).
		Where("id = ?", target.ID).
		Exec(context.Background()))
}

func (s storage) SecretTargetList(secretID int64) ([]*model.SecretTarget, error) {
	var targets []*model.SecretTarget
	return targets, s.db.NewSelect().Model(&targets).
		Where("secret_id = ?", secretID).
		Order("id ASC").
		Scan(context.Background())
}

func (s storage) SecretTargetFind(secretID int64, orgID int64, repoID int64) (*model.SecretTarget, error) {
	target := new(model.SecretTarget)
	return target, wrapGet(s.db.NewSelect().Model(target).
		Where("secret_id = ? AND org_id = ? AND repo_id = ?", secretID, orgID, repoID).
		Scan(context.Background()))
}

func (s storage) SecretTargetDeleteBySecretID(secretID int64) error {
	_, err := s.db.NewDelete().Model((*model.SecretTarget)(nil)).
		Where("secret_id = ?", secretID).
		Exec(context.Background())
	return err
}

func (s storage) SecretTargetCheckConflict(secretID int64, secretName string, orgID int64, repoID int64) error {
	// Check 1: Is there a first-owner secret with the same name targeting this entity?
	exists, err := s.db.NewSelect().Model((*model.Secret)(nil)).
		Where("id != ? AND name = ?", secretID, secretName).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			if repoID != 0 {
				sq = sq.WhereOr("repo_id = ?", repoID)
			}
			if orgID != 0 {
				sq = sq.WhereOr("(org_id = ? AND repo_id = 0)", orgID)
			}
			return sq
		}).
		Exists(context.Background())
	if err != nil {
		return err
	}
	if exists {
		return model.ErrSecretNameConflict
	}

	// Check 2: Is there another secret with the same name already shared to this entity via secret_targets?
	exists, err = s.db.NewSelect().Model((*model.SecretTarget)(nil)).
		Join("JOIN secrets ON secrets.id = secret_target.secret_id").
		Where("secret_target.secret_id != ? AND secrets.name = ?", secretID, secretName).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			if repoID != 0 {
				sq = sq.WhereOr("secret_target.repo_id = ?", repoID)
			}
			if orgID != 0 {
				sq = sq.WhereOr("(secret_target.org_id = ? AND secret_target.repo_id = 0)", orgID)
			}
			return sq
		}).
		Exists(context.Background())
	if err != nil {
		return err
	}
	if exists {
		return model.ErrSecretNameConflict
	}

	return nil
}
