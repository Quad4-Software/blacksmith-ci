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

package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// capturePublisher hands the stream's receiver to the test, so the test can
// deliver messages synchronously and in order.
type capturePublisher struct {
	receivers chan pubsub.Receiver
}

func (p *capturePublisher) Publish(pubsub.Message) {}

func (p *capturePublisher) Subscribe(ctx context.Context, receiver pubsub.Receiver) {
	p.receivers <- receiver
	<-ctx.Done()
}

// streamedRepos opens /stream/events as user (nil for anonymous), delivers one
// event per repo and returns the repos whose events reached the client.
func streamedRepos(t *testing.T, user *model.User, permitted []*model.Repo, repos []*model.Repo) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	publisher := &capturePublisher{receivers: make(chan pubsub.Receiver, 1)}
	previous := server.Config.Services.Pubsub
	server.Config.Services.Pubsub = publisher
	t.Cleanup(func() { server.Config.Services.Pubsub = previous })

	_store := store_mocks.NewMockStore(t)
	if user != nil {
		_store.On("RepoList", user, false, true).Return(permitted, nil)
	}

	router := gin.New()
	router.GET("/stream/events", func(c *gin.Context) {
		c.Set("store", _store)
		if user != nil {
			c.Set("user", user)
		}
		EventStreamSSE(c)
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream/events", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var receive pubsub.Receiver
	select {
	case receive = <-publisher.receivers:
	case <-ctx.Done():
		t.Fatal("stream never subscribed")
	}

	// A trailing public event marks the end of the delivered events.
	end := &model.Repo{FullName: "end/marker", Visibility: model.VisibilityPublic}
	for _, repo := range append(repos, end) {
		receive(pubsub.Message{Labels: pubsub.RepoLabels(repo), Data: []byte(repo.FullName)})
	}

	var got []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		if data == end.FullName {
			return got
		}
		got = append(got, data)
	}
	t.Fatalf("stream ended before the end marker: %v", scanner.Err())
	return nil
}

func TestEventStreamSSEHonorsCrowVisibility(t *testing.T) {
	// All three repos are public on the forge; only their Crow visibility differs.
	public := &model.Repo{FullName: "octo/public", Visibility: model.VisibilityPublic}
	internal := &model.Repo{FullName: "octo/internal", Visibility: model.VisibilityInternal}
	private := &model.Repo{FullName: "octo/private", Visibility: model.VisibilityPrivate}
	repos := []*model.Repo{public, internal, private}

	t.Run("anonymous", func(t *testing.T) {
		got := streamedRepos(t, nil, nil, repos)
		assert.Equal(t, []string{"octo/public"}, got)
	})

	t.Run("logged in without repo permission", func(t *testing.T) {
		got := streamedRepos(t, &model.User{ID: 1}, nil, repos)
		assert.Equal(t, []string{"octo/public", "octo/internal"}, got)
	})

	t.Run("logged in with repo permission", func(t *testing.T) {
		got := streamedRepos(t, &model.User{ID: 2}, []*model.Repo{internal, private}, repos)
		assert.Equal(t, []string{"octo/public", "octo/internal", "octo/private"}, got)
	})
}
