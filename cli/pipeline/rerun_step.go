// Copyright 2026 Crow CI Authors
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
	"strconv"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/common"
	"codefloe.com/crowci/crow/v5/cli/internal"
	crow "codefloe.com/crowci/crow/v5/crow-go/crow"
)

var PipelineRerunStepCmd = &cli.Command{
	Name:      "rerun-step",
	Usage:     "rerun a specific step in a pipeline workflow",
	ArgsUsage: "<repo-id|repo-full-name> <pipeline> <workflow-id> <step-id>",
	Action:    pipelineRerunStep,
	Flags:     common.GlobalFlags,
}

const (
	stepIDArgIndex   = 3
	minRerunStepArgs = 4
)

func rerunStepWithClient(client crow.Client, repoIDOrFullName string, pipelineNum string, workflowID string, stepID string) error {
	repoID, err := internal.ParseRepo(client, repoIDOrFullName)
	if err != nil {
		return err
	}
	number, err := strconv.ParseInt(pipelineNum, 10, 64)
	if err != nil {
		return err
	}
	wfID, err := strconv.ParseInt(workflowID, 10, 64)
	if err != nil {
		return err
	}
	sID, err := strconv.ParseInt(stepID, 10, 64)
	if err != nil {
		return err
	}

	_, err = client.StepRerun(repoID, number, wfID, sID)
	if err != nil {
		return err
	}

	fmt.Printf("Rerunning step %d in workflow %d, pipeline %s#%d\n", sID, wfID, repoIDOrFullName, number)
	return nil
}

func pipelineRerunStep(ctx context.Context, c *cli.Command) (err error) {
	repoIDOrFullName := c.Args().First()
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	if c.Args().Len() < minRerunStepArgs {
		return fmt.Errorf("invalid syntax: expected <repo> <pipeline> <workflow-id> <step-id>")
	}
	return rerunStepWithClient(client, repoIDOrFullName, c.Args().Get(1), c.Args().Get(workflowIDArgIndex), c.Args().Get(stepIDArgIndex))
}
