package errors

import (
	"errors"

	"go.uber.org/multierr"

	"codefloe.com/crowci/crow/v5/pipeline/errors/types"
)

type LinterErrorData struct {
	File   string `json:"file"`
	Field  string `json:"field"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type DeprecationErrorData struct {
	File  string `json:"file"`
	Field string `json:"field"`
	Docs  string `json:"docs"`
}

type BadHabitErrorData struct {
	File  string `json:"file"`
	Field string `json:"field"`
	Docs  string `json:"docs"`
}

type CompilerErrorData struct {
	File string `json:"file"`
	Step string `json:"step,omitempty"`
}

func GetLinterData(e *types.PipelineError) *LinterErrorData {
	if e.Type != types.PipelineErrorTypeLinter {
		return nil
	}

	if data, ok := e.Data.(*LinterErrorData); ok {
		return data
	}

	return nil
}

// NewCompilerError creates a new compiler error with file and step context.
func NewCompilerError(message, file, step string) *types.PipelineError {
	return &types.PipelineError{
		Type:    types.PipelineErrorTypeCompiler,
		Message: message,
		Data: &CompilerErrorData{
			File: file,
			Step: step,
		},
	}
}

func GetPipelineErrors(err error) []*types.PipelineError {
	var pipelineErrors []*types.PipelineError
	for _, _err := range multierr.Errors(err) {
		var err *types.PipelineError
		if errors.As(_err, &err) {
			pipelineErrors = append(pipelineErrors, err)
		} else {
			pipelineErrors = append(pipelineErrors, &types.PipelineError{
				Message: _err.Error(),
				Type:    types.PipelineErrorTypeGeneric,
			})
		}
	}

	return pipelineErrors
}

func HasBlockingErrors(err error) bool {
	if err == nil {
		return false
	}

	errs := GetPipelineErrors(err)

	for _, err := range errs {
		if !err.IsWarning {
			return true
		}
	}

	return false
}
