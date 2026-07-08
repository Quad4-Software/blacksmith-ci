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

package secret

import (
	"fmt"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

type db struct {
	store store.Store
}

// NewDB returns a new local secret service.
func NewDB(store store.Store) Service {
	return &db{store: store}
}

func (d *db) SecretFind(repo *model.Repo, name string) (*model.Secret, error) {
	return d.store.SecretFind(repo, name)
}

func (d *db) SecretList(repo *model.Repo, p *model.ListOptions) ([]*model.Secret, error) {
	return d.store.SecretList(repo, false, p)
}

func (d *db) SecretListPipeline(repo *model.Repo, _ *model.Pipeline) ([]*model.Secret, error) {
	s, err := d.store.SecretList(repo, true, &model.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	// Classify each secret as first-owner or shared relative to this repo.
	isFirstOwner := func(secret *model.Secret) bool {
		if secret.IsRepository() && secret.RepoID == repo.ID {
			return true
		}
		if secret.IsOrganization() && secret.OrgID == repo.OrgID {
			return true
		}
		return secret.IsGlobal()
	}

	// Priority: first-owner repo > first-owner org > first-owner global > shared
	secrets := make([]*model.Secret, 0, len(s))
	uniq := make(map[string]struct{})

	// Pass 1: first-owner secrets by priority
	for _, condition := range []struct {
		IsRepository   bool
		IsOrganization bool
		IsGlobal       bool
	}{
		{IsRepository: true},
		{IsOrganization: true},
		{IsGlobal: true},
	} {
		for _, secret := range s {
			if !isFirstOwner(secret) {
				continue
			}
			if secret.IsRepository() != condition.IsRepository ||
				secret.IsOrganization() != condition.IsOrganization ||
				secret.IsGlobal() != condition.IsGlobal {
				continue
			}
			if _, ok := uniq[secret.Name]; ok {
				continue
			}
			uniq[secret.Name] = struct{}{}
			secrets = append(secrets, secret)
		}
	}

	// Pass 2: shared secrets (not first-owner for this repo)
	sharedNames := make(map[string]struct{})
	for _, secret := range s {
		if isFirstOwner(secret) {
			continue
		}
		if _, ok := uniq[secret.Name]; ok {
			// First-owner secret shadows this shared secret — skip silently
			continue
		}
		if _, ok := sharedNames[secret.Name]; ok {
			// Multiple shared secrets with the same name — runtime safety net
			return nil, fmt.Errorf("multiple shared secrets named %q target this repository", secret.Name)
		}
		sharedNames[secret.Name] = struct{}{}
		uniq[secret.Name] = struct{}{}
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

func (d *db) SecretCreate(_ *model.Repo, in *model.Secret) error {
	return d.store.SecretCreate(in)
}

func (d *db) SecretUpdate(_ *model.Repo, in *model.Secret) error {
	return d.store.SecretUpdate(in)
}

func (d *db) SecretDelete(repo *model.Repo, name string) error {
	secret, err := d.store.SecretFind(repo, name)
	if err != nil {
		return err
	}
	return d.store.SecretDelete(secret)
}

func (d *db) OrgSecretFind(owner int64, name string) (*model.Secret, error) {
	return d.store.OrgSecretFind(owner, name)
}

func (d *db) OrgSecretList(owner int64, p *model.ListOptions) ([]*model.Secret, error) {
	return d.store.OrgSecretList(owner, p)
}

func (d *db) OrgSecretCreate(_ int64, in *model.Secret) error {
	return d.store.SecretCreate(in)
}

func (d *db) OrgSecretUpdate(_ int64, in *model.Secret) error {
	return d.store.SecretUpdate(in)
}

func (d *db) OrgSecretDelete(owner int64, name string) error {
	secret, err := d.store.OrgSecretFind(owner, name)
	if err != nil {
		return err
	}
	return d.store.SecretDelete(secret)
}

func (d *db) GlobalSecretFind(owner string) (*model.Secret, error) {
	return d.store.GlobalSecretFind(owner)
}

func (d *db) GlobalSecretList(p *model.ListOptions) ([]*model.Secret, error) {
	return d.store.GlobalSecretList(p)
}

func (d *db) GlobalSecretCreate(in *model.Secret) error {
	return d.store.SecretCreate(in)
}

func (d *db) GlobalSecretUpdate(in *model.Secret) error {
	return d.store.SecretUpdate(in)
}

func (d *db) GlobalSecretDelete(name string) error {
	secret, err := d.store.GlobalSecretFind(name)
	if err != nil {
		return err
	}
	return d.store.SecretDelete(secret)
}

func (d *db) SecretTargetCreate(secretID int64, orgID int64, repoID int64) error {
	// Look up the secret to get its name for conflict checking
	secret, err := d.store.SecretFindByID(secretID)
	if err != nil {
		return fmt.Errorf("secret not found: %d", secretID)
	}

	if err := d.store.SecretTargetCheckConflict(secretID, secret.Name, orgID, repoID); err != nil {
		return err
	}

	return d.store.SecretTargetCreate(&model.SecretTarget{
		SecretID: secretID,
		OrgID:    orgID,
		RepoID:   repoID,
	})
}

func (d *db) SecretTargetDelete(secretID int64, orgID int64, repoID int64) error {
	target, err := d.store.SecretTargetFind(secretID, orgID, repoID)
	if err != nil {
		return err
	}
	return d.store.SecretTargetDelete(target)
}

func (d *db) SecretTargetList(secretID int64) ([]*model.SecretTarget, error) {
	return d.store.SecretTargetList(secretID)
}

func (d *db) SecretTransferOwner(secretID int64, newOrgID int64, newRepoID int64, keepOldOwnerAsTarget bool) error {
	secret, err := d.store.SecretFindByID(secretID)
	if err != nil {
		return fmt.Errorf("secret not found: %d", secretID)
	}

	oldOrgID := secret.OrgID
	oldRepoID := secret.RepoID

	secret.OrgID = newOrgID
	secret.RepoID = newRepoID
	if err := d.store.SecretUpdate(secret); err != nil {
		return err
	}

	// Remove the new owner from targets if it was previously a target
	if target, err := d.store.SecretTargetFind(secretID, newOrgID, newRepoID); err == nil && target != nil {
		_ = d.store.SecretTargetDelete(target)
	}

	// Optionally keep old owner as a target
	if keepOldOwnerAsTarget && (oldOrgID != 0 || oldRepoID != 0) {
		_ = d.store.SecretTargetCreate(&model.SecretTarget{
			SecretID: secretID,
			OrgID:    oldOrgID,
			RepoID:   oldRepoID,
		})
	}

	return nil
}
