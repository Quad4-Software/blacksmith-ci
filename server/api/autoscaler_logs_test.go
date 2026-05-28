// Copyright 2021 Woodpecker Authors
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

package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/proclog"
	storeMocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

func TestPostAutoscalerLogs(t *testing.T) {
	storeMock := storeMocks.NewMockStore(t)
	var captured []*model.ProcLogEntry
	storeMock.On("ProcLogInsert", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			entries, ok := args.Get(1).([]*model.ProcLogEntry)
			require.True(t, ok)
			captured = append(captured, entries...)
		}).Return(nil)

	r := proclog.NewReceiver(storeMock)
	body := strings.Join([]string{
		`{"ts":1700000000000000000,"level":"info","msg":"line a"}`,
		`{"ts":1700000000100000000,"level":"warn","msg":"line b"}`,
	}, "\n")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequestWithContext(
		context.Background(), http.MethodPost, "/api/v1/autoscalers/3/logs",
		bytes.NewBufferString(body),
	)
	c.Params = gin.Params{{Key: "autoscaler_id", Value: "3"}}
	// Simulate auth middleware populating the autoscaler in context.
	c.Set("autoscaler", &model.Autoscaler{ID: 3})

	PostAutoscalerLogs(r)(c)
	c.Writer.WriteHeaderNow()

	require.Equal(t, http.StatusNoContent, w.Code)
	require.Len(t, captured, 2)
	require.Equal(t, "line a", captured[0].Line)
	require.Equal(t, model.ProcLogSourceTypeAutoscaler, captured[0].SourceType)
	require.Equal(t, int64(3), captured[0].SourceID)
}
