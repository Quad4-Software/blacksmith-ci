// Copyright 2026 Crow Authors
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

package yaml

import (
	"slices"
	"strings"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
)

// ManualVariable is a resolved, user-settable variable surfaced to the manual
// trigger UI. Field tags carry both the API JSON shape and (for declared
// variables) the YAML shape.
type ManualVariable struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required,omitempty"`
}

// isBuiltinIdent reports whether an identifier refers to built-in CI metadata
// or the manual "task" selector rather than a user-settable variable.
func isBuiltinIdent(name string) bool {
	return name == "CI" || strings.HasPrefix(name, "CI_") || name == "task"
}

// extractEvaluateVariables parses a single expr-lang expression and returns the
// user-settable identifier names referenced (in first-seen order, built-ins
// filtered) and, per name, any string literals inferred from "==" / "in"
// comparisons. Invalid expressions return empty results.
func extractEvaluateVariables(expression string) (names []string, options map[string][]string) {
	options = map[string][]string{}
	tree, err := parser.Parse(expression)
	if err != nil {
		return nil, options
	}

	seen := map[string]bool{}
	addName := func(name string) {
		if isBuiltinIdent(name) || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	addOption := func(name, literal string) {
		if isBuiltinIdent(name) {
			return
		}
		if !slices.Contains(options[name], literal) {
			options[name] = append(options[name], literal)
		}
	}

	ast.Walk(&tree.Node, walkFunc(func(node *ast.Node) {
		switch n := (*node).(type) {
		case *ast.IdentifierNode:
			addName(n.Value)
		case *ast.BinaryNode:
			switch n.Operator {
			case "==":
				if id, lit, ok := identAndString(n.Left, n.Right); ok {
					addOption(id, lit)
				}
			case "in":
				if id, ok := n.Left.(*ast.IdentifierNode); ok {
					if arr, ok := n.Right.(*ast.ArrayNode); ok {
						for _, el := range arr.Nodes {
							if s, ok := el.(*ast.StringNode); ok {
								addOption(id.Value, s.Value)
							}
						}
					}
				}
			}
		}
	}))

	return names, options
}

// identAndString matches "<ident> == <string>" in either argument order.
func identAndString(a, b ast.Node) (ident string, literal string, ok bool) {
	if id, isID := a.(*ast.IdentifierNode); isID {
		if s, isStr := b.(*ast.StringNode); isStr {
			return id.Value, s.Value, true
		}
	}
	if id, isID := b.(*ast.IdentifierNode); isID {
		if s, isStr := a.(*ast.StringNode); isStr {
			return id.Value, s.Value, true
		}
	}
	return "", "", false
}

// walkFunc adapts a function to the ast.Visitor interface.
type walkFunc func(*ast.Node)

func (f walkFunc) Visit(node *ast.Node) { f(node) }

// ExtractManualVariables parses a single workflow YAML document and returns the
// user-settable variables, merging the explicit "variables:" declaration block
// (which wins) with names/options auto-extracted from every workflow- and
// step-level "when.evaluate" expression. The result is sorted by name.
func ExtractManualVariables(data []byte) ([]ManualVariable, error) {
	workflow, err := ParseBytes(data)
	if err != nil {
		return nil, err
	}
	declared, err := ParseVariableDefs(data)
	if err != nil {
		return nil, err
	}

	// Collect evaluate expressions: workflow-level + step-level.
	var expressions []string
	for _, c := range workflow.When.Constraints {
		if c.Evaluate != "" {
			expressions = append(expressions, c.Evaluate)
		}
	}
	for _, step := range workflow.Steps.ContainerList {
		for _, c := range step.When.Constraints {
			if c.Evaluate != "" {
				expressions = append(expressions, c.Evaluate)
			}
		}
	}

	extractedOptions := map[string][]string{}
	extractedNames := map[string]bool{}
	for _, expr := range expressions {
		names, opts := extractEvaluateVariables(expr)
		for _, name := range names {
			extractedNames[name] = true
		}
		for name, lits := range opts {
			for _, lit := range lits {
				if !slices.Contains(extractedOptions[name], lit) {
					extractedOptions[name] = append(extractedOptions[name], lit)
				}
			}
		}
	}

	result := make([]ManualVariable, 0, len(declared)+len(extractedNames))
	for name, def := range declared {
		result = append(result, ManualVariable{
			Name:        name,
			Description: def.Description,
			Options:     def.Options,
			Default:     def.Default,
			Required:    def.Required,
		})
	}
	for name := range extractedNames {
		if _, isDeclared := declared[name]; isDeclared {
			continue
		}
		result = append(result, ManualVariable{Name: name, Options: extractedOptions[name]})
	}

	slices.SortFunc(result, func(a, b ManualVariable) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}
