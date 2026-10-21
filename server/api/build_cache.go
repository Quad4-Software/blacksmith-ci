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
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/services/buildcache"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// The cache step endpoints below are plain HTTP rather than huma operations:
// they authenticate with a workflow's cache token instead of a user, and the
// blob endpoints stream archives of arbitrary size.

// CacheRestoreRequest asks for the archive of a key.
type CacheRestoreRequest struct {
	Key         string   `json:"key"`
	RestoreKeys []string `json:"restore_keys,omitempty"`
}

// CacheRestoreResponse is a restored entry and where to download it.
type CacheRestoreResponse struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Scope string `json:"scope"`
	Size  int64  `json:"size"`
	Exact bool   `json:"exact"`
	URL   string `json:"url"`
}

// CacheReserveRequest announces an archive about to be uploaded.
type CacheReserveRequest struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
	// Multipart is set by cache steps that can upload in parts.
	Multipart bool `json:"multipart,omitempty"`
}

// CacheReserveResponse is where to upload a reserved archive.
type CacheReserveResponse struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
	// UploadID, PartSize and PartURLs replace URL for an upload in parts:
	// part i holds the bytes from i*PartSize and is PUT to PartURLs[i].
	UploadID string   `json:"upload_id,omitempty"`
	PartSize int64    `json:"part_size,omitempty"`
	PartURLs []string `json:"part_urls,omitempty"`
}

// CacheCommitRequest finishes an upload.
type CacheCommitRequest struct {
	ID int64 `json:"id"`
	// UploadID is the upload in parts to assemble, if any.
	UploadID string `json:"upload_id,omitempty"`
}

// cacheAccess authorizes a cache step request, writing the error response and
// returning false when it is refused.
func cacheAccess(c *gin.Context) (*buildcache.Service, buildcache.Access, bool) {
	service := server.Config.Services.BuildCache
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build caching is not enabled on this server"})
		return nil, buildcache.Access{}, false
	}
	raw, err := token.ExtractToken(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": buildcache.ErrUnauthorized.Error()})
		return nil, buildcache.Access{}, false
	}
	access, err := service.Authorize(raw)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return nil, buildcache.Access{}, false
	}
	return service, access, true
}

// cacheError maps service errors to HTTP status codes the cache step reacts to.
func cacheError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, buildcache.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, buildcache.ErrExists):
		status = http.StatusConflict
	case errors.Is(err, buildcache.ErrReadOnly):
		status = http.StatusForbidden
	case errors.Is(err, buildcache.ErrQuotaExceeded):
		status = http.StatusInsufficientStorage
	case errors.Is(err, buildcache.ErrInvalidKey):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, buildcache.ErrObjectTooLarge):
		status = http.StatusRequestEntityTooLarge
	default:
		log.Error().Err(err).Msg("build cache request failed")
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

// PostCacheRestore finds the archive for a key or one of its restore keys.
func PostCacheRestore(c *gin.Context) {
	service, access, ok := cacheAccess(c)
	if !ok {
		return
	}
	var req CacheRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hit, err := service.Restore(c, access, req.Key, req.RestoreKeys)
	if err != nil {
		cacheError(c, err)
		return
	}
	c.JSON(http.StatusOK, CacheRestoreResponse{
		ID:    hit.Entry.ID,
		Key:   hit.Entry.Key,
		Scope: hit.Entry.Scope,
		Size:  hit.Entry.Size,
		Exact: hit.Exact,
		URL:   hit.URL,
	})
}

// PostCacheReserve reserves room for an archive and returns its upload URL.
func PostCacheReserve(c *gin.Context) {
	service, access, ok := cacheAccess(c)
	if !ok {
		return
	}
	var req CacheReserveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	reservation, err := service.Reserve(c, access, req.Key, req.Size)
	if err != nil {
		cacheError(c, err)
		return
	}
	if req.Multipart {
		if err := service.UploadInParts(c, reservation); err != nil {
			// The single upload URL still works.
			log.Warn().Err(err).Int64("id", reservation.Entry.ID).Msg("could not start a build cache upload in parts")
		}
	}
	c.JSON(http.StatusCreated, CacheReserveResponse{
		ID:       reservation.Entry.ID,
		URL:      reservation.URL,
		UploadID: reservation.UploadID,
		PartSize: reservation.PartSize,
		PartURLs: reservation.PartURLs,
	})
}

