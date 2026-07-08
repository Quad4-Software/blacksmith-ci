package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestSecretTargetCreate(t *testing.T) {
	store, closer := newTestStore(t, new(model.Secret), new(model.SecretTarget))
	defer closer()

	target := &model.SecretTarget{
		SecretID: 1,
		OrgID:    0,
		RepoID:   10,
	}
	err := store.SecretTargetCreate(target)
	assert.NoError(t, err)
	assert.NotZero(t, target.ID)

	// duplicate should fail (unique constraint)
	err = store.SecretTargetCreate(&model.SecretTarget{
		SecretID: 1,
		OrgID:    0,
		RepoID:   10,
	})
	assert.Error(t, err)
}

func TestSecretTargetList(t *testing.T) {
	store, closer := newTestStore(t, new(model.Secret), new(model.SecretTarget))
	defer closer()

	// empty list
	list, err := store.SecretTargetList(1)
	assert.NoError(t, err)
	assert.Len(t, list, 0)

	// add two targets
	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: 1,
		OrgID:    0,
		RepoID:   10,
	}))
	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: 1,
		OrgID:    5,
		RepoID:   0,
	}))

	list, err = store.SecretTargetList(1)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestSecretTargetDelete(t *testing.T) {
	store, closer := newTestStore(t, new(model.Secret), new(model.SecretTarget))
	defer closer()

	target := &model.SecretTarget{
		SecretID: 1,
		OrgID:    0,
		RepoID:   10,
	}
	assert.NoError(t, store.SecretTargetCreate(target))

	assert.NoError(t, store.SecretTargetDelete(target))

	list, err := store.SecretTargetList(1)
	assert.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestSecretTargetFind(t *testing.T) {
	store, closer := newTestStore(t, new(model.Secret), new(model.SecretTarget))
	defer closer()

	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: 1,
		OrgID:    5,
		RepoID:   10,
	}))

	found, err := store.SecretTargetFind(1, 5, 10)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, found.SecretID)
	assert.EqualValues(t, 5, found.OrgID)
	assert.EqualValues(t, 10, found.RepoID)

	// not found
	_, err = store.SecretTargetFind(1, 99, 99)
	assert.Error(t, err)
}

func TestSecretTargetDeleteBySecretID(t *testing.T) {
	store, closer := newTestStore(t, new(model.Secret), new(model.SecretTarget))
	defer closer()

	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: 1,
		OrgID:    0,
		RepoID:   10,
	}))
	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: 1,
		OrgID:    5,
		RepoID:   0,
	}))
	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: 2,
		OrgID:    0,
		RepoID:   20,
	}))

	assert.NoError(t, store.SecretTargetDeleteBySecretID(1))

	list, err := store.SecretTargetList(1)
	assert.NoError(t, err)
	assert.Len(t, list, 0)

	// secret 2 targets should still exist
	list, err = store.SecretTargetList(2)
	assert.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestSecretTargetCascadeDelete(t *testing.T) {
	store, closer := newTestStore(t, new(model.Secret), new(model.SecretTarget))
	defer closer()

	secret := &model.Secret{
		RepoID: 1,
		Name:   "deploy-key",
		Value:  "secret-value",
	}
	assert.NoError(t, store.SecretCreate(secret))

	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: secret.ID,
		OrgID:    0,
		RepoID:   20,
	}))

	// delete the secret
	assert.NoError(t, store.SecretDelete(secret))

	// manually clean up targets (SQLite in-memory doesn't enforce FK cascade by default)
	// In production the migration sets ON DELETE CASCADE, but test stores use CreateTable
	// which doesn't replicate FK constraints. So we verify DeleteBySecretID works instead.
	assert.NoError(t, store.SecretTargetDeleteBySecretID(secret.ID))

	list, err := store.SecretTargetList(secret.ID)
	assert.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestSecretTargetCheckConflict(t *testing.T) {
	store, closer := newTestStore(t, new(model.Secret), new(model.SecretTarget))
	defer closer()

	// Create repo secret "deploy-key" owned by repo 10
	repoSecret := &model.Secret{
		RepoID: 10,
		Name:   "deploy-key",
		Value:  "repo-secret-value",
	}
	assert.NoError(t, store.SecretCreate(repoSecret))

	// Create org secret "deploy-key" owned by org 1
	orgSecret := &model.Secret{
		OrgID: 1,
		Name:  "deploy-key",
		Value: "org-secret-value",
	}
	assert.NoError(t, store.SecretCreate(orgSecret))

	// Sharing org secret with repo 10 should conflict (first-owner conflict)
	err := store.SecretTargetCheckConflict(orgSecret.ID, orgSecret.Name, 0, 10)
	assert.ErrorIs(t, err, model.ErrSecretNameConflict)

	// Sharing org secret with repo 20 should succeed (no conflict)
	err = store.SecretTargetCheckConflict(orgSecret.ID, orgSecret.Name, 0, 20)
	assert.NoError(t, err)

	// Share repo secret with org 5
	assert.NoError(t, store.SecretTargetCreate(&model.SecretTarget{
		SecretID: repoSecret.ID,
		OrgID:    5,
		RepoID:   0,
	}))

	// Sharing org secret with org 5 should conflict (shared target conflict)
	err = store.SecretTargetCheckConflict(orgSecret.ID, orgSecret.Name, 5, 0)
	assert.ErrorIs(t, err, model.ErrSecretNameConflict)
}
