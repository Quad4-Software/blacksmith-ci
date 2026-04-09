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

package pipeline

import (
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/output"
	"codefloe.com/crowci/crow/v5/cli/pipeline/deploy"
	"codefloe.com/crowci/crow/v5/cli/pipeline/log"
	crow "codefloe.com/crowci/crow/v5/crow-go/crow"
)

// Command exports the pipeline command set.
var Command = &cli.Command{
	Name:  "pipeline",
	Usage: "manage pipelines",
	Commands: []*cli.Command{
		pipelineApproveCmd,
		PipelineCancelWorkflowCmd,
		PipelineRerunStepCmd,
		pipelineCreateCmd,
		pipelineDeclineCmd,
		deploy.Command,
		pipelineKillCmd,
		pipelineLastCmd,
		buildPipelineListCmd(),
		log.Command,
		pipelinePsCmd,
		pipelinePurgeCmd,
		pipelineQueueCmd,
		pipelineShowCmd,
		pipelineStartCmd,
		pipelineStopCmd,
	},
}

func pipelineOutput(c *cli.Command, pipelines []*crow.Pipeline, fd ...io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))
	noHeader := c.Bool("output-no-headers")

	var out io.Writer
	out = os.Stdout
	if len(fd) > 0 {
		out = fd[0]
	}

	switch outFmt {
	case "json":
		return output.WriteJSON(out, pipelines)
	case "yaml":
		return output.WriteYAML(out, pipelines)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}

		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		if err := tmpl.Execute(out, pipelines); err != nil {
			return err
		}
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)
		cols := []string{"Number", "Status", "Event", "Branch", "Message", "Author"}

		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		for _, resource := range pipelines {
			if err := table.Write(cols, resource); err != nil {
				return err
			}
		}
		table.Flush()
	}

	return nil
}
