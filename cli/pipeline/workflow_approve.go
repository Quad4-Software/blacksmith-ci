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

	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

const (
	minWorkflowApprovalArgs = 3
	workflowNameArgIndex    = 2
)

var workflowApproveCmd = &cli.Command{
	Name:      "approve",
	Usage:     "approve a workflow approval gate",
	ArgsUsage: "<repo-id|repo-full-name> <pipeline> <workflow-name>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "reason",
			Usage: "optional reason recorded in the audit trail",
		},
	},
	Action: workflowApproveAction,
}

func workflowApproveAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() < minWorkflowApprovalArgs {
		return fmt.Errorf("invalid syntax: expected <repo> <pipeline> <workflow-name>")
	}
	repoIDOrFullName := c.Args().First()
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	return workflowApproveWithClient(client, repoIDOrFullName, c.Args().Get(1), c.Args().Get(workflowNameArgIndex), c.String("reason"))
}

func workflowApproveWithClient(client crow.Client, repoIDOrFullName, pipelineNum, workflowName, reason string) error {
	repoID, number, wfPID, err := resolveWorkflow(client, repoIDOrFullName, pipelineNum, workflowName)
	if err != nil {
		return err
	}

	if _, err = client.WorkflowApprove(repoID, number, wfPID, reason); err != nil {
		return err
	}

	fmt.Printf("Approving workflow %s in pipeline %s#%d\n", workflowName, repoIDOrFullName, number)
	return nil
}

// resolveWorkflow translates a (repo, pipeline-number, workflow-name) tuple
// into the (repoID, pipelineNumber, workflow PID) tuple the API expects.
func resolveWorkflow(client crow.Client, repoIDOrFullName, pipelineNum, workflowName string) (int64, int64, int, error) {
	if workflowName == "" {
		return 0, 0, 0, fmt.Errorf("invalid syntax: workflow name is required")
	}
	repoID, err := internal.ParseRepo(client, repoIDOrFullName)
	if err != nil {
		return 0, 0, 0, err
	}
	number, err := strconv.ParseInt(pipelineNum, 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid syntax: pipeline number must be an integer: %w", err)
	}

	pl, err := client.Pipeline(repoID, number)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, wf := range pl.Workflows {
		if wf.Name == workflowName {
			return repoID, number, wf.PID, nil
		}
	}
	return 0, 0, 0, fmt.Errorf("workflow %q not found in pipeline %s#%d", workflowName, repoIDOrFullName, number)
}
