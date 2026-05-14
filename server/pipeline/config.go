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
	forge_types "codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
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
