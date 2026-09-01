// Copyright 2022 Woodpecker Authors
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

package agent

import (
	"io"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline"
	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	pipelinelog "codefloe.com/crowci/crow/v6/pipeline/log"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
)

func (r *Runner) stepLogWriter(workflow *rpc.Workflow, step *backend.Step) *pipelinelog.LineWriter {
	if writer, ok := r.logWriters.Load(step.UUID); ok {
		if lineWriter, valid := writer.(*pipelinelog.LineWriter); valid {
			return lineWriter
		}
		r.logWriters.Delete(step.UUID)
	}

	var secrets []string
	for _, secret := range workflow.Config.Secrets {
		secrets = append(secrets, secret.Value)
	}
	writer := pipelinelog.NewLineWriterWithType(r.client, step.UUID, rpc.LogEntryStdout, secrets...)
	actual, loaded := r.logWriters.LoadOrStore(step.UUID, writer)
	if !loaded {
		return writer
	}
	if lineWriter, valid := actual.(*pipelinelog.LineWriter); valid {
		return lineWriter
	}
	r.logWriters.Store(step.UUID, writer)
	return writer
}

func (r *Runner) createProgressLogger(workflow *rpc.Workflow) pipeline.Progress {
	return func(step *backend.Step, message string) {
		writer := r.stepLogWriter(workflow, step)
		if _, err := writer.WriteProgress([]byte(message)); err != nil {
			log.Error().Err(err).Str("step_uuid", step.UUID).Msg("write preparation progress")
		}
	}
}

// createLogger returns the callback the pipeline runtime invokes to stream a
// step's logs. Like the tracer it ships everything synchronously, and the
// runtime joins the goroutines that call it before Run/RunAdopted return.
func (r *Runner) createLogger(_logger zerolog.Logger, workflow *rpc.Workflow) pipeline.Logger {
	return func(step *backend.Step, rc io.ReadCloser) error {
		defer rc.Close()

		logger := _logger.With().
			Str("image", step.Image).
			Logger()

		logger.Debug().Msg("log stream opened")

		logStream := r.stepLogWriter(workflow, step)
		defer r.logWriters.Delete(step.UUID)
		if err := pipelinelog.CopyLineByLine(logStream, rc, pipeline.MaxLogLineLength); err != nil {
			logger.Error().Err(err).Msg("copy limited logStream part")
		}

		logger.Debug().Msg("log stream copied, close ...")

		return nil
	}
}
