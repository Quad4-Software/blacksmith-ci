// Copyright 2021 Woodpecker Authors
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

package model

import "github.com/uptrace/bun"

// Config represents a pipeline configuration.
type Config struct {
	bun.BaseModel `bun:"table:configs"`

	ID     int64  `json:"-"    bun:"id,pk,autoincrement"`
	RepoID int64  `json:"-"    bun:"repo_id,unique:s"`
	Hash   string `json:"hash" bun:"hash,unique:s"`
	Name   string `json:"name" bun:"name,unique:s"`
	Data   []byte `json:"data" bun:"data"`
	// GeneratedFrom names the config this one was produced from, for configs that
	// are not fetched from the forge but computed by the server. Currently this is
	// the .jsonnet source a rendered YAML document was evaluated out of. Empty for
	// configs that came from the forge verbatim.
	//
	// Generated configs exist so the evaluated pipeline can be inspected after the
	// fact; they must never be fed back into the step builder, or the source would
	// be evaluated a second time and every workflow would be built twice.
	GeneratedFrom string `json:"generated_from,omitempty" bun:"generated_from"`
} //	@name	Config

// IsGenerated reports whether the config was produced by the server rather than
// fetched from the forge.
func (c Config) IsGenerated() bool {
	return c.GeneratedFrom != ""
}

func (Config) TableName() string {
	return "configs"
}

// PipelineConfig is the n:n relation between Pipeline and Config.
type PipelineConfig struct {
	bun.BaseModel `bun:"table:pipeline_configs"`

	ConfigID   int64 `json:"-"   bun:"config_id,notnull,unique:s"`
	PipelineID int64 `json:"-"   bun:"pipeline_id,notnull,unique:s"`
}

func (PipelineConfig) TableName() string {
	return "pipeline_configs"
}
