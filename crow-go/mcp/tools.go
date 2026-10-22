// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"codefloe.com/crowci/crow/v6/crow-go/crow"
)

const (
	defaultListLimit = 10
	maxListLimit     = 50
)

type tools struct {
	reg *Registry
	opt Options
}

type pipelineInput struct {
	TargetInput
}

type stepLogsInput struct {
	TargetInput
	Step    string `json:"step,omitempty" jsonschema:"step PID or name; defaults to the first failed step"`
	Tail    int    `json:"tail,omitempty" jsonschema:"number of trailing lines (default 200, max 500)"`
	Grep    string `json:"grep,omitempty" jsonschema:"regular expression; searches the whole log instead of the tail"`
	Context int    `json:"context,omitempty" jsonschema:"lines of context around each grep match"`
}

type waitInput struct {
	TargetInput
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"how long to wait (default 600, max 1800)"`
}

type listInput struct {
	TargetInput
	Branch string `json:"branch,omitempty" jsonschema:"only pipelines for this branch"`
	Status string `json:"status,omitempty" jsonschema:"only pipelines with this status, e.g. failure"`
	Limit  int    `json:"limit,omitempty" jsonschema:"number of pipelines (default 10, max 50)"`
}

type lintInput struct {
	Paths []string `json:"paths,omitempty" jsonschema:"files or directories to lint (default .crow/)"`
}

func ptr[T any](v T) *T { return &v }

func readOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)}
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func (t *tools) register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_pipeline",
		Description: "Show a pipeline's status with failed steps first. Without a target, shows the pipelines for the checkout's HEAD commit.",
		Annotations: readOnly(),
	}, t.getPipeline)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_step_logs",
		Description: "Show the tail of a step's log, or grep it. Defaults to the first failed step of the pipeline.",
		Annotations: readOnly(),
	}, t.getStepLogs)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_pipelines",
		Description: "List recent pipelines of a repository.",
		Annotations: readOnly(),
	}, t.listPipelines)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "wait_for_pipeline",
		Description: "Wait until a pipeline finishes and return its summary. Without a target, waits for the pipelines of the checkout's HEAD commit, including ones not created yet.",
		Annotations: readOnly(),
	}, t.waitForPipeline)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_instances",
		Description: "List the configured Crow instances.",
		Annotations: readOnly(),
	}, t.listInstances)

	if t.opt.Linter != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "lint_config",
			Description: "Lint local pipeline config files.",
			Annotations: readOnly(),
		}, t.lintConfig)
	}

	if t.opt.AllowWrite {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "restart_pipeline",
			Description: "Restart a pipeline. This creates a new pipeline; pass the returned url to wait_for_pipeline to follow it.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(false)},
		}, t.restartPipeline)
		mcp.AddTool(s, &mcp.Tool{
			Name:        "cancel_pipeline",
			Description: "Cancel a running pipeline.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(false)},
		}, t.cancelPipeline)
	}
}

func (t *tools) resolve(ctx context.Context, in TargetInput) (*Resolved, error) {
	return t.reg.Resolve(ctx, in, t.opt.Discoverer)
}

func noPipelines(res *Resolved) string {
	return fmt.Sprintf("no pipelines found for %s (%s)", res.Repo.FullName, res.Note)
}

func formatAll(res *Resolved) string {
	var parts []string
	if res.Note != "" {
		parts = append(parts, res.Note)
	}
	for _, p := range res.Pipelines {
		parts = append(parts, FormatPipeline(res, p))
	}
	return strings.Join(parts, "\n")
}

func (t *tools) getPipeline(ctx context.Context, _ *mcp.CallToolRequest, in pipelineInput) (*mcp.CallToolResult, any, error) {
	res, err := t.resolve(ctx, in.TargetInput)
	if err != nil {
		return nil, nil, err
	}
	if len(res.Pipelines) == 0 {
		return text(noPipelines(res)), nil, nil
	}
	return text(formatAll(res)), nil, nil
}

func (t *tools) getStepLogs(ctx context.Context, _ *mcp.CallToolRequest, in stepLogsInput) (*mcp.CallToolResult, any, error) {
	res, err := t.resolve(ctx, in.TargetInput)
	if err != nil {
		return nil, nil, err
	}
	if len(res.Pipelines) == 0 {
		return text(noPipelines(res)), nil, nil
	}
	p := res.Pipelines[0]

	selector := in.Step
	if res.StepPID > 0 {
		selector = fmt.Sprint(res.StepPID)
	}
	step, err := PickStep(p, selector)
	if err != nil {
		return nil, nil, err
	}

	entries, err := res.Client.StepLogEntries(res.Repo.ID, p.Number, step.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("get logs of step %q: %w", step.Name, err)
	}
	out, err := FormatLog(stepLabel(p, step), entries, LogOptions{Tail: in.Tail, Grep: in.Grep, Context: in.Context})
	if err != nil {
		return nil, nil, err
	}
	header := fmt.Sprintf("pipeline #%d step %s (%s) %s\n", p.Number, stepLabel(p, step), step.State, res.Link(p, step.PID))
	return text(header + out), nil, nil
}

func stepLabel(p *crow.Pipeline, step *crow.Step) string {
	for _, wf := range currentWorkflows(p) {
		for _, s := range wf.Children {
			if s == step {
				return clean(wf.Name) + "/" + clean(s.Name)
			}
		}
	}
	return clean(step.Name)
}

func (t *tools) listPipelines(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	res, err := t.reg.ResolveRepo(ctx, in.TargetInput, t.opt.Discoverer)
	if err != nil {
		return nil, nil, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	limit = min(limit, maxListLimit)

	list, err := res.Client.PipelineList(res.Repo.ID, crow.PipelineListOptions{
		ListOptions: crow.ListOptions{PerPage: limit},
		Branch:      in.Branch,
		Status:      in.Status,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list pipelines: %w", err)
	}
	if len(list) == 0 {
		return text("no pipelines"), nil, nil
	}
	rows := make([]string, 0, len(list))
	for _, p := range list[:min(len(list), limit)] {
		rows = append(rows, FormatPipelineRow(res, p))
	}
	return text(strings.Join(rows, "\n")), nil, nil
}

func isDone(status string) bool {
	switch status {
	case crow.StatusPending, crow.StatusRunning, "created":
		return false
	default:
		// Blocked pipelines wait for a human approval the agent cannot give,
		// so they are reported rather than waited on.
		return true
	}
}

func (t *tools) waitForPipeline(ctx context.Context, _ *mcp.CallToolRequest, in waitInput) (*mcp.CallToolResult, any, error) {
	wait := defaultWait
	if in.TimeoutSeconds > 0 {
		wait = time.Duration(in.TimeoutSeconds) * time.Second
	}
	wait = min(wait, t.opt.MaxWait)
	deadline := time.Now().Add(wait)

	res, err := t.resolve(ctx, in.TargetInput)
	if err != nil {
		return nil, nil, err
	}

	for {
		if res.FromCheckout && !res.MatchedHead {
			// The pipeline for HEAD may not have been created yet.
			if !sleepUntil(ctx, deadline, t.opt.PollInterval) {
				return text(fmt.Sprintf("timed out after %s; %s", wait, noPipelines(res))), nil, nil
			}
			if res, err = t.resolve(ctx, in.TargetInput); err != nil {
				return nil, nil, err
			}
			continue
		}

		done := true
		for _, p := range res.Pipelines {
			if !isDone(p.Status) {
				done = false
			}
		}
		if done {
			return text(formatAll(res)), nil, nil
		}

		if !sleepUntil(ctx, deadline, t.opt.PollInterval) {
			return text(fmt.Sprintf("timed out after %s; still running\n%s", wait, formatAll(res))), nil, nil
		}
		for i, p := range res.Pipelines {
			if isDone(p.Status) {
				continue
			}
			fresh, err := res.Client.Pipeline(res.Repo.ID, p.Number)
			if err != nil {
				return nil, nil, fmt.Errorf("get pipeline %d: %w", p.Number, err)
			}
			res.Pipelines[i] = fresh
		}
	}
}

// sleepUntil sleeps for interval, or until deadline or ctx ends. It reports
// whether there is time left to poll again.
func sleepUntil(ctx context.Context, deadline time.Time, interval time.Duration) bool {
	left := time.Until(deadline)
	if left <= 0 {
		return false
	}
	timer := time.NewTimer(min(interval, left))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return time.Now().Before(deadline)
	}
}

func (t *tools) listInstances(context.Context, *mcp.CallToolRequest, any) (*mcp.CallToolResult, any, error) {
	rows := make([]string, 0, len(t.reg.Instances()))
	for _, inst := range t.reg.Instances() {
		row := fmt.Sprintf("%s %s", inst.Name, inst.URL)
		if len(inst.ForgeHosts) > 0 {
			row += " forge hosts: " + strings.Join(inst.ForgeHosts, ", ")
		}
		rows = append(rows, row)
	}
	return text(strings.Join(rows, "\n")), nil, nil
}

func (t *tools) lintConfig(ctx context.Context, _ *mcp.CallToolRequest, in lintInput) (*mcp.CallToolResult, any, error) {
	paths := in.Paths
	if len(paths) == 0 {
		paths = []string{".crow/"}
	}
	report, err := t.opt.Linter(ctx, paths)
	if err != nil {
		if report == "" {
			return nil, nil, err
		}
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: report}}}, nil, nil
	}
	return text(report), nil, nil
}

func (t *tools) singlePipeline(ctx context.Context, in TargetInput) (*Resolved, *crow.Pipeline, error) {
	res, err := t.resolve(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	switch len(res.Pipelines) {
	case 0:
		return nil, nil, errors.New(noPipelines(res))
	case 1:
		return res, res.Pipelines[0], nil
	default:
		return nil, nil, fmt.Errorf("the target matches %d pipelines; pass a url or number to pick one", len(res.Pipelines))
	}
}

func (t *tools) restartPipeline(ctx context.Context, _ *mcp.CallToolRequest, in pipelineInput) (*mcp.CallToolResult, any, error) {
	res, p, err := t.singlePipeline(ctx, in.TargetInput)
	if err != nil {
		return nil, nil, err
	}
	fresh, err := res.Client.PipelineStart(res.Repo.ID, p.Number, crow.PipelineStartOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("restart pipeline %d: %w", p.Number, err)
	}
	link := res.Link(fresh, 0)
	return text(fmt.Sprintf("restarted #%d as #%d %s\nfollow it with wait_for_pipeline url=%s", p.Number, fresh.Number, link, link)), nil, nil
}

func (t *tools) cancelPipeline(ctx context.Context, _ *mcp.CallToolRequest, in pipelineInput) (*mcp.CallToolResult, any, error) {
	res, p, err := t.singlePipeline(ctx, in.TargetInput)
	if err != nil {
		return nil, nil, err
	}
	if err := res.Client.PipelineStop(res.Repo.ID, p.Number); err != nil {
		return nil, nil, fmt.Errorf("cancel pipeline %d: %w", p.Number, err)
	}
	return text(fmt.Sprintf("canceled #%d %s", p.Number, res.Link(p, 0))), nil, nil
}
