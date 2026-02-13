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
	"crypto/sha256"
	"encoding/hex"

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) AccessTokenCreate(token *model.AccessToken) error {
	_, err := s.engine.Insert(token)
	return err
}

func (s storage) AccessTokenFind(id int64) (*model.AccessToken, error) {
	token := new(model.AccessToken)
	return token, wrapGet(s.engine.ID(id).Get(token))
}

func (s storage) AccessTokenFindByHash(hash string) (*model.AccessToken, error) {
	token := new(model.AccessToken)
	return token, wrapGet(s.engine.Where("token_hash = ?", hash).Get(token))
}

func (s storage) AccessTokenList(userID int64, p *model.ListOptions) ([]*model.AccessToken, error) {
	tokens := make([]*model.AccessToken, 0)
	return tokens, s.paginate(p).Where("user_id = ?", userID).
		OrderBy("created_at DESC").Find(&tokens)
}

func (s storage) AccessTokenUpdate(token *model.AccessToken) error {
	_, err := s.engine.ID(token.ID).AllCols().Update(token)
	return err
}

func (s storage) AccessTokenDelete(token *model.AccessToken) error {
	return wrapDelete(s.engine.ID(token.ID).Delete(new(model.AccessToken)))
}

func (s storage) AccessTokenUpdateLastUsed(id int64, timestamp int64) error {
	_, err := s.engine.ID(id).Cols("last_used").Update(&model.AccessToken{
		LastUsed: timestamp,
	})
	return err
}

// HashAccessToken creates a SHA256 hash of a token for storage.
func HashAccessToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
