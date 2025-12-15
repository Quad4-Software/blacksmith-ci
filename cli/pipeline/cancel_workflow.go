// Copyright 2025 Woodpecker Authors
// Copyright 2025 Crow CI Authors
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

	"codeberg.org/crowci/crow/v5/cli/common"
	"codeberg.org/crowci/crow/v5/cli/internal"
	crow "codeberg.org/crowci/crow/v5/crow-go/crow"
)

var PipelineCancelWorkflowCmd = &cli.Command{
	Name:      "cancel-workflow",
	Usage:     "cancel a specific workflow in a pipeline",
	ArgsUsage: "<repo-id|repo-full-name> <pipeline> <workflow-id>",
	Action:    pipelineCancelWorkflow,
	Flags:     common.GlobalFlags,
}

const (
	workflowIDArgIndex    = 2
	minCancelWorkflowArgs = 3
)

func cancelWorkflowWithClient(client crow.Client, repoIDOrFullName string, pipelineNum string, workflowID string) error {
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

	err = client.WorkflowCancel(repoID, number, wfID)
	if err != nil {
		return err
	}

	fmt.Printf("Canceling workflow %d in pipeline %s#%d\n", wfID, repoIDOrFullName, number)
	return nil
}

func pipelineCancelWorkflow(ctx context.Context, c *cli.Command) (err error) {
	repoIDOrFullName := c.Args().First()
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	if c.Args().Len() < minCancelWorkflowArgs {
		return fmt.Errorf("invalid syntax: expected <repo> <pipeline> <workflow-id>")
	}
	return cancelWorkflowWithClient(client, repoIDOrFullName, c.Args().Get(1), c.Args().Get(workflowIDArgIndex))
}
