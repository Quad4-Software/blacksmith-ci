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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/groupmap"
	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

type fakeProvider struct {
	groupmap.Mapping
	name, issuer string
}

func (p fakeProvider) Name() string   { return p.name }
func (p fakeProvider) Issuer() string { return p.issuer }

func newFakeProvider(m groupmap.Mapping) fakeProvider {
	return fakeProvider{Mapping: m, name: "ldap", issuer: "ldap://dir"}
}

func withOpenRegistration(t *testing.T, open bool) {
	t.Helper()
	original := server.Config.Permissions.Open
	server.Config.Permissions.Open = open
	t.Cleanup(func() { server.Config.Permissions.Open = original })
}

func TestSignInIdentity(t *testing.T) {
	t.Run("signs in an existing identity and applies admin mapping", func(t *testing.T) {
		withOpenRegistration(t, false)
		p := newFakeProvider(groupmap.Mapping{AdminGroups: []string{"admins"}})
		_store := new(store_mocks.MockStore)
		identity := &model.UserIdentity{ID: 3, UserID: 7, Provider: "ldap", Issuer: "ldap://dir", Subject: "sub"}
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(identity, nil)
		_store.On("GetUser", int64(7)).Return(&model.User{ID: 7, Login: "jane"}, nil)
		_store.On("UserIdentityUpdate", mock.MatchedBy(func(i *model.UserIdentity) bool {
			return i.ID == 3 && i.Email == "jane@corp.example" && assert.ObjectsAreEqual(model.JSONSliceString{"admins"}, i.Groups) && i.LastLogin > 0
		})).Return(nil).Once()
		_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool { return u.ID == 7 && u.Admin })).Return(nil).Once()

		user, err := signInIdentity(_store, p, &idpClaims{Subject: "sub", Email: "jane@corp.example", Username: "jane", Groups: []string{"admins"}})
		require.NoError(t, err)
		assert.True(t, user.Admin)
		_store.AssertExpectations(t)
	})

	t.Run("refuses registration when it is closed", func(t *testing.T) {
		withOpenRegistration(t, false)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(nil, types.RecordNotExist)

		_, err := signInIdentity(_store, newFakeProvider(groupmap.Mapping{}), &idpClaims{Subject: "sub", Username: "jane"})
		assert.ErrorIs(t, err, errRegistrationClosed)
		_store.AssertNotCalled(t, "UserCreateWithIdentity", mock.Anything, mock.Anything)
	})

	t.Run("registers an unknown identity when registration is open", func(t *testing.T) {
		withOpenRegistration(t, true)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(nil, types.RecordNotExist)
		_store.On("UserCreateWithIdentity",
			mock.MatchedBy(func(u *model.User) bool { return u.Login == "jane" && !u.Admin && u.Hash != "" }),
			mock.MatchedBy(func(i *model.UserIdentity) bool {
				return i.Provider == "ldap" && i.Issuer == "ldap://dir" && i.Subject == "sub" && i.Username == "jane@corp"
			}),
		).Return(nil).Once()

		user, err := signInIdentity(_store, newFakeProvider(groupmap.Mapping{}), &idpClaims{Subject: "sub", Username: "jane@corp"})
		require.NoError(t, err)
		assert.Equal(t, "jane", user.Login)
		_store.AssertExpectations(t)
	})

	t.Run("refuses a login or org name that is taken", func(t *testing.T) {
		withOpenRegistration(t, true)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(nil, types.RecordNotExist)
		_store.On("UserCreateWithIdentity", mock.Anything, mock.Anything).Return(types.ErrNameTaken)

		_, err := signInIdentity(_store, newFakeProvider(groupmap.Mapping{}), &idpClaims{Subject: "sub", Username: "jane"})
		assert.ErrorIs(t, err, errLoginTaken)
	})

	t.Run("refuses a missing or invalid username", func(t *testing.T) {
		withOpenRegistration(t, true)
		for username, want := range map[string]error{"": errMissingUsername, "  ": errMissingUsername, "bad name": errInvalidUsername} {
			_store := new(store_mocks.MockStore)
			_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(nil, types.RecordNotExist)

			_, err := signInIdentity(_store, newFakeProvider(groupmap.Mapping{}), &idpClaims{Subject: "sub", Username: username})
			assert.ErrorIs(t, err, want, "username %q", username)
			_store.AssertNotCalled(t, "UserCreateWithIdentity", mock.Anything, mock.Anything)
		}
	})

	t.Run("revokes a managed role the groups no longer grant", func(t *testing.T) {
		withOpenRegistration(t, false)
		p := newFakeProvider(groupmap.Mapping{RoleGroups: map[string][]string{"operator": {"ops"}}})
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(&model.UserIdentity{ID: 3, UserID: 7}, nil)
		_store.On("GetUser", int64(7)).Return(&model.User{ID: 7, Login: "jane", Roles: model.JSONSliceString{"auditor", "operator"}}, nil)
		_store.On("UserIdentityUpdate", mock.Anything).Return(nil)
		_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool {
			return assert.ObjectsAreEqual(model.JSONSliceString{"auditor"}, u.Roles)
		})).Return(nil).Once()

		user, err := signInIdentity(_store, p, &idpClaims{Subject: "sub", Username: "jane", Groups: []string{"devs"}})
		require.NoError(t, err)
		assert.Equal(t, model.JSONSliceString{"auditor"}, user.Roles)
		_store.AssertExpectations(t)
	})
}

