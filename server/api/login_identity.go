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
	"encoding/base32"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// identityProvider is what the shared sign-in flow needs from OIDC or LDAP.
type identityProvider interface {
	// Name is stored as user_identities.provider.
	Name() string
	// Issuer scopes the subject: the OIDC issuer URL or the LDAP URL.
	Issuer() string
	IsAllowed(groups []string) bool
	RestrictsGroups() bool
	ManagesAdmin() bool
	IsAdmin(groups []string) bool
	ManagedRoles() []string
	RolesFor(groups []string) []string
}

// idpClaims are the verified user attributes from a sign-in with any provider.
type idpClaims struct {
	Subject, Email, Username string
	Groups                   []string
}

// Errors of the shared flow; handlers map them to their error codes.
var (
	errRegistrationClosed    = errors.New("registration_closed")
	errMissingUsername       = errors.New("idp_missing_username")
	errInvalidUsername       = errors.New("idp_invalid_username")
	errLoginTaken            = errors.New("idp_login_taken")
	errIdentityTaken         = errors.New("identity_taken")
	errIdentityAlreadyLinked = errors.New("identity_already_linked")
	errForgeSignInRequired   = errors.New("forge_sign_in_required")
)

var identityErrors = []error{
	errRegistrationClosed,
	errMissingUsername,
	errInvalidUsername,
	errLoginTaken,
	errIdentityTaken,
	errIdentityAlreadyLinked,
	errForgeSignInRequired,
}

// identityErrorCode returns the error code for err: the message of the
// sentinels above, "internal_error" for anything else.
func identityErrorCode(err error) string {
	for _, sentinel := range identityErrors {
		if errors.Is(err, sentinel) {
			return sentinel.Error()
		}
	}
	return "internal_error"
}

// identityProviders returns the enabled identity providers, OIDC first.
func identityProviders() []identityProvider {
	var providers []identityProvider
	// Append only non-nil pointers: a nil pointer in an interface is not nil.
	if p := server.Config.Services.OIDC; p != nil {
		providers = append(providers, p)
	}
	if p := server.Config.Services.LDAP; p != nil {
		providers = append(providers, p)
	}
	return providers
}

// Column sizes of user_identities; longer claims are stored truncated, as they
// are informational only.
const (
	maxIdentityEmailLen    = 500
	maxIdentityUsernameLen = 250
	// The OIDC spec limits sub to 255 ASCII characters.
	maxIdentitySubjectLen = 255
)

func truncateClaim(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}

func refreshIdentity(identity *model.UserIdentity, claims *idpClaims) {
	identity.Email = truncateClaim(claims.Email, maxIdentityEmailLen)
	identity.Username = truncateClaim(claims.Username, maxIdentityUsernameLen)
	identity.Groups = claims.Groups
	identity.LastLogin = time.Now().Unix()
}

// signInIdentity finds or registers the user of an identity, refreshes the
// identity, applies the provider's admin and role mapping and saves the user.
func signInIdentity(_store store.Store, provider identityProvider, claims *idpClaims) (*model.User, error) {
	identity, err := _store.UserIdentityGetByProviderSubject(provider.Name(), provider.Issuer(), claims.Subject)
	var user *model.User
	switch {
	case err == nil:
		user, err = _store.GetUser(identity.UserID)
		if err != nil {
			return nil, fmt.Errorf("cannot get user %d of identity provider %s identity: %w", identity.UserID, provider.Name(), err)
		}
		refreshIdentity(identity, claims)
		if err := _store.UserIdentityUpdate(identity); err != nil {
			log.Error().Err(err).Msgf("cannot update identity provider %s identity of %s", provider.Name(), user.Login)
		}
	case errors.Is(err, types.RecordNotExist):
		if user, err = registerIdentityUser(_store, provider, claims); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("cannot look up identity provider %s identity: %w", provider.Name(), err)
	}

	changed := false
	if provider.ManagesAdmin() {
		// Identity provider sign-in never consults CROW_ADMIN because the username
		// is not forge-verified; admin comes only from the provider's admin groups.
		if admin := provider.IsAdmin(claims.Groups); admin != user.Admin {
			user.Admin = admin
			changed = true
			log.Info().Bool("admin", admin).Msgf("admin status of %s set from identity provider %s groups", user.Login, provider.Name())
		}
	}
	oldRoles := strings.Join(user.Roles, ",")
	if len(provider.ManagedRoles()) > 0 && applyProviderRoles(user, provider, claims.Groups) {
		changed = true
		log.Info().Str("old", oldRoles).Str("new", strings.Join(user.Roles, ",")).Msgf("roles of %s set from identity provider %s groups", user.Login, provider.Name())
	}
	if changed {
		if err := _store.UpdateUser(user); err != nil {
			return nil, fmt.Errorf("cannot update admin status and roles of %s: %w", user.Login, err)
		}
	}
	return user, nil
}

