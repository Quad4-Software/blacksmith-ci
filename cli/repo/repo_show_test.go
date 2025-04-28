package repo

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	crow "codeberg.org/crowci/crow/v3/crow-go/crow"
	crow_mocks "codeberg.org/crowci/crow/v3/crow-go/crow/mocks"
)

func TestRepoShow(t *testing.T) {
	tests := []struct {
		name          string
		repoID        int64
		mockRepo      *crow.Repo
		mockError     error
		expectedError bool
		expected      *crow.Repo
		args          []string
	}{
		{
			name:     "valid repo by ID",
			repoID:   123,
			mockRepo: &crow.Repo{Name: "test-repo"},
			expected: &crow.Repo{Name: "test-repo"},
			args:     []string{"show", "123"},
		},
		{
			name:     "valid repo by full name",
			repoID:   456,
			mockRepo: &crow.Repo{ID: 456, Name: "repo", Owner: "owner"},
			expected: &crow.Repo{ID: 456, Name: "repo", Owner: "owner"},
			args:     []string{"show", "owner/repo"},
		},
		{
			name:          "invalid repo ID",
			repoID:        999,
			expectedError: true,
			args:          []string{"show", "invalid"},
			mockError:     errors.New("repo not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(crow_mocks.MockClient)
			mockClient.On("Repo", tt.repoID).Return(tt.mockRepo, tt.mockError).Maybe()
			mockClient.On("RepoLookup", "owner/repo").Return(tt.mockRepo, nil).Maybe()

			command := repoShowCmd
			command.Writer = io.Discard
			command.Action = func(_ context.Context, c *cli.Command) error {
				output, err := repoShow(c, mockClient)
				if tt.expectedError {
					assert.Error(t, err)
					return nil
				}

				assert.NoError(t, err)
				assert.Equal(t, tt.expected, output)
				return nil
			}

			_ = command.Run(t.Context(), tt.args)
		})
	}
}
