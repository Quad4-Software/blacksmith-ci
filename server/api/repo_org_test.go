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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/secret"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore"
)

const repoOrgTestForgeID int64 = 1

// repoOrgTestUserID is the Crow user syncing the repo; it does not exist in
// the store, so users created by tests never collide with it.
const repoOrgTestUserID int64 = 1000

type repoOrgFixture struct {
	store  store.Store
	oldOrg *model.Org
	newOrg *model.Org
	repo   *model.Repo
}

// newRepoOrgFixture creates a repo owned by org "old-org", plus an org
// "new-org", each with an org secret and an org registry.
func newRepoOrgFixture(t *testing.T) *repoOrgFixture {
	t.Helper()
	s, err := datastore.NewEngine(&store.Opts{Driver: "sqlite3", Config: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background(), false))
	t.Cleanup(func() { _ = s.Close() })

	oldOrg := &model.Org{Name: "old-org", ForgeID: repoOrgTestForgeID}
	newOrg := &model.Org{Name: "new-org", ForgeID: repoOrgTestForgeID}
	require.NoError(t, s.OrgCreate(oldOrg))
	require.NoError(t, s.OrgCreate(newOrg))

	for _, org := range []*model.Org{oldOrg, newOrg} {
		require.NoError(t, s.SecretCreate(&model.Secret{
			OrgID:  org.ID,
			Name:   org.Name + "_token",
			Value:  "value-of-" + org.Name,
			Events: []model.WebhookEvent{model.EventPush},
		}))
		require.NoError(t, s.RegistryCreate(&model.Registry{
			OrgID:    org.ID,
			Address:  org.Name + ".registry.example.com",
			Username: org.Name,
			Password: "password-of-" + org.Name,
		}))
	}

	repo := &model.Repo{
		ForgeID:       repoOrgTestForgeID,
		ForgeRemoteID: "42",
		OrgID:         oldOrg.ID,
		Owner:         oldOrg.Name,
		Name:          "app",
		FullName:      oldOrg.Name + "/app",
		IsActive:      true,
	}
	require.NoError(t, s.CreateRepo(repo))

	return &repoOrgFixture{store: s, oldOrg: oldOrg, newOrg: newOrg, repo: repo}
}

func secretNames(t *testing.T, s store.Store, repo *model.Repo) []string {
	t.Helper()
	secrets, err := secret.NewDB(s).SecretListPipeline(repo, &model.Pipeline{})
	require.NoError(t, err)
	names := make([]string, 0, len(secrets))
	for _, sec := range secrets {
		names = append(names, sec.Name)
	}
	return names
}

func registryAddresses(t *testing.T, s store.Store, repo *model.Repo) []string {
	t.Helper()
	registries, err := s.RegistryList(repo, true, &model.ListOptions{All: true})
	require.NoError(t, err)
	addresses := make([]string, 0, len(registries))
	for _, reg := range registries {
		addresses = append(addresses, reg.Address)
	}
	return addresses
}

// transferRepo simulates what the hook, move and repair paths do when the
// forge reports a new owner: Update the repo from the forge, sync its org,
// persist it and reload it.
func transferRepo(t *testing.T, f *repoOrgFixture, _forge *forge_mocks.MockForge, newOwner string) *model.Repo {
	t.Helper()
	from := &model.Repo{
		ForgeRemoteID: f.repo.ForgeRemoteID,
		Owner:         newOwner,
		Name:          f.repo.Name,
		FullName:      newOwner + "/" + f.repo.Name,
	}
	f.repo.Update(from)
	require.NoError(t, syncRepoOrg(context.Background(), f.store, _forge, &model.User{ID: repoOrgTestUserID}, f.repo))
	require.NoError(t, f.store.UpdateRepo(f.repo))

	reloaded, err := f.store.GetRepo(f.repo.ID)
	require.NoError(t, err)
	return reloaded
}

