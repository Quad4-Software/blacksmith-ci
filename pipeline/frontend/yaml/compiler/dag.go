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
	"slices"
	"sort"

	backend_types "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// stepRole marks the steps the compiler adds around the work steps.
type stepRole uint8

const (
	roleDefault stepRole = iota
	// The cache restore step runs after clone; every work step depends on it.
	roleCacheRestore
	// The cache save steps run after every work step.
	roleCacheSave
)

type dagCompilerStep struct {
	step                    *backend_types.Step
	position                int
	name                    string
	dependsOn               []string
	hasExplicitDependencies bool
	role                    stepRole
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
		c.setCacheSaveDependencies()
		return c.compileByDependsOn()
	}
	c.setDependenciesForSequence()
	c.setCacheSaveDependencies()
	return c.compileSequence()
}

// isWorkStep reports whether a step comes from the workflow's `steps`.
func (s *dagCompilerStep) isWorkStep() bool {
	return s.position >= 0 && s.role == roleDefault
}

// setCacheSaveDependencies makes the cache save steps wait for every work
// step, so they run last and are skipped when a work step failed.
func (c dagCompiler) setCacheSaveDependencies() {
	var workStepNames []string
	for _, step := range c.steps {
		if step.isWorkStep() {
			workStepNames = append(workStepNames, step.name)
		}
	}
	for _, step := range c.steps {
		if step.role == roleCacheSave {
			step.dependsOn = workStepNames
			step.step.DependsOn = workStepNames
		}
	}
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
		if step.isWorkStep() && (!step.hasExplicitDependencies || len(step.dependsOn) == 0) && len(cloneStepNames) > 0 {
			step.dependsOn = cloneStepNames
			step.step.DependsOn = cloneStepNames
		}
	}

	c.setCacheRestoreDependencies()
}

// setCacheRestoreDependencies makes every work step that does not wait on
// another work step also wait for the cache restore step. A step with an
// explicit `depends_on: [clone]` would otherwise read the workspace while the
// caches are still being extracted into it. Steps further down the graph wait
// for the restore through the steps they depend on.
func (c dagCompiler) setCacheRestoreDependencies() {
	restoreName := ""
	workStepNames := make(map[string]struct{}, len(c.steps))
	for _, step := range c.steps {
		if step.role == roleCacheRestore {
			restoreName = step.name
		}
		if step.isWorkStep() {
			workStepNames[step.name] = struct{}{}
		}
	}
	if restoreName == "" {
		return
	}

	for _, step := range c.steps {
		if !step.isWorkStep() || slices.Contains(step.dependsOn, restoreName) {
			continue
		}
		waitsOnWorkStep := slices.ContainsFunc(step.dependsOn, func(dep string) bool {
			_, ok := workStepNames[dep]
			return ok
		})
		if waitsOnWorkStep {
			continue
		}
		step.dependsOn = append(slices.Clone(step.dependsOn), restoreName)
		step.step.DependsOn = step.dependsOn
	}
}

// resolveFilteredDependencies replaces each dependency on a step that its
// `when` filter removed from the workflow with the dependencies of that step,
// so a dependent keeps its place in the graph instead of failing on an unknown
// step. The filtered map holds the `depends_on` of each removed step. A name
// that is neither present nor filtered is kept, so a typo still fails the
// compilation with ErrStepMissingDependency.
func resolveFilteredDependencies(steps []*dagCompilerStep, filtered map[string][]string) {
	if len(filtered) == 0 {
		return
	}
	present := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		present[step.name] = struct{}{}
	}

	for _, step := range steps {
		if len(step.dependsOn) == 0 {
			continue
		}
		resolved := make([]string, 0, len(step.dependsOn))
		// seen also stops the walk on a cycle among filtered steps.
		seen := make(map[string]struct{}, len(step.dependsOn))
		var resolve func(dep string)
		resolve = func(dep string) {
			if _, ok := seen[dep]; ok {
				return
			}
			seen[dep] = struct{}{}
			if _, ok := present[dep]; !ok {
				if deps, ok := filtered[dep]; ok {
					for _, d := range deps {
						resolve(d)
					}
					return
				}
			}
			resolved = append(resolved, dep)
		}
		for _, dep := range step.dependsOn {
			resolve(dep)
		}
		step.dependsOn = resolved
		step.step.DependsOn = resolved
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
		// The cache restore step already depends on clone; work steps start after it.
		if step.role == roleCacheRestore {
			previousStepName = step.name
		}
	}

	for _, step := range c.steps {
		if step.isWorkStep() && !step.hasExplicitDependencies {
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

// dfsColor tracks the DFS state of a step during cycle detection.
type dfsColor uint8

const (
	// White (the zero value) marks a step that has not been visited yet.
	dfsWhite dfsColor = iota
	// Grey marks a step on the current DFS path; reaching it again is a cycle.
	dfsGrey
	// Black marks a fully explored step that is known to be cycle-free.
	dfsBlack
)

// dfsVisit walks the dependencies of the named step and reports a cycle when
// it reaches a step that is still on the current path. Fully explored steps are
// marked black and skipped on later visits, so sharing one color map across all
// start nodes keeps the whole check linear in steps plus dependency edges.
func dfsVisit(steps map[string]*dagCompilerStep, name string, colors map[string]dfsColor, path []string) error {
	switch colors[name] {
	case dfsGrey:
		return &ErrStepDependencyCycle{path: path}
	case dfsBlack:
		return nil
	case dfsWhite:
	}

	// Guard against an unknown dependency name. convertDAGToStages validates
	// dependencies before calling dfsVisit, but this keeps the traversal safe
	// against a nil map entry (which would otherwise panic) and returns the
	// proper error instead.
	step, ok := steps[name]
	if !ok {
		parent := name
		if len(path) > 0 {
			parent = path[len(path)-1]
		}
		return &ErrStepMissingDependency{name: parent, dep: name}
	}

	colors[name] = dfsGrey
	path = append(path, name)

	for _, dep := range step.dependsOn {
		if err := dfsVisit(steps, dep, colors, path); err != nil {
			return err
		}
	}

	colors[name] = dfsBlack

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
	}

	// check if there are cycles, sharing the colors so each step is explored once
	colors := make(map[string]dfsColor, len(steps))
	for name := range steps {
		if err := dfsVisit(steps, name, colors, []string{}); err != nil {
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
