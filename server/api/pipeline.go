// Copyright 2022 Woodpecker Authors
// Copyright 2021 Informatyka Boguslawski sp. z o.o. sp.k., http://www.ib.pl/
// Copyright 2018 Drone.IO Inc.
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
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	crow_jsonnet "codefloe.com/crowci/crow/v6/pipeline/frontend/jsonnet"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/metadata"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/constraint"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/forge"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pipeline"
	"codefloe.com/crowci/crow/v6/server/pipeline/stepbuilder"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// tagRefPrefix is the git ref namespace for tags. The pipeline metadata builder
// derives CI_COMMIT_TAG from a ref carrying this prefix.
const tagRefPrefix = "refs/tags/"

// pipelineErr converts a pipeline error to an appropriate huma error.
func pipelineErr(err error) error {
	switch {
	case errors.Is(err, &pipeline.ErrNotFound{}):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, &pipeline.ErrBadRequest{}):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, pipeline.ErrFiltered):
		// for debugging purpose we add a header
		return huma.ErrorWithHeaders(
			huma.NewError(http.StatusNoContent, err.Error()),
			http.Header{"Pipeline-Filtered": []string{"true"}},
		)
	default:
		return huma.Error500InternalServerError(err.Error())
	}
}

// forgeErr converts a forge error to an appropriate huma error.
func forgeErr(err error) error {
	return huma.Error500InternalServerError(err.Error())
}

type CreatePipelineBody struct {
	Branch    string            `json:"branch,omitempty" doc:"branch to run pipeline on; mutually exclusive with tag"`
	Tag       string            `json:"tag,omitempty" doc:"tag to run pipeline on; mutually exclusive with branch"`
	Variables map[string]string `json:"variables,omitempty" doc:"pipeline variables"`
	Workflows []string          `json:"workflows,omitempty" doc:"workflows to filter"`
}

// manualRef is the resolved target of a manual pipeline run: the commit to
// build and the branch and ref the pipeline records.
type manualRef struct {
	Commit *model.Commit
	// Branch is empty for a tag, matching what a tag webhook produces: a tag is
	// not on a branch, so `when: branch` must not match it.
	Branch string
	// Ref is the bare branch name for a branch, or refs/tags/<tag> for a tag.
	// The metadata builder keys CI_COMMIT_TAG off that prefix.
	Ref string
}

// resolveManualRef turns the branch or tag selected in the manual pipeline
// dialog into the commit and ref a pipeline should carry. Exactly one of branch
// and tag must be set.
//
// Both the pipeline-creation and the config-listing endpoints go through this
// so the two cannot drift on what a manual run against a tag looks like.
func resolveManualRef(
	c *gin.Context,
	_forge forge.Forge,
	user *model.User,
	repo *model.Repo,
	branch, tag string,
) (*manualRef, error) {
	switch {
	case branch == "" && tag == "":
		return nil, huma.Error400BadRequest("either branch or tag is required")
	case branch != "" && tag != "":
		return nil, huma.Error400BadRequest("branch and tag are mutually exclusive")
	}

	if tag != "" {
		tagForge, ok := _forge.(forge.TagForge)
		if !ok {
			return nil, huma.Error501NotImplemented("this forge does not support running pipelines on tags")
		}
		// Accept both `v1.0.0` and `refs/tags/v1.0.0` without double-prefixing.
		tag = strings.TrimPrefix(tag, tagRefPrefix)
		commit, err := tagForge.TagHead(c, user, repo, tag)
		if err != nil {
			return nil, forgeErr(err)
		}
		return &manualRef{Commit: commit, Ref: tagRefPrefix + tag}, nil
	}

	commit, err := _forge.BranchHead(c, user, repo, branch)
	if err != nil {
		return nil, forgeErr(err)
	}
	return &manualRef{Commit: commit, Branch: branch, Ref: branch}, nil
}

type CreatePipelineInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Body   CreatePipelineBody
}

type CreatePipelineOutput struct {
	Body *model.Pipeline
}

func CreatePipeline(ctx context.Context, input *CreatePipelineInput) (*CreatePipelineOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	if !repo.AllowManual {
		return nil, huma.Error403Forbidden("repo does not allow manual pipelines")
	}

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}

	user := session.User(c)

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	target, err := resolveManualRef(c, _forge, user, repo, input.Body.Branch, input.Body.Tag)
	if err != nil {
		return nil, err
	}
	lastCommit := target.Commit

	allowedVars, varErr := collectAllowedVariables(c, _forge, user, repo, target, input.Body.Workflows)
	if varErr != nil {
		return nil, forgeErr(varErr)
	}
	if vErr := validateManualVariables(input.Body.Variables, allowedVars); vErr != nil {
		return nil, huma.Error400BadRequest(vErr.Error())
	}

	pipelineOpts := &model.PipelineOptions{
		Branch:    target.Branch,
		Ref:       target.Ref,
		Variables: input.Body.Variables,
		Workflows: input.Body.Workflows,
	}
	tmpPipeline := createTmpPipeline(model.EventManual, lastCommit, user, pipelineOpts)

	createOpts := pipeline.CreateOptions{
		WorkflowFilter: pipelineOpts.Workflows,
	}
	pl, err := pipeline.Create(c, _store, repo, tmpPipeline, createOpts)
	if err != nil {
		return nil, pipelineErr(err)
	}
	return &CreatePipelineOutput{Body: pl}, nil
}

