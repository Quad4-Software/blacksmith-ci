package pipeline

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	backend_types "codefloe.com/crowci/crow/v5/pipeline/backend/types"
	"codefloe.com/crowci/crow/v5/server/model"
)

func TestUpdateWorkflowNumberInConfig(t *testing.T) {
	cfg := &backend_types.Config{
		Stages: []*backend_types.Stage{
			{
				Steps: []*backend_types.Step{
					{
						Name: "clone",
						Environment: map[string]string{
							"CI_WORKFLOW_NUMBER": "1",
							"CI_REPO_OWNER":      "owner",
						},
					},
					{
						Name: "build",
						Environment: map[string]string{
							"CI_WORKFLOW_NUMBER": "1",
							"CI_REPO_OWNER":      "owner",
						},
					},
				},
			},
			{
				Steps: []*backend_types.Step{
					{
						Name: "test",
						Environment: map[string]string{
							"CI_WORKFLOW_NUMBER": "1",
						},
					},
				},
			},
		},
	}

	updateWorkflowNumberInConfig(cfg, 5)

	for _, stage := range cfg.Stages {
		for _, step := range stage.Steps {
			assert.Equal(t, "5", step.Environment["CI_WORKFLOW_NUMBER"], "step %s should have updated CI_WORKFLOW_NUMBER", step.Name)
		}
	}
	// Other env vars should be untouched
	assert.Equal(t, "owner", cfg.Stages[0].Steps[0].Environment["CI_REPO_OWNER"])
}

func TestCollectRerunWorkflows(t *testing.T) {
	tests := []struct {
		name      string
		target    *model.Workflow
		workflows []*model.Workflow
		expected  []string
	}{
		{
			name:   "single workflow with no deps",
			target: &model.Workflow{ID: 1, Name: "build"},
			workflows: []*model.Workflow{
				{ID: 1, Name: "build"},
			},
			expected: []string{"build"},
		},
		{
			name:   "includes downstream dependents",
			target: &model.Workflow{ID: 1, Name: "build"},
			workflows: []*model.Workflow{
				{ID: 1, Name: "build"},
				{ID: 2, Name: "test", DependsOn: []string{"build"}},
				{ID: 3, Name: "deploy", DependsOn: []string{"test"}},
			},
			expected: []string{"build", "deploy", "test"},
		},
		{
			name:   "includes upstream dependencies",
			target: &model.Workflow{ID: 3, Name: "deploy", DependsOn: []string{"test"}},
			workflows: []*model.Workflow{
				{ID: 1, Name: "build"},
				{ID: 2, Name: "test", DependsOn: []string{"build"}},
				{ID: 3, Name: "deploy", DependsOn: []string{"test"}},
			},
			expected: []string{"build", "deploy", "test"},
		},
		{
			name:   "leaf workflow includes only its upstream chain",
			target: &model.Workflow{ID: 3, Name: "docs-release", DependsOn: []string{"images"}},
			workflows: []*model.Workflow{
				{ID: 1, Name: "images"},
				{ID: 2, Name: "release", DependsOn: []string{"images"}},
				{ID: 3, Name: "docs-release", DependsOn: []string{"images"}},
				{ID: 4, Name: "unrelated"},
			},
			expected: []string{"docs-release", "images"},
		},
		{
			name:   "mid-chain workflow includes both upstream and downstream",
			target: &model.Workflow{ID: 2, Name: "test", DependsOn: []string{"build"}},
			workflows: []*model.Workflow{
				{ID: 1, Name: "build"},
				{ID: 2, Name: "test", DependsOn: []string{"build"}},
				{ID: 3, Name: "deploy", DependsOn: []string{"test"}},
				{ID: 4, Name: "notify", DependsOn: []string{"deploy"}},
			},
			expected: []string{"build", "deploy", "notify", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collectRerunWorkflows(tt.target, tt.workflows)
			sort.Strings(result)
			assert.Equal(t, tt.expected, result)
		})
	}
}
