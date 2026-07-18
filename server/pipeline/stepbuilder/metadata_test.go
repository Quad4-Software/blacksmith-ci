// Copyright 2023 crow Authors
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
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/frontend/metadata"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
)

func TestMetadataFromStruct(t *testing.T) {
	forge := new(forge_mocks.MockForge)
	forge.On("Name").Return("gitea")
	forge.On("URL").Return("https://gitea.com")

	testCases := []struct {
		name             string
		forge            metadata.ServerForge
		repo             *model.Repo
		pipeline, prev   *model.Pipeline
		workflow         *model.Workflow
		sysURL           string
		expectedMetadata metadata.Metadata
		expectedEnviron  map[string]string
	}{
		{
			name:             "Test with empty info",
			expectedMetadata: metadata.Metadata{Sys: metadata.System{Name: "crow"}},
			expectedEnviron: map[string]string{
				"CI":                  "crow",
				"CI_PIPELINE_CREATED": "0", "CI_PIPELINE_FILES": "[]", "CI_PIPELINE_NUMBER": "0",
				"CI_PIPELINE_PARENT": "0", "CI_PIPELINE_STARTED": "0", "CI_PIPELINE_URL": "/repos/0/pipeline/0",
				"CI_PREV_PIPELINE_CREATED":  "0",
				"CI_PREV_PIPELINE_FINISHED": "0", "CI_PREV_PIPELINE_NUMBER": "0", "CI_PREV_PIPELINE_PARENT": "0",
				"CI_PREV_PIPELINE_STARTED": "0", "CI_PREV_PIPELINE_URL": "/repos/0/pipeline/0",
				"CI_REPO_PRIVATE": "false", "CI_REPO_TRUSTED": "false", "CI_REPO_TRUSTED_NETWORK": "false", "CI_REPO_TRUSTED_SECURITY": "false", "CI_REPO_TRUSTED_VOLUMES": "false",
				"CI_STEP_NUMBER": "0", "CI_STEP_URL": "/repos/0/pipeline/0", "CI_SYSTEM_NAME": "crow",
				"CI_WORKFLOW_NUMBER": "0", "CI_WORKSPACE": "/crow/src",
			},
		},
		{
			name:     "Test with forge",
			forge:    forge,
			repo:     &model.Repo{FullName: "testUser/testRepo", ForgeURL: "https://gitea.com/testUser/testRepo", Clone: "https://gitea.com/testUser/testRepo.git", CloneSSH: "git@gitea.com:testUser/testRepo.git", Branch: "main", IsSCMPrivate: true},
			pipeline: &model.Pipeline{Number: 3, ChangedFiles: []string{"test.go", "markdown file.md"}},
			prev:     &model.Pipeline{Number: 2},
			workflow: &model.Workflow{Name: "hello"},
			sysURL:   "https://example.com",
			expectedMetadata: metadata.Metadata{
				Forge: metadata.Forge{Type: "gitea", URL: "https://gitea.com"},
				Sys:   metadata.System{Name: "crow", Host: "example.com", URL: "https://example.com"},
				Repo:  metadata.Repo{Owner: "testUser", Name: "testRepo", ForgeURL: "https://gitea.com/testUser/testRepo", CloneURL: "https://gitea.com/testUser/testRepo.git", CloneSSHURL: "git@gitea.com:testUser/testRepo.git", Branch: "main", Private: true},
				Curr: metadata.Pipeline{
					Number: 3,
					Commit: metadata.Commit{ChangedFiles: []string{"test.go", "markdown file.md"}},
				},
				Prev:     metadata.Pipeline{Number: 2},
				Workflow: metadata.Workflow{Name: "hello"},
			},
			expectedEnviron: map[string]string{
				"CI":            "crow",
				"CI_FORGE_TYPE": "gitea", "CI_FORGE_URL": "https://gitea.com",
				"CI_PIPELINE_CREATED": "0", "CI_PIPELINE_FILES": `["test.go","markdown file.md"]`,
				"CI_PIPELINE_NUMBER": "3", "CI_PIPELINE_PARENT": "0", "CI_PIPELINE_STARTED": "0", "CI_PIPELINE_URL": "https://example.com/repos/0/pipeline/3",
				"CI_PREV_PIPELINE_CREATED":  "0",
				"CI_PREV_PIPELINE_FINISHED": "0", "CI_PREV_PIPELINE_NUMBER": "2", "CI_PREV_PIPELINE_PARENT": "0",
				"CI_PREV_PIPELINE_STARTED": "0", "CI_PREV_PIPELINE_URL": "https://example.com/repos/0/pipeline/2", "CI_REPO": "testUser/testRepo", "CI_REPO_CLONE_URL": "https://gitea.com/testUser/testRepo.git", "CI_REPO_CLONE_SSH_URL": "git@gitea.com:testUser/testRepo.git",
				"CI_REPO_DEFAULT_BRANCH": "main", "CI_REPO_NAME": "testRepo", "CI_REPO_OWNER": "testUser", "CI_REPO_PRIVATE": "true",
				"CI_REPO_TRUSTED": "false", "CI_REPO_TRUSTED_NETWORK": "false", "CI_REPO_TRUSTED_SECURITY": "false", "CI_REPO_TRUSTED_VOLUMES": "false",
				"CI_REPO_URL": "https://gitea.com/testUser/testRepo", "CI_STEP_NUMBER": "0", "CI_STEP_URL": "https://example.com/repos/0/pipeline/3", "CI_SYSTEM_HOST": "example.com",
				"CI_SYSTEM_NAME": "crow", "CI_SYSTEM_URL": "https://example.com", "CI_WORKFLOW_NAME": "hello", "CI_WORKFLOW_NUMBER": "0",
				"CI_WORKSPACE": "/crow/src/gitea.com/testUser/testRepo",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := MetadataFromStruct(testCase.forge, testCase.repo, testCase.pipeline, testCase.prev, testCase.workflow, testCase.sysURL)
			assert.EqualValues(t, testCase.expectedMetadata, result)
			assert.EqualValues(t, testCase.expectedEnviron, result.Environ())
		})
	}
}
