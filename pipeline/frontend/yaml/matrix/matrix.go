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

package matrix

import (
	"fmt"
	"strings"

	errorTypes "codeberg.org/crowci/crow/v5/pipeline/errors/types"
	"codeberg.org/crowci/crow/v5/shared/yaml"
)

const (
	limitTags        = 10
	DefaultLimitAxis = 25
)

// Matrix represents the pipeline matrix.
type Matrix map[string][]string

// Axis represents a single permutation of entries from the pipeline matrix.
// If a NAME (case-insensitive) key is present, it is used for workflow naming and not passed as an environment variable.
type Axis map[string]string

// String returns a string representation of an Axis as a comma-separated list
// of environment variables.
func (a Axis) String() string {
	var envs []string
	for k, v := range a {
		envs = append(envs, k+"="+v)
	}
	return strings.Join(envs, " ")
}

// Parse parses the Yaml matrix definition with the default limit.
func Parse(data []byte) ([]Axis, error) {
	return ParseWithLimit(data, DefaultLimitAxis)
}

// ParseWithLimit parses the Yaml matrix definition with a configurable limit.
// If limit is 0, no limit is enforced.
func ParseWithLimit(data []byte, limit int) ([]Axis, error) {
	axis, err := parseList(data)
	if err == nil && len(axis) != 0 {
		// Check limit for explicit matrix include list
		if limit > 0 && len(axis) > limit {
			return nil, &errorTypes.PipelineError{
				Message: fmt.Sprintf("matrix permutation limit exceeded: %d requested, maximum is %d", len(axis), limit),
				Type:    errorTypes.PipelineErrorTypeCompiler,
			}
		}
		return axis, nil
	}

	matrix, err := parse(data)
	if err != nil {
		return nil, err
	}

	if len(matrix) == 0 {
		return []Axis{}, nil
	}

	return calc(matrix, limit)
}

// ParseString parses the Yaml string matrix definition with the default limit.
func ParseString(data string) ([]Axis, error) {
	return Parse([]byte(data))
}

// ParseStringWithLimit parses the Yaml string matrix definition with a configurable limit.
func ParseStringWithLimit(data string, limit int) ([]Axis, error) {
	return ParseWithLimit([]byte(data), limit)
}

func calc(matrix Matrix, limit int) ([]Axis, error) {
	// calculate number of permutations and extract the list of tags
	// (ie go_version, redis_version, etc)
	var perm int
	var tags []string
	for k, v := range matrix {
		perm *= len(v)
		if perm == 0 {
			perm = len(v)
		}
		tags = append(tags, k)
	}

	// Check if the number of permutations exceeds the limit
	if limit > 0 && perm > limit {
		return nil, &errorTypes.PipelineError{
			Message: fmt.Sprintf("matrix permutation limit exceeded: %d permutations requested, maximum is %d", perm, limit),
			Type:    errorTypes.PipelineErrorTypeCompiler,
		}
	}

	// structure to hold the transformed result set
	var axisList []Axis

	// for each axis calculate the unique set of values that should be used.
	for p := 0; p < perm; p++ {
		axis := map[string]string{}
		decrease := perm
		for i, tag := range tags {
			elems := matrix[tag]
			decrease /= len(elems)
			elem := p / decrease % len(elems)
			axis[tag] = elems[elem]

			// enforce a maximum number of tags in the pipeline matrix.
			if i > limitTags {
				break
			}
		}

		// append to the list of axis.
		axisList = append(axisList, axis)
	}

	return axisList, nil
}

func parse(raw []byte) (Matrix, error) {
	data := struct {
		Matrix map[string][]string
	}{}
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return nil, &errorTypes.PipelineError{Message: err.Error(), Type: errorTypes.PipelineErrorTypeCompiler}
	}
	return data.Matrix, nil
}

func parseList(raw []byte) ([]Axis, error) {
	data := struct {
		Matrix struct {
			Include []Axis
		}
	}{}

	if err := yaml.Unmarshal(raw, &data); err != nil {
		return nil, &errorTypes.PipelineError{Message: err.Error(), Type: errorTypes.PipelineErrorTypeCompiler}
	}
	return data.Matrix.Include, nil
}
