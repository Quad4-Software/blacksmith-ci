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
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store/types"
	"codefloe.com/crowci/crow/v5/shared/token"
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
	AuthType   string `json:"auth_type,omitempty"`
}

// UserEmailLookup resolves a forge login name to the user's configured email.
// Returns empty string if the user is not found.
type UserEmailLookup func(login string) string

// UserIDLookup resolves a forge login name to the user's internal ID.
// Returns 0 if the user is not found.
type UserIDLookup func(login string) int64

type service struct {
	store           model.NotificationConfigStore
	unsubStore      model.EmailUnsubscribeStore
	smtpCfg         SMTPConfig
	serverHost      string
	jwtSecret       string
	userEmailLookup UserEmailLookup
	userIDLookup    UserIDLookup
}

// NewService creates a notification service.
func NewService(store model.NotificationConfigStore, unsubStore model.EmailUnsubscribeStore, smtpCfg SMTPConfig, serverHost, jwtSecret string, userEmailLookup UserEmailLookup, userIDLookup UserIDLookup) Service {
	return &service{
		store:           store,
		unsubStore:      unsubStore,
		smtpCfg:         smtpCfg,
		serverHost:      serverHost,
		jwtSecret:       jwtSecret,
		userEmailLookup: userEmailLookup,
		userIDLookup:    userIDLookup,
	}
}

