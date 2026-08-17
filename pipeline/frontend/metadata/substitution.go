// Copyright 2023 Woodpecker Authors
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

package metadata

import (
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/envsubst"
)

func EnvVarSubst(yamlText string, environ map[string]string) (string, error) {
	// envsubst scans the whole string for ${...} and does not understand YAML, so
	// a ${...} inside a comment line is wrongly parsed as a substitution and can
	// fail (e.g. `# Avoids "${ARRAY[@]}"` -> "missing closing brace"). Skip comment
	// lines (first non-whitespace char is '#'): both top-level YAML comments and
	// shell comments inside a `commands: |` block are never meant to be
	// interpolated, and this makes envsubst effectively YAML-comment-aware.
	lines := strings.Split(yamlText, "\n")
	var b strings.Builder
	b.Grow(len(yamlText))
	for idx, ln := range lines {
		if idx > 0 {
			b.WriteByte('\n')
		}
		if isCommentLine(ln) {
			b.WriteString(ln)
			continue
		}
		out, err := envsubst.Eval(ln, func(name string) (string, bool) {
			env, ok := environ[name]
			if ok && strings.Contains(env, "\n") {
				env = fmt.Sprintf("%q", env)
			}
			return env, true
		})
		if err != nil {
			// envsubst reports no position, so add the line number: without it the
			// user only sees e.g. "missing closing brace" for a whole config.
			// Escaping a literal `$` as `$$` is the usual fix.
			return "", fmt.Errorf("variable substitution failed on line %d: %w (escape a literal '$' as '$$')", idx+1, err)
		}
		b.WriteString(out)
	}
	return b.String(), nil
}

// isCommentLine reports whether a line is a comment: its first non-whitespace
// character is '#'.
//
// This is a text heuristic, not YAML parsing, so it also matches lines that YAML
// treats as string data, e.g. heredoc content written by a `commands:` block or a
// continuation line of a folded quoted scalar. Such lines are not interpolated
// either; use `$$` to write a literal '$' anywhere substitution is unwanted.
func isCommentLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "#")
}
