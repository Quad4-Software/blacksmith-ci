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
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	mail "github.com/wneessen/go-mail"
)

// SMTPConfig holds server-level SMTP configuration from env vars / CLI flags.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	AuthType string
}

// smtpAuthType resolves the configured auth type string to a go-mail SMTPAuthType.
// Supported values: LOGIN, PLAIN, CRAM-MD5, AUTO, NONE.
// Defaults to LOGIN if unrecognized or empty — LOGIN is the most universally
// compatible mechanism for username/password auth. AUTO can pick a stronger
// mechanism (e.g. SCRAM-SHA-256) that the server advertises but may not
// properly support with plain credentials.
func smtpAuthType(authType string) mail.SMTPAuthType {
	switch strings.ToUpper(strings.TrimSpace(authType)) {
	case "LOGIN":
		return mail.SMTPAuthLogin
	case "PLAIN":
		return mail.SMTPAuthPlain
	case "CRAM-MD5":
		return mail.SMTPAuthCramMD5
	case "AUTO":
		return mail.SMTPAuthAutoDiscover
	case "NONE":
		return mail.SMTPAuthNoAuth
	default:
		return mail.SMTPAuthLogin
	}
}

// IsConfigured returns true if the minimum SMTP settings are provided.
func (c SMTPConfig) IsConfigured() bool {
	return c.Host != "" && c.From != ""
}

const (
	smtpPortSSL       = 465
	smtpPortPlaintext = 25
	commitHashLen     = 8
)

// sendEmail sends an email using the configured SMTP settings.
func sendEmail(cfg SMTPConfig, to []string, subject, body string) error {
	msg := mail.NewMsg()
	if err := msg.From(cfg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := msg.To(to...); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, body)

	var opts []mail.Option

	switch cfg.Port {
	case smtpPortSSL:
		opts = append(opts, mail.WithSSLPort(false))
	case smtpPortPlaintext:
		opts = append(opts, mail.WithPort(cfg.Port), mail.WithTLSPolicy(mail.TLSOpportunistic))
	default:
		opts = append(opts, mail.WithPort(cfg.Port), mail.WithTLSPolicy(mail.TLSMandatory))
	}

	resolvedAuth := smtpAuthType(cfg.AuthType)
	if cfg.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(resolvedAuth),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
		)
	}

	log.Debug().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("from", cfg.From).
		Strs("to", to).
		Str("auth_type", string(resolvedAuth)).
		Str("subject", subject).
		Msg("sending SMTP email")

	if zerolog.GlobalLevel() <= zerolog.DebugLevel {
		opts = append(opts, mail.WithDebugLog())
	}

	client, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		log.Error().Err(err).Str("host", cfg.Host).Int("port", cfg.Port).Str("auth_type", string(resolvedAuth)).Msg("failed to create SMTP client")
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	if err := client.DialAndSend(msg); err != nil {
		log.Error().Err(err).Str("host", cfg.Host).Int("port", cfg.Port).Str("auth_type", string(resolvedAuth)).Msg("failed to send email")
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Info().Str("host", cfg.Host).Strs("to", to).Str("subject", subject).Msg("email sent successfully")
	return nil
}