func TestSyncRepoOrgAfterTransferToKnownOrg(t *testing.T) {
	f := newRepoOrgFixture(t)
	assert.Contains(t, secretNames(t, f.store, f.repo), "old-org_token")

	// The new org is already known to Crow, so the forge is never asked.
	repo := transferRepo(t, f, forge_mocks.NewMockForge(t), "new-org")

	assert.Equal(t, f.newOrg.ID, repo.OrgID)
	names := secretNames(t, f.store, repo)
	assert.Contains(t, names, "new-org_token")
	assert.NotContains(t, names, "old-org_token")
	addresses := registryAddresses(t, f.store, repo)
	assert.Contains(t, addresses, "new-org.registry.example.com")
	assert.NotContains(t, addresses, "old-org.registry.example.com")
}

func TestSyncRepoOrgAfterTransferToUnknownOrg(t *testing.T) {
	f := newRepoOrgFixture(t)

	_forge := forge_mocks.NewMockForge(t)
	_forge.On("Org", mock.Anything, mock.Anything, "fresh-org").
		Return(&model.Org{Name: "fresh-org"}, nil).Once()
	// The old org still exists on the forge, so this is a transfer.
	_forge.On("Org", mock.Anything, mock.Anything, "old-org").
		Return(&model.Org{Name: "old-org"}, nil).Once()

	repo := transferRepo(t, f, _forge, "fresh-org")

	created, err := f.store.OrgFindByName("fresh-org", repoOrgTestForgeID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, repo.OrgID)
	assert.Equal(t, repoOrgTestForgeID, created.ForgeID)
	assert.NotContains(t, secretNames(t, f.store, repo), "old-org_token")
	assert.NotContains(t, registryAddresses(t, f.store, repo), "old-org.registry.example.com")
}

// addSibling adds another repo of the old org, owned by the Crow user userID.
func addSibling(t *testing.T, f *repoOrgFixture, userID int64) *model.Repo {
	t.Helper()
	sibling := &model.Repo{
		ForgeID:       repoOrgTestForgeID,
		ForgeRemoteID: "43",
		OrgID:         f.oldOrg.ID,
		UserID:        userID,
		Owner:         f.oldOrg.Name,
		Name:          "lib",
		FullName:      f.oldOrg.Name + "/lib",
		IsActive:      true,
	}
	require.NoError(t, f.store.CreateRepo(sibling))
	return sibling
}

func expectNewOwner(_forge *forge_mocks.MockForge, name string) {
	_forge.On("Org", mock.Anything, mock.Anything, name).Return(&model.Org{Name: name}, nil).Once()
}

func expectOldOrgError(_forge *forge_mocks.MockForge) {
	_forge.On("Org", mock.Anything, mock.Anything, "old-org").Return(nil, assert.AnError).Once()
}

// assertTransferred checks that repo moved to a new org named "new-owner".
func assertTransferred(t *testing.T, f *repoOrgFixture, repo *model.Repo) {
	t.Helper()
	created, err := f.store.OrgFindByName("new-owner", repoOrgTestForgeID)
	require.NoError(t, err)
	assert.NotEqual(t, f.oldOrg.ID, created.ID)
	assert.Equal(t, created.ID, repo.OrgID)
	old, err := f.store.OrgGet(f.oldOrg.ID)
	require.NoError(t, err)
	assert.Equal(t, "old-org", old.Name, "the old org must not be renamed")
	assert.NotContains(t, secretNames(t, f.store, repo), "old-org_token")
	assert.NotContains(t, registryAddresses(t, f.store, repo), "old-org.registry.example.com")
}

func assertRenamed(t *testing.T, f *repoOrgFixture, repo *model.Repo, newOwner string) {
	t.Helper()
	assert.Equal(t, f.oldOrg.ID, repo.OrgID)
	renamed, err := f.store.OrgGet(f.oldOrg.ID)
	require.NoError(t, err)
	assert.Equal(t, newOwner, renamed.Name)
	assert.Contains(t, secretNames(t, f.store, repo), "old-org_token")
	assert.Contains(t, registryAddresses(t, f.store, repo), "old-org.registry.example.com")

	// Another repo of the renamed org now matches without any forge call.
	other := &model.Repo{ForgeID: repoOrgTestForgeID, OrgID: f.oldOrg.ID, Owner: newOwner}
	require.NoError(t, syncRepoOrg(context.Background(), f.store, forge_mocks.NewMockForge(t), &model.User{ID: 1}, other))
	assert.Equal(t, f.oldOrg.ID, other.OrgID)
}