func createTmpPipeline(event model.WebhookEvent, commit *model.Commit, user *model.User, opts *model.PipelineOptions) *model.Pipeline {
	messageText := strings.TrimSpace(commit.Message)
	if messageText == "" {
		messageText = "Manual run"
	}
	if len(opts.Variables) > 0 {
		varsText := ""
		first := true
		for k, v := range opts.Variables {
			if !first {
				varsText += ", "
			}
			varsText += k + ":" + fmt.Sprint(v)
			first = false
		}
		messageText += ". Extra vars: " + varsText
	}

	return &model.Pipeline{
		Event:     event,
		Commit:    commit.SHA,
		Branch:    opts.Branch,
		Timestamp: time.Now().UTC().Unix(),

		Avatar:  user.Avatar,
		Message: messageText,

		Ref:                 cmp.Or(opts.Ref, opts.Branch),
		AdditionalVariables: opts.Variables,

		Author: user.Login,
		Email:  user.Email,

		ForgeURL: commit.ForgeURL,
	}
}

// manualTmpPipeline builds the unpersisted pipeline used to fetch and evaluate
// configs for a manual run, so config listing and pipeline creation agree on
// which commit and ref the workflows are resolved against.
func manualTmpPipeline(target *manualRef) *model.Pipeline {
	return &model.Pipeline{
		Event:  model.EventManual,
		Commit: target.Commit.SHA,
		Branch: target.Branch,
		Ref:    target.Ref,
	}
}

// collectAllowedVariables returns the union of manual variables across the
// selected workflows (all workflows if selectedWorkflows is empty), keyed by name.
func collectAllowedVariables(
	c *gin.Context,
	_forge forge.Forge,
	user *model.User,
	repo *model.Repo,
	target *manualRef,
	selectedWorkflows []string,
) (map[string]yaml.ManualVariable, error) {
	tmpPipeline := manualTmpPipeline(target)
	configService := server.Config.Services.Manager.ConfigServiceFromRepo(repo)
	forgeYamlConfigs, err := configService.Fetch(c, _forge, user, repo, tmpPipeline, nil, false)
	if err != nil {
		return nil, err
	}
	forgeYamlConfigs, err = preprocessJsonnetConfigs(forgeYamlConfigs, repo, tmpPipeline, _forge)
	if err != nil {
		return nil, err
	}

	selected := make(map[string]bool, len(selectedWorkflows))
	for _, name := range selectedWorkflows {
		selected[name] = true
	}

	allowed := map[string]yaml.ManualVariable{}
	for _, forgeConfig := range forgeYamlConfigs {
		workflowName := stepbuilder.SanitizePath(forgeConfig.Name)
		parsed, parseErr := yaml.ParseBytes(forgeConfig.Data)
		if parseErr == nil && parsed.Name != "" {
			workflowName = parsed.Name
		}
		explicitlySelected := selected[workflowName]
		if len(selected) > 0 && !explicitlySelected {
			continue
		}
		// A workflow the user explicitly selected must contribute its variables;
		// failing silently here would bypass required-variable enforcement.
		if parseErr != nil {
			if explicitlySelected {
				return nil, fmt.Errorf("failed to parse selected workflow %q: %w", forgeConfig.Name, parseErr)
			}
			log.Warn().Err(parseErr).Str("file", forgeConfig.Name).Msg("skipping unparseable workflow config for variable validation")
			continue
		}
		vars, varErr := yaml.ExtractManualVariables(forgeConfig.Data)
		if varErr != nil {
			if explicitlySelected {
				return nil, fmt.Errorf("failed to extract variables for selected workflow %q: %w", forgeConfig.Name, varErr)
			}
			log.Warn().Err(varErr).Str("file", forgeConfig.Name).Msg("skipping workflow with variable extraction error")
			continue
		}
		for _, v := range vars {
			allowed[v.Name] = v
		}
	}
	return allowed, nil
}

type GetPipelinesInput struct {
	PaginationParams
	RepoID   int64  `path:"repo_id" doc:"the repository id"`
	Before   string `query:"before" doc:"only return pipelines before this RFC3339 date"`
	After    string `query:"after" doc:"only return pipelines after this RFC3339 date"`
	Branch   string `query:"branch" doc:"filter pipelines by branch"`
	Event    string `query:"event" doc:"filter pipelines by webhook events (comma separated)"`
	Ref      string `query:"ref" doc:"filter pipelines by strings contained in ref"`
	Status   string `query:"status" doc:"filter pipelines by status"`
	Search   string `query:"search" doc:"search pipelines by number, message, branch, or author"`
	Workflow string `query:"workflow" doc:"filter pipelines by workflow names (comma separated)"`
}

type GetPipelinesOutput struct {
	Body []*model.Pipeline
}

func GetPipelines(ctx context.Context, input *GetPipelinesInput) (*GetPipelinesOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	filter := &model.PipelineFilter{
		Branch:      input.Branch,
		RefContains: input.Ref,
		Search:      input.Search,
	}

	if input.Workflow != "" {
		filter.WorkflowNames = strings.Split(input.Workflow, ",")
	}

	if input.Event != "" {
		eventList := strings.Split(input.Event, ",")
		wel := make(model.WebhookEventList, 0, len(eventList))
		for _, event := range eventList {
			we := model.WebhookEvent(event)
			if err := we.Validate(); err != nil {
				return nil, huma.Error400BadRequest(err.Error())
			}
			wel = append(wel, we)
		}
		filter.Events = wel
	}

	if input.Status != "" {
		ps := model.StatusValue(input.Status)
		if err := ps.Validate(); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		filter.Status = ps
	}

	if input.Before != "" {
		beforeDt, err := time.Parse(time.RFC3339, input.Before)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid 'before' date format", err)
		}
		filter.Before = beforeDt.Unix()
	}

	if input.After != "" {
		afterDt, err := time.Parse(time.RFC3339, input.After)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid 'after' date format", err)
		}
		filter.After = afterDt.Unix()
	}

	pagination := &model.ListOptions{Page: input.Page, PerPage: input.PerPage}
	pipelines, err := store.FromContext(c).GetPipelineList(repo, pagination, filter)
	if err != nil {
		return nil, huma.Error500InternalServerError("error getting pipeline list", err)
	}
	if pipelines == nil {
		pipelines = []*model.Pipeline{}
	}
	return &GetPipelinesOutput{Body: pipelines}, nil
}

type GetWorkflowNamesInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type GetWorkflowNamesOutput struct {
	Body []string
}

func GetWorkflowNames(ctx context.Context, input *GetWorkflowNamesInput) (*GetWorkflowNamesOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	names, err := store.FromContext(c).WorkflowNamesByRepo(repo.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("error getting workflow names", err)
	}
	if names == nil {
		names = []string{}
	}
	return &GetWorkflowNamesOutput{Body: names}, nil
}

type DeletePipelineInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

func DeletePipeline(ctx context.Context, input *DeletePipelineInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	if ok := pipelineDeleteAllowed(pl); !ok {
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Cannot delete pipeline with status %s", pl.Status))
	}

	err = store.FromContext(c).DeletePipeline(pl)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error deleting pipeline", err)
	}

	return nil, nil
}

type GetPipelineInput struct {
	RepoID int64  `path:"repo_id" doc:"the repository id"`
	Number string `path:"number" doc:"the number of the pipeline, OR 'latest'"`
}

type GetPipelineOutput struct {
	Body *model.Pipeline
}

type GetPipelineDiffInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the pipeline number"`
}

type GetPipelineDiffOutput struct {
	Body struct {
		Diff string `json:"diff"`
	}
}

// GetPipelineDiff returns the forge-provided unified diff for a pipeline.
func GetPipelineDiff(ctx context.Context, input *GetPipelineDiffInput) (*GetPipelineDiffOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	configuredForge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}
	diffForge, ok := configuredForge.(forge.DiffForge)
	if !ok {
		return nil, huma.Error501NotImplemented("pipeline diffs are not supported by this forge")
	}

	user, err := _store.GetUser(repo.UserID)
	if err != nil {
		return nil, dbErr(err)
	}
	if userForge, userForgeErr := _store.UserForgeGet(user.ID, repo.ForgeID); userForgeErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}
	if err := forge.RefreshUserForge(c, configuredForge, _store, user, repo.ForgeID); err != nil {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	diff, err := diffForge.Diff(c, user, repo, pl)
	if err != nil {
		return nil, huma.Error502BadGateway("could not load pipeline diff", err)
	}
	output := &GetPipelineDiffOutput{}
	output.Body.Diff = string(diff)
	return output, nil
}

func GetPipeline(ctx context.Context, input *GetPipelineInput) (*GetPipelineOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	if input.Number == "latest" {
		return getPipelineLast(ctx)
	}

	repo := session.Repo(c)
	num, err := parseNumber(input.Number)
	if err != nil {
		return nil, err
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		return nil, dbErr(err)
	}
	if pl.Workflows, err = _store.WorkflowGetTree(pl); err != nil {
		return nil, huma.Error500InternalServerError("error getting workflow tree", err)
	}

	populateAlwaysApproved(_store, repo, pl)
	pipeline.AnnotateAutoscalerProvisioning(_store, pl)

	return &GetPipelineOutput{Body: pl}, nil
}

// parseNumber parses a string into an int64, returning a huma 400 error on failure.
func parseNumber(s string) (int64, error) {
	if s == "" {
		return 0, huma.Error400BadRequest("invalid number: empty")
	}
	var num int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, huma.Error400BadRequest("invalid number: " + s)
		}
		num = num*10 + int64(ch-'0') //nolint:mnd
	}
	return num, nil
}

func getPipelineLast(ctx context.Context) (*GetPipelineOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	branch := c.DefaultQuery("branch", repo.Branch)

	pl, err := _store.GetPipelineLast(repo, branch)
	if err != nil {
		return nil, dbErr(err)
	}

	if pl.Workflows, err = _store.WorkflowGetTree(pl); err != nil {
		return nil, huma.Error500InternalServerError("error getting workflow tree", err)
	}

	populateAlwaysApproved(_store, repo, pl)
	pipeline.AnnotateAutoscalerProvisioning(_store, pl)

	return &GetPipelineOutput{Body: pl}, nil
}

func populateAlwaysApproved(_store store.Store, repo *model.Repo, pl *model.Pipeline) {
	if pl.Ref != "" {
		// Only a successful lookup of a matching record marks the ref as
		// always-approved; a missing record or datastore error must not.
		if apr, err := _store.ApprovedPullRequestFind(repo.ID, pl.Ref); err == nil && apr != nil {
			pl.IsAlwaysApproved = true
		}
	}
}

type GetStepLogsInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
	StepID int64 `path:"stepId" doc:"the step id"`
}

type GetStepLogsOutput struct {
	Body []*model.LogEntry
}

