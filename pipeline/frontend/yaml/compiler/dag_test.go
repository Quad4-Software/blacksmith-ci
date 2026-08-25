// Copyright 2023 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	backend_types "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestConvertDAGToStages(t *testing.T) {
	steps := map[string]*dagCompilerStep{
		"step1": {
			step:      &backend_types.Step{},
			dependsOn: []string{"step3"},
		},
		"step2": {
			step:      &backend_types.Step{},
			dependsOn: []string{"step1"},
		},
		"step3": {
			step:      &backend_types.Step{},
			dependsOn: []string{"step2"},
		},
	}
	_, err := convertDAGToStages(steps)
	assert.ErrorIs(t, err, &ErrStepDependencyCycle{})

	steps = map[string]*dagCompilerStep{
		"step1": {
			step:      &backend_types.Step{},
			dependsOn: []string{"step2"},
		},
		"step2": {
			step: &backend_types.Step{},
		},
	}
	_, err = convertDAGToStages(steps)
	assert.NoError(t, err)

	steps = map[string]*dagCompilerStep{
		"a": {
			step: &backend_types.Step{},
		},
		"b": {
			step:      &backend_types.Step{},
			dependsOn: []string{"a"},
		},
		"c": {
			step:      &backend_types.Step{},
			dependsOn: []string{"a"},
		},
		"d": {
			step:      &backend_types.Step{},
			dependsOn: []string{"b", "c"},
		},
	}
	_, err = convertDAGToStages(steps)
	assert.NoError(t, err)

	steps = map[string]*dagCompilerStep{
		"step1": {
			step:      &backend_types.Step{},
			dependsOn: []string{"not-existing-step"},
		},
	}
	_, err = convertDAGToStages(steps)
	assert.ErrorIs(t, err, &ErrStepMissingDependency{})

	steps = map[string]*dagCompilerStep{
		"echo env": {
			position: 0,
			name:     "echo env",
			step: &backend_types.Step{
				UUID:  "01HJDPEW6R7J0JBE3F1T7Q0TYX",
				Type:  "commands",
				Name:  "echo env",
				Image: "bash",
			},
		},
		"echo 1": {
			position:  1,
			name:      "echo 1",
			dependsOn: []string{"echo env", "echo 2"},
			step: &backend_types.Step{
				UUID:  "01HJDPF770QGRZER8RF79XVS4M",
				Type:  "commands",
				Name:  "echo 1",
				Image: "bash",
			},
		},
		"echo 2": {
			position: 2,
			name:     "echo 2",
			step: &backend_types.Step{
				UUID:  "01HJDPFF5RMEYZW0YTGR1Y1ZR0",
				Type:  "commands",
				Name:  "echo 2",
				Image: "bash",
			},
		},
	}
	stages, err := convertDAGToStages(steps)
	assert.NoError(t, err)
	assert.EqualValues(t, []*backend_types.Stage{{
		Steps: []*backend_types.Step{{
			UUID:  "01HJDPEW6R7J0JBE3F1T7Q0TYX",
			Type:  "commands",
			Name:  "echo env",
			Image: "bash",
		}, {
			UUID:  "01HJDPFF5RMEYZW0YTGR1Y1ZR0",
			Type:  "commands",
			Name:  "echo 2",
			Image: "bash",
		}},
	}, {
		Steps: []*backend_types.Step{{
			UUID:  "01HJDPF770QGRZER8RF79XVS4M",
			Type:  "commands",
			Name:  "echo 1",
			Image: "bash",
		}},
	}}, stages)
}

// TestDfsVisitUnknownDependency verifies that dfsVisit returns
// ErrStepMissingDependency instead of panicking on a nil map entry when it
// encounters a dependency name that is not present in the step map. This
// guards the traversal independently of convertDAGToStages' pre-check.
func TestDfsVisitUnknownDependency(t *testing.T) {
	steps := map[string]*dagCompilerStep{
		"step1": {
			step:      &backend_types.Step{},
			name:      "step1",
			dependsOn: []string{"missing"},
		},
	}

	err := dfsVisit(steps, "step1", make(map[string]struct{}), []string{})
	assert.ErrorIs(t, err, &ErrStepMissingDependency{})

	var missingErr *ErrStepMissingDependency
	if assert.ErrorAs(t, err, &missingErr) {
		assert.Equal(t, "step1", missingErr.name)
		assert.Equal(t, "missing", missingErr.dep)
	}
}

func TestIsDag(t *testing.T) {
	steps := []*dagCompilerStep{
		{
			step: &backend_types.Step{},
		},
	}
	c := newDAGCompiler(steps)
	assert.False(t, c.isDAG())

	steps = []*dagCompilerStep{
		{
			step:                    &backend_types.Step{},
			dependsOn:               []string{},
			hasExplicitDependencies: true,
		},
	}
	c = newDAGCompiler(steps)
	assert.True(t, c.isDAG())

	steps = []*dagCompilerStep{
		{
			step:                    &backend_types.Step{},
			dependsOn:               []string{"other-step"},
			hasExplicitDependencies: true,
		},
	}
	c = newDAGCompiler(steps)
	assert.True(t, c.isDAG())
}

// The runtime schedules steps from Step.DependsOn alone and does not fall back
// to the stage grouping, so the compiler has to express sequential order there
// too. Without this, a workflow that sets no depends_on would run in parallel.
func TestCompileAlwaysExpressesOrderInDependsOn(t *testing.T) {
	// createProcess already copies the container's depends_on onto the compiled
	// step, so the fixture mirrors that.
	newStep := func(name string, position int, dependsOn []string, explicit bool) *dagCompilerStep {
		return &dagCompilerStep{
			step:                    &backend_types.Step{Name: name, DependsOn: dependsOn},
			position:                position,
			name:                    name,
			dependsOn:               dependsOn,
			hasExplicitDependencies: explicit,
		}
	}

	t.Run("sequential workflow chains every step", func(t *testing.T) {
		steps := []*dagCompilerStep{
			newStep("clone", -1, nil, false),
			newStep("build", 0, nil, false),
			newStep("test", 1, nil, false),
		}
		_, err := newDAGCompiler(steps).compile()
		assert.NoError(t, err)
		assert.Empty(t, steps[0].step.DependsOn, "the clone step starts the chain")
		assert.Equal(t, []string{"clone"}, steps[1].step.DependsOn)
		assert.Equal(t, []string{"build"}, steps[2].step.DependsOn)
	})

	t.Run("sequential workflow without a clone step chains the rest", func(t *testing.T) {
		steps := []*dagCompilerStep{
			newStep("build", 0, nil, false),
			newStep("test", 1, nil, false),
		}
		_, err := newDAGCompiler(steps).compile()
		assert.NoError(t, err)
		assert.Empty(t, steps[0].step.DependsOn)
		assert.Equal(t, []string{"build"}, steps[1].step.DependsOn)
	})

	t.Run("dag workflow anchors dependency-less steps on clone", func(t *testing.T) {
		steps := []*dagCompilerStep{
			newStep("clone", -1, nil, false),
			newStep("build", 0, nil, false),
			newStep("deploy", 1, []string{"build"}, true),
		}
		_, err := newDAGCompiler(steps).compile()
		assert.NoError(t, err)
		assert.Equal(t, []string{"clone"}, steps[1].step.DependsOn)
		assert.Equal(t, []string{"build"}, steps[2].step.DependsOn)
	})
}
