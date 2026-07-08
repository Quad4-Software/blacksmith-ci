// Copyright 2024 Woodpecker Authors
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

package datastore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) AccessTokenCreate(token *model.AccessToken) error {
	_, err := s.db.NewInsert().Model(token).Exec(context.Background())
	return err
}

func (s storage) AccessTokenFind(id int64) (*model.AccessToken, error) {
	token := new(model.AccessToken)
	return token, wrapGet(s.db.NewSelect().Model(token).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) AccessTokenFindByHash(hash string) (*model.AccessToken, error) {
	token := new(model.AccessToken)
	return token, wrapGet(s.db.NewSelect().Model(token).Where("token_hash = ?", hash).Scan(context.Background()))
}

func (s storage) AccessTokenList(userID int64, p *model.ListOptions) ([]*model.AccessToken, error) {
	tokens := make([]*model.AccessToken, 0)
	return tokens, s.paginate(s.db.NewSelect().Model(&tokens).Where("user_id = ?", userID).Order("created_at DESC"), p).Scan(context.Background())
}

func (s storage) AccessTokenUpdate(token *model.AccessToken) error {
	_, err := s.db.NewUpdate().Model(token).WherePK().Exec(context.Background())
	return err
}

func (s storage) AccessTokenDelete(token *model.AccessToken) error {
	return wrapDelete(s.db.NewDelete().Model((*model.AccessToken)(nil)).Where("id = ?", token.ID).Exec(context.Background()))
}

func (s storage) AccessTokenUpdateLastUsed(id int64, timestamp int64) error {
	_, err := s.db.NewUpdate().Model(&model.AccessToken{LastUsed: timestamp}).Column("last_used").Where("id = ?", id).Exec(context.Background())
	return err
}

// HashAccessToken creates a SHA256 hash of a token for storage.
func HashAccessToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