func GetStepLogs(ctx context.Context, input *GetStepLogsInput) (*GetStepLogsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	step, err := _store.StepLoad(input.StepID)
	if err != nil {
		return nil, dbErr(err)
	}

	if step.PipelineID != pl.ID {
		// make sure we cannot read arbitrary logs by id
		return nil, huma.Error400BadRequest(fmt.Sprintf("step with id %d is not part of repo %s", input.StepID, repo.FullName))
	}

	logs, err := server.Config.Services.LogStore.LogFind(step)
	if err != nil {
		return nil, dbErr(err)
	}

	return &GetStepLogsOutput{Body: logs}, nil
}

type DeleteStepLogsInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
	StepID int64 `path:"stepId" doc:"the step id"`
}

func DeleteStepLogs(ctx context.Context, input *DeleteStepLogsInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	_pipeline, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	_step, err := _store.StepLoad(input.StepID)
	if err != nil {
		return nil, dbErr(err)
	}

	if _step.PipelineID != _pipeline.ID {
		// make sure we cannot read arbitrary logs by id
		return nil, huma.Error400BadRequest(fmt.Sprintf("step with id %d is not part of repo %s", input.StepID, repo.FullName))
	}

	switch _step.State {
	case model.StatusRunning, model.StatusPreparing, model.StatusPending:
		return nil, huma.Error422UnprocessableEntity("Cannot delete logs for a pending, preparing, or running step")
	}

	err = server.Config.Services.LogStore.LogDelete(_step)
	if err != nil {
		return nil, dbErr(err)
	}

	return nil, nil
}

type GetPipelineConfigInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

type GetPipelineConfigOutput struct {
	Body []*model.Config
}

