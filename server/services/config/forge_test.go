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

package config_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/config"
)

func TestFetch(t *testing.T) {
	t.Parallel()

	type file struct {
		name string
		data []byte
	}

	dummyData := []byte("TEST")

	testTable := []struct {
		name              string
		repoConfig        string
		files             []file
		expectedFileNames []string
		expectedError     bool
	}{
		{
			name:       "Default config - .crow/",
			repoConfig: "",
			files: []file{{
				name: ".crow/text.txt",
				data: dummyData,
			}, {
				name: ".crow/release.yml",
				data: dummyData,
			}, {
				name: ".crow/image.png",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".crow/release.yml",
			},
			expectedError: false,
		},
		{
			name:       "Default config with .yaml - .crow/",
			repoConfig: "",
			files: []file{{
				name: ".crow/text.txt",
				data: dummyData,
			}, {
				name: ".crow/release.yaml",
				data: dummyData,
			}, {
				name: ".crow/image.png",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".crow/release.yaml",
			},
			expectedError: false,
		},
		{
			name:       "Default config with .yaml, .yml mix - .crow/",
			repoConfig: "",
			files: []file{{
				name: ".crow/text.txt",
				data: dummyData,
			}, {
				name: ".crow/release.yaml",
				data: dummyData,
			}, {
				name: ".crow/other.yml",
				data: dummyData,
			}, {
				name: ".crow/image.png",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".crow/release.yaml",
				".crow/other.yml",
			},
			expectedError: false,
		},
		{
			name:       "Default config check .woodpecker.yaml before .woodpecker.yml",
			repoConfig: "",
			files: []file{{
				name: ".woodpecker.yaml",
				data: dummyData,
			}, {
				name: ".woodpecker.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".woodpecker.yaml",
			},
			expectedError: false,
		},
		{
			name:       "Override via API with custom config",
			repoConfig: "",
			files: []file{{
				name: ".woodpecker.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".woodpecker.yml",
			},
			expectedError: false,
		},
		{
			name:       "Use old config on 204 response",
			repoConfig: "",
			files: []file{{
				name: ".woodpecker.yaml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".woodpecker.yaml",
			},
			expectedError: false,
		},
		{
			name:              "Default config - Empty repo",
			repoConfig:        "",
			files:             []file{},
			expectedFileNames: []string{},
			expectedError:     true,
		},
		{
			name:       "Default config - Additional sub-folders",
			repoConfig: "",
			files: []file{{
				name: ".crow/test.yml",
				data: dummyData,
			}, {
				name: ".crow/sub-folder/config.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".crow/test.yml",
			},
			expectedError: false,
		},
		{
			name:       "Default config - Additional none .yml files",
			repoConfig: "",
			files: []file{{
				name: ".crow/notes.txt",
				data: dummyData,
			}, {
				name: ".crow/image.png",
				data: dummyData,
			}, {
				name: ".crow/test.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".crow/test.yml",
			},
			expectedError: false,
		},
		{
			name:       "Default config - Empty Folder",
			repoConfig: " ",
			files: []file{{
				name: ".crow/.keep",
				data: dummyData,
			}, {
				name: ".woodpecker.yml",
				data: nil,
			}, {
				name: ".woodpecker.yaml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".woodpecker.yaml",
			},
			expectedError: false,
		},
		{
			name:       "Special config - folder (ignoring default files)",
			repoConfig: ".my-ci-folder/",
			files: []file{{
				name: ".crow/test.yml",
				data: dummyData,
			}, {
				name: ".woodpecker.yml",
				data: dummyData,
			}, {
				name: ".woodpecker.yaml",
				data: dummyData,
			}, {
				name: ".my-ci-folder/test.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".my-ci-folder/test.yml",
			},
			expectedError: false,
		},
		{
			name:       "Special config - folder",
			repoConfig: ".my-ci-folder/",
			files: []file{{
				name: ".my-ci-folder/test.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".my-ci-folder/test.yml",
			},
			expectedError: false,
		},
		{
			name:       "Special config - subfolder",
			repoConfig: ".my-ci-folder/my-config/",
			files: []file{{
				name: ".my-ci-folder/my-config/test.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".my-ci-folder/my-config/test.yml",
			},
			expectedError: false,
		},
		{
			name:       "Special config - file",
			repoConfig: ".config.yml",
			files: []file{{
				name: ".config.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".config.yml",
			},
			expectedError: false,
		},
		{
			name:       "Special config - file inside subfolder",
			repoConfig: ".my-ci-folder/sub-folder/config.yml",
			files: []file{{
				name: ".my-ci-folder/sub-folder/config.yml",
				data: dummyData,
			}},
			expectedFileNames: []string{
				".my-ci-folder/sub-folder/config.yml",
			},
			expectedError: false,
		},
		{
			name:              "Special config - empty repo",
			repoConfig:        ".config.yml",
			files:             []file{},
			expectedFileNames: []string{},
			expectedError:     true,
		},
		{
			name:              "Default config - No config files found (should preserve ErrConfigNotFound)",
			repoConfig:        "",
			files:             []file{},
			expectedFileNames: []string{},
			expectedError:     true,
		},
		{
			name:              "Custom config - No config files found (should preserve ErrConfigNotFound)",
			repoConfig:        ".my-custom-config.yml",
			files:             []file{},
			expectedFileNames: []string{},
			expectedError:     true,
		},
		{
			name:              "GetContentsOrList error should be treated as config not found",
			repoConfig:        "",
			files:             []file{},
			expectedFileNames: []string{},
			expectedError:     true,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			repo := &model.Repo{Owner: "laszlocph", Name: "multipipeline", Config: tt.repoConfig}

			f := new(forge_mocks.MockForge)
			dirs := map[string][]*forge_types.FileMeta{}
			for _, file := range tt.files {
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, file.name).Once().Return(file.data, nil)
				path := filepath.Dir(file.name)
				if path != "." {
					dirs[path] = append(dirs[path], &forge_types.FileMeta{
						Name: file.name,
						Data: file.data,
					})
				}
			}

			for path, files := range dirs {
				f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, path).Once().Return(files, nil)
			}

			// if the previous mocks do not match return not found errors
			switch tt.name {
			case "Default config - No config files found (should preserve ErrConfigNotFound)",
				"Custom config - No config files found (should preserve ErrConfigNotFound)":
				// Return ErrConfigNotFound for all config files
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow.yaml").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".crow.yaml"}})
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow.yml").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".crow.yml"}})
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow.jsonnet").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".crow.jsonnet"}})
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".woodpecker.yaml").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".woodpecker.yaml"}})
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".woodpecker.yml").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".woodpecker.yml"}})
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".my-custom-config.yml").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".my-custom-config.yml"}})
				f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".crow"}})
				f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".woodpecker").Return(nil, &forge_types.ErrConfigNotFound{Configs: []string{".woodpecker"}})
			case "GetContentsOrList error should be treated as config not found":
				// Return GetContentsOrList errors for all config files
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow.yaml").Return(nil, fmt.Errorf("GetContentsOrList"))
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow.yml").Return(nil, fmt.Errorf("GetContentsOrList"))
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow.jsonnet").Return(nil, fmt.Errorf("GetContentsOrList"))
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".woodpecker.yaml").Return(nil, fmt.Errorf("GetContentsOrList"))
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".woodpecker.yml").Return(nil, fmt.Errorf("GetContentsOrList"))
				f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow").Return(nil, fmt.Errorf("GetContentsOrList"))
				f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".woodpecker").Return(nil, fmt.Errorf("GetContentsOrList"))
			default:
				// For existing tests, keep the generic error behavior
				f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("file not found"))
				f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("directory not found"))
			}

			configFetcher := config.NewForge(
				time.Second*3,
				3,
			)
			files, err := configFetcher.Fetch(
				t.Context(),
				f,
				&model.User{AccessToken: "xxx"},
				repo,
				&model.Pipeline{Commit: "89ab7b2d6bfb347144ac7c557e638ab402848fee"},
				nil,
				false,
			)
			if tt.expectedError && err == nil {
				t.Fatal("expected an error")
			} else if !tt.expectedError && err != nil {
				t.Fatal("error fetching config:", err)
			}

			// Test that ErrConfigNotFound is preserved for proper upstream handling
			if tt.expectedError && err != nil {
				var configNotFoundErr *forge_types.ErrConfigNotFound
				if errors.As(err, &configNotFoundErr) {
					t.Logf("Correctly preserved ErrConfigNotFound error: %v", err)
				} else {
					if errors.Is(err, &forge_types.ErrConfigNotFound{}) {
						t.Logf("Correctly wrapped but detectable ErrConfigNotFound error: %v", err)
					} else {
						t.Errorf("Expected ErrConfigNotFound error, but got: %v", err)
					}
				}
			}

			matchingFiles := make([]string, len(files))
			for i := range files {
				matchingFiles[i] = files[i].Name
			}
			assert.ElementsMatch(t, tt.expectedFileNames, matchingFiles, "expected some other pipeline files")
		})
	}
}

func TestFetchCanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(nil)
	f := forge_mocks.NewMockForge(t)
	f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, context.Canceled).Maybe()
	f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, context.Canceled).Maybe()
	_, err := config.NewForge(time.Second, 3).Fetch(ctx, f, &model.User{}, &model.Repo{}, &model.Pipeline{}, nil, false)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, f.Calls, "a canceled request must not make forge calls or retries")
}

func TestFetchRetriesWithFreshDeadline(t *testing.T) {
	t.Parallel()
	f := forge_mocks.NewMockForge(t)
	f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow").Run(func(args mock.Arguments) {
		ctx, ok := args.Get(0).(context.Context)
		require.True(t, ok)
		<-ctx.Done()
	}).Return(nil, context.DeadlineExceeded).Once()
	files := []*forge_types.FileMeta{{Name: ".crow/build.yaml", Data: []byte("steps: []")}}
	f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".crow").Run(func(args mock.Arguments) {
		ctx, ok := args.Get(0).(context.Context)
		require.True(t, ok)
		assert.NoError(t, ctx.Err())
	}).Return(files, nil).Once()
	f.On("Dir", mock.Anything, mock.Anything, mock.Anything, mock.Anything, ".woodpecker").Return(nil, context.DeadlineExceeded).Maybe()
	f.On("File", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded).Maybe()
	got, err := config.NewForge(20*time.Millisecond, 3).Fetch(t.Context(), f, &model.User{}, &model.Repo{}, &model.Pipeline{}, nil, false)
	assert.NoError(t, err)
	assert.Equal(t, files, got)
	assert.Len(t, f.Calls, 2, "an expired attempt must not probe fallback paths")
}
