package errors

import (
	"errors"

	"codefloe.com/crowci/crow/v6/pipeline/errors/types"
)

type LinterErrorData struct {
	File   string `json:"file"`
	Field  string `json:"field"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	// Trusted names the repository "Privileges & Security" option that would
	// resolve this error ("security", "network" or "volumes"). Empty for
	// linter errors unrelated to trusted privileges.
	Trusted string `json:"trusted,omitempty"`
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
	for _, _err := range unwrapErrors(err) {
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

// unwrapErrors recursively flattens all errors wrapped by errors.Join,
// returning leaf errors. A single non-joined error returns a one-element slice.
func unwrapErrors(err error) []error {
	if err == nil {
		return nil
	}
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		var result []error
		for _, e := range multi.Unwrap() {
			result = append(result, unwrapErrors(e)...)
		}
		return result
	}
	return []error{err}
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
