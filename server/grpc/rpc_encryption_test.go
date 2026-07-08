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

package grpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/encryption/types"
	encryptedStore "codefloe.com/crowci/crow/v6/server/services/encryption/wrapper/store"
	manager_mocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	"codefloe.com/crowci/crow/v6/server/store"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// fakeEncryption is a deterministic, reversible encoding for tests. It is
// emphatically NOT secure — its only purpose is to produce visibly-distinct
// ciphertext from plaintext and to fail on AAD mismatch so this test can
// prove whether the encryption wrapper was bypassed.
type fakeEncryption struct{}

func (fakeEncryption) Encrypt(plaintext, aad string) (string, error) {
	return "enc|" + aad + "|" + base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (fakeEncryption) Decrypt(ciphertext, aad string) (string, error) {
	prefix := "enc|" + aad + "|"
	if !strings.HasPrefix(ciphertext, prefix) {
		return "", fmt.Errorf("aad mismatch or not encrypted: ciphertext=%q aad=%q", ciphertext, aad)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (fakeEncryption) Disable() error { return nil }

// TestUpdateForgeStatus_RequiresEncryptedStore is the regression test for
// codefloe.com/crowci/crow#974 ("Crow won't update commit status in forgejo
// even if the pipeline success"). The bug had two layers:
//
//  1. user_forge tokens were stored as ciphertext bound to placeholder row
//     id 0 (fixed in PR #982 by inserting first, then encrypting). Without
//     that fix, every UserForgeGet decrypt failed.
//
//  2. After (1), the gRPC, cron, integrations and maintenance services were
//     constructed with the unwrapped base store, so even though the wrapper
//     could decrypt correctly, those services bypassed it entirely. The
//     runtime forwarded raw Tink ciphertext as a Bearer token to the forge,
//     which correctly responded "access token does not exist".
//
// This test exercises the gRPC status-update path with both wirings:
//
//   - "encrypted store": (*RPC).updateForgeStatus reads via the encryption
//     wrapper, sees plaintext, and forwards plaintext to forge.Status. This
//     is the post-fix behavior and must always hold.
//
//   - "raw store" (control): (*RPC).updateForgeStatus reads ciphertext
//     directly from the base store and forwards ciphertext to forge.Status.
//     This is the pre-fix behavior and demonstrates that the test would
//     have caught the bug — anyone reverting the wrapper indirection in
//     cmd/server/server.go would also have to delete the control case
//     here, making the regression visible in code review.
func TestUpdateForgeStatus_RequiresEncryptedStore(t *testing.T) {
	const (
		userID           = int64(7)
		forgeID          = int64(3)
		userForgeRowID   = int64(42)
		plaintextAccess  = "plaintext-access-token"
		plaintextRefresh = "plaintext-refresh-token"
	)

	enc := fakeEncryption{}
	ciphertextAccess, err := enc.Encrypt(plaintextAccess, fmt.Sprintf("%d:access", userForgeRowID))
	require.NoError(t, err)
	ciphertextRefresh, err := enc.Encrypt(plaintextRefresh, fmt.Sprintf("%d:refresh", userForgeRowID))
	require.NoError(t, err)
	require.NotEqual(t, plaintextAccess, ciphertextAccess, "fakeEncryption must produce distinct ciphertext")

	repo := &model.Repo{ID: 1, UserID: userID, ForgeID: forgeID, FullName: "test/repo"}
	pipeline := &model.Pipeline{ID: 1, RepoID: 1, Number: 1, Status: model.StatusSuccess}
	workflow := &model.Workflow{ID: 1}

	cases := []struct {
		name              string
		useWrappedStore   bool
		expectedAccessTok string
		expectedRefreshTk string
	}{
		{
			name:              "encrypted store decrypts to plaintext",
			useWrappedStore:   true,
			expectedAccessTok: plaintextAccess,
			expectedRefreshTk: plaintextRefresh,
		},
		{
			name:              "raw store leaks ciphertext (control)",
			useWrappedStore:   false,
			expectedAccessTok: ciphertextAccess,
			expectedRefreshTk: ciphertextRefresh,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origManager := server.Config.Services.Manager
			t.Cleanup(func() { server.Config.Services.Manager = origManager })

			// Base store returns ciphertext on UserForgeGet, mirroring the
			// on-disk DB state after a successful encrypted UserForgeCreate.
			// The user row's tokens are empty — updateForgeStatus overwrites
			// user.AccessToken with userForge.AccessToken anyway, so the
			// user-row tokens are not the bug surface.
			mockBase := store_mocks.NewMockStore(t)
			mockBase.On("GetUser", userID).Return(&model.User{ID: userID, Login: "u"}, nil)
			mockBase.On("UserForgeGet", userID, forgeID).Return(&model.UserForge{
				ID:           userForgeRowID,
				UserID:       userID,
				ForgeID:      forgeID,
				AccessToken:  ciphertextAccess,
				RefreshToken: ciphertextRefresh,
				// Far-future expiry so RefreshUserForge's fast-path check
				// short-circuits; combined with MockForge not implementing
				// the Refresher interface, no token-refresh code runs.
				Expiry: time.Now().Add(24 * time.Hour).Unix(),
			}, nil)

			var rpcStore store.Store
			if tc.useWrappedStore {
				rpcStore = newEncryptedComposite(t, mockBase, enc)
			} else {
				rpcStore = mockBase
			}

			// Capturing forge: record the access/refresh tokens that
			// updateForgeStatus actually forwards on the Status call.
			mockForge := forge_mocks.NewMockForge(t)
			var capturedAccess, capturedRefresh string
			mockForge.On("Status",
				mock.Anything, // ctx
				mock.AnythingOfType("*model.User"),
				repo, pipeline, workflow,
			).Run(func(args mock.Arguments) {
				u, ok := args.Get(1).(*model.User)
				require.True(t, ok, "Status arg #1 must be *model.User")
				// model.User.AccessToken / RefreshToken are marked
				// deprecated in favor of UserForge, but updateForgeStatus
				// still copies the UserForge tokens onto these fields and
				// every forge's Status implementation reads them. This
				// test asserts on the exact value the forge would see;
				// the project's golangci-lint config already excludes
				// SA1019 for the migration period.
				capturedAccess = u.AccessToken
				capturedRefresh = u.RefreshToken
			}).Return(nil)

			mockMgr := manager_mocks.NewMockManager(t)
			mockMgr.On("ForgeFromRepo", repo).Return(mockForge, nil)
			server.Config.Services.Manager = mockMgr

			rpc := &RPC{store: rpcStore, metrics: newTestGRPCMetrics()}
			rpc.updateForgeStatus(context.Background(), repo, pipeline, workflow)

			assert.Equal(t, tc.expectedAccessTok, capturedAccess,
				"forge.Status must receive the expected access token for this wiring")
			assert.Equal(t, tc.expectedRefreshTk, capturedRefresh,
				"forge.Status must receive the expected refresh token for this wiring")
		})
	}
}

// newEncryptedComposite mirrors the production setupEncryptedStore flow,
// minus the cli-driven Builder, so the test exercises the same Composite
// wrapper that ships in production. Any divergence would weaken the
// regression guarantee.
func newEncryptedComposite(t *testing.T, base store.Store, enc types.EncryptionService) store.Store {
	t.Helper()

	composite := encryptedStore.NewCompositeStore(
		base,
		encryptedStore.NewSecretStore(base),
		encryptedStore.NewRegistryStore(base),
		encryptedStore.NewUserStore(base),
		encryptedStore.NewForgeStore(base),
		encryptedStore.NewUserForgeStore(base),
	)
	require.NoError(t, composite.SetEncryptionService(enc))
	return composite
}