func TestSyncRepoOrgTransferFromInaccessibleOrg(t *testing.T) {
	// The old org is private and the repo's user is not a member of it, so the
	// forge cannot resolve its name, but its other repo still lives there.
	f := newRepoOrgFixture(t)
	sibling := addSibling(t, f, repoOrgTestUserID)

	_forge := forge_mocks.NewMockForge(t)
	expectNewOwner(_forge, "new-owner")
	expectOldOrgError(_forge)
	_forge.On("Repo", mock.Anything, mock.Anything, sibling.ForgeRemoteID, "old-org", "lib").
		Return(&model.Repo{Owner: "old-org", Name: "lib"}, nil).Once()

	repo := transferRepo(t, f, _forge, "new-owner")
	assertTransferred(t, f, repo)
}

func TestSyncRepoOrgTransferWithoutEvidence(t *testing.T) {
	t.Run("no siblings", func(t *testing.T) {
		f := newRepoOrgFixture(t)
		_forge := forge_mocks.NewMockForge(t)
		expectNewOwner(_forge, "new-owner")
		expectOldOrgError(_forge)

		repo := transferRepo(t, f, _forge, "new-owner")
		assertTransferred(t, f, repo)
	})

	t.Run("sibling fetch fails", func(t *testing.T) {
		f := newRepoOrgFixture(t)
		addSibling(t, f, repoOrgTestUserID)
		_forge := forge_mocks.NewMockForge(t)
		expectNewOwner(_forge, "new-owner")
		expectOldOrgError(_forge)
		_forge.On("Repo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, assert.AnError).Once()

		repo := transferRepo(t, f, _forge, "new-owner")
		assertTransferred(t, f, repo)
	})

	t.Run("sibling user has no token for this forge", func(t *testing.T) {
		f := newRepoOrgFixture(t)
		other := &model.User{Login: "other", Hash: "hash-other", ForgeID: repoOrgTestForgeID + 1, AccessToken: "other-forge-token"}
		require.NoError(t, f.store.CreateUser(other))
		addSibling(t, f, other.ID)
		// The mock fails the test if the sibling is fetched with the other
		// forge's token.
		_forge := forge_mocks.NewMockForge(t)
		expectNewOwner(_forge, "new-owner")
		expectOldOrgError(_forge)

		repo := transferRepo(t, f, _forge, "new-owner")
		assertTransferred(t, f, repo)
	})
}

func TestSyncRepoOrgTransferWhileOldOrgExists(t *testing.T) {
	// The old name resolves to its own namespace, so siblings that moved too
	// are no evidence of a rename.
	f := newRepoOrgFixture(t)
	addSibling(t, f, repoOrgTestUserID)
	_forge := forge_mocks.NewMockForge(t)
	expectNewOwner(_forge, "new-owner")
	_forge.On("Org", mock.Anything, mock.Anything, "old-org").Return(&model.Org{Name: "old-org"}, nil).Once()

	repo := transferRepo(t, f, _forge, "new-owner")
	assertTransferred(t, f, repo)
}

func TestSyncRepoOrgTransferWhenSiblingsDisagree(t *testing.T) {
	f := newRepoOrgFixture(t)
	moved := addSibling(t, f, repoOrgTestUserID)
	stayed := &model.Repo{
		ForgeID: repoOrgTestForgeID, ForgeRemoteID: "44", OrgID: f.oldOrg.ID, UserID: repoOrgTestUserID,
		Owner: "old-org", Name: "docs", FullName: "old-org/docs",
	}
	require.NoError(t, f.store.CreateRepo(stayed))

	_forge := forge_mocks.NewMockForge(t)
	expectNewOwner(_forge, "new-owner")
	expectOldOrgError(_forge)
	_forge.On("Repo", mock.Anything, mock.Anything, moved.ForgeRemoteID, mock.Anything, mock.Anything).
		Return(&model.Repo{Owner: "new-owner"}, nil).Maybe()
	_forge.On("Repo", mock.Anything, mock.Anything, stayed.ForgeRemoteID, mock.Anything, mock.Anything).
		Return(&model.Repo{Owner: "old-org"}, nil).Once()

	repo := transferRepo(t, f, _forge, "new-owner")
	assertTransferred(t, f, repo)
}

func TestSyncRepoOrgRenameByRedirect(t *testing.T) {
	f := newRepoOrgFixture(t)
	_forge := forge_mocks.NewMockForge(t)
	_forge.On("Org", mock.Anything, mock.Anything, "renamed-org").
		Return(&model.Org{Name: "renamed-org", IsUser: true}, nil).Once()
	// The forge redirects the old name to the renamed org.
	_forge.On("Org", mock.Anything, mock.Anything, "old-org").
		Return(&model.Org{Name: "renamed-org", IsUser: true}, nil).Once()

	repo := transferRepo(t, f, _forge, "renamed-org")
	assertRenamed(t, f, repo, "renamed-org")
	renamed, err := f.store.OrgGet(f.oldOrg.ID)
	require.NoError(t, err)
	assert.True(t, renamed.IsUser)
}

func TestSyncRepoOrgRenameBySibling(t *testing.T) {
	f := newRepoOrgFixture(t)
	// The sibling belongs to another Crow user, so it is fetched with that
	// user's token for this forge.
	other := &model.User{Login: "other", Hash: "hash-other", ForgeID: repoOrgTestForgeID + 1, AccessToken: "other-forge-token"}
	require.NoError(t, f.store.CreateUser(other))
	require.NoError(t, f.store.UserForgeCreate(&model.UserForge{
		UserID:        other.ID,
		ForgeID:       repoOrgTestForgeID,
		ForgeRemoteID: "99",
		AccessToken:   "this-forge-token",
	}))
	sibling := addSibling(t, f, other.ID)

	_forge := forge_mocks.NewMockForge(t)
	expectNewOwner(_forge, "renamed-org")
	expectOldOrgError(_forge)
	_forge.On("Repo", mock.Anything, mock.MatchedBy(func(u *model.User) bool {
		return u.ID == other.ID && u.AccessToken == "this-forge-token"
	}), sibling.ForgeRemoteID, "old-org", "lib").
		Return(&model.Repo{Owner: "renamed-org", Name: "lib"}, nil).Once()

	repo := transferRepo(t, f, _forge, "renamed-org")
	assertRenamed(t, f, repo, "renamed-org")
}

func TestSyncRepoOrgHealsStaleOrgID(t *testing.T) {
	// A repo transferred before the fix has the new owner but the old OrgID.
	f := newRepoOrgFixture(t)
	f.repo.Owner = f.newOrg.Name
	f.repo.FullName = f.newOrg.Name + "/app"
	require.NoError(t, f.store.UpdateRepo(f.repo))

	require.NoError(t, syncRepoOrg(context.Background(), f.store, forge_mocks.NewMockForge(t), &model.User{ID: 1}, f.repo))
	assert.Equal(t, f.newOrg.ID, f.repo.OrgID)
}

func TestSyncRepoOrgKeepsUnchangedOwner(t *testing.T) {
	f := newRepoOrgFixture(t)

	// Owner names are case-insensitive on the forges, so a case change is not
	// a transfer.
	repo := transferRepo(t, f, forge_mocks.NewMockForge(t), "Old-Org")

	assert.Equal(t, f.oldOrg.ID, repo.OrgID)
	assert.Contains(t, secretNames(t, f.store, repo), "old-org_token")
}

func TestSyncRepoOrgForgeError(t *testing.T) {
	f := newRepoOrgFixture(t)

	_forge := forge_mocks.NewMockForge(t)
	_forge.On("Org", mock.Anything, mock.Anything, "gone-org").
		Return(nil, assert.AnError).Once()

	f.repo.Owner = "gone-org"
	err := syncRepoOrg(context.Background(), f.store, _forge, &model.User{ID: 1}, f.repo)
	require.Error(t, err)
	// The caller aborts on the error, so the stale OrgID is never persisted
	// alongside the new owner.
	assert.Equal(t, f.oldOrg.ID, f.repo.OrgID)
}
