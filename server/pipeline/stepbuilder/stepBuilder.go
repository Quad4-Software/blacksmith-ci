// Copyright 2025 Crow Authors
// Copyright 2022 Woodpecker Authors
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

package stepbuilder

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/pipeline"
	backend_types "codefloe.com/crowci/crow/v5/pipeline/backend/types"
	pipeline_errors "codefloe.com/crowci/crow/v5/pipeline/errors"
	errorTypes "codefloe.com/crowci/crow/v5/pipeline/errors/types"
	crow_jsonnet "codefloe.com/crowci/crow/v5/pipeline/frontend/jsonnet"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/metadata"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/compiler"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/compiler/settings"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/constraint"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/linter"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/matrix"
	yaml_types "codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/types"
	"codefloe.com/crowci/crow/v5/server"
	forge_types "codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
)

// StepBuilder Takes the hook data and the yaml and returns in internal data model.
type StepBuilder struct {
	Repo                  *model.Repo
	Curr                  *model.Pipeline
	Prev                  *model.Pipeline
	Netrc                 *model.Netrc
	Secs                  []*model.Secret
	Regs                  []*model.Registry
	Host                  string
	Yamls                 []*forge_types.FileMeta
	Envs                  map[string]string
	Forge                 metadata.ServerForge
	DefaultLabels         map[string]string
	ProxyOpts             compiler.ProxyOptions
	ExternalSecretFetcher settings.ExternalSecretBatchFetcher
	WorkflowFilter        []string // If set, only build workflows with these names
}

type Item struct {
	Workflow     *model.Workflow
	Labels       map[string]string
	DependsOn    []string
	RunsOn       []string
	Config       *backend_types.Config
	OriginalName string                   // Store original filename for dependency resolution
	Notify       *yaml_types.NotifyConfig // Per-workflow notification override from YAML
}

func (b *StepBuilder) Build() (items []*Item, errorsAndWarnings error) {
	b.Yamls = forge_types.SortByName(b.Yamls)

	// Pre-process Jsonnet files: evaluate them and replace with YAML output.
	// .libsonnet files are kept only as import sources and removed from the build list.
	b.Yamls, errorsAndWarnings = b.preprocessJsonnet(b.Yamls)
	if errorsAndWarnings != nil && pipeline_errors.HasBlockingErrors(errorsAndWarnings) {
		return nil, errorsAndWarnings
	}

	// If a workflow filter is set, expand it to include dependencies
	if len(b.WorkflowFilter) > 0 {
		b.WorkflowFilter = b.expandFilterWithDependencies()
	}

	pidSequence := 1

	for _, y := range b.Yamls {
		// matrix axes
		axes, err := matrix.ParseStringWithLimit(string(y.Data), server.Config.Pipeline.MaxMatrixSize)
		if err != nil {
			return nil, err
		}
		if len(axes) == 0 {
			axes = append(axes, matrix.Axis{})
		}

		for i, axis := range axes {
			// Prepare environment for variable substitution
			env := make(map[string]string)
			for k, v := range axis {
				env[k] = v
			}

			// 1. Check for matrix NAME (case-insensitive)
			matrixName, hasMatrixName := getCaseInsensitive(axis, "NAME")
			if hasMatrixName {
				delete(axis, matrixName) // Remove NAME from axis/env
			}

			// 2. Parse YAML for top-level name (after env subst)
			parsedYAML, _ := yaml.ParseString(string(y.Data))
			yamlName := ""
			if parsedYAML != nil && parsedYAML.Name != "" {
				yamlName = parsedYAML.Name
			}

			// 3. Variable substitution for names
			var workflowName string
			switch {
			case hasMatrixName && matrixName != "":
				if subst, _ := metadata.EnvVarSubst(matrixName, env); subst != "" {
					workflowName = subst
				} else {
					workflowName = matrixName
				}
			case yamlName != "":
				if subst, _ := metadata.EnvVarSubst(yamlName, env); subst != "" {
					workflowName = subst
				} else {
					workflowName = yamlName
				}
			default:
				workflowName = SanitizePath(y.Name)
			}

			// Determine pipeline name (YAML name field or file name)
			var pipelineName string
			if yamlName != "" {
				// Use the YAML name field if present
				if subst, _ := metadata.EnvVarSubst(yamlName, env); subst != "" {
					pipelineName = subst
				} else {
					pipelineName = yamlName
				}
			} else {
				// Fallback to file name
				pipelineName = SanitizePath(y.Name)
			}

			// If workflow filter is set, skip workflows not in the filter list
			if len(b.WorkflowFilter) > 0 && !b.isWorkflowInFilter(workflowName, pipelineName) {
				log.Debug().Str("workflow", workflowName).Msg("skipping workflow not in filter list")
				continue
			}

			workflow := &model.Workflow{
				PID:          pidSequence,
				State:        model.StatusPending,
				Environ:      model.JSONMapStringString(axis),
				Name:         workflowName,
				PipelineName: pipelineName,
			}
			if len(axes) > 1 {
				workflow.AxisID = i + 1
			}
			item, err := b.genItemForWorkflow(workflow, axis, string(y.Data), y.Name)
			if err != nil && pipeline_errors.HasBlockingErrors(err) {
				return nil, err
			} else if err != nil {
				errorsAndWarnings = errors.Join(errorsAndWarnings, err)
			}

			if item == nil {
				continue
			}
			items = append(items, item)
			pidSequence++
		}

		// TODO: add summary workflow that send status back based on workflows generated by matrix function
		// depend on https://github.com/woodpecker-ci/woodpecker/issues/778
	}

	items = filterItemsWithMissingDependencies(items)

	// check if at least one step can start if slice is not empty
	if len(items) > 0 && !stepListContainsItemsToRun(items) {
		return nil, fmt.Errorf("pipeline has no steps to run")
	}

	return items, errorsAndWarnings
}

