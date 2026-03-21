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
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/shared/token"
)

type GetUnsubscribeInput struct {
	Token string `query:"token" required:"true" doc:"signed unsubscribe token"`
}

func GetUnsubscribe(ctx context.Context, input *GetUnsubscribeInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	parsed, err := token.Parse([]token.Type{token.UnsubscribeToken}, input.Token, func(_ *token.Token) (string, error) {
		return server.Config.Server.JWTSecret, nil
	})
	if err != nil {
		log.Debug().Err(err).Msg("invalid unsubscribe token")
		writeUnsubscribePage(c.Writer, "Invalid or expired unsubscribe link.", false)
		return nil, nil
	}

	userIDStr := parsed.Get("user_id")
	repoIDStr := parsed.Get("repo_id")
	email := parsed.Get("email")
	repoName := parsed.Get("repo")

	userID, _ := strconv.ParseInt(userIDStr, 10, 64)
	repoID, _ := strconv.ParseInt(repoIDStr, 10, 64)

	if userID == 0 || repoID == 0 || email == "" {
		writeUnsubscribePage(c.Writer, "Invalid unsubscribe link.", false)
		return nil, nil
	}

	unsub := &model.EmailUnsubscribe{
		UserID:    userID,
		RepoID:    repoID,
		Email:     email,
		CreatedAt: time.Now().Unix(),
	}

	if err := _store.EmailUnsubscribeCreate(unsub); err != nil {
		log.Error().Err(err).Int64("user_id", userID).Int64("repo_id", repoID).Msg("failed to create unsubscribe record")
		writeUnsubscribePage(c.Writer, "Something went wrong. Please try again.", false)
		return nil, nil
	}

	msg := fmt.Sprintf("You have been unsubscribed from email notifications for <strong>%s</strong>.", repoName)
	writeUnsubscribePage(c.Writer, msg, true)
	return nil, nil
}

func writeUnsubscribePage(w http.ResponseWriter, message string, success bool) {
	icon := "&#10060;" // red X
	if success {
		icon = "&#9989;" // green check
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Crow CI — Email Notifications</title>
<style>body{font-family:system-ui,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#f5f5f5}
.card{background:#fff;border-radius:8px;padding:2rem 3rem;box-shadow:0 2px 8px rgba(0,0,0,.1);text-align:center;max-width:480px}
.icon{font-size:3rem;margin-bottom:1rem}</style>
</head><body><div class="card"><div class="icon">%s</div><p>%s</p></div></body></html>`, icon, message)
}
