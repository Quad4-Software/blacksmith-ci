package pipeline

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
)

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
