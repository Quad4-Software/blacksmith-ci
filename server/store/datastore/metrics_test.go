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

package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

// TestGetBuildTimeStatsByRepo verifies SUM(finished - started) is decoded
// correctly. Regression test for codefloe.com/crowci/crow#967 where large
// int64 sums could not be scanned into a float64 struct field.
func TestGetBuildTimeStatsByRepo(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo), new(model.Pipeline), new(model.Org))
	defer closer()

	org := &model.Org{Name: "acme", ForgeID: 1}
	assert.NoError(t, store.OrgCreate(org))

	repo := &model.Repo{
		UserID:   1,
		OrgID:    org.ID,
		FullName: "acme/test",
		Owner:    "acme",
		Name:     "test",
	}
	assert.NoError(t, store.CreateRepo(repo))

	// Use a large duration to reproduce the int64 SUM overflow path.
	const bigDuration int64 = 5_239_595
	pipelines := []*model.Pipeline{
		{RepoID: repo.ID, Status: model.StatusSuccess, Started: 1_000_000, Finished: 1_000_000 + bigDuration},
		{RepoID: repo.ID, Status: model.StatusSuccess, Started: 2_000_000, Finished: 2_000_000 + bigDuration},
	}
	for _, p := range pipelines {
		assert.NoError(t, store.CreatePipeline(p))
	}

	stats, err := store.GetBuildTimeStatsByRepo(0, 0, nil)
	assert.NoError(t, err)
	assert.Contains(t, stats, repo.ID)
	assert.InDelta(t, float64(bigDuration), stats[repo.ID]["avg"], 0.001)
	assert.InDelta(t, float64(bigDuration*2), stats[repo.ID]["total"], 0.001)
}

func TestGetBuildTimeStatsByOrg(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo), new(model.Pipeline), new(model.Org))
	defer closer()

	org := &model.Org{Name: "acme", ForgeID: 1}
	assert.NoError(t, store.OrgCreate(org))

	repo := &model.Repo{
		UserID:   1,
		OrgID:    org.ID,
		FullName: "acme/test",
		Owner:    "acme",
		Name:     "test",
	}
	assert.NoError(t, store.CreateRepo(repo))

	const bigDuration int64 = 5_239_595
	p := &model.Pipeline{RepoID: repo.ID, Status: model.StatusSuccess, Started: 1_000_000, Finished: 1_000_000 + bigDuration}
	assert.NoError(t, store.CreatePipeline(p))

	stats, err := store.GetBuildTimeStatsByOrg(0, 0, nil)
	assert.NoError(t, err)
	assert.Contains(t, stats, org.ID)
	assert.InDelta(t, float64(bigDuration), stats[org.ID]["total"], 0.001)
}

func TestGetMetricSeries(t *testing.T) {
	store, closer := newTestStore(t,
		new(model.Repo), new(model.Org), new(model.Pipeline), new(model.Workflow), new(model.Step))
	defer closer()

	org := &model.Org{Name: "acme", ForgeID: 1}
	assert.NoError(t, store.OrgCreate(org))
	repo := &model.Repo{UserID: 1, OrgID: org.ID, FullName: "acme/test", Owner: "acme", Name: "test"}
	assert.NoError(t, store.CreateRepo(repo))

	// Two finished pipelines a month apart.
	p1 := &model.Pipeline{RepoID: repo.ID, Status: model.StatusSuccess, Started: 1_700_000_000, Finished: 1_700_000_060}
	assert.NoError(t, store.CreatePipeline(p1))
	p2 := &model.Pipeline{RepoID: repo.ID, Status: model.StatusSuccess, Started: 1_702_700_000, Finished: 1_702_700_120}
	assert.NoError(t, store.CreatePipeline(p2))

	assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{
		{PipelineID: p1.ID, PID: 1, Name: "wf", AgentPersistentID: "pid-1", AgentName: "artemis", Started: 1_700_000_000, Finished: 1_700_000_060},
		{PipelineID: p2.ID, PID: 1, Name: "wf", AgentPersistentID: "pid-1", AgentName: "artemis", Started: 1_702_700_000, Finished: 1_702_700_120},
	}))

	// repo dimension / pipeline_count
	pts, err := store.GetMetricSeries(model.MetricQueryOptions{Dimension: model.MetricDimensionRepo, Metric: model.MetricPipelineCount})
	assert.NoError(t, err)
	assert.Len(t, pts, 1)
	assert.Equal(t, "acme/test", pts[0].Label)
	assert.Equal(t, float64(2), pts[0].Value)

	// org dimension / total_build_time
	pts, err = store.GetMetricSeries(model.MetricQueryOptions{Dimension: model.MetricDimensionOrg, Metric: model.MetricTotalBuildTime})
	assert.NoError(t, err)
	assert.Len(t, pts, 1)
	assert.Equal(t, "acme", pts[0].Label)
	assert.InDelta(t, float64(180), pts[0].Value, 0.001)

	// agent dimension / workflow_count
	pts, err = store.GetMetricSeries(model.MetricQueryOptions{Dimension: model.MetricDimensionAgent, Metric: model.MetricWorkflowCount})
	assert.NoError(t, err)
	assert.Len(t, pts, 1)
	assert.Equal(t, "artemis", pts[0].Label)
	assert.Equal(t, "pid-1", pts[0].Key)
	assert.Equal(t, float64(2), pts[0].Value)

	// time dimension / workflow_count bucketed by month -> two distinct buckets, ordered.
	// (workflow_count buckets by workflows.started, which the test controls; the
	// pipeline metrics bucket by pipelines.created, set to now by CreatePipeline.)
	pts, err = store.GetMetricSeries(model.MetricQueryOptions{Dimension: model.MetricDimensionTime, Metric: model.MetricWorkflowCount, Bucket: model.MetricBucketMonth})
	assert.NoError(t, err)
	assert.Len(t, pts, 2)
	assert.Equal(t, "2023-11", pts[0].Label)
	assert.Equal(t, "2023-12", pts[1].Label)
	assert.Equal(t, float64(1), pts[0].Value)
	assert.Equal(t, float64(1), pts[1].Value)

	// repo filter that matches nothing yields an empty series
	pts, err = store.GetMetricSeries(model.MetricQueryOptions{Dimension: model.MetricDimensionRepo, Metric: model.MetricPipelineCount, RepoIDs: []int64{999}})
	assert.NoError(t, err)
	assert.Empty(t, pts)
}
