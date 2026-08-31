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
