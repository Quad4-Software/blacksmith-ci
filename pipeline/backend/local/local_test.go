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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

func TestSetupWorkflowRejectsTempVolumes(t *testing.T) {
	err := New().SetupWorkflow(context.Background(), &types.Config{
		TempVolumes: []string{"crow_tmp_dind"},
	}, "task-uuid")
	assert.ErrorContains(t, err, "does not support `temp_volumes`")
}
