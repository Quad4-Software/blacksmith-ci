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
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pipeline/stepbuilder"
	"codefloe.com/crowci/crow/v6/server/store"
)

func findOrPersistPipelineConfig(store store.Store, currentPipeline *model.Pipeline, forgeYamlConfig *forge_types.FileMeta) (*model.Config, error) {
	// Preserve the original file name (including extension) so that
	// on restart, .jsonnet files can be re-recognized by preprocessJsonnet.
	// Consumers that need a display-friendly name apply SanitizePath themselves.
	return store.ConfigPersist(&model.Config{
		RepoID: currentPipeline.RepoID,
		Name:   forgeYamlConfig.Name,
		Data:   forgeYamlConfig.Data,
	})
}

// persistGeneratedConfigs stores the YAML documents a Jsonnet evaluation produced
// and links them to the pipeline, so the config view can show what was actually
// parsed rather than only the source that produced it.
//
// Generated configs are recorded per pipeline because the evaluation depends on
// pipeline metadata (CI_PIPELINE_NUMBER, CI_PIPELINE_FILES, …); re-evaluating the
// source later would not necessarily reproduce them.
func persistGeneratedConfigs(_store store.Store, currentPipeline *model.Pipeline, generated []*stepbuilder.GeneratedConfig) error {
	if len(generated) == 0 {
		return nil
	}

	// A pipeline can be parsed more than once (a blocked pipeline is parsed at
	// creation and again on approval), so skip configs that are already linked
	// rather than tripping the unique constraint on pipeline_configs.
	existing, err := _store.ConfigsForPipeline(currentPipeline.ID)
	if err != nil {
		return err
	}
	linked := make(map[int64]struct{}, len(existing))
	for _, config := range existing {
		linked[config.ID] = struct{}{}
	}

	for _, g := range generated {
		config, err := _store.ConfigPersist(&model.Config{
			RepoID:        currentPipeline.RepoID,
			Name:          g.Name,
			Data:          g.Data,
			GeneratedFrom: g.SourceName,
		})
		if err != nil {
			return err
		}
		if _, ok := linked[config.ID]; ok {
			continue
		}
		if err := linkPipelineConfigs(_store, []*model.Config{config}, currentPipeline.ID); err != nil {
			return err
		}
		linked[config.ID] = struct{}{}
	}

	return nil
}

// forgeConfigsForPipeline loads the configs linked to a pipeline, minus the
// server-generated ones. Every path that re-parses a stored pipeline (approve,
// restart, rerun) must load through this rather than ConfigsForPipeline directly:
// handing a generated document back to the step builder alongside the .jsonnet
// it came from evaluates that source twice and builds every one of its
// workflows twice over.
func forgeConfigsForPipeline(_store store.Store, pipelineID int64) ([]*model.Config, error) {
	configs, err := _store.ConfigsForPipeline(pipelineID)
	if err != nil {
		return nil, err
	}
	return forgeConfigs(configs), nil
}

// forgeConfigs drops server-generated configs, leaving only the files that were
// fetched from the forge.
func forgeConfigs(configs []*model.Config) []*model.Config {
	result := make([]*model.Config, 0, len(configs))
	for _, config := range configs {
		if config.IsGenerated() {
			continue
		}
		result = append(result, config)
	}

	return result
}
