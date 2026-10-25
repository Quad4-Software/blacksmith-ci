package crow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientAdminCaches(t *testing.T) {
	var gotLimits BuildCacheOrgLimits
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/caches", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"enabled":true,"overview":{"defaults":{"max_size":100},"orgs":[{"org_id":5,"name":"octocat","is_user":true,"settings":{"max_size":-1}}]}}`))
	})
	mux.HandleFunc("GET /api/v1/admin/caches/orgs/5", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"org_id":5,"name":"octocat","limits":{"ttl":60},"repo_list":[{"repo_id":1,"full_name":"octocat/hello"}]}`))
	})
	mux.HandleFunc("PUT /api/v1/admin/caches/orgs/5", func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotLimits))
		_, _ = w.Write([]byte(`{"org_id":5,"settings":{"max_size":10,"max_egress":-1,"ttl":0}}`))
	})
	mux.HandleFunc("DELETE /api/v1/admin/caches/orgs/5", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"removed":3,"freed":42}`))
	})
	mux.HandleFunc("POST /api/v1/admin/caches/cleanup", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"removed":1,"freed":7}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := NewClient(ts.URL, http.DefaultClient)

	caches, err := client.AdminCacheList()
	assert.NoError(t, err)
	assert.True(t, caches.Enabled)
	assert.Equal(t, int64(100), caches.Overview.Defaults.MaxSize)
	assert.Equal(t, int64(-1), caches.Overview.Orgs[0].Settings.MaxSize)

	org, err := client.AdminCacheOrg(5)
	assert.NoError(t, err)
	assert.Equal(t, "octocat", org.Name)
	assert.Equal(t, int64(60), org.Limits.TTL)
	assert.Equal(t, "octocat/hello", org.RepoList[0].FullName)

	org, err = client.AdminCacheOrgUpdate(5, &BuildCacheOrgLimits{MaxSize: 10, MaxEgress: -1})
	assert.NoError(t, err)
	assert.Equal(t, BuildCacheOrgLimits{MaxSize: 10, MaxEgress: -1}, gotLimits)
	assert.Equal(t, BuildCacheOrgLimits{MaxSize: 10, MaxEgress: -1}, org.Settings)

	purged, err := client.AdminCacheOrgPurge(5)
	assert.NoError(t, err)
	assert.Equal(t, &BuildCacheCleanup{Removed: 3, Freed: 42}, purged)

	cleaned, err := client.AdminCacheCleanup()
	assert.NoError(t, err)
	assert.Equal(t, &BuildCacheCleanup{Removed: 1, Freed: 7}, cleaned)
}
