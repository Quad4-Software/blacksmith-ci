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
	"bufio"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/proclog"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
)

const (
	maxAutoscalerLogBatch = 1000
	// AutoscalerLogScanBuf is the maximum size of a single NDJSON line (1 MiB).
	autoscalerLogScanBuf = 1 << 20
)

type autoscalerLogLine struct {
	TS    int64  `json:"ts"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// PostAutoscalerLogs accepts a newline-delimited JSON body of log entries.
func PostAutoscalerLogs(r *proclog.Receiver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if r == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "proclog disabled"})
			return
		}

		as := session.Autoscaler(c)
		if as == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		entries := make([]*model.ProcLogEntry, 0, 128)
		scanner := bufio.NewScanner(c.Request.Body)
		scanner.Buffer(make([]byte, 0, autoscalerLogScanBuf), autoscalerLogScanBuf)
		for scanner.Scan() {
			if len(entries) >= maxAutoscalerLogBatch {
				break
			}
			raw := scanner.Bytes()
			if len(raw) == 0 {
				continue
			}
			var line autoscalerLogLine
			if err := json.Unmarshal(raw, &line); err != nil {
				log.Warn().Err(err).Msg("autoscaler sent malformed log line; skipping")
				continue
			}
			entries = append(entries, &model.ProcLogEntry{
				Time:  line.TS,
				Level: line.Level,
				Line:  line.Msg,
			})
		}
		if err := scanner.Err(); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := r.Ingest(c.Request.Context(), proclog.SourceTypeAutoscaler, as.ID, entries); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
