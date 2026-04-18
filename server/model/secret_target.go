package model

import (
	"errors"

	"github.com/uptrace/bun"
)

var (
	ErrSecretTargetExists   = errors.New("secret target already exists")
	ErrSecretTargetNotFound = errors.New("secret target not found")
	ErrSecretNameConflict   = errors.New("a secret with this name already targets this entity")
)

// SecretTarget represents a sharing assignment for a secret to an additional org or repo.
type SecretTarget struct {
	bun.BaseModel `bun:"table:secret_targets"`

	ID        int64 `json:"id"         bun:"id,pk,autoincrement"`
	SecretID  int64 `json:"secret_id"  bun:"secret_id,notnull,unique:st"`
	OrgID     int64 `json:"org_id"     bun:"org_id,notnull,default:0,unique:st"`
	RepoID    int64 `json:"repo_id"    bun:"repo_id,notnull,default:0,unique:st"`
	CreatedAt int64 `json:"created_at" bun:",nullzero"`
}

// TableName returns the database table name.
func (SecretTarget) TableName() string {
	return "secret_targets"
}

// SecretTargetStore persists secret target information to storage.
type SecretTargetStore interface {
	SecretTargetCreate(target *SecretTarget) error
	SecretTargetDelete(target *SecretTarget) error
	SecretTargetList(secretID int64) ([]*SecretTarget, error)
	SecretTargetFind(secretID int64, orgID int64, repoID int64) (*SecretTarget, error)
	SecretTargetDeleteBySecretID(secretID int64) error
	SecretTargetCheckConflict(secretID int64, secretName string, orgID int64, repoID int64) error
}
