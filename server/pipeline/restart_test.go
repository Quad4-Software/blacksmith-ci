package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v4/server/model"
)

func TestCreateNewOutOfOld(t *testing.T) {
	tests := []struct {
		name string
		old  *model.Pipeline
	}{
		{
			name: "pipeline with existing title",
			old: &model.Pipeline{
				ID:     100,
				Number: 42,
				Title:  "Fix bug in authentication",
				Status: model.StatusSuccess,
			},
		},
		{
			name: "pipeline without title",
			old: &model.Pipeline{
				ID:     200,
				Number: 73,
				Title:  "",
				Status: model.StatusFailure,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPipeline := createNewOutOfOld(tt.old)

			// Check that the new pipeline has been reset properly
			assert.Equal(t, int64(0), newPipeline.ID)
			assert.Equal(t, int64(0), newPipeline.Number)
			assert.Equal(t, model.StatusPending, newPipeline.Status)
			assert.Equal(t, int64(0), newPipeline.Started)
			assert.Equal(t, int64(0), newPipeline.Finished)
			assert.Nil(t, newPipeline.Errors)

			// Check that the title is preserved unchanged
			assert.Equal(t, tt.old.Title, newPipeline.Title)
		})
	}
}