// applyProviderRoles sets every role the provider manages from the user's
// groups, keeps the other valid roles, and drops unknown stored names. It
// reports whether the roles changed.
func applyProviderRoles(user *model.User, provider identityProvider, groups []string) bool {
	managed := provider.ManagedRoles()
	next := []string{}
	for _, r := range user.Roles {
		if model.Role(r).IsValid() && !slices.Contains(managed, r) {
			next = append(next, r)
		}
	}
	next = append(next, provider.RolesFor(groups)...)
	slices.Sort(next)
	next = slices.Compact(next)
	changed := !slices.Equal(next, []string(user.Roles))
	user.Roles = model.JSONSliceString(next)
	return changed
}

// registerIdentityUser creates a user for an unknown identity.
func registerIdentityUser(_store store.Store, provider identityProvider, claims *idpClaims) (*model.User, error) {
	login, valid := oidc.NormalizeUsername(claims.Username)

	// Identity provider sign-in never consults CROW_ADMIN because the username
	// is not forge-verified; admin comes only from the provider's admin groups.
	if !server.Config.Permissions.Open && !provider.RestrictsGroups() && !provider.IsAdmin(claims.Groups) {
		log.Warn().Str("subject", claims.Subject).Msgf("cannot register identity provider %s user, registration closed", provider.Name())
		return nil, errRegistrationClosed
	}
	if strings.TrimSpace(claims.Username) == "" {
		log.Warn().Str("subject", claims.Subject).Msgf("identity provider %s sent no username", provider.Name())
		return nil, errMissingUsername
	}
	if !valid {
		log.Warn().Str("username", claims.Username).Msgf("identity provider %s username is not a valid Crow login", provider.Name())
		return nil, errInvalidUsername
	}

	// Existing users must link their identity explicitly from user settings, so
	// a matching login never signs anyone in. The username comes from the IdP
	// and is not forge-verified, so it must not collide with any user or any org
	// on any forge (case-insensitive): a forge org of that name would look like
	// the user's personal namespace. The store checks this in the same
	// transaction that creates the user, org and identity.
	user := &model.User{
		Login: login,
		Email: truncateClaim(claims.Email, maxIdentityEmailLen),
		Admin: false,
		Hash:  base32.StdEncoding.EncodeToString(random.GetRandomBytes(32)),
	}
	identity := &model.UserIdentity{Provider: provider.Name(), Issuer: provider.Issuer(), Subject: claims.Subject}
	refreshIdentity(identity, claims)
	if err := _store.UserCreateWithIdentity(user, identity); errors.Is(err, types.ErrNameTaken) {
		log.Warn().Msgf("cannot register identity provider %s user %s: login or org name already taken", provider.Name(), login)
		return nil, errLoginTaken
	} else if err != nil {
		return nil, fmt.Errorf("cannot create identity provider %s user %s: %w", provider.Name(), login, err)
	}
	log.Info().Msgf("registered user %s through identity provider %s", login, provider.Name())
	return user, nil
}

// linkIdentity attaches the identity to the user and returns it.
func linkIdentity(_store store.Store, provider identityProvider, user *model.User, claims *idpClaims) (*model.UserIdentity, error) {
	existing, err := _store.UserIdentityGetByProviderSubject(provider.Name(), provider.Issuer(), claims.Subject)
	switch {
	case err == nil && existing.UserID != user.ID:
		log.Warn().Msgf("identity provider %s identity %s is already linked to user %d", provider.Name(), claims.Subject, existing.UserID)
		return nil, errIdentityTaken
	case err == nil:
		refreshIdentity(existing, claims)
		if err := _store.UserIdentityUpdate(existing); err != nil {
			log.Error().Err(err).Msgf("cannot update identity provider %s identity of %s", provider.Name(), user.Login)
		}
		return existing, nil
	case errors.Is(err, types.RecordNotExist):
	default:
		return nil, fmt.Errorf("cannot look up identity provider %s identity: %w", provider.Name(), err)
	}

	identities, err := _store.UserIdentityList(user.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot list identities of %s: %w", user.Login, err)
	}
	for _, id := range identities {
		if id.Provider == provider.Name() {
			return nil, errIdentityAlreadyLinked
		}
	}
	// A legacy forge user with neither a forge remote ID nor a forge link is
	// only found at forge sign-in by login, and that lookup skips users with
	// an identity. They must sign in with their forge once first, which
	// records the forge link.
	if len(identities) == 0 && user.ForgeRemoteID == "" {
		forges, err := _store.UserForgeList(user.ID)
		if err != nil {
			return nil, fmt.Errorf("cannot list forges of %s: %w", user.Login, err)
		}
		if len(forges) == 0 {
			log.Warn().Msgf("user %s must sign in with their forge before linking an identity provider %s identity", user.Login, provider.Name())
			return nil, errForgeSignInRequired
		}
	}
	identity := &model.UserIdentity{UserID: user.ID, Provider: provider.Name(), Issuer: provider.Issuer(), Subject: claims.Subject}
	refreshIdentity(identity, claims)
	if err := _store.UserIdentityCreate(identity); err != nil {
		return nil, fmt.Errorf("cannot link identity provider %s identity to %s: %w", provider.Name(), user.Login, err)
	}
	log.Info().Msgf("user %s linked an identity provider %s identity", user.Login, provider.Name())
	return identity, nil
}
