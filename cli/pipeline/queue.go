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
	"text/template"

	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v5/cli/common"
	"codeberg.org/crowci/crow/v5/cli/internal"
	"codeberg.org/crowci/crow/v5/cli/output"
	crow "codeberg.org/crowci/crow/v5/crow-go/crow"
)

var pipelineQueueCmd = &cli.Command{
	Name:      "queue",
	Usage:     "show pipeline queue",
	ArgsUsage: " ",
	Action:    pipelineQueue,
	Flags:     common.OutputFlags("table"),
}

func pipelineQueue(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	pipelines, err := client.PipelineQueue()
	if err != nil {
		return err
	}

	if len(pipelines) == 0 {
		fmt.Println("there are no pending or running pipelines")
		return nil
	}

	return queueOutput(c, pipelines)
}

func queueOutput(c *cli.Command, feeds []*crow.Feed, fd ...io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))
	noHeader := c.Bool("output-no-headers")

	var out io.Writer = os.Stdout
	if len(fd) > 0 {
		out = fd[0]
	}

	switch outFmt {
	case "json":
		return output.WriteJSON(out, feeds)
	case "yaml":
		return output.WriteYAML(out, feeds)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}
		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		return tmpl.Execute(out, feeds)
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)
		cols := []string{"Number", "Status", "Event", "Branch", "Author", "Message"}
		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		for _, feed := range feeds {
			if err := table.Write(cols, feed); err != nil {
				return err
			}
		}
		table.Flush()
	}

	return nil
}
