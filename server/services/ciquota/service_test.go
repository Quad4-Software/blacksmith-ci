// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package ciquota

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReport(t *testing.T) {
	started := time.Unix(1_700_000_000, 0).UTC()
	usage := Usage{
		RunnerID:    "public-linux-1",
		Repository:  "octo/repo",
		ExternalRef: "crow:workflow:42",
		Started:     started,
		Stopped:     started.Add(90 * time.Second),
		Seconds:     90,
		Class:       "linux",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, usagePath, r.URL.Path)
		assert.Equal(t, "Bearer reporter-token", r.Header.Get("Authorization"))
		var got Usage
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, usage, got)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	err := New(Config{URL: server.URL, Token: "reporter-token"}).Report(context.Background(), usage)
	require.NoError(t, err)
}

func TestReportRejectsUnexpectedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	err := New(Config{URL: server.URL, Token: "reporter-token"}).Report(context.Background(), Usage{})
	require.ErrorContains(t, err, "401 Unauthorized")
}

func TestConfigIsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   bool
	}{
		{name: "complete", config: Config{URL: "https://forge.example", Token: "token"}, want: true},
		{name: "missing URL", config: Config{Token: "token"}, want: false},
		{name: "missing token", config: Config{URL: "https://forge.example"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.config.IsConfigured())
		})
	}
}

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		allowed   bool
		reason    string
		errorText string
	}{
		{name: "allowed", body: `{"allowed":true,"remaining_seconds":42}`, allowed: true},
		{name: "denied", body: `{"allowed":false,"reason":"macOS allowance exhausted"}`, reason: "macOS allowance exhausted"},
		{name: "unauthorized", status: 401, errorText: "401 Unauthorized"},
		{name: "unavailable", status: 503, errorText: "503 Service Unavailable"},
		{name: "missing decision", body: `{}`, errorText: "missing allowed"},
		{name: "null decision", body: `{"allowed":null}`, errorText: "missing allowed"},
		{name: "invalid response", body: `<html>`, errorText: "decode CI quota check"},
		{name: "trailing response", body: `{"allowed":true} {}`, errorText: "trailing data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/forge"+checkPath, r.URL.Path)
				assert.Equal(t, "org/repo with space", r.URL.Query().Get("repo"))
				assert.Equal(t, "macos", r.URL.Query().Get("class"))
				assert.Equal(t, "public mac & arm", r.URL.Query().Get("runner_id"))
				assert.Equal(t, "Bearer reporter-token", r.Header.Get("Authorization"))
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			result, err := New(Config{URL: srv.URL + "/forge/", Token: "reporter-token"}).Check(t.Context(), "org/repo with space", "macos", "public mac & arm")
			if tc.errorText != "" {
				require.ErrorContains(t, err, tc.errorText)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, result.Allowed)
			assert.Equal(t, tc.reason, result.Reason)
		})
	}
}

func TestCheckCanceled(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(nil)
	result, err := New(Config{URL: "http://127.0.0.1:1", Token: "token"}).Check(ctx, "org/repo", "linux", "public-linux-1")
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, result)
}

func TestReportAcceptsUnbilledAndDuplicateUsage(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			t.Cleanup(srv.Close)
			require.NoError(t, New(Config{URL: srv.URL, Token: "reporter-token"}).Report(t.Context(), Usage{RunnerID: "unlisted-agent"}))
		})
	}
}
