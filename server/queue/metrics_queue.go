// Copyright 2025 Crow authors
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

package queue

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	prometheus_auto "github.com/prometheus/client_golang/prometheus/promauto"

	"codefloe.com/crowci/crow/v5/server/model"
)

var queueWaitTime = prometheus_auto.NewHistogram(prometheus.HistogramOpts{
	Namespace: "crow",
	Name:      "queue_wait_seconds",
	Help:      "Time from task push to agent pickup in seconds.",
	Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
})

// WithMetrics wraps a Queue with Prometheus instrumentation for queue wait time.
func WithMetrics(q Queue) Queue {
	return &metricsQueue{Queue: q, waitTime: queueWaitTime}
}

type metricsQueue struct {
	Queue
	waitTime   prometheus.Observer
	enqueuedAt sync.Map // task ID -> time.Time
}

func (q *metricsQueue) Push(c context.Context, task *model.Task) error {
	if err := q.Queue.Push(c, task); err != nil {
		return err
	}
	q.enqueuedAt.Store(task.ID, time.Now())
	return nil
}

func (q *metricsQueue) PushAtOnce(c context.Context, tasks []*model.Task) error {
	if err := q.Queue.PushAtOnce(c, tasks); err != nil {
		return err
	}
	now := time.Now()
	for _, task := range tasks {
		q.enqueuedAt.Store(task.ID, now)
	}
	return nil
}

func (q *metricsQueue) Poll(c context.Context, agentID int64, f FilterFn) (*model.Task, error) {
	task, err := q.Queue.Poll(c, agentID, f)
	if task != nil {
		if enqueued, ok := q.enqueuedAt.LoadAndDelete(task.ID); ok {
			if t, ok := enqueued.(time.Time); ok {
				q.waitTime.Observe(time.Since(t).Seconds())
			}
		}
	}
	return task, err
}

func (q *metricsQueue) Done(c context.Context, id string, exitStatus model.StatusValue) error {
	q.enqueuedAt.Delete(id)
	return q.Queue.Done(c, id, exitStatus)
}

func (q *metricsQueue) Error(c context.Context, id string, err error) error {
	q.enqueuedAt.Delete(id)
	return q.Queue.Error(c, id, err)
}

func (q *metricsQueue) ErrorAtOnce(c context.Context, ids []string, err error) error {
	for _, id := range ids {
		q.enqueuedAt.Delete(id)
	}
	return q.Queue.ErrorAtOnce(c, ids, err)
}

func (q *metricsQueue) Evict(c context.Context, id string) error {
	q.enqueuedAt.Delete(id)
	return q.Queue.Evict(c, id)
}

func (q *metricsQueue) EvictAtOnce(c context.Context, ids []string) error {
	for _, id := range ids {
		q.enqueuedAt.Delete(id)
	}
	return q.Queue.EvictAtOnce(c, ids)
}

func (q *metricsQueue) Reclaim(c context.Context, task *model.Task) error {
	q.enqueuedAt.Delete(task.ID)
	return q.Queue.Reclaim(c, task)
}
