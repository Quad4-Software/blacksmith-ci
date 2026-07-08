// Copyright 2023 Woodpecker Authors
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

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	crow_jsonnet "codefloe.com/crowci/crow/v6/pipeline/frontend/jsonnet"
	"codefloe.com/crowci/crow/v6/shared/constant"
)

func DetectPipelineConfig() (isDir bool, config string, _ error) {
	for _, config := range constant.DefaultConfigOrder {
		shouldBeDir := strings.HasSuffix(config, "/")
		config = strings.TrimSuffix(config, "/")

		if fi, err := os.Stat(config); err == nil && shouldBeDir == fi.IsDir() {
			return fi.IsDir(), config, nil
		}
	}

	return false, "", fmt.Errorf("could not detect pipeline config")
}

func RunPipelineFunc(ctx context.Context, c *cli.Command, fileFunc, dirFunc func(context.Context, *cli.Command, string) error) error {
	if c.Args().Len() == 0 {
		isDir, path, err := DetectPipelineConfig()
		if err != nil {
			return err
		}
		if isDir {
			return dirFunc(ctx, c, path)
		}
		return fileFunc(ctx, c, path)
	}

	multiArgs := c.Args().Len() > 1
	for _, arg := range c.Args().Slice() {
		fi, err := os.Stat(arg)
		if err != nil {
			return err
		}
		if multiArgs {
			fmt.Println("#", fi.Name())
		}
		if fi.IsDir() {
			if err := dirFunc(ctx, c, arg); err != nil {
				return err
			}
		} else {
			if err := fileFunc(ctx, c, arg); err != nil {
				return err
			}
		}
		if multiArgs {
			fmt.Println("")
		}
	}

	return nil
}

// LoadSiblingFiles reads .libsonnet and .jsonnet files from the directory for use as Jsonnet imports.
func LoadSiblingFiles(dir, excludeFile string) ([]*crow_jsonnet.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var siblings []*crow_jsonnet.File
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		if fullPath == excludeFile {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".libsonnet") || strings.HasSuffix(entry.Name(), ".jsonnet") {
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, err
			}
			siblings = append(siblings, &crow_jsonnet.File{
				Name: fullPath,
				Data: data,
			})
		}
	}
	return siblings, nil
}
