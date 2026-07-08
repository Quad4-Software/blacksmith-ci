// Copyright 2021 Woodpecker Authors
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

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/proclog"
)

// streamProcLogs is the shared SSE handler used by both agent and autoscaler endpoints.
// Writes entries as SSE messages: `id: <entry_id>\ndata: <json>\n\n`.
func streamProcLogs(c *gin.Context, s *proclog.Stream, sourceType string, sourceID int64) {
	if s == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "proclog disabled"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	rw := c.Writer
	flusher, ok := rw.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "Streaming not supported")
		return
	}

	if _, err := io.WriteString(rw, ": ping\n\n"); err != nil {
		return
	}
	flusher.Flush()

	ctx, cancel := context.WithCancelCause(c.Request.Context())
	defer cancel(nil)

	ch, err := s.Subscribe(ctx, sourceType, sourceID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			if _, err := io.WriteString(rw, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case entry, ok := <-ch:
			if !ok {
				return
			}
			buf, err := json.Marshal(entry)
			if err != nil {
				log.Warn().Err(err).Msg("proc-log SSE marshal failed")
				continue
			}
			if _, err := io.WriteString(rw, "id: "+strconv.FormatInt(entry.ID, 10)+"\ndata: "); err != nil {
				return
			}
			if _, err := rw.Write(buf); err != nil {
				return
			}
			if _, err := io.WriteString(rw, "\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// StreamAgentLogsSSE handles GET /api/v1/agents/:agent_id/logs/stream.
func StreamAgentLogsSSE(s *proclog.Stream) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("agent_id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid agent_id"})
			return
		}
		streamProcLogs(c, s, proclog.SourceTypeAgent, id)
	}
}

// StreamAutoscalerLogsSSE handles GET /api/v1/autoscalers/:autoscaler_id/logs/stream.
func StreamAutoscalerLogsSSE(s *proclog.Stream) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("autoscaler_id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid autoscaler_id"})
			return
		}
		streamProcLogs(c, s, proclog.SourceTypeAutoscaler, id)
	}
}
