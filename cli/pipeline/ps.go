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
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/template"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/common"
	"codefloe.com/crowci/crow/v5/cli/internal"
	"codefloe.com/crowci/crow/v5/cli/output"
	crow "codefloe.com/crowci/crow/v5/crow-go/crow"
)

var pipelinePsCmd = &cli.Command{
	Name:      "ps",
	Usage:     "show pipeline steps",
	ArgsUsage: "<repo-id|repo-full-name> <pipeline>",
	Action:    pipelinePs,
	Flags:     common.OutputFlags("table"),
}

func pipelinePs(ctx context.Context, c *cli.Command) error {
	repoIDOrFullName := c.Args().First()
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	repoID, err := internal.ParseRepo(client, repoIDOrFullName)
	if err != nil {
		return fmt.Errorf("invalid repo '%s': %w", repoIDOrFullName, err)
	}

	pipelineArg := c.Args().Get(1)
	var number int64

	if pipelineArg == "last" || len(pipelineArg) == 0 {
		// Fetch the pipeline number from the last pipeline
		pipeline, err := client.PipelineLast(repoID, crow.PipelineLastOptions{})
		if err != nil {
			return err
		}

		number = pipeline.Number
	} else {
		number, err = strconv.ParseInt(pipelineArg, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid pipeline '%s': %w", pipelineArg, err)
		}
	}

	pipeline, err := client.Pipeline(repoID, number)
	if err != nil {
		return err
	}

	// Collect all steps with their workflow names
	var steps []*stepInfo
	for _, workflow := range pipeline.Workflows {
		for _, step := range workflow.Children {
			steps = append(steps, &stepInfo{Workflow: workflow.Name, Step: step})
		}
	}

	return stepsOutput(c, steps)
}

type stepInfo struct {
	Workflow string `json:"workflow"`
	*crow.Step
}

func stepsOutput(c *cli.Command, steps []*stepInfo, fd ...io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))
	noHeader := c.Bool("output-no-headers")

	var out io.Writer = os.Stdout
	if len(fd) > 0 {
		out = fd[0]
	}

	switch outFmt {
	case "json":
		return output.WriteJSON(out, steps)
	case "yaml":
		return output.WriteYAML(out, steps)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}
		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		return tmpl.Execute(out, steps)
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)
		cols := []string{"Workflow", "PID", "Name", "State", "Started", "Stopped"}
		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		for _, step := range steps {
			if err := table.Write(cols, step); err != nil {
				return err
			}
		}
		table.Flush()
	}

	return nil
}
