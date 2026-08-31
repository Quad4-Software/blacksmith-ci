// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package ciquota reports workflow runtime to a Forgejo CI quota ledger.
package ciquota

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	usagePath         = "/api/v1/quota/ci/usage"
	httpClientTimeout = 10 * time.Second
)

// Reporter records completed CI workflow usage.
type Reporter interface {
	Report(context.Context, Usage) error
}

// Config configures a Forgejo CI quota reporter.
type Config struct {
	URL   string
	Token string
}

// IsConfigured reports whether all required reporter settings are present.
func (c Config) IsConfigured() bool {
	return strings.TrimSpace(c.URL) != "" && strings.TrimSpace(c.Token) != ""
}

// Usage is one completed workflow's billable runtime.
type Usage struct {
	Repository  string    `json:"repo"`
	ExternalRef string    `json:"external_ref"`
	Started     time.Time `json:"started"`
	Stopped     time.Time `json:"stopped"`
	Seconds     int64     `json:"seconds"`
	Class       string    `json:"class"`
}

type reporter struct {
	url    string
	token  string
	client *http.Client
}

// New returns a Forgejo CI quota reporter.
func New(config Config) Reporter {
	return &reporter{
		url:    strings.TrimRight(config.URL, "/") + usagePath,
		token:  config.Token,
		client: &http.Client{Timeout: httpClientTimeout},
	}
}

// Report sends usage to Forgejo. Reusing ExternalRef is safe because the
// Forgejo endpoint is idempotent.
func (r *reporter) Report(ctx context.Context, usage Usage) error {
	body, err := json.Marshal(usage)
	if err != nil {
		return fmt.Errorf("marshal CI quota usage: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create CI quota usage request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CrowCI")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("report CI quota usage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("report CI quota usage: Forgejo returned %s", resp.Status)
	}
	return nil
}
