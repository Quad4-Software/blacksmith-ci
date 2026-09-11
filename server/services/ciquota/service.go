// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package ciquota checks CI allowances and reports workflow runtime to Forgejo.
package ciquota

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxCheckResponseBytes = 64 * 1024
	checkPath             = "/api/v1/quota/ci/check"
	usagePath             = "/api/v1/quota/ci/usage"
	httpClientTimeout     = 10 * time.Second
)

// Reporter checks whether work may start and records completed CI workflow usage.
type Reporter interface {
	Report(context.Context, Usage) error
	Check(context.Context, string, string, string) (*Decision, error)
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
	// RunnerID is the registered global agent name, matched against Forgejo configuration.
	RunnerID    string    `json:"runner_id"`
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
		url:    strings.TrimRight(config.URL, "/"),
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url+usagePath, bytes.NewReader(body))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("report CI quota usage: Forgejo returned %s", resp.Status)
	}
	return nil
}

// Decision is Forgejo's current allowance for one repository and runner class.
// Checking does not reserve time; completed workflows still need reporting.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// Check asks Forgejo whether a workflow may start. An invalid or unavailable
// response is an error, never an implicit permission to run.
func (r *reporter) Check(ctx context.Context, repository, class, runnerID string) (*Decision, error) {
	query := url.Values{"repo": {repository}, "class": {class}, "runner_id": {runnerID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url+checkPath+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create CI quota check request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CrowCI")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check CI quota: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check CI quota: Forgejo returned %s", resp.Status)
	}
	// A missing allowed field is a protocol error, not a quota denial.
	var result struct {
		Allowed *bool  `json:"allowed"`
		Reason  string `json:"reason"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxCheckResponseBytes))
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode CI quota check: %w", err)
	}
	if result.Allowed == nil {
		return nil, fmt.Errorf("CI quota check response is missing allowed")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("CI quota check response contains trailing data")
	}
	return &Decision{Allowed: *result.Allowed, Reason: result.Reason}, nil
}
