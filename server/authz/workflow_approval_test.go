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

package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	services_mocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestCanApproveWorkflow_EmptyListFallsBackToRepoPerm(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	user := &model.User{ID: 1, Login: "alice"}
	repo := &model.Repo{ID: 10, ForgeID: 100, FullName: "org/r"}
	wf := &model.Workflow{ID: 50, RequiresApproval: true}

	st.On("PermFind", user, repo).Return(&model.Perm{Push: true}, nil)

	ok, _, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestCanApproveWorkflow_EmptyListAdminAllowed(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	user := &model.User{ID: 1, Login: "alice"}
	repo := &model.Repo{ID: 10, ForgeID: 100, FullName: "org/r"}
	wf := &model.Workflow{ID: 50, RequiresApproval: true}

	st.On("PermFind", user, repo).Return(&model.Perm{Admin: true}, nil)

	ok, _, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestCanApproveWorkflow_EmptyListNoPermDenies(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	user := &model.User{ID: 1, Login: "alice"}
	repo := &model.Repo{ID: 10, ForgeID: 100, FullName: "org/r"}
	wf := &model.Workflow{ID: 50, RequiresApproval: true}

	st.On("PermFind", user, repo).Return(&model.Perm{Push: false}, nil)

	ok, reason, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, reason, "lacking repo approve permission")
}

func TestCanApproveWorkflow_DirectUserMatch(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	user := &model.User{ID: 1, Login: "alice_crow"}
	repo := &model.Repo{ID: 10, ForgeID: 100, FullName: "org/r"}
	wf := &model.Workflow{
		ID: 50, RequiresApproval: true,
		ApprovalUsers: []string{"alice"},
	}

	st.On("UserForgeGet", int64(1), int64(100)).
		Return(&model.UserForge{ForgeLogin: "alice"}, nil)

	ok, _, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestCanApproveWorkflow_NoLinkedForgeIdentityDenies(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	user := &model.User{ID: 1, Login: "alice"}
	repo := &model.Repo{ID: 10, ForgeID: 100, FullName: "org/r"}
	wf := &model.Workflow{
		ID: 50, RequiresApproval: true,
		ApprovalUsers: []string{"someone"},
	}

	st.On("UserForgeGet", int64(1), int64(100)).
		Return((*model.UserForge)(nil), errors.New("not found"))

	ok, reason, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, reason, "no linked identity on forge")
}

func TestCanApproveWorkflow_TeamMatch(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	frg := forge_mocks.NewMockForge(t)

	user := &model.User{ID: 1}
	repo := &model.Repo{ID: 10, ForgeID: 100}
	wf := &model.Workflow{
		ID: 50, RequiresApproval: true,
		ApprovalUsers: []string{},
		ApprovalTeams: []string{"myorg/admins"},
	}

	st.On("UserForgeGet", int64(1), int64(100)).
		Return(&model.UserForge{ForgeLogin: "bob"}, nil)
	fmgr.On("ForgeFromRepo", repo).Return(frg, nil)
	frg.On("TeamMemberships", ctx, mock.Anything).
		Return([]*model.TeamMembership{{Org: "myorg", Team: "admins"}}, nil)

	ok, _, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestCanApproveWorkflow_TeamNoMatchDenies(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	frg := forge_mocks.NewMockForge(t)

	user := &model.User{ID: 1}
	repo := &model.Repo{ID: 10, ForgeID: 100}
	wf := &model.Workflow{
		ID: 50, RequiresApproval: true,
		ApprovalUsers: []string{"someoneelse"},
		ApprovalTeams: []string{"otherorg/admins"},
	}

	st.On("UserForgeGet", int64(1), int64(100)).
		Return(&model.UserForge{ForgeLogin: "bob"}, nil)
	fmgr.On("ForgeFromRepo", repo).Return(frg, nil)
	frg.On("TeamMemberships", ctx, mock.Anything).
		Return([]*model.TeamMembership{{Org: "myorg", Team: "admins"}}, nil)

	ok, reason, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, reason, "not in approvers list")
}

func TestCanApproveWorkflow_TeamMatch_GitLabSubgroup(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	frg := forge_mocks.NewMockForge(t)

	user := &model.User{ID: 1}
	repo := &model.Repo{ID: 10, ForgeID: 100}
	wf := &model.Workflow{
		ID: 50, RequiresApproval: true,
		ApprovalTeams: []string{"parent/sub/team"},
	}

	st.On("UserForgeGet", int64(1), int64(100)).
		Return(&model.UserForge{ForgeLogin: "bob"}, nil)
	fmgr.On("ForgeFromRepo", repo).Return(frg, nil)
	frg.On("TeamMemberships", ctx, mock.Anything).
		Return([]*model.TeamMembership{{Org: "parent", Team: "sub/team"}}, nil)

	ok, _, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestCanApproveWorkflow_TeamNoMatch_StrictNoInheritance(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	frg := forge_mocks.NewMockForge(t)

	user := &model.User{ID: 1}
	repo := &model.Repo{ID: 10, ForgeID: 100}
	wf := &model.Workflow{
		ID: 50, RequiresApproval: true,
		ApprovalTeams: []string{"parent/sub"},
	}

	st.On("UserForgeGet", int64(1), int64(100)).
		Return(&model.UserForge{ForgeLogin: "bob"}, nil)
	fmgr.On("ForgeFromRepo", repo).Return(frg, nil)
	frg.On("TeamMemberships", ctx, mock.Anything).
		Return([]*model.TeamMembership{{Org: "parent", Team: "other"}}, nil)

	ok, reason, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, reason, "not in approvers list")
}

func TestCanApproveWorkflow_TeamNotSupportedByForge(t *testing.T) {
	ctx := context.Background()
	st := store_mocks.NewMockStore(t)
	fmgr := services_mocks.NewMockManager(t)
	frg := forge_mocks.NewMockForge(t)

	user := &model.User{ID: 1}
	repo := &model.Repo{ID: 10, ForgeID: 100}
	wf := &model.Workflow{
		ID: 50, RequiresApproval: true,
		ApprovalTeams: []string{"myorg/admins"},
	}

	st.On("UserForgeGet", int64(1), int64(100)).
		Return(&model.UserForge{ForgeLogin: "bob"}, nil)
	fmgr.On("ForgeFromRepo", repo).Return(frg, nil)
	frg.On("TeamMemberships", ctx, mock.Anything).
		Return(nil, nil) // forge has no team concept

	ok, reason, err := CanApproveWorkflow(ctx, st, fmgr, user, wf, repo)
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, reason, "not in approvers list")
}
