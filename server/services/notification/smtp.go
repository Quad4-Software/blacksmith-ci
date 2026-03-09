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

	mail "github.com/wneessen/go-mail"
)

// SMTPConfig holds server-level SMTP configuration from env vars / CLI flags.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
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
	msg.SetBodyString(mail.TypeTextPlain, body)

	var opts []mail.Option

	switch cfg.Port {
	case smtpPortSSL:
		opts = append(opts, mail.WithSSLPort(false))
	case smtpPortPlaintext:
		opts = append(opts, mail.WithPort(cfg.Port), mail.WithTLSPolicy(mail.TLSOpportunistic))
	default:
		opts = append(opts, mail.WithPort(cfg.Port), mail.WithTLSPolicy(mail.TLSMandatory))
	}

	if cfg.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
		)
	}

	client, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
