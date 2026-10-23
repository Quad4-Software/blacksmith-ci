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

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/services/buildcache"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// Admin and org endpoints managing build caches: the overview of all orgs,
// per-org limits, cleanup and buckets orgs connect.

// cacheService returns the build cache service, or a 404 when caching is off.
func cacheService() (*buildcache.Service, error) {
	service := server.Config.Services.BuildCache
	if service == nil {
		return nil, huma.Error404NotFound("build caching is not enabled on this server")
	}
	return service, nil
}

// cacheManageError maps service errors of the management endpoints.
func cacheManageError(err error) error {
	switch {
	case errors.Is(err, buildcache.ErrNotFound), errors.Is(err, types.RecordNotExist):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, buildcache.ErrBucketsDisabled):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, buildcache.ErrInvalidBucket), errors.Is(err, buildcache.ErrInvalidSettings):
		return huma.Error422UnprocessableEntity(err.Error())
	default:
		log.Error().Err(err).Msg("build cache management request failed")
		return huma.Error500InternalServerError(err.Error())
	}
}

type GetAdminCachesInput struct{}

// AdminCaches is the build cache of the whole server.
type AdminCaches struct {
	Enabled  bool                 `json:"enabled" doc:"whether build caching is enabled on this server"`
	Overview *buildcache.Overview `json:"overview,omitempty"`
} //	@name	AdminCaches

type GetAdminCachesOutput struct {
	Body AdminCaches
}

func GetAdminCaches(_ context.Context, _ *GetAdminCachesInput) (*GetAdminCachesOutput, error) {
	out := &GetAdminCachesOutput{}
	service := server.Config.Services.BuildCache
	if service == nil {
		return out, nil
	}
	overview, err := service.Overview()
	if err != nil {
		return nil, cacheManageError(err)
	}
	out.Body = AdminCaches{Enabled: true, Overview: overview}
	return out, nil
}

type GetAdminCacheOrgInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
}

type CacheOrgOutput struct {
	Body buildcache.OrgDetail
}

func GetAdminCacheOrg(_ context.Context, input *GetAdminCacheOrgInput) (*CacheOrgOutput, error) {
	return cacheOrg(input.OrgID)
}

func cacheOrg(orgID int64) (*CacheOrgOutput, error) {
	service, err := cacheService()
	if err != nil {
		return nil, err
	}
	detail, err := service.Org(orgID)
	if err != nil {
		return nil, cacheManageError(err)
	}
	return &CacheOrgOutput{Body: *detail}, nil
}

// CacheOrgSettingsInput are the limits of an org's caches in the server's
// storage. 0 applies the server default, -1 lifts the limit.
type CacheOrgSettingsInput struct {
	MaxSize   int64 `json:"max_size"   doc:"maximum size of the org's caches in bytes; 0 is the server default, -1 unlimited"`
	MaxEgress int64 `json:"max_egress" doc:"maximum bytes restored per calendar month (UTC); 0 is the server default, -1 unlimited"`
	TTL       int64 `json:"ttl"        doc:"seconds an entry is kept without being restored; 0 is the server default, -1 keeps entries until evicted by size"`
}

type PutAdminCacheOrgInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
	Body  CacheOrgSettingsInput
}

func PutAdminCacheOrg(ctx context.Context, input *PutAdminCacheOrgInput) (*CacheOrgOutput, error) {
	service, err := cacheService()
	if err != nil {
		return nil, err
	}
	if _, err := server.Config.Services.Manager.Store().OrgGet(input.OrgID); err != nil {
		return nil, cacheManageError(err)
	}
	err = service.SetOrgSettings(ctx, &model.BuildCacheSettings{
		OrgID:     input.OrgID,
		MaxSize:   input.Body.MaxSize,
		MaxEgress: input.Body.MaxEgress,
		TTL:       input.Body.TTL,
	})
	if err != nil {
		return nil, cacheManageError(err)
	}
	return cacheOrg(input.OrgID)
}

type DeleteCacheOrgInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
}

type CacheCleanupOutput struct {
	Body buildcache.CleanupResult
}

func DeleteAdminCacheOrg(ctx context.Context, input *DeleteCacheOrgInput) (*CacheCleanupOutput, error) {
	return purgeCacheOrg(ctx, input.OrgID)
}

func purgeCacheOrg(ctx context.Context, orgID int64) (*CacheCleanupOutput, error) {
	service, err := cacheService()
	if err != nil {
		return nil, err
	}
	result, err := service.PurgeOrg(ctx, orgID)
	if err != nil {
		return nil, cacheManageError(err)
	}
	return &CacheCleanupOutput{Body: result}, nil
}

type PostAdminCacheCleanupInput struct{}

func PostAdminCacheCleanup(ctx context.Context, _ *PostAdminCacheCleanupInput) (*CacheCleanupOutput, error) {
	service, err := cacheService()
	if err != nil {
		return nil, err
	}
	result, err := service.Cleanup(ctx)
	if err != nil {
		return nil, cacheManageError(err)
	}
	return &CacheCleanupOutput{Body: result}, nil
}

// Org endpoints, for the admins of an org.

type GetOrgCachesInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
}

// OrgCaches is an org's cache as its admins see it.
type OrgCaches struct {
	Enabled      bool                  `json:"enabled"       doc:"whether build caching is enabled on this server"`
	AllowBuckets bool                  `json:"allow_buckets" doc:"whether the org may connect a bucket of its own"`
	Defaults     buildcache.Limits     `json:"defaults"      doc:"the server defaults, applied where the org has no limits or retention of its own"`
	Detail       *buildcache.OrgDetail `json:"detail,omitempty"`
} //	@name	OrgCaches

type GetOrgCachesOutput struct {
	Body OrgCaches
}

func GetOrgCaches(ctx context.Context, _ *GetOrgCachesInput) (*GetOrgCachesOutput, error) {
	org := session.Org(ginCtx(ctx))
	out := &GetOrgCachesOutput{}
	service := server.Config.Services.BuildCache
	if service == nil {
		return out, nil
	}
	detail, err := service.Org(org.ID)
	if err != nil {
		return nil, cacheManageError(err)
	}
	out.Body = OrgCaches{Enabled: true, AllowBuckets: service.BucketsAllowed(), Defaults: service.DefaultLimits(), Detail: detail}
	return out, nil
}

func DeleteOrgCaches(ctx context.Context, _ *DeleteCacheOrgInput) (*CacheCleanupOutput, error) {
	return purgeCacheOrg(ctx, session.Org(ginCtx(ctx)).ID)
}

type PutOrgCacheBucketInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
	Body  buildcache.BucketInput
}

type PutOrgCacheBucketOutput struct {
	Body *model.BuildCacheBucket
}

func PutOrgCacheBucket(ctx context.Context, input *PutOrgCacheBucketInput) (*PutOrgCacheBucketOutput, error) {
	service, err := cacheService()
	if err != nil {
		return nil, err
	}
	bucket, err := service.ConnectBucket(ctx, session.Org(ginCtx(ctx)).ID, input.Body)
	if err != nil {
		return nil, cacheManageError(err)
	}
	return &PutOrgCacheBucketOutput{Body: bucket}, nil
}

func DeleteOrgCacheBucket(ctx context.Context, _ *DeleteCacheOrgInput) (*struct{}, error) {
	service, err := cacheService()
	if err != nil {
		return nil, err
	}
	if err := service.DisconnectBucket(ctx, session.Org(ginCtx(ctx)).ID); err != nil {
		return nil, cacheManageError(err)
	}
	return nil, nil
}
