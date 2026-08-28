// Copyright 2025 Crow Authors
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

package local

import (
	"os"
	"strings"
)

// agentEnvPrefixes name the environment variables that configure the agent
// itself rather than a pipeline.
//
// Steps inherit the agent process's environment, because a host toolchain that
// cannot see PATH or the caller's locale is the whole point of this backend.
// That environment also holds the agent's own configuration, and with it
// CROW_AGENT_SECRET: the server accepts that token from anybody, registers the
// caller as a system agent and then hands it tasks belonging to every
// repository on the instance, secrets included. A step that can read it owns
// the instance, so it never reaches one.
var agentEnvPrefixes = []string{"CROW_", "WOODPECKER_"}

// agentSecretFileVars name the environment variables the agent reads its token
// from a file with.
var agentSecretFileVars = []string{"CROW_AGENT_SECRET_FILE", "WOODPECKER_AGENT_SECRET_FILE"}

// stepEnviron returns the agent process environment without the agent's own
// configuration. The variables the server sends for a step are applied on top
// of this by the caller and are unaffected.
func stepEnviron() []string {
	hostEnv := os.Environ()
	env := make([]string, 0, len(hostEnv))
	for _, entry := range hostEnv {
		if isAgentEnv(entry) {
			continue
		}
		env = append(env, entry)
	}
	return env
}

// isAgentEnv reports whether a "key=value" entry configures the agent.
func isAgentEnv(entry string) bool {
	name, _, found := strings.Cut(entry, "=")
	if !found {
		name = entry
	}
	for _, prefix := range agentEnvPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// agentSecretFiles returns the paths the agent may have read its token from.
//
// Keeping the variables out of a step's environment hides where the token is,
// not the token: the standard sandbox profile allows reading every file it does
// not explicitly deny, so the paths are denied there by name.
func agentSecretFiles() []string {
	var paths []string
	for _, name := range agentSecretFileVars {
		if path := os.Getenv(name); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
