package pipeline

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/model"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

func TestPipelineLogPurge(t *testing.T) {
	tests := []struct {
		name              string
		repo              *model.Repo
		expectedOlderThan time.Duration
		expectedKeepMin   int64
		mockDeleteError   error
		wantDelete        int
		expectError       bool
	}{
		{
			name: "Valid LogsDurationKeep",
			repo: &model.Repo{
				FullName:             "test/repo1",
				LogsDurationKeep:     "24h",
				LogsPipelinesKeepMin: 1,
			},
			expectedOlderThan: 24 * time.Hour,
			expectedKeepMin:   5,
			wantDelete:        2,
			expectError:       false,
		},
		{
			name: "Combine KeepMin and Duration",
			repo: &model.Repo{
				FullName:             "test/repo3",
				LogsDurationKeep:     "24h",
				LogsPipelinesKeepMin: 2,
			},
			expectedOlderThan: 24 * time.Hour,
			expectedKeepMin:   2,
			wantDelete:        2, // only two step logs (2*2) should be deleted
			expectError:       false,
		},
		{
			name: "Only DurationKeep",
			repo: &model.Repo{
				FullName:             "test/repo4",
				LogsDurationKeep:     "24h",
				LogsPipelinesKeepMin: -1,
			},
			expectedOlderThan: 24 * time.Hour,
			expectedKeepMin:   -1,
			wantDelete:        2, // should delete nr. 4
			expectError:       false,
		},
		{
			name: "Only KeepMin",
			repo: &model.Repo{
				FullName:             "test/repo5",
				LogsDurationKeep:     "",
				LogsPipelinesKeepMin: 3,
			},
			expectedKeepMin: 3,
			wantDelete:      2, // should delete nr. 4
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(store_mocks.MockStore)

			mockStore.On("GetPipeline", mock.Anything)
			mockStore.On("GetPipelineCountByRepo", mock.Anything).Return(int64(4), nil)
			mockStore.On("GetPipelineLogPurged", mock.Anything).Return(false, nil)
			mockStore.On("SetPipelineLogPurged", mock.Anything).Return(nil)
			mockStore.On("GetPipelineCount", mock.Anything).Return(int64(4), nil) // Return an int64 value
			mockStore.On("GetPipelineListKeyset", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*model.Pipeline{
				{
					Number:  1,
					Created: time.Now().Add(-48 * time.Hour).Unix(), // Older than 48 hours
				},
				{
					Number:  2,
					Created: time.Now().Add(-20 * time.Hour).Unix(),
				},
				{
					Number:  3,
					Created: time.Now().Add(-19 * time.Hour).Unix(),
				},
				{
					Number:  4,
					Created: time.Now().Add(-18 * time.Hour).Unix(),
				},
			}, nil)
			mockStore.On("StepList", mock.Anything, mock.Anything).Return([]*model.Step{
				{
					ID: 1,
				},
				{
					ID: 2,
				},
			}, nil)

			// Set up the mock store to expect the LogDelete call
			if tt.mockDeleteError != nil {
				mockStore.On("LogDelete", mock.Anything, mock.Anything).Return(tt.mockDeleteError)
			} else if tt.wantDelete > 0 {
				mockStore.On("LogDelete", mock.Anything, mock.Anything).Return(nil).Times(tt.wantDelete)
			}

			// Call the function
			err := pipelineLogPurge(mockStore, tt.repo)

			// Check the results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Assert that the StepList method was called
			mockStore.AssertCalled(t, "StepList", mock.Anything, mock.Anything)

			// Assert that the LogDelete method was called
			mockStore.AssertCalled(t, "LogDelete", mock.Anything, mock.Anything)

			// Verify that LogDelete was called the expected number of times
			mockStore.AssertNumberOfCalls(t, "LogDelete", tt.wantDelete)
		})
	}
}
