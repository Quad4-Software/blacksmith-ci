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

package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

// Service handles sending pipeline notifications.
type Service interface {
	// Dispatch evaluates notification config and sends email if appropriate.
	Dispatch(ctx context.Context, repo *model.Repo, pipeline *model.Pipeline, previousStatus model.StatusValue) error
	// TestSMTP sends a test email to the configured From address.
	TestSMTP(ctx context.Context) error
	// SMTPStatus returns the non-sensitive SMTP configuration for display.
	SMTPStatus() SMTPStatus
}

// SMTPStatus is the non-sensitive SMTP info returned to the admin UI.
type SMTPStatus struct {
	Configured bool   `json:"configured"`
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	From       string `json:"from,omitempty"`
}

type service struct {
	store      model.NotificationConfigStore
	smtpCfg    SMTPConfig
	serverHost string
}

// NewService creates a notification service.
func NewService(store model.NotificationConfigStore, smtpCfg SMTPConfig, serverHost string) Service {
	return &service{
		store:      store,
		smtpCfg:    smtpCfg,
		serverHost: serverHost,
	}
}

func (s *service) SMTPStatus() SMTPStatus {
	return SMTPStatus{
		Configured: s.smtpCfg.IsConfigured(),
		Host:       s.smtpCfg.Host,
		Port:       s.smtpCfg.Port,
		From:       s.smtpCfg.From,
	}
}

func (s *service) TestSMTP(_ context.Context) error {
	if !s.smtpCfg.IsConfigured() {
		return fmt.Errorf("SMTP is not configured")
	}
	return sendEmail(s.smtpCfg, []string{s.smtpCfg.From}, "Crow CI - SMTP Test", "This is a test email from Crow CI. Your SMTP configuration is working correctly.")
}

func (s *service) Dispatch(_ context.Context, repo *model.Repo, pipeline *model.Pipeline, previousStatus model.StatusValue) error {
	if !s.smtpCfg.IsConfigured() {
		return nil
	}

	config, err := s.store.NotificationConfigFind(repo.ID)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			return nil
		}
		return fmt.Errorf("loading notification config for repo %d: %w", repo.ID, err)
	}
	if !config.IsEnabled {
		return nil
	}

	events := config.Events
	if pipeline.NotifyOverride != nil {
		if pipeline.NotifyOverride.Disabled {
			return nil
		}
		if len(pipeline.NotifyOverride.Events) > 0 {
			events = pipeline.NotifyOverride.Events
		}
	}

	if len(events) == 0 {
		events = []model.NotificationEvent{model.NotificationEventFailure}
	}

	if !shouldNotify(events, pipeline.Status, previousStatus) {
		return nil
	}

	recipient := pipeline.Email
	if recipient == "" {
		log.Debug().Int64("pipeline_id", pipeline.ID).Msg("no commit author email, skipping notification")
		return nil
	}

	subject := fmt.Sprintf("[%s] %s #%d on %s",
		strings.ToUpper(string(pipeline.Status)),
		repo.FullName,
		pipeline.Number,
		pipeline.Branch,
	)

	body := buildEmailBody(s.serverHost, repo, pipeline)

	if err := sendEmail(s.smtpCfg, []string{recipient}, subject, body); err != nil {
		return fmt.Errorf("failed to send notification for %s#%d: %w", repo.FullName, pipeline.Number, err)
	}

	log.Info().
		Str("repo", repo.FullName).
		Int64("pipeline", pipeline.Number).
		Str("status", string(pipeline.Status)).
		Str("to", recipient).
		Msg("sent pipeline notification email")

	return nil
}

func buildEmailBody(serverHost string, repo *model.Repo, pipeline *model.Pipeline) string {
	link := fmt.Sprintf("%s/repos/%d/pipeline/%d", strings.TrimRight(serverHost, "/"), repo.ID, pipeline.Number)

	commit := pipeline.Commit
	if len(commit) > commitHashLen {
		commit = commit[:8]
	}

	message := pipeline.Message
	if idx := strings.IndexByte(message, '\n'); idx > 0 {
		message = message[:idx]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\n", repo.FullName)
	fmt.Fprintf(&b, "Pipeline:   #%d\n", pipeline.Number)
	fmt.Fprintf(&b, "Status:     %s\n", pipeline.Status)
	fmt.Fprintf(&b, "Branch:     %s\n", pipeline.Branch)
	fmt.Fprintf(&b, "Commit:     %s\n", commit)
	fmt.Fprintf(&b, "Message:    %s\n", message)
	fmt.Fprintf(&b, "\nView: %s\n", link)
	return b.String()
}

func shouldNotify(events []model.NotificationEvent, current, previous model.StatusValue) bool {
	for _, ev := range events {
		switch ev {
		case model.NotificationEventSuccess:
			if current == model.StatusSuccess {
				return true
			}
		case model.NotificationEventFailure:
			if current == model.StatusFailure || current == model.StatusError || current == model.StatusKilled {
				return true
			}
		case model.NotificationEventStatusChange:
			if previous != "" && normalizeTerminal(current) != normalizeTerminal(previous) {
				return true
			}
		}
	}
	return false
}

func normalizeTerminal(s model.StatusValue) string {
	switch s {
	case model.StatusSuccess:
		return "success"
	case model.StatusFailure, model.StatusError, model.StatusKilled:
		return "failure"
	default:
		return ""
	}
}
