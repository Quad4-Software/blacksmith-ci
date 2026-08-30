// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type repoConfigsStreamEvent struct {
	Stage   string                   `json:"stage"`
	Current int                      `json:"current,omitempty"`
	Total   int                      `json:"total,omitempty"`
	Result  *ManualWorkflowsResponse `json:"result,omitempty"`
	Error   string                   `json:"error,omitempty"`
}

// StreamRepoConfigs reports workflow discovery progress and finishes with the
// same result returned by GetRepoConfigs.
func StreamRepoConfigs(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "Streaming not supported")
		return
	}

	input := &GetRepoConfigsInput{
		RepoID: c.GetInt64("repo_id"),
		Branch: c.Query("branch"),
		Tag:    c.Query("tag"),
		Filter: c.Query("filter"),
	}
	ctx := context.WithValue(c.Request.Context(), GinContextKey(), c)
	send := func(event repoConfigsStreamEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			log.Error().Err(err).Msg("could not marshal repo config progress")
			return
		}
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	result, err := getRepoConfigs(ctx, input, func(stage string, current, total int) {
		send(repoConfigsStreamEvent{Stage: stage, Current: current, Total: total})
	})
	if err != nil {
		send(repoConfigsStreamEvent{Stage: "error", Error: err.Error()})
		return
	}

	send(repoConfigsStreamEvent{Stage: "complete", Result: &result.Body})
}
