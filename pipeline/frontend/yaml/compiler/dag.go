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
	"sort"

	backend_types "codeberg.org/crowci/crow/v3/pipeline/backend/types"
)

type dagCompilerStep struct {
	step                    *backend_types.Step
	position                int
	name                    string
	dependsOn               []string
	hasExplicitDependencies bool
}

type dagCompiler struct {
	steps []*dagCompilerStep
}

func newDAGCompiler(steps []*dagCompilerStep) dagCompiler {
	return dagCompiler{
		steps: steps,
	}
}

func (c dagCompiler) isDAG() bool {
	for _, v := range c.steps {
		if v.hasExplicitDependencies {
			return true
		}
	}
	return false
}

func (c dagCompiler) compile() ([]*backend_types.Stage, error) {
	if c.isDAG() {
		c.setDependenciesForDAG()
		return c.compileByDependsOn()
	}
	c.setDependenciesForSequence()
	return c.compileSequence()
}

func (c dagCompiler) setDependenciesForDAG() {
	// Find clone step names
	cloneStepNames := make([]string, 0)
	for _, step := range c.steps {
		if step.position < 0 { // Clone steps have negative positions
			cloneStepNames = append(cloneStepNames, step.name)
		}
	}

	// For DAG mode, set implicit clone dependencies for steps without explicit dependencies
	for _, step := range c.steps {
		if step.position >= 0 && (!step.hasExplicitDependencies || len(step.dependsOn) == 0) && len(cloneStepNames) > 0 {
			step.dependsOn = cloneStepNames
			step.step.DependsOn = cloneStepNames
		}
	}
}

func (c dagCompiler) setDependenciesForSequence() {
	// Find clone step names
	cloneStepNames := make([]string, 0)
	for _, step := range c.steps {
		if step.position < 0 { // Clone steps have negative positions
			cloneStepNames = append(cloneStepNames, step.name)
		}
	}

	// For sequential mode, create a dependency chain
	var previousStepName string
	if len(cloneStepNames) > 0 {
		// If there are clone steps, the first step depends on clone
		previousStepName = cloneStepNames[0] // Use first clone step as dependency
	}

	for _, step := range c.steps {
		if step.position >= 0 && !step.hasExplicitDependencies {
			if previousStepName != "" {
				step.dependsOn = []string{previousStepName}
				step.step.DependsOn = []string{previousStepName}
			}
			previousStepName = step.name
		}
	}
}

func (c dagCompiler) compileSequence() ([]*backend_types.Stage, error) {
	stages := make([]*backend_types.Stage, 0, len(c.steps))

	for _, s := range c.steps {
		stages = append(stages, &backend_types.Stage{
			Steps: []*backend_types.Step{s.step},
		})
	}

	return stages, nil
}

func (c dagCompiler) compileByDependsOn() ([]*backend_types.Stage, error) {
	stepMap := make(map[string]*dagCompilerStep, len(c.steps))
	for _, s := range c.steps {
		stepMap[s.name] = s
	}
	return convertDAGToStages(stepMap)
}

func dfsVisit(steps map[string]*dagCompilerStep, name string, visited map[string]struct{}, path []string) error {
	if _, ok := visited[name]; ok {
		return &ErrStepDependencyCycle{path: path}
	}

	visited[name] = struct{}{}
	path = append(path, name)

	for _, dep := range steps[name].dependsOn {
		if err := dfsVisit(steps, dep, visited, path); err != nil {
			return err
		}
	}

	delete(visited, name)

	return nil
}

func convertDAGToStages(steps map[string]*dagCompilerStep) ([]*backend_types.Stage, error) {
	addedSteps := make(map[string]struct{})
	stages := make([]*backend_types.Stage, 0)

	for name, step := range steps {
		// check if all depends_on are valid
		for _, dep := range step.dependsOn {
			if _, ok := steps[dep]; !ok {
				return nil, &ErrStepMissingDependency{name: name, dep: dep}
			}
		}

		// check if there are cycles
		visited := make(map[string]struct{})
		if err := dfsVisit(steps, name, visited, []string{}); err != nil {
			return nil, err
		}
	}

	for len(steps) > 0 {
		addedNodesThisLevel := make(map[string]struct{})
		stage := new(backend_types.Stage)

		var stepsToAdd []*dagCompilerStep
		for name, step := range steps {
			if allDependenciesSatisfied(step, addedSteps) {
				stepsToAdd = append(stepsToAdd, step)
				addedNodesThisLevel[name] = struct{}{}
				delete(steps, name)
			}
		}

		// as steps are from a map that has no deterministic order,
		// we sort the steps by original config position to make the order similar between pipelines
		sort.Slice(stepsToAdd, func(i, j int) bool {
			return stepsToAdd[i].position < stepsToAdd[j].position
		})

		for i := range stepsToAdd {
			stage.Steps = append(stage.Steps, stepsToAdd[i].step)
		}

		for name := range addedNodesThisLevel {
			addedSteps[name] = struct{}{}
		}

		stages = append(stages, stage)
	}

	return stages, nil
}

func allDependenciesSatisfied(step *dagCompilerStep, addedSteps map[string]struct{}) bool {
	for _, childName := range step.dependsOn {
		_, ok := addedSteps[childName]
		if !ok {
			return false
		}
	}
	return true
}
