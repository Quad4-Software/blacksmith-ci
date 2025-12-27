// Copyright 2021 Woodpecker Authors
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

//go:build !cgo

package datastore

import (
	"xorm.io/xorm"

	"codeberg.org/crowci/crow/v5/server/store"
)

// configureEngine applies driver-specific configuration.
// For non-CGO builds (no SQLite support), just apply standard connection settings.
func configureEngine(engine *xorm.Engine, opts *store.Opts) error {
	engine.SetMaxOpenConns(opts.XORM.MaxOpenConns)
	engine.SetMaxIdleConns(opts.XORM.MaxIdleConns)
	engine.SetConnMaxLifetime(opts.XORM.ConnMaxLifetime)

	return nil
}