// PostCacheCommit makes an uploaded archive restorable.
func PostCacheCommit(c *gin.Context) {
	service, access, ok := cacheAccess(c)
	if !ok {
		return
	}
	var req CacheCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry, err := service.Commit(c, access, req.ID, req.UploadID)
	if err != nil {
		cacheError(c, err)
		return
	}
	c.JSON(http.StatusOK, entry)
}

// GetCacheBlob streams an archive kept in server-side storage.
func GetCacheBlob(c *gin.Context) {
	service, access, ok := cacheAccess(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cache id"})
		return
	}
	r, size, err := service.OpenBlob(c, access, id)
	if err != nil {
		cacheError(c, err)
		return
	}
	defer r.Close()
	c.DataFromReader(http.StatusOK, size, "application/zstd", r, nil)
}

// PutCacheBlob receives an archive for server-side storage.
func PutCacheBlob(c *gin.Context) {
	service, access, ok := cacheAccess(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cache id"})
		return
	}
	if err := service.WriteBlob(c, access, id, c.Request.Body); err != nil {
		cacheError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// purgeBuildCaches removes the caches of a deleted repo. A failure only leaves
// archives behind that the cleanup expires later, so it is logged, not returned.
func purgeBuildCaches(ctx context.Context, repo *model.Repo) {
	service := server.Config.Services.BuildCache
	if service == nil {
		return
	}
	if err := service.Purge(ctx, repo.ID); err != nil {
		log.Error().Err(err).Str("repo", repo.FullName).Msg("could not remove build caches of deleted repo")
	}
}

// Repo-facing endpoints for listing and removing caches.

type GetRepoCachesInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

// RepoCaches is the build cache of a repository.
type RepoCaches struct {
	Enabled bool                `json:"enabled" doc:"whether build caching is enabled on this server"`
	Size    int64               `json:"size" doc:"total size of the repository's caches in bytes"`
	Entries []*model.BuildCache `json:"entries"`
} //	@name	RepoCaches

type GetRepoCachesOutput struct {
	Body RepoCaches
}

func GetRepoCaches(ctx context.Context, _ *GetRepoCachesInput) (*GetRepoCachesOutput, error) {
	repo := session.Repo(ginCtx(ctx))
	out := &GetRepoCachesOutput{Body: RepoCaches{Entries: []*model.BuildCache{}}}
	service := server.Config.Services.BuildCache
	if service == nil {
		return out, nil
	}
	entries, err := service.List(repo.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	out.Body.Enabled = true
	out.Body.Entries = entries
	for _, entry := range entries {
		out.Body.Size += entry.Size
	}
	return out, nil
}

type DeleteRepoCacheInput struct {
	RepoID  int64 `path:"repo_id" doc:"the repository id"`
	CacheID int64 `path:"cache_id" doc:"the cache entry id"`
}

func DeleteRepoCache(ctx context.Context, input *DeleteRepoCacheInput) (*struct{}, error) {
	repo := session.Repo(ginCtx(ctx))
	service := server.Config.Services.BuildCache
	if service == nil {
		return nil, huma.Error404NotFound("build caching is not enabled on this server")
	}
	if err := service.Delete(ctx, repo.ID, input.CacheID); err != nil {
		if errors.Is(err, buildcache.ErrNotFound) {
			return nil, huma.Error404NotFound("cache entry not found")
		}
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

type DeleteRepoCachesInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

func DeleteRepoCaches(ctx context.Context, _ *DeleteRepoCachesInput) (*struct{}, error) {
	repo := session.Repo(ginCtx(ctx))
	service := server.Config.Services.BuildCache
	if service == nil {
		return nil, huma.Error404NotFound("build caching is not enabled on this server")
	}
	if err := service.Purge(ctx, repo.ID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}
