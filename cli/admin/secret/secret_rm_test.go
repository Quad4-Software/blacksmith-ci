// Copyright 2025 Crow Authors
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

package secret

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestSecretDeleteMissingName(t *testing.T) {
	command := &cli.Command{
		Name:   "rm",
		Writer: io.Discard,
		Flags:  secretDeleteCmd.Flags,
		Action: secretDelete,
	}

	// Running without the --name flag must fail before any API call.
	err := command.Run(t.Context(), []string{"rm"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing secret name")
}