func GetPipelineConfig(ctx context.Context, input *GetPipelineConfigInput) (*GetPipelineConfigOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	configs, err := _store.ConfigsForPipeline(pl.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Get workflows to filter configs to only those that were actually executed
	workflows, err := _store.WorkflowGetTree(pl)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Collect unique pipeline names from executed workflows
	executedPipelineNames := make(map[string]struct{})
	for _, wf := range workflows {
		if wf.PipelineName != "" {
			executedPipelineNames[wf.PipelineName] = struct{}{}
		}
	}

	// Filter configs to only include those with executed workflows
	filteredConfigs := filterConfigsByExecutedWorkflows(configs, executedPipelineNames)

	return &GetPipelineConfigOutput{Body: groupConfigsBySource(filteredConfigs)}, nil
}

// filterConfigsByExecutedWorkflows returns only configs that have corresponding executed workflows.
//
// Jsonnet sources and libraries are always kept: neither is a workflow in its own
// right, so matching them against executed workflow names would only ever hide
// them, and the point of showing a generated document is being able to read it
// next to the source it came from.
func filterConfigsByExecutedWorkflows(configs []*model.Config, executedPipelineNames map[string]struct{}) []*model.Config {
	if len(executedPipelineNames) == 0 {
		return configs // Return all configs if no workflow info available
	}

	var filtered []*model.Config
	for _, config := range configs {
		if crow_jsonnet.IsJsonnetFile(config.Name) || crow_jsonnet.IsLibsonnetFile(config.Name) {
			filtered = append(filtered, config)
			continue
		}
		pipelineName := getConfigPipelineName(config)
		if _, exists := executedPipelineNames[pipelineName]; exists {
			filtered = append(filtered, config)
		}
	}

	return filtered
}

// groupConfigsBySource moves each generated config directly behind the config it
// was generated from, so the UI can render the list in order without needing to
// know how the two relate. Configs that are neither a source nor generated keep
// their relative order.
func groupConfigsBySource(configs []*model.Config) []*model.Config {
	generatedBySource := make(map[string][]*model.Config)
	for _, config := range configs {
		if config.IsGenerated() {
			generatedBySource[config.GeneratedFrom] = append(generatedBySource[config.GeneratedFrom], config)
		}
	}
	if len(generatedBySource) == 0 {
		return configs
	}

	grouped := make([]*model.Config, 0, len(configs))
	for _, config := range configs {
		if config.IsGenerated() {
			// appended after its source below
			continue
		}
		grouped = append(grouped, config)
		grouped = append(grouped, generatedBySource[config.Name]...)
		delete(generatedBySource, config.Name)
	}

	// Anything whose source is no longer around (filtered out, or the source row
	// was pruned) still belongs in the response rather than silently vanishing.
	for _, config := range configs {
		if _, orphaned := generatedBySource[config.GeneratedFrom]; config.IsGenerated() && orphaned {
			grouped = append(grouped, config)
		}
	}

	return grouped
}

// getConfigPipelineName determines the pipeline name for a config.
// This matches the logic in stepbuilder: YAML name field if present, otherwise sanitized file name.
func getConfigPipelineName(config *model.Config) string {
	parsed, err := yaml.ParseBytes(config.Data)
	if err == nil && parsed.Name != "" {
		return parsed.Name
	}
	return stepbuilder.SanitizePath(config.Name)
}

// ManualWorkflow represents a workflow that can be manually triggered.
type ManualWorkflow struct {
	Name         string                `json:"name"`
	PipelineName string                `json:"pipeline_name"`
	DependsOn    []string              `json:"depends_on,omitempty"`
	HasManual    bool                  `json:"has_manual"`
	Variables    []yaml.ManualVariable `json:"variables,omitempty"`
}

// ManualWorkflowsResponse is the response for the GetManualWorkflows endpoint.
type ManualWorkflowsResponse struct {
	Workflows []ManualWorkflow `json:"workflows"`
	Warnings  []string         `json:"warnings,omitempty"`
}

type GetRepoConfigsInput struct {
	RepoID int64  `path:"repo_id" doc:"the repository id"`
	Branch string `query:"branch" doc:"the branch to get configs from; mutually exclusive with tag"`
	Tag    string `query:"tag" doc:"the tag to get configs from; mutually exclusive with branch"`
	Filter string `query:"filter" doc:"filter workflows by event type (e.g., 'manual')"`
}

type GetRepoConfigsOutput struct {
	Body ManualWorkflowsResponse
}

func GetRepoConfigs(ctx context.Context, input *GetRepoConfigsInput) (*GetRepoConfigsOutput, error) {
	return getRepoConfigs(ctx, input, nil)
}

type repoConfigsProgressFunc func(stage string, current, total int)

func getRepoConfigs(ctx context.Context, input *GetRepoConfigsInput, progress repoConfigsProgressFunc) (*GetRepoConfigsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	// Resolve the selected branch or tag to the commit its configs come from
	target, err := resolveManualRef(c, _forge, user, repo, input.Branch, input.Tag)
	if err != nil {
		return nil, err
	}

	// Create a temporary pipeline to fetch configs (we won't persist it)
	tmpPipeline := manualTmpPipeline(target)

	// Fetch the configs from the forge
	reportRepoConfigsProgress(progress, "fetching", 0, 0)
	configService := server.Config.Services.Manager.ConfigServiceFromRepo(repo)
	forgeYamlConfigs, err := configService.Fetch(c, _forge, user, repo, tmpPipeline, nil, false)
	if err != nil {
		log.Debug().Err(err).Str("repo", repo.FullName).Msg("error fetching configs")
		return nil, huma.Error500InternalServerError("Error fetching pipeline configs: " + err.Error())
	}

	if len(forgeYamlConfigs) == 0 {
		return &GetRepoConfigsOutput{Body: ManualWorkflowsResponse{
			Workflows: []ManualWorkflow{},
		}}, nil
	}

	// Pre-process Jsonnet files: evaluate them to YAML before parsing
	reportRepoConfigsProgress(progress, "preprocessing", 0, len(forgeYamlConfigs))
	forgeYamlConfigs, err = preprocessJsonnetConfigs(forgeYamlConfigs, repo, tmpPipeline, _forge)
	if err != nil {
		log.Debug().Err(err).Msg("error preprocessing Jsonnet configs")
		return nil, huma.Error500InternalServerError("Error preprocessing Jsonnet configs: " + err.Error())
	}

	// Parse each YAML file and extract workflow information
	workflows := make([]ManualWorkflow, 0, len(forgeYamlConfigs))
	workflowMap := make(map[string]ManualWorkflow) // for dependency resolution
	var warnings []string

	reportRepoConfigsProgress(progress, "parsing", 0, len(forgeYamlConfigs))
	for index, forgeConfig := range forgeYamlConfigs {
		parsed, err := yaml.ParseBytes(forgeConfig.Data)
		if err != nil {
			log.Debug().Err(err).Str("file", forgeConfig.Name).Msg("error parsing yaml")
			reportRepoConfigsProgress(progress, "parsing", index+1, len(forgeYamlConfigs))
			continue
		}

		// Determine workflow name
		workflowName := parsed.Name
		if workflowName == "" {
			workflowName = stepbuilder.SanitizePath(forgeConfig.Name)
		}

		// Check if workflow has manual trigger
		hasManual := workflowHasManualTrigger(parsed.When)
		log.Debug().
			Str("workflow", workflowName).
			Bool("hasManual", hasManual).
			Int("constraintCount", len(parsed.When.Constraints)).
			Msg("checking workflow for manual trigger")

		manualVars, varErr := yaml.ExtractManualVariables(forgeConfig.Data)
		if varErr != nil {
			log.Debug().Err(varErr).Str("file", forgeConfig.Name).Msg("error extracting manual variables")
			manualVars = nil
		}

		wf := ManualWorkflow{
			Name:         workflowName,
			PipelineName: stepbuilder.SanitizePath(forgeConfig.Name),
			DependsOn:    parsed.DependsOn,
			HasManual:    hasManual,
			Variables:    manualVars,
		}

		workflowMap[workflowName] = wf
		reportRepoConfigsProgress(progress, "parsing", index+1, len(forgeYamlConfigs))

		// If filtering by manual, only include workflows with manual trigger
		if input.Filter == "manual" && !hasManual {
			continue
		}

		workflows = append(workflows, wf)
	}

	// Generate warnings for dependencies that don't have manual trigger
	if input.Filter == "manual" {
		for _, wf := range workflows {
			for _, dep := range wf.DependsOn {
				if depWf, exists := workflowMap[dep]; exists && !depWf.HasManual {
					warnings = append(warnings, fmt.Sprintf("Workflow '%s' depends on '%s' which does not have a manual trigger", wf.Name, dep))
				}
			}
		}
	}

	return &GetRepoConfigsOutput{Body: ManualWorkflowsResponse{
		Workflows: workflows,
		Warnings:  warnings,
	}}, nil
}

func reportRepoConfigsProgress(progress repoConfigsProgressFunc, stage string, current, total int) {
	if progress != nil {
		progress(stage, current, total)
	}
}

// workflowHasManualTrigger checks if a workflow's when constraint includes the manual event.
func workflowHasManualTrigger(when constraint.When) bool {
	// If no constraints, check default behavior
	if when.IsEmpty() {
		log.Debug().Msg("workflowHasManualTrigger: when is empty, returning false")
		return false // Default doesn't include manual
	}

	// Check each constraint for manual event
	for i, c := range when.Constraints {
		log.Debug().
			Int("constraintIndex", i).
			Strs("events", c.Event).
			Msg("workflowHasManualTrigger: checking constraint")

		// If event is not specified in constraint, it doesn't filter by event
		if len(c.Event) == 0 {
			// No event filter means all events match, including manual
			log.Debug().Msg("workflowHasManualTrigger: no event filter, returning true")
			return true
		}
		// Check if manual is in the event list
		for _, event := range c.Event {
			if event == metadata.EventManual {
				log.Debug().Msg("workflowHasManualTrigger: found manual event, returning true")
				return true
			}
		}
	}

	log.Debug().Msg("workflowHasManualTrigger: no manual trigger found, returning false")
	return false
}

// preprocessJsonnetConfigs evaluates any .jsonnet files in the config list,
// replacing them with the resulting YAML. This mirrors the preprocessing in
// stepBuilder.preprocessJsonnet but without full pipeline metadata (which is
// not needed for detecting manual triggers in when constraints).
func preprocessJsonnetConfigs(files []*forge_types.FileMeta, repo *model.Repo, pl *model.Pipeline, forge metadata.ServerForge) ([]*forge_types.FileMeta, error) {
	var result []*forge_types.FileMeta
	for _, f := range files {
		if crow_jsonnet.IsLibsonnetFile(f.Name) {
			continue
		}
		if !crow_jsonnet.IsJsonnetFile(f.Name) {
			result = append(result, f)
			continue
		}

		// Build sibling files for imports
		siblingFiles := make([]*crow_jsonnet.File, len(files))
		for i, sf := range files {
			siblingFiles[i] = &crow_jsonnet.File{Name: sf.Name, Data: sf.Data}
		}

		// Build extVars from metadata, matching the step builder's behavior
		workflowMetadata := stepbuilder.MetadataFromStruct(forge, repo, pl, nil, &model.Workflow{}, server.Config.Server.Host)
		extVars := workflowMetadata.Environ()

		evaluated, err := crow_jsonnet.Evaluate(f.Name, f.Data, siblingFiles, extVars)
		if err != nil {
			return nil, fmt.Errorf("jsonnet evaluation failed for %s: %w", f.Name, err)
		}

		if len(evaluated) == 1 {
			result = append(result, &forge_types.FileMeta{
				Name: f.Name,
				Data: evaluated[0],
			})
		} else {
			baseName := strings.TrimSuffix(f.Name, ".jsonnet")
			for i, doc := range evaluated {
				result = append(result, &forge_types.FileMeta{
					Name: fmt.Sprintf("%s-%d.yaml", baseName, i),
					Data: doc,
				})
			}
		}
	}
	return result, nil
}

type GetPipelineMetadataInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

type GetPipelineMetadataOutput struct {
	Body *metadata.Metadata
}

func GetPipelineMetadata(ctx context.Context, input *GetPipelineMetadataInput) (*GetPipelineMetadataOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	_store := store.FromContext(c)
	currentPipeline, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}

	prevPipeline, err := _store.GetPipelineLastBefore(repo, currentPipeline.Branch, currentPipeline.Number)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		return nil, dbErr(err)
	}

	md := stepbuilder.MetadataFromStruct(forge, repo, currentPipeline, prevPipeline, nil, server.Config.Server.Host)
	return &GetPipelineMetadataOutput{Body: &md}, nil
}

type CancelPipelineInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

func CancelPipeline(ctx context.Context, input *CancelPipelineInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	if err := pipeline.Cancel(c, _forge, _store, repo, user, pl); err != nil {
		return nil, pipelineErr(err)
	}
	return nil, nil
}

type PostApprovalInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

type PostApprovalOutput struct {
	Body *model.Pipeline
}

func PostApproval(ctx context.Context, input *PostApprovalInput) (*PostApprovalOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	newPipeline, err := pipeline.Approve(c, _store, pl, user, repo)
	if err != nil {
		return nil, pipelineErr(err)
	}
	return &PostApprovalOutput{Body: newPipeline}, nil
}

type PostApprovalAlwaysInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

type PostApprovalAlwaysOutput struct {
	Body *model.Pipeline
}

func PostApprovalAlways(ctx context.Context, input *PostApprovalAlwaysInput) (*PostApprovalAlwaysOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	// Create the always-approved record for this PR ref
	if pl.Ref != "" {
		apr := &model.ApprovedPullRequest{
			RepoID:     repo.ID,
			Ref:        pl.Ref,
			ApprovedBy: user.Login,
		}
		if err := _store.ApprovedPullRequestCreate(apr); err != nil {
			log.Warn().Err(err).Msgf("failed to create always-approved record for ref %s (may already exist)", pl.Ref)
		}
	}

	newPipeline, err := pipeline.Approve(c, _store, pl, user, repo)
	if err != nil {
		return nil, pipelineErr(err)
	}
	return &PostApprovalAlwaysOutput{Body: newPipeline}, nil
}

type DeleteApprovalAlwaysInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

func DeleteApprovalAlways(ctx context.Context, input *DeleteApprovalAlwaysInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	if err := _store.ApprovedPullRequestDelete(repo.ID, pl.Ref); err != nil {
		return nil, dbErr(err)
	}

	return nil, nil
}

type PostDeclineInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

type PostDeclineOutput struct {
	Body *model.Pipeline
}

func PostDecline(ctx context.Context, input *PostDeclineInput) (*PostDeclineOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	pl, err = pipeline.Decline(c, _store, pl, user, repo)
	if err != nil {
		return nil, pipelineErr(err)
	}
	return &PostDeclineOutput{Body: pl}, nil
}

