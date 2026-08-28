package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
)

func TestCancelQueueTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		running    []*model.Task
		queueError error
		wantStatus int
	}{
		{
			name:       "cancels running task",
			running:    []*model.Task{{ID: "42"}},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "rejects unknown task",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "reports queue error",
			running:    []*model.Task{{ID: "42"}},
			queueError: errors.New("cancel failed"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue_mocks.NewMockQueue(t)
			q.On("Info", mock.Anything).Return(queue.InfoT{Running: tt.running})
			if len(tt.running) > 0 {
				q.On("Error", mock.Anything, "42", queue.ErrCancel).Return(tt.queueError)
			}

			originalQueue := server.Config.Services.Queue
			server.Config.Services.Queue = q
			t.Cleanup(func() { server.Config.Services.Queue = originalQueue })

			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Params = gin.Params{{Key: "task_id", Value: "42"}}

			api.CancelQueueTask(context)
			context.Writer.WriteHeaderNow()

			require.Equal(t, tt.wantStatus, response.Code)
		})
	}
}
