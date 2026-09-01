package errors_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	pipeline_errors "codefloe.com/crowci/crow/v6/pipeline/errors"
	"codefloe.com/crowci/crow/v6/pipeline/errors/types"
)

type quickFixError struct{}

func (quickFixError) Error() string {
	return `Secret "crow_aws_security_groups" is not allowed to be used with pipeline event "pull_request"`
}

func (quickFixError) QuickFixSecretName() string {
	return "crow_aws_security_groups"
}

func (quickFixError) QuickFixEvent() string {
	return "pull_request"
}

func TestGetPipelineErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		title    string
		err      error
		expected []*types.PipelineError
	}{
		{
			title:    "nil error",
			err:      nil,
			expected: nil,
		},
		{
			title: "warning",
			err: &types.PipelineError{
				IsWarning: true,
			},
			expected: []*types.PipelineError{
				{
					IsWarning: true,
				},
			},
		},
		{
			title: "pipeline error",
			err: &types.PipelineError{
				IsWarning: false,
			},
			expected: []*types.PipelineError{
				{
					IsWarning: false,
				},
			},
		},
		{
			title: "multiple warnings",
			err: errors.Join(
				&types.PipelineError{
					IsWarning: true,
				},
				&types.PipelineError{
					IsWarning: true,
				},
			),
			expected: []*types.PipelineError{
				{
					IsWarning: true,
				},
				{
					IsWarning: true,
				},
			},
		},
		{
			title: "multiple errors and warnings",
			err: errors.Join(
				&types.PipelineError{
					IsWarning: true,
				},
				&types.PipelineError{
					IsWarning: false,
				},
				errors.New("some error"),
			),
			expected: []*types.PipelineError{
				{
					IsWarning: true,
				},
				{
					IsWarning: false,
				},
				{
					Type:      types.PipelineErrorTypeGeneric,
					IsWarning: false,
					Message:   "some error",
				},
			},
		},
	}

	for _, test := range tests {
		assert.Equalf(t, pipeline_errors.GetPipelineErrors(test.err), test.expected, test.title)
	}
}

func TestHasBlockingErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		title    string
		err      error
		expected bool
	}{
		{
			title:    "nil error",
			err:      nil,
			expected: false,
		},
		{
			title: "warning",
			err: &types.PipelineError{
				IsWarning: true,
			},
			expected: false,
		},
		{
			title: "pipeline error",
			err: &types.PipelineError{
				IsWarning: false,
			},
			expected: true,
		},
		{
			title: "multiple warnings",
			err: errors.Join(
				&types.PipelineError{
					IsWarning: true,
				},
				&types.PipelineError{
					IsWarning: true,
				},
			),
			expected: false,
		},
		{
			title: "multiple errors and warnings",
			err: errors.Join(
				&types.PipelineError{
					IsWarning: true,
				},
				&types.PipelineError{
					IsWarning: false,
				},
				errors.New("some error"),
			),
			expected: true,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, pipeline_errors.HasBlockingErrors(test.err))
	}
}

func TestNewCompilerErrorIncludesQuickFixData(t *testing.T) {
	t.Parallel()

	err := pipeline_errors.NewCompilerError(quickFixError{}, ".crow/check.yaml", "Crow (check)")
	data, ok := err.Data.(*pipeline_errors.CompilerErrorData)
	assert.True(t, ok)
	if !ok {
		t.Fatal("compiler error data should be CompilerErrorData")
	}

	assert.Equal(t, types.PipelineErrorTypeCompiler, err.Type)
	assert.Equal(t, `Secret "crow_aws_security_groups" is not allowed to be used with pipeline event "pull_request"`, err.Message)
	assert.Equal(t, ".crow/check.yaml", data.File)
	assert.Equal(t, "Crow (check)", data.Step)
	assert.Equal(t, "crow_aws_security_groups", data.SecretName)
	assert.Equal(t, "pull_request", data.Event)
}
