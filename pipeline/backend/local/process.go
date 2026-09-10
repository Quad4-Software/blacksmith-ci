// Copyright 2026 Crow Authors
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
	"errors"
	"slices"
)

func cleanupWorkflowProcesses(state *workflowState) error {
	state.stepCMDsMu.RLock()
	cleanups := slices.Clone(state.processCleanups)
	state.stepCMDsMu.RUnlock()
	var errs []error
	// Stop process groups before scanning for detached descendants, and attempt
	// every cleanup even if another step could not be stopped.
	for _, cleanup := range slices.Backward(cleanups) {
		if err := cleanup(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