type GetPipelineQueueInput struct{}

type GetPipelineQueueOutput struct {
	Body []*model.Feed
}

func GetPipelineQueue(ctx context.Context, _ *GetPipelineQueueInput) (*GetPipelineQueueOutput, error) {
	c := ginCtx(ctx)
	out, err := store.FromContext(c).GetPipelineQueue()
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting pipeline queue", err)
	}
	return &GetPipelineQueueOutput{Body: out}, nil
}

type PostPipelineInput struct {
	RepoID     int64  `path:"repo_id" doc:"the repository id"`
	Number     int64  `path:"number" doc:"the number of the pipeline"`
	Event      string `query:"event" doc:"override the event type"`
	DeployTo   string `query:"deploy_to" doc:"override the target deploy value"`
	DeployTask string `query:"deploy_task" doc:"override the deploy task"`
}

type PostPipelineOutput struct {
	Body *model.Pipeline
}

func PostPipeline(ctx context.Context, input *PostPipelineInput) (*PostPipelineOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	user, err := _store.GetUser(repo.UserID)
	if err != nil {
		return nil, dbErr(err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	// This ensures we use the correct OAuth tokens for the forge the repo belongs to.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	// Store the original event type before any modifications
	originalEvent := pl.Event
	log.Debug().
		Str("stored_original_event", string(originalEvent)).
		Str("current_event", string(pl.Event)).
		Int64("pipeline_number", pl.Number).
		Msg("Stored original event before modifications")

	// refresh the token to make sure, pipeline.Restart can still obtain the pipeline config if necessary again
	refreshUserTokenForForge(c, user, repo.ForgeID)

	// make Deploy task overridable
	if input.DeployTask != "" {
		pl.DeployTask = input.DeployTask
	}

	// make Event overridable to deploy
	// TODO: refactor to use own proper API for deploy
	if input.Event != "" {
		log.Debug().
			Str("query_event", input.Event).
			Str("original_event", string(originalEvent)).
			Msg("Event query parameter found")
		pl.Event = model.WebhookEvent(input.Event)
		if pl.Event != model.EventDeploy {
			return nil, huma.Error400BadRequest(model.ErrInvalidWebhookEvent.Error())
		}

		if !repo.AllowDeploy {
			return nil, huma.Error403Forbidden("repo does not allow deployments")
		}

		// Check team restriction for deployments
		if repo.DeployTeam != "" {
			triggeringUser := session.User(c)
			_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
			if err != nil {
				return nil, huma.Error500InternalServerError(err.Error())
			}
			teams, err := _forge.Teams(c, triggeringUser)
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to fetch user teams: %s", err))
			}
			isMember := false
			for _, t := range teams {
				if t.Login == repo.DeployTeam {
					isMember = true
					break
				}
			}
			if !isMember {
				return nil, huma.Error403Forbidden(fmt.Sprintf("user not in deploy team %q", repo.DeployTeam))
			}
		}

		if input.DeployTo != "" {
			pl.DeployTo = input.DeployTo
		}
	}

	// Read query string parameters into pipelineParams, exclude reserved params
	envs := map[string]string{}
	for key, val := range c.Request.URL.Query() {
		switch key {
		// Skip some options of the endpoint
		case "fork", "event", "deploy_to", "deploy_task":
			continue
		default:
			// We only accept string literals, because pipeline parameters will be
			// injected as environment variables. Reject CROW_-prefixed keys so
			// callers cannot override or forge reserved Crow environment variables.
			if strings.HasPrefix(strings.ToUpper(key), "CROW_") {
				return nil, huma.Error400BadRequest(fmt.Sprintf("invalid parameter %q: keys with the CROW_ prefix are reserved", key))
			}
			envs[key] = val[0]
		}
	}

	// Pass original event in envs to help determine if this is a deployment from a non-deployment
	log.Debug().
		Str("pl.Event", string(pl.Event)).
		Str("originalEvent", string(originalEvent)).
		Str("model.EventDeploy", string(model.EventDeploy)).
		Bool("pl.Event == model.EventDeploy", pl.Event == model.EventDeploy).
		Bool("originalEvent != model.EventDeploy", originalEvent != model.EventDeploy).
		Bool("condition", pl.Event == model.EventDeploy && originalEvent != model.EventDeploy).
		Msg("Checking deployment condition")

	if pl.Event == model.EventDeploy && originalEvent != model.EventDeploy {
		envs["CROW_ORIGINAL_EVENT"] = string(originalEvent)
		log.Debug().
			Str("original_event", string(originalEvent)).
			Str("new_event", string(pl.Event)).
			Int64("pipeline_number", pl.Number).
			Msg("Triggering deployment from non-deployment pipeline")
	} else {
		log.Debug().
			Str("original_event", string(originalEvent)).
			Str("new_event", string(pl.Event)).
			Int64("pipeline_number", pl.Number).
			Bool("is_deployment", pl.Event == model.EventDeploy).
			Bool("original_is_deployment", originalEvent == model.EventDeploy).
			Msg("Not marking as deployment from other event")
	}
	log.Debug().
		Interface("envs_passed_to_restart", envs).
		Str("pl.Event", string(pl.Event)).
		Bool("has_crow_original_event", envs["CROW_ORIGINAL_EVENT"] != "").
		Msg("About to call pipeline.Restart")
	newPipeline, err := pipeline.Restart(c, _store, pl, user, repo, envs)
	if err != nil {
		return nil, pipelineErr(err)
	}

	// Notify any active streaming sessions for the old pipeline that it has been restarted
	go notifyPipelineRestart(pl, newPipeline)
	return &PostPipelineOutput{Body: newPipeline}, nil
}

type DeletePipelineLogsInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Number int64 `path:"number" doc:"the number of the pipeline"`
}

