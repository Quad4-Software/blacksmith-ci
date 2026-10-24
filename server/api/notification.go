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

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/services/notification"
)

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
