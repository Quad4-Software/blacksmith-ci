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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/shared/httputil"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// accountsCookie holds the session tokens of every account the browser has
// authenticated with, enabling quick-switching between them without a full
// logout/login cycle. It is HttpOnly + Secure just like the active session
// cookie, so the tokens never leave the server/browser trust boundary.
const accountsCookie = "crow_accounts"

// maxSwitchAccounts caps how many accounts are remembered for quick-switching
// so the cookie stays well below the ~4KB browser limit.
const maxSwitchAccounts = 8

// minSwitchAccounts is the number of accounts below which the switcher is not
// worth surfacing (a single account needs no switching UI).
const minSwitchAccounts = 2

// sessionAccount is a single remembered account: its user ID plus the signed
// session token proving the browser authenticated as that user.
type sessionAccount struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
}

// AccountInfo is the display representation of a remembered account, injected
// into the web config for the account-switcher UI.
type AccountInfo struct {
	ID     int64  `json:"id"`
	Login  string `json:"login"`
	Avatar string `json:"avatar_url"`
	Active bool   `json:"active"`
}

// readAccounts decodes the remembered accounts from the request cookie.
// An absent or malformed cookie yields an empty list rather than an error.
func readAccounts(r *http.Request) []sessionAccount {
	cookie, err := r.Cookie(accountsCookie)
	if err != nil || cookie.Value == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}
	var accounts []sessionAccount
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return nil
	}
	return accounts
}

// writeAccounts persists the remembered accounts back to the cookie. An empty
// list clears the cookie entirely.
func writeAccounts(w http.ResponseWriter, r *http.Request, accounts []sessionAccount) {
	if len(accounts) == 0 {
		httputil.DelCookie(w, r, accountsCookie)
		return
	}
	raw, err := json.Marshal(accounts)
	if err != nil {
		log.Error().Err(err).Msg("cannot marshal switch accounts")
		return
	}
	httputil.SetCookie(w, r, accountsCookie, base64.RawURLEncoding.EncodeToString(raw))
}

// rememberAccount adds (or refreshes) an account in the remembered list and
// moves it to the front. The active account is always kept first so that
// logout can fall back to the most recently used remaining account.
func rememberAccount(w http.ResponseWriter, r *http.Request, id int64, tokenString string) {
	accounts := readAccounts(r)

	next := make([]sessionAccount, 0, len(accounts)+1)
	next = append(next, sessionAccount{ID: id, Token: tokenString})
	for _, acc := range accounts {
		if acc.ID == id {
			continue // replaced by the fresh entry above
		}
		next = append(next, acc)
		if len(next) >= maxSwitchAccounts {
			break
		}
	}

	writeAccounts(w, r, next)
}

// forgetAccount removes an account from the remembered list and returns the
// updated list.
func forgetAccount(w http.ResponseWriter, r *http.Request, id int64) []sessionAccount {
	accounts := readAccounts(r)
	next := make([]sessionAccount, 0, len(accounts))
	for _, acc := range accounts {
		if acc.ID != id {
			next = append(next, acc)
		}
	}
	writeAccounts(w, r, next)
	return next
}

// validAccountToken parses a stored session token and returns the owning user
// when the token is still valid. Invalid/expired tokens return nil so callers
// can prune them.
func validAccountToken(_store store.Store, acc sessionAccount) *model.User {
	var user *model.User
	_, err := token.Parse([]token.Type{token.SessToken}, acc.Token, func(t *token.Token) (string, error) {
		userID, err := strconv.ParseInt(t.Get("user-id"), 10, 64)
		if err != nil {
			return "", err
		}
		user, err = _store.GetUser(userID)
		if err != nil {
			return "", err
		}
		return user.Hash, nil
	})
	if err != nil || user == nil || user.ID != acc.ID {
		return nil
	}
	return user
}

// SwitchAccounts resolves the remembered accounts into display info for the
// active browser, pruning any stale tokens along the way. The active user is
// always included even if the cookie predates this feature.
func SwitchAccounts(c *gin.Context, activeUser *model.User) []AccountInfo {
	if activeUser == nil {
		return nil
	}
	_store := store.FromContext(c)

	accounts := readAccounts(c.Request)
	infos := make([]AccountInfo, 0, len(accounts)+1)
	seen := make(map[int64]bool, len(accounts)+1)
	pruned := false
	next := make([]sessionAccount, 0, len(accounts))

	for _, acc := range accounts {
		user := validAccountToken(_store, acc)
		if user == nil {
			pruned = true
			continue
		}
		next = append(next, acc)
		if seen[user.ID] {
			continue
		}
		seen[user.ID] = true
		infos = append(infos, AccountInfo{
			ID:     user.ID,
			Login:  user.Login,
			Avatar: user.Avatar,
			Active: user.ID == activeUser.ID,
		})
	}

	// Ensure the active account is always present (e.g. sessions created before
	// this feature existed have no entry in the accounts cookie yet).
	if !seen[activeUser.ID] {
		infos = append([]AccountInfo{{
			ID:     activeUser.ID,
			Login:  activeUser.Login,
			Avatar: activeUser.Avatar,
			Active: true,
		}}, infos...)
	}

	if pruned {
		writeAccounts(c.Writer, c.Request, next)
	}

	// A single account is not worth surfacing a switcher for.
	if len(infos) < minSwitchAccounts {
		return nil
	}
	return infos
}

// SwitchAccount swaps the active session cookie to a different, already
// authenticated account. Only accounts whose session token is present in the
// crow_accounts cookie and still valid can be activated, so this never mints a
// session for an account the browser has not logged into.
func SwitchAccount(c *gin.Context) {
	rootPath := server.Config.Server.RootPath

	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil {
		c.Redirect(http.StatusSeeOther, rootPath+"/")
		return
	}

	_store := store.FromContext(c)
	for _, acc := range readAccounts(c.Request) {
		if acc.ID != id {
			continue
		}
		if validAccountToken(_store, acc) == nil {
			// Stale token: drop it and bail out to a clean state.
			forgetAccount(c.Writer, c.Request, id)
			break
		}
		httputil.SetCookie(c.Writer, c.Request, "user_sess", acc.Token)
		c.Redirect(http.StatusSeeOther, rootPath+"/")
		return
	}

	log.Warn().Int64("user_id", id).Msg("cannot switch to unknown account")
	c.Redirect(http.StatusSeeOther, rootPath+"/")
}
