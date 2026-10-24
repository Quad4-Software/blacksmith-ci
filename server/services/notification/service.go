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
	"fmt"
	"html"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
)

// Service handles sending pipeline notifications.
type Service interface {
	// SendPipelineEmail sends one pipeline outcome email to a single recipient.
	// The statusLabel is shown in the subject, for example "fixed" or "failure".
	SendPipelineEmail(ctx context.Context, to string, repo *model.Repo, pipeline *model.Pipeline, statusLabel, unsubscribeURL string) error
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

type service struct {
	smtpCfg    SMTPConfig
	serverHost string
}

// NewService creates a notification service.
func NewService(smtpCfg SMTPConfig, serverHost string) Service {
	return &service{smtpCfg: smtpCfg, serverHost: serverHost}
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

func (s *service) SendPipelineEmail(_ context.Context, to string, repo *model.Repo, pipeline *model.Pipeline, statusLabel, unsubscribeURL string) error {
	if !s.smtpCfg.IsConfigured() {
		return nil
	}

	refLabel := pipeline.Branch
	if isPullEvent(pipeline.Event) {
		if pr := extractPRNumber(pipeline.Ref); pr != "" {
			refLabel = "PR #" + pr
		}
	}

	subject := fmt.Sprintf("[Crow CI] %s: build #%d on %s (%s)",
		repo.FullName,
		pipeline.Number,
		refLabel,
		statusLabel,
	)
	body := buildEmailBody(s.serverHost, repo, pipeline, refLabel, unsubscribeURL)

	if err := sendEmail(s.smtpCfg, []string{to}, subject, body); err != nil {
		return fmt.Errorf("failed to send notification for %s#%d: %w", repo.FullName, pipeline.Number, err)
	}

	log.Info().
		Str("repo", repo.FullName).
		Int64("pipeline", pipeline.Number).
		Str("status", string(pipeline.Status)).
		Msg("sent pipeline notification email")
	return nil
}

func isPullEvent(e model.WebhookEvent) bool {
	return e == model.EventPull || e == model.EventPullClosed || e == model.EventPullMerged || e == model.EventPullEdited
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
	if isPullEvent(pipeline.Event) {
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
		fmt.Fprintf(&b, `<p style="margin-top:2em;font-size:12px;color:#888"><a href="%s" style="color:#888">Unsubscribe</a> from notifications for this watch.</p>`, unsubscribeURL)
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
