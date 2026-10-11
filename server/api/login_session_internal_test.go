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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestAdoptPrimaryForgeMovesSSOOrg(t *testing.T) {
	userForge := &model.UserForge{ID: 9, ForgeID: 1, ForgeRemoteID: "remote-42"}
	forgeUser := &model.User{Login: "jane-forge", AccessToken: "at"}
	placeholder := func() *model.Org { return &model.Org{ID: 5, Name: "jane", IsUser: true} }

	t.Run("adopts the forge org and moves secrets and registries", func(t *testing.T) {
		user := &model.User{ID: 42, Login: "jane", OrgID: 5}
		s := store_mocks.NewMockStore(t)
		s.On("OrgGet", int64(5)).Return(placeholder(), nil)
		s.On("OrgFindByName", "jane-forge", int64(1)).Return(&model.Org{ID: 8, Name: "jane-forge", ForgeID: 1}, nil)
		s.On("OrgUpdate", mock.MatchedBy(func(o *model.Org) bool { return o.ID == 8 && o.IsUser })).Return(nil)
		s.On("OrgSecretList", int64(5), mock.Anything).Return([]*model.Secret{
			{ID: 1, OrgID: 5, Name: "token"},
			{ID: 2, OrgID: 77, Name: "shared-from-elsewhere"},
		}, nil)
		s.On("OrgSecretFind", int64(8), "token").Return(nil, types.RecordNotExist)
		s.On("SecretUpdate", mock.MatchedBy(func(sec *model.Secret) bool { return sec.ID == 1 && sec.OrgID == 8 })).Return(nil)
		s.On("OrgRegistryList", int64(5), mock.Anything).Return([]*model.Registry{{ID: 3, OrgID: 5, Address: "docker.io"}}, nil)
		s.On("OrgRegistryFind", int64(8), "docker.io").Return(nil, types.RecordNotExist)
		s.On("RegistryUpdate", mock.MatchedBy(func(r *model.Registry) bool { return r.ID == 3 && r.OrgID == 8 })).Return(nil)
		s.On("OrgDelete", int64(5)).Return(nil)
		s.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool { return u.OrgID == 8 && u.ForgeID == 1 })).Return(nil)

		require.NoError(t, adoptPrimaryForge(s, user, userForge, forgeUser))
		assert.Equal(t, int64(8), user.OrgID)
	})

	t.Run("keeps the SSO org when a secret name is taken in the forge org", func(t *testing.T) {
		user := &model.User{ID: 42, Login: "jane", OrgID: 5}
		s := store_mocks.NewMockStore(t)
		s.On("OrgGet", int64(5)).Return(placeholder(), nil)
		s.On("OrgFindByName", "jane-forge", int64(1)).Return(&model.Org{ID: 8, Name: "jane-forge", ForgeID: 1}, nil)
		s.On("OrgUpdate", mock.Anything).Return(nil)
		s.On("OrgSecretList", int64(5), mock.Anything).Return([]*model.Secret{{ID: 1, OrgID: 5, Name: "token"}}, nil)
		s.On("OrgSecretFind", int64(8), "token").Return(&model.Secret{ID: 4, OrgID: 8, Name: "token"}, nil)
		s.On("OrgRegistryList", int64(5), mock.Anything).Return([]*model.Registry{}, nil)
		s.On("UpdateUser", mock.Anything).Return(nil)

		require.NoError(t, adoptPrimaryForge(s, user, userForge, forgeUser))
		assert.Equal(t, int64(8), user.OrgID)
		s.AssertNotCalled(t, "SecretUpdate", mock.Anything)
		s.AssertNotCalled(t, "OrgDelete", mock.Anything)
	})

	t.Run("leaves an org that already belongs to a forge alone", func(t *testing.T) {
		user := &model.User{ID: 42, Login: "jane", OrgID: 5}
		s := store_mocks.NewMockStore(t)
		s.On("OrgGet", int64(5)).Return(&model.Org{ID: 5, Name: "jane", ForgeID: 2, IsUser: true}, nil)
		s.On("UpdateUser", mock.Anything).Return(nil)

		require.NoError(t, adoptPrimaryForge(s, user, userForge, forgeUser))
		assert.Equal(t, int64(5), user.OrgID)
	})
}