func DeletePipelineLogs(ctx context.Context, input *DeletePipelineLogsInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	steps, err := _store.StepList(pl)
	if err != nil {
		return nil, huma.Error500InternalServerError("error listing steps", err)
	}

	if ok := pipelineDeleteAllowed(pl); !ok {
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Cannot delete logs for pipeline with status %s", pl.Status))
	}

	for _, step := range steps {
		if lErr := server.Config.Services.LogStore.LogDelete(step); lErr != nil {
			err = errors.Join(err, lErr)
		}
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("Error deleting pipeline logs", err)
	}

	return nil, nil
}

type CancelWorkflowInput struct {
	RepoID     int64 `path:"repo_id" doc:"the repository id"`
	Number     int64 `path:"number" doc:"the number of the pipeline"`
	WorkflowID int64 `path:"workflow_id" doc:"the id of the workflow to cancel"`
}

func CancelWorkflow(ctx context.Context, input *CancelWorkflowInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	if err := pipeline.CancelWorkflow(c, _forge, _store, repo, user, pl, input.WorkflowID); err != nil {
		return nil, pipelineErr(err)
	}
	return nil, nil
}

type RerunWorkflowInput struct {
	RepoID     int64 `path:"repo_id" doc:"the repository id"`
	Number     int64 `path:"number" doc:"the pipeline's number"`
	WorkflowID int64 `path:"workflow_id" doc:"the workflow's id"`
}

type RerunWorkflowOutput struct {
	Body *model.Pipeline
}

func RerunWorkflow(ctx context.Context, input *RerunWorkflowInput) (*RerunWorkflowOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	if err := pipeline.RerunWorkflow(c, _forge, _store, pl, input.WorkflowID, user, repo); err != nil {
		return nil, pipelineErr(err)
	}
	return &RerunWorkflowOutput{Body: pl}, nil
}

type RerunStepInput struct {
	RepoID     int64 `path:"repo_id" doc:"the repository id"`
	Number     int64 `path:"number" doc:"the pipeline's number"`
	WorkflowID int64 `path:"workflow_id" doc:"the workflow's id"`
	StepID     int64 `path:"step_id" doc:"the step's id"`
}

type RerunStepOutput struct {
	Body *model.Pipeline
}

func RerunStep(ctx context.Context, input *RerunStepInput) (*RerunStepOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("cannot get forge from repo", err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	pl, err := _store.GetPipelineNumber(repo, input.Number)
	if err != nil {
		return nil, dbErr(err)
	}

	if err := pipeline.RerunStep(c, _forge, _store, pl, input.WorkflowID, input.StepID, user, repo); err != nil {
		return nil, pipelineErr(err)
	}
	return &RerunStepOutput{Body: pl}, nil
}

// notifyPipelineRestart sends redirect signals to any active log streaming sessions
// for the old pipeline, directing them to the new restarted pipeline.
func notifyPipelineRestart(oldPipeline, newPipeline *model.Pipeline) {
	log.Info().Int64("oldPipelineID", oldPipeline.ID).Int64("newPipelineID", newPipeline.ID).Msg("notifying active sessions of pipeline restart")

	// Get workflows for the old pipeline to find step IDs
	oldWorkflows, err := server.Config.Services.Store.WorkflowGetTree(oldPipeline)
	if err != nil {
		log.Error().Err(err).Int64("oldPipelineID", oldPipeline.ID).Msg("failed to get old pipeline workflows for restart notification")
		return
	}

	// Get workflows for the new pipeline to find corresponding step IDs
	newWorkflows, err := server.Config.Services.Store.WorkflowGetTree(newPipeline)
	if err != nil {
		log.Error().Err(err).Int64("newPipelineID", newPipeline.ID).Msg("failed to get new pipeline workflows for restart notification")
		return
	}

	// Create a mapping from old step names to new step IDs
	newStepsByName := make(map[string]int64)
	for _, newWorkflow := range newWorkflows {
		for _, newStep := range newWorkflow.Children {
			newStepsByName[newStep.Name] = newStep.ID
		}
	}

	// Send redirect signals to old step streaming sessions
	for _, oldWorkflow := range oldWorkflows {
		for _, oldStep := range oldWorkflow.Children {
			if newStepID, exists := newStepsByName[oldStep.Name]; exists {
				// Send redirect signal from old step to new step
				redirectData := fmt.Sprintf(`{"redirect":true,"reason":"pipeline_restarted","oldPipelineNumber":%d,"newPipelineNumber":%d,"newStepID":%d}`,
					oldPipeline.Number, newPipeline.Number, newStepID)

				err := server.Config.Services.Logs.Write(context.Background(), oldStep.ID, []*model.LogEntry{
					{
						ID:     -1, // Special ID to indicate this is a redirect signal
						StepID: oldStep.ID,
						Time:   time.Now().Unix(),
						Line:   -1,
						Data:   []byte(redirectData),
						Type:   model.LogEntryMetadata,
					},
				})

				if err != nil {
					log.Error().Err(err).Int64("oldStepID", oldStep.ID).Int64("newStepID", newStepID).Msg("failed to send restart redirect signal")
				} else {
					log.Debug().Int64("oldStepID", oldStep.ID).Int64("newStepID", newStepID).Str("stepName", oldStep.Name).Msg("sent restart redirect signal")
				}
			}
		}
	}
}