func (s *service) SMTPStatus() SMTPStatus {
	authType := s.smtpCfg.AuthType
	if authType == "" {
		authType = "LOGIN"
	}
	return SMTPStatus{
		Configured: s.smtpCfg.IsConfigured(),
		Host:       s.smtpCfg.Host,
		Port:       s.smtpCfg.Port,
		From:       s.smtpCfg.From,
		AuthType:   authType,
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
	if strings.Contains(recipient, "noreply") && s.userEmailLookup != nil && pipeline.Author != "" {
		if userEmail := s.userEmailLookup(pipeline.Author); userEmail != "" {
			log.Debug().Str("commit_email", recipient).Str("user_email", userEmail).Msg("commit email is noreply, using user account email")
			recipient = userEmail
		}
	}
	if recipient == "" || strings.Contains(recipient, "noreply") {
		log.Debug().Str("email", recipient).Int64("pipeline_id", pipeline.ID).Msg("no deliverable email for commit author, skipping notification")
		return nil
	}

	// Check if user has unsubscribed from this repo's notifications
	var userID int64
	if s.userIDLookup != nil && pipeline.Author != "" {
		userID = s.userIDLookup(pipeline.Author)
	}
	if userID > 0 && s.unsubStore != nil {
		if _, err := s.unsubStore.EmailUnsubscribeFind(userID, repo.ID); err == nil {
			log.Debug().Int64("user_id", userID).Int64("repo_id", repo.ID).Msg("user has unsubscribed from notifications, skipping")
			return nil
		}
	}

	refLabel := pipeline.Branch
	if pipeline.Event == model.EventPull || pipeline.Event == model.EventPullClosed || pipeline.Event == model.EventPullMerged || pipeline.Event == model.EventPullEdited {
		if pr := extractPRNumber(pipeline.Ref); pr != "" {
			refLabel = "PR #" + pr
		}
	}

	subject := fmt.Sprintf("[Crow CI] %s: build #%d on %s (%s)",
		repo.FullName,
		pipeline.Number,
		refLabel,
		statusChangeLabel(pipeline.Status, previousStatus),
	)

	unsubscribeURL := s.buildUnsubscribeURL(userID, repo, recipient)
	body := buildEmailBody(s.serverHost, repo, pipeline, refLabel, unsubscribeURL)

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

func (s *service) buildUnsubscribeURL(userID int64, repo *model.Repo, email string) string {
	if userID == 0 || s.jwtSecret == "" {
		return ""
	}

	t := token.New(token.UnsubscribeToken)
	t.Set("user_id", strconv.FormatInt(userID, 10))
	t.Set("repo_id", strconv.FormatInt(repo.ID, 10))
	t.Set("email", email)
	t.Set("repo", repo.FullName)

	signed, err := t.Sign(s.jwtSecret)
	if err != nil {
		log.Error().Err(err).Msg("failed to sign unsubscribe token")
		return ""
	}

	return fmt.Sprintf("%s/api/v1/notifications/unsubscribe?token=%s",
		strings.TrimRight(s.serverHost, "/"),
		url.QueryEscape(signed),
	)
}

func buildEmailBody(serverHost string, repo *model.Repo, pipeline *model.Pipeline, refLabel, unsubscribeURL string) string {
	link := fmt.Sprintf("%s/repos/%d/pipeline/%d", strings.TrimRight(serverHost, "/"), repo.ID, pipeline.Number)

	commit := pipeline.Commit
	if len(commit) > commitHashLen {
		commit = commit[:8]
	}

	message := pipeline.Message
	if idx := strings.IndexByte(message, '\n'); idx > 0 {
		message = message[:idx]
	}

	// Use "Pull Request" label for PR events, "Branch" otherwise
	refRowLabel := "Branch"
	if pipeline.Event == model.EventPull || pipeline.Event == model.EventPullClosed || pipeline.Event == model.EventPullMerged || pipeline.Event == model.EventPullEdited {
		refRowLabel = "Pull Request"
	}

	rows := []struct{ Label, Value string }{
		{"Repository", repo.FullName},
		{"Pipeline", fmt.Sprintf("#%d", pipeline.Number)},
		{"Status", string(pipeline.Status)},
		{refRowLabel, refLabel},
		{"Commit", commit},
		{"Message", message},
	}

	var b strings.Builder
	b.WriteString(`<table style="border-collapse:collapse;font-family:sans-serif">`)
	for _, r := range rows {
		// r.Label values are constant, hard-coded strings; r.Value carries
		// user-controlled data (repo name, ref label, commit message) and must
		// be HTML-escaped to prevent markup/script injection into the email.
		fmt.Fprintf(&b,
			`<tr><td style="padding:4px 12px 4px 0;font-weight:bold;white-space:nowrap;vertical-align:top">%s</td>`+
				`<td style="padding:4px 0">%s</td></tr>`,
			r.Label, html.EscapeString(r.Value),
		)
	}
	b.WriteString(`</table>`)
	fmt.Fprintf(&b, `<p><a href="%s">View pipeline</a></p>`, link)

	if unsubscribeURL != "" {
		fmt.Fprintf(&b, `<p style="margin-top:2em;font-size:12px;color:#888"><a href="%s" style="color:#888">Unsubscribe</a> from notifications for this repository.</p>`, unsubscribeURL)
	}

	return b.String()
}

// extractPRNumber extracts the PR number from a git ref like "refs/pull/123/head".
func extractPRNumber(ref string) string {
	// Forgejo/Gitea/GitHub: refs/pull/<number>/head
	// GitLab: refs/merge-requests/<number>/head
	for _, prefix := range []string{"refs/pull/", "refs/merge-requests/"} {
		if strings.HasPrefix(ref, prefix) {
			rest := ref[len(prefix):]
			if idx := strings.IndexByte(rest, '/'); idx > 0 {
				return rest[:idx]
			}
		}
	}
	return ""
}

// statusChangeLabel returns a human-readable label for the status transition.
func statusChangeLabel(current, previous model.StatusValue) string {
	if previous != "" {
		curNorm := normalizeTerminal(current)
		prevNorm := normalizeTerminal(previous)
		if curNorm != prevNorm && curNorm != "" && prevNorm != "" {
			if curNorm == "success" {
				return "fixed"
			}
			return "broken"
		}
	}
	return string(current)
}

func shouldNotify(events []model.NotificationEvent, current, previous model.StatusValue) bool {
	// Only notify on genuine build outcomes. Builds that were canceled,
	// superseded by a newer build (StatusKilled), declined, or skipped did not
	// fail because of the code under test and must never trigger a notification.
	if !isNotifiableStatus(current) {
		return false
	}
	for _, ev := range events {
		switch ev {
		case model.NotificationEventSuccess:
			if current == model.StatusSuccess {
				return true
			}
		case model.NotificationEventFailure:
			if current == model.StatusFailure || current == model.StatusError {
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

// isNotifiableStatus reports whether a terminal pipeline status represents a
// genuine build outcome worth emailing about. Canceled/superseded (killed),
// declined, skipped, blocked and non-terminal states are intentionally excluded.
func isNotifiableStatus(s model.StatusValue) bool {
	switch s {
	case model.StatusSuccess, model.StatusFailure, model.StatusError:
		return true
	default:
		return false
	}
}

func normalizeTerminal(s model.StatusValue) string {
	switch s {
	case model.StatusSuccess:
		return "success"
	case model.StatusFailure, model.StatusError:
		return "failure"
	default:
		return ""
	}
}
