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

package grpc

import (
	"github.com/prometheus/client_golang/prometheus"
	prometheus_auto "github.com/prometheus/client_golang/prometheus/promauto"
)

// GRPCMetrics holds all Prometheus metrics for the gRPC layer.
type GRPCMetrics struct {
	PipelineCount    *prometheus.CounterVec
	PipelineTime     *prometheus.GaugeVec
	PipelineDuration *prometheus.HistogramVec

	AgentRegistrations   prometheus.Counter
	AgentUnregistrations prometheus.Counter
}

func NewGRPCMetrics() *GRPCMetrics {
	return &GRPCMetrics{
		PipelineTime: prometheus_auto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "crow",
			Name:      "pipeline_time",
			Help:      "Pipeline time.",
		}, []string{"repo", "branch", "status", "pipeline"}),
		PipelineCount: prometheus_auto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "crow",
			Name:      "pipeline_count",
			Help:      "Pipeline count.",
		}, []string{"repo", "branch", "status", "pipeline"}),
		PipelineDuration: prometheus_auto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "crow",
			Name:      "pipeline_duration_seconds",
			Help:      "Pipeline execution duration in seconds.",
			Buckets:   []float64{30, 60, 120, 300, 600, 1200, 1800, 3600},
		}, []string{"repo", "branch", "status"}),
		AgentRegistrations: prometheus_auto.NewCounter(prometheus.CounterOpts{
			Namespace: "crow",
			Name:      "agent_registrations_total",
			Help:      "Total number of agent registrations.",
		}),
		AgentUnregistrations: prometheus_auto.NewCounter(prometheus.CounterOpts{
			Namespace: "crow",
			Name:      "agent_unregistrations_total",
			Help:      "Total number of agent unregistrations.",
		}),
	}
}