// preprocessJsonnet evaluates .jsonnet files and expands them into YAML FileMeta entries.
// .libsonnet files are removed from the result (they are import-only).
func (b *StepBuilder) preprocessJsonnet(files []*forge_types.FileMeta) ([]*forge_types.FileMeta, error) {
	// Collect sibling files for Jsonnet imports (all files including .libsonnet)
	var result []*forge_types.FileMeta
	for _, f := range files {
		if crow_jsonnet.IsLibsonnetFile(f.Name) {
			// .libsonnet files are only used as imports, skip them from the build list
			continue
		}

		if !crow_jsonnet.IsJsonnetFile(f.Name) {
			result = append(result, f)
			continue
		}

		// Build environment variables for Jsonnet extVars
		workflowMetadata := MetadataFromStruct(b.Forge, b.Repo, b.Curr, b.Prev, &model.Workflow{}, b.Host)
		extVars := workflowMetadata.Environ()
		for k, v := range b.Envs {
			if _, exists := extVars[k]; !exists {
				extVars[k] = v
			}
		}

		// Convert FileMeta to jsonnet.File for the evaluation
		siblingFiles := make([]*crow_jsonnet.File, len(files))
		for i, sf := range files {
			siblingFiles[i] = &crow_jsonnet.File{Name: sf.Name, Data: sf.Data}
		}

		evaluated, err := crow_jsonnet.Evaluate(f.Name, f.Data, siblingFiles, extVars)
		if err != nil {
			return nil, &errorTypes.PipelineError{Message: err.Error(), Type: errorTypes.PipelineErrorTypeCompiler}
		}

		if len(evaluated) == 1 {
			// Single document: replace the file data in-place
			result = append(result, &forge_types.FileMeta{
				Name: f.Name,
				Data: evaluated[0],
			})
		} else {
			// Multi-document: expand into separate entries
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

func (b *StepBuilder) genItemForWorkflow(workflow *model.Workflow, axis matrix.Axis, data string, originalFileName string) (item *Item, errorsAndWarnings error) {
	workflowMetadata := MetadataFromStruct(b.Forge, b.Repo, b.Curr, b.Prev, workflow, b.Host)
	environ := b.environmentVariables(workflowMetadata, axis)

	// extract user-defined variables from the "variables:" block and add them
	// to the substitution environment (lower priority than CI/matrix vars)
	if userVars, err := yaml.ParseVariablesString(data); err == nil {
		for k, v := range userVars {
			if _, exists := environ[k]; !exists {
				environ[k] = v
			}
		}
	}

	// add global environment variables for substituting
	for k, v := range b.Envs {
		if _, exists := environ[k]; exists {
			// don't override existing values
			continue
		}
		environ[k] = v
	}

	// substitute vars
	substituted, err := metadata.EnvVarSubst(data, environ)
	if err != nil {
		return nil, errors.Join(errorsAndWarnings, err)
	}

	// parse yaml pipeline
	parsed, err := yaml.ParseString(substituted)
	if err != nil {
		return nil, &errorTypes.PipelineError{Message: err.Error(), Type: errorTypes.PipelineErrorTypeCompiler}
	}

	// lint pipeline
	errorsAndWarnings = errors.Join(errorsAndWarnings, linter.New(
		linter.WithTrusted(linter.TrustedConfiguration{
			Network:  b.Repo.Trusted.Network,
			Volumes:  b.Repo.Trusted.Volumes,
			Security: b.Repo.Trusted.Security,
		}),
		linter.PrivilegedPluginsWithMatchType(server.Config.Pipeline.PrivilegedPlugins, server.Config.Pipeline.PrivilegedPluginsMatchType),
		linter.WithTrustedClonePlugins(server.Config.Pipeline.TrustedClonePlugins),
		linter.WithApprovalTimeoutMax(server.Config.Pipeline.ApprovalGateTimeoutMax),
	).Lint([]*linter.WorkflowConfig{{
		Workflow:  parsed,
		File:      workflow.Name,
		RawConfig: data,
	}}))
	if pipeline_errors.HasBlockingErrors(errorsAndWarnings) {
		return nil, errorsAndWarnings
	}

	// checking if filtered.
	if match, err := parsed.When.Match(workflowMetadata, true, environ); !match && err == nil {
		log.Debug().Str("pipeline", workflow.Name).Msg(
			"marked as skipped, does not match metadata",
		)
		return nil, nil
	} else if err != nil {
		log.Debug().Str("pipeline", workflow.Name).Msg(
			"pipeline config could not be parsed",
		)
		return nil, errors.Join(errorsAndWarnings, err)
	}

	ir, err := b.toInternalRepresentation(parsed, environ, workflowMetadata, workflow.ID, workflow.Name)
	if err != nil {
		return nil, errors.Join(errorsAndWarnings, err)
	}

	if len(ir.Stages) == 0 {
		return nil, nil
	}

	// Copy dependencies to the workflow model for API response and database storage
	workflow.DependsOn = parsed.DependsOn

	item = &Item{
		Workflow:     workflow,
		Config:       ir,
		Labels:       parsed.Labels,
		DependsOn:    parsed.DependsOn,
		RunsOn:       parsed.RunsOn,
		OriginalName: SanitizePath(originalFileName), // Store original filename for dependency resolution
		Notify:       parsed.Notify,
	}
	if len(item.Labels) == 0 {
		item.Labels = make(map[string]string, len(b.DefaultLabels))
		// Set default labels if no labels are defined in the pipeline
		maps.Copy(item.Labels, b.DefaultLabels)
	}

	// "crowci" namespace is reserved for internal use
	for key := range item.Labels {
		if strings.HasPrefix(key, pipeline.InternalLabelPrefix) {
			log.Debug().Str("forge", b.Forge.Name()).Str("repo", b.Repo.FullName).Str("label", key).Msg("dropped pipeline label with reserved prefix crowci")
			delete(item.Labels, key)
		}
	}

	// Add Crow managed labels to the pipeline
	item.Labels[pipeline.LabelForgeRemoteID] = b.Forge.Name()
	item.Labels[pipeline.LabelRepoForgeID] = string(b.Repo.ForgeRemoteID)
	item.Labels[pipeline.LabelRepoID] = strconv.FormatInt(b.Repo.ID, 10)
	item.Labels[pipeline.LabelRepoName] = b.Repo.Name
	item.Labels[pipeline.LabelRepoFullName] = b.Repo.FullName
	item.Labels[pipeline.LabelBranch] = b.Repo.Branch
	item.Labels[pipeline.LabelOrgID] = strconv.FormatInt(b.Repo.OrgID, 10)

	for stageI := range item.Config.Stages {
		for stepI := range item.Config.Stages[stageI].Steps {
			item.Config.Stages[stageI].Steps[stepI].WorkflowLabels = item.Labels
		}
	}

	return item, errorsAndWarnings
}

func stepListContainsItemsToRun(items []*Item) bool {
	for i := range items {
		if items[i].Workflow.State == model.StatusPending {
			return true
		}
	}
	return false
}

func filterItemsWithMissingDependencies(items []*Item) []*Item {
	itemsToRemove := make([]*Item, 0)

	for _, item := range items {
		for _, dep := range item.DependsOn {
			if !containsItemWithName(dep, items) {
				itemsToRemove = append(itemsToRemove, item)
			}
		}
	}

	if len(itemsToRemove) > 0 {
		filtered := make([]*Item, 0)
		for _, item := range items {
			if !containsItemWithName(item.Workflow.Name, itemsToRemove) {
				filtered = append(filtered, item)
			}
		}
		// Recursive to handle transitive deps
		return filterItemsWithMissingDependencies(filtered)
	}

	return items
}

func containsItemWithName(name string, items []*Item) bool {
	for _, item := range items {
		if name == item.Workflow.Name || name == item.OriginalName {
			return true
		}
	}
	return false
}

// isWorkflowInFilter checks if a workflow should be included based on the filter.
// It matches against both the workflow name and the pipeline name (sanitized file name).
func (b *StepBuilder) isWorkflowInFilter(workflowName, pipelineName string) bool {
	for _, filter := range b.WorkflowFilter {
		if filter == workflowName || filter == pipelineName {
			return true
		}
	}
	return false
}

// expandFilterWithDependencies expands the workflow filter to include dependencies.
// This ensures that when a workflow is selected, its dependencies (that have manual trigger) are also included.
func (b *StepBuilder) expandFilterWithDependencies() []string {
	// Build a map of workflow info: name -> (dependsOn, hasManual)
	type workflowInfo struct {
		name      string
		aliases   []string // other names that refer to this workflow (e.g., file name)
		dependsOn []string
		hasManual bool
	}

	workflowMap := make(map[string]*workflowInfo)

	for _, y := range b.Yamls {
		parsed, err := yaml.ParseBytes(y.Data)
		if err != nil {
			continue
		}

		// Determine workflow name
		name := parsed.Name
		if name == "" {
			name = SanitizePath(y.Name)
		}
		sanitizedFileName := SanitizePath(y.Name)

		// Check if workflow has manual trigger
		hasManual := workflowHasManualTrigger(parsed.When)

		info := &workflowInfo{
			name:      name,
			aliases:   []string{sanitizedFileName},
			dependsOn: parsed.DependsOn,
			hasManual: hasManual,
		}

		workflowMap[name] = info
		// Also index by sanitized file name for lookups
		if sanitizedFileName != name {
			workflowMap[sanitizedFileName] = info
		}
	}

	// Now expand the filter to include dependencies (only those with manual trigger)
	expanded := make(map[string]bool)
	for _, f := range b.WorkflowFilter {
		expanded[f] = true
	}

	// Keep expanding until no new dependencies are added
	changed := true
	for changed {
		changed = false
		for name := range expanded {
			info, exists := workflowMap[name]
			if !exists {
				continue
			}
			for _, dep := range info.dependsOn {
				if expanded[dep] {
					continue
				}
				// Only add dependency if it has manual trigger
				if depInfo, depExists := workflowMap[dep]; depExists && depInfo.hasManual {
					expanded[dep] = true
					changed = true
					log.Debug().Str("workflow", name).Str("dependency", dep).Msg("adding dependency to workflow filter")
				}
			}
		}
	}

	// Convert back to slice
	result := make([]string, 0, len(expanded))
	for name := range expanded {
		result = append(result, name)
	}
	return result
}

// workflowHasManualTrigger checks if a workflow's when constraint includes the manual event.
func workflowHasManualTrigger(when constraint.When) bool {
	// If no constraints, default doesn't include manual
	if when.IsEmpty() {
		return false
	}

	// Check each constraint for manual event
	for _, c := range when.Constraints {
		// If event is not specified in constraint, it matches all events including manual
		if len(c.Event) == 0 {
			return true
		}
		// Check if manual is in the event list
		for _, event := range c.Event {
			if event == metadata.EventManual {
				return true
			}
		}
	}

	return false
}

func (b *StepBuilder) environmentVariables(metadata metadata.Metadata, axis matrix.Axis) map[string]string {
	environ := metadata.Environ()
	for k, v := range axis {
		environ[k] = v
	}
	return environ
}

func (b *StepBuilder) toInternalRepresentation(parsed *yaml_types.Workflow, environ map[string]string, metadata metadata.Metadata, workflowID int64, fileName string) (*backend_types.Config, error) {
	// Handle external secrets if fetcher is configured
	externalSecrets, err := b.injectExternalSecrets(parsed)
	if err != nil {
		return nil, pipeline_errors.NewCompilerError(
			fmt.Sprintf("### External Secrets Error\n\n%v", err),
			fileName, "",
		)
	}

	var secrets []compiler.Secret

	// Add external secrets first so they get masked in logs.
	// Each must have a unique name since compiler stores secrets in a map keyed by name.
	for key, value := range externalSecrets {
		secrets = append(secrets, compiler.Secret{
			Name:  "external:" + key,
			Value: value,
		})
	}
	for _, sec := range b.Secs {
		var events []string
		for _, event := range sec.Events {
			events = append(events, string(event))
		}

		secrets = append(secrets, compiler.Secret{
			Name:           sec.Name,
			Value:          sec.Value,
			AllowedPlugins: sec.Images,
			Events:         events,
		})
	}

	var registries []compiler.Registry
	for _, reg := range b.Regs {
		registries = append(registries, compiler.Registry{
			Hostname: reg.Address,
			Username: reg.Username,
			Password: reg.Password,
		})
	}

	return compiler.New(
		compiler.WithEnviron(environ),
		compiler.WithEnviron(b.Envs),
		compiler.WithFile(fileName),
		// TODO: server deps should be moved into StepBuilder fields and set on StepBuilder creation
		compiler.WithEscalatedWithMatchType(server.Config.Pipeline.PrivilegedPlugins, server.Config.Pipeline.PrivilegedPluginsMatchType),
		compiler.WithVolumes(server.Config.Pipeline.Volumes...),
		compiler.WithNetworks(server.Config.Pipeline.Networks...),
		compiler.WithLocal(false),
		compiler.WithOption(
			compiler.WithNetrc(
				b.Netrc.Login,
				b.Netrc.Password,
				b.Netrc.Machine,
			),
			b.Repo.IsSCMPrivate || server.Config.Pipeline.AuthenticatePublicRepos,
		),
		compiler.WithDefaultClonePlugin(server.Config.Pipeline.DefaultClonePlugin),
		compiler.WithTrustedClonePlugins(append(b.Repo.NetrcTrustedPlugins, server.Config.Pipeline.TrustedClonePlugins...)),
		compiler.WithRegistry(registries...),
		compiler.WithSecret(secrets...),
		compiler.WithPrefix(
			fmt.Sprintf(
				"crow_%s_%d",
				strings.ToLower(ulid.Make().String()),
				workflowID,
			),
		),
		compiler.WithProxy(b.ProxyOpts),
		compiler.WithWorkspaceFromURL(compiler.DefaultWorkspaceBase, b.Repo.ForgeURL),
		compiler.WithMetadata(metadata),
		compiler.WithTrustedSecurity(b.Repo.Trusted.Security),
		compiler.WithContainerNameScheme(server.Config.Pipeline.ContainerNameScheme),
		compiler.WithOrgID(b.Repo.OrgID),
	).Compile(parsed)
}

// injectExternalSecrets collects and injects external secrets into the workflow.
// This is called before compilation to replace from_secret object references with actual values.
// Returns a map of fetched secret values for log masking purposes.
func (b *StepBuilder) injectExternalSecrets(workflow *yaml_types.Workflow) (map[string]string, error) {
	if b.ExternalSecretFetcher == nil {
		// No fetcher configured, external secrets not supported
		return nil, nil
	}

	// Collect all external secret references from all steps
	collected := settings.CollectedExternalSecrets{
		Refs:      make([]settings.ExternalSecretRef, 0),
		RefsByKey: make(map[string]settings.ExternalSecretRef),
	}

	// Scan all step containers
	for _, container := range workflow.Steps.ContainerList {
		b.collectFromMap(container.Settings, &collected)
		b.collectFromMap(container.Environment, &collected)
	}

	// Scan clone containers
	for _, container := range workflow.Clone.ContainerList {
		b.collectFromMap(container.Settings, &collected)
		b.collectFromMap(container.Environment, &collected)
	}

	// Scan service containers
	for _, container := range workflow.Services.ContainerList {
		b.collectFromMap(container.Settings, &collected)
		b.collectFromMap(container.Environment, &collected)
	}

	// If no external secrets found, nothing to do
	if len(collected.Refs) == 0 {
		return nil, nil
	}

	log.Debug().
		Int("count", len(collected.Refs)).
		Int64("pipeline_id", b.Curr.ID).
		Msg("Found external secret references")

	// Fetch all external secrets
	ctx := context.Background()
	fetched, err := settings.FetchExternalSecrets(ctx, &collected, b.ExternalSecretFetcher)
	if err != nil {
		return nil, err
	}

	// Inject fetched secrets back into the workflow
	for _, container := range workflow.Steps.ContainerList {
		if err := b.injectIntoMap(container.Settings, fetched); err != nil {
			return nil, err
		}
		if err := b.injectIntoMap(container.Environment, fetched); err != nil {
			return nil, err
		}
	}

	for _, container := range workflow.Clone.ContainerList {
		if err := b.injectIntoMap(container.Settings, fetched); err != nil {
			return nil, err
		}
		if err := b.injectIntoMap(container.Environment, fetched); err != nil {
			return nil, err
		}
	}

	for _, container := range workflow.Services.ContainerList {
		if err := b.injectIntoMap(container.Settings, fetched); err != nil {
			return nil, err
		}
		if err := b.injectIntoMap(container.Environment, fetched); err != nil {
			return nil, err
		}
	}

	log.Debug().
		Int("injected", len(fetched.Secrets)).
		Int64("pipeline_id", b.Curr.ID).
		Msg("Injected external secrets")

	// Return the fetched secrets for log masking
	return fetched.Secrets, nil
}

// collectFromMap collects external secret references from a map.
func (b *StepBuilder) collectFromMap(m map[string]any, collected *settings.CollectedExternalSecrets) {
	if m == nil {
		return
	}

	for _, v := range m {
		c := settings.CollectExternalSecrets(v)
		for key, ref := range c.RefsByKey {
			if _, exists := collected.RefsByKey[key]; !exists {
				collected.RefsByKey[key] = ref
				collected.Refs = append(collected.Refs, ref)
			}
		}
	}
}

// injectIntoMap injects fetched external secrets into a map.
func (b *StepBuilder) injectIntoMap(m map[string]any, fetched *settings.FetchedExternalSecrets) error {
	if m == nil {
		return nil
	}

	for k, v := range m {
		injected, err := settings.InjectExternalSecrets(v, fetched)
		if err != nil {
			return err
		}
		m[k] = injected
	}

	return nil
}

func SanitizePath(path string) string {
	path = filepath.Base(path)
	path = strings.TrimSuffix(path, ".yml")
	path = strings.TrimSuffix(path, ".yaml")
	path = strings.TrimSuffix(path, ".jsonnet")
	path = strings.TrimPrefix(path, ".")
	return path
}

// getCaseInsensitive returns the value and true if the key exists in the map (case-insensitive).
func getCaseInsensitive(m map[string]string, key string) (string, bool) {
	for k, v := range m {
		if equalFold(k, key) {
			return v, true
		}
	}
	return "", false
}

// equalFold is a simple ASCII case-insensitive comparison.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if unicode.ToLower(rune(a[i])) != unicode.ToLower(rune(b[i])) {
			return false
		}
	}
	return true
}
