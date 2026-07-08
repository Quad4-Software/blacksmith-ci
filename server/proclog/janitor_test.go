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

package proclog

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	storeMocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestJanitorRunsOnceAndTrimsAllSources(t *testing.T) {
	storeMock := storeMocks.NewMockStore(t)
	storeMock.On("ProcLogListSources", mock.Anything).Return([]model.ProcLogSourceKey{
		{SourceType: SourceTypeAgent, SourceID: 1},
		{SourceType: SourceTypeAutoscaler, SourceID: 9},
	}, nil).Once()
	storeMock.On("ProcLogTrim", mock.Anything, SourceTypeAgent, int64(1), 100).Return(nil).Once()
	storeMock.On("ProcLogTrim", mock.Anything, SourceTypeAutoscaler, int64(9), 100).Return(nil).Once()

	j := NewJanitor(storeMock, JanitorOpts{KeepPerSource: 100, Interval: time.Hour})
	require.NoError(t, j.RunOnce(context.Background()))
}
