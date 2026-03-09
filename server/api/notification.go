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
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/services/notification"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

// ── Repo notification config ────────────────────────────────────────────────.

type GetRepoNotificationConfigInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type GetRepoNotificationConfigOutput struct {
	Body *model.NotificationConfig
}

func GetRepoNotificationConfig(ctx context.Context, input *GetRepoNotificationConfigInput) (*GetRepoNotificationConfigOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	config, err := _store.NotificationConfigFind(repo.ID)
	if err != nil {
		return &GetRepoNotificationConfigOutput{Body: &model.NotificationConfig{
			RepoID:    repo.ID,
			IsEnabled: false,
			Events:    []model.NotificationEvent{model.NotificationEventFailure},
		}}, nil
	}

	return &GetRepoNotificationConfigOutput{Body: config}, nil
}

type PutRepoNotificationConfigInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Body   model.NotificationConfig
}

type PutRepoNotificationConfigOutput struct {
	Body *model.NotificationConfig
}

func PutRepoNotificationConfig(ctx context.Context, input *PutRepoNotificationConfigInput) (*PutRepoNotificationConfigOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	config := &input.Body
	config.RepoID = repo.ID

	if err := _store.NotificationConfigUpsert(config); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to save notification config: %s", err))
	}

	return &PutRepoNotificationConfigOutput{Body: config}, nil
}

type DeleteRepoNotificationConfigInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type DeleteRepoNotificationConfigOutput struct {
	Body struct{}
}

func DeleteRepoNotificationConfig(ctx context.Context, _ *DeleteRepoNotificationConfigInput) (*DeleteRepoNotificationConfigOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	if err := _store.NotificationConfigDelete(repo.ID); err != nil && !errors.Is(err, types.RecordNotExist) {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to delete notification config: %s", err))
	}

	return &DeleteRepoNotificationConfigOutput{}, nil
}

// ── Admin SMTP ──────────────────────────────────────────────────────────────.

type GetAdminSMTPStatusInput struct{}

type GetAdminSMTPStatusOutput struct {
	Body notification.SMTPStatus
}

func GetAdminSMTPStatus(ctx context.Context, _ *GetAdminSMTPStatusInput) (*GetAdminSMTPStatusOutput, error) {
	svc := server.Config.Services.NotificationService
	if svc == nil {
		return &GetAdminSMTPStatusOutput{Body: notification.SMTPStatus{Configured: false}}, nil
	}
	return &GetAdminSMTPStatusOutput{Body: svc.SMTPStatus()}, nil
}

type PostAdminTestSMTPInput struct{}

type PostAdminTestSMTPOutput struct {
	Body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
}

func PostAdminTestSMTP(ctx context.Context, _ *PostAdminTestSMTPInput) (*PostAdminTestSMTPOutput, error) {
	svc := server.Config.Services.NotificationService
	if svc == nil {
		return nil, huma.Error400BadRequest("SMTP is not configured. Set CROW_SMTP_HOST and CROW_SMTP_FROM environment variables.")
	}

	out := &PostAdminTestSMTPOutput{}
	if err := svc.TestSMTP(ctx); err != nil {
		out.Body.Success = false
		out.Body.Message = err.Error()
	} else {
		out.Body.Success = true
		out.Body.Message = "Test email sent successfully to the configured From address"
	}
	return out, nil
}