func TestLinkIdentity(t *testing.T) {
	p := newFakeProvider(groupmap.Mapping{})
	claims := &idpClaims{Subject: "sub", Username: "jane"}

	t.Run("refuses an identity owned by another user", func(t *testing.T) {
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(&model.UserIdentity{ID: 3, UserID: 9}, nil)

		_, err := linkIdentity(_store, p, &model.User{ID: 7, Login: "jane"}, claims)
		assert.ErrorIs(t, err, errIdentityTaken)
	})

	t.Run("refuses a second identity of the same provider", func(t *testing.T) {
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(7)).Return([]*model.UserIdentity{{ID: 1, UserID: 7, Provider: "ldap", Subject: "other"}}, nil)

		_, err := linkIdentity(_store, p, &model.User{ID: 7, Login: "jane"}, claims)
		assert.ErrorIs(t, err, errIdentityAlreadyLinked)
		_store.AssertNotCalled(t, "UserIdentityCreate", mock.Anything)
	})

	t.Run("links next to an identity of another provider", func(t *testing.T) {
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(7)).Return([]*model.UserIdentity{{ID: 1, UserID: 7, Provider: "oidc", Subject: "sub"}}, nil)
		_store.On("UserIdentityCreate", mock.MatchedBy(func(i *model.UserIdentity) bool {
			return i.UserID == 7 && i.Provider == "ldap" && i.Issuer == "ldap://dir" && i.Subject == "sub"
		})).Return(nil).Once()

		identity, err := linkIdentity(_store, p, &model.User{ID: 7, Login: "jane"}, claims)
		require.NoError(t, err)
		assert.Equal(t, "ldap", identity.Provider)
		_store.AssertExpectations(t)
	})

	t.Run("requires a legacy forge user to sign in with their forge first", func(t *testing.T) {
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", "ldap://dir", "sub").Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(7)).Return([]*model.UserIdentity{}, nil)
		_store.On("UserForgeList", int64(7)).Return([]*model.UserForge{}, nil)

		_, err := linkIdentity(_store, p, &model.User{ID: 7, Login: "jane"}, claims)
		assert.ErrorIs(t, err, errForgeSignInRequired)
		_store.AssertNotCalled(t, "UserIdentityCreate", mock.Anything)
	})
}

func TestIdentityErrorCode(t *testing.T) {
	assert.Equal(t, "idp_login_taken", identityErrorCode(errLoginTaken))
	assert.Equal(t, "registration_closed", identityErrorCode(errRegistrationClosed))
	assert.Equal(t, "internal_error", identityErrorCode(errors.New("x")))
}
