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

package model

import "github.com/uptrace/bun"

// BuildCache is one archive of workspace paths saved by a workflow's cache
// step under a key. Entries are immutable: a key is written once per scope and
// replaced only after it was evicted.
type BuildCache struct {
	bun.BaseModel `bun:"table:build_caches"`

	ID     int64 `json:"id"         bun:"id,pk,autoincrement"`
	RepoID int64 `json:"repo_id"    bun:"repo_id,notnull,unique:build_cache_key"`

	// Scope is the git ref the entry was saved from, e.g. "refs/heads/main".
	// A workflow reads its own scope and the default branch, and writes only
	// its own, so one branch cannot plant a cache another branch restores.
	Scope string `json:"scope"      bun:"scope,type:varchar(255),notnull,unique:build_cache_key"`

	// Key is the user-defined cache key after template rendering.
	Key string `json:"key"        bun:"cache_key,type:varchar(255),notnull,unique:build_cache_key"`

	// Size is the archive size in bytes: the reserved size while the upload is
	// pending, the stored size once committed.
	Size int64 `json:"size"       bun:"size,notnull,default:0"`

	// BucketID is the bucket the archive is stored in: 0 for the server's
	// storage, otherwise the BuildCacheBucket its org connected.
	BucketID int64 `json:"bucket_id"  bun:"bucket_id,notnull,default:0"`

	// Committed is false while the archive is being uploaded. Pending entries
	// are never restored and are removed when their upload is abandoned.
	Committed bool  `json:"committed"  bun:"committed,notnull,default:false"`
	Created   int64 `json:"created"    bun:"created,notnull,default:0"`
	LastUsed  int64 `json:"last_used"  bun:"last_used,notnull,default:0"`
} //	@name	BuildCache

// TableName returns the database table name.
func (BuildCache) TableName() string {
	return "build_caches"
}

// Bucket selectors of BuildCacheFilter.BucketID besides a bucket's ID.
const (
	// BuildCacheGlobalStorage selects entries kept in the server's storage.
	BuildCacheGlobalStorage int64 = 0
	// BuildCacheAnyStorage selects entries regardless of where they are kept.
	BuildCacheAnyStorage int64 = -1
	// BuildCacheOwnBuckets selects entries kept in any bucket an org connected.
	BuildCacheOwnBuckets int64 = -2
)

// BuildCacheFilter narrows the cache entries a store query covers. Zero values
// do not filter, except BucketID, whose zero selects the server's storage.
type BuildCacheFilter struct {
	RepoID int64
	// OrgID selects the entries of the repos currently owned by this org.
	OrgID int64
	// BucketID is a bucket ID or one of the BuildCache*Storage selectors.
	BucketID int64
	// NotOrgIDs and NotBucketIDs exclude entries of these orgs and buckets.
	NotOrgIDs    []int64
	NotBucketIDs []int64
}

// BuildCacheSize is the stored size of a group of cache entries.
type BuildCacheSize struct {
	OrgID  int64 `json:"org_id"  bun:"org_id"`
	RepoID int64 `json:"repo_id" bun:"repo_id"`
	// Own is set for entries in a bucket the org connected.
	Own     bool  `json:"own"     bun:"own"`
	Size    int64 `json:"size"    bun:"size"`
	Entries int64 `json:"entries" bun:"entries"`
	Repos   int64 `json:"repos"   bun:"repos"`
} //	@name	BuildCacheSize

// Limit values of BuildCacheSettings. Positive values are explicit limits.
const (
	// BuildCacheLimitDefault applies the server-wide default.
	BuildCacheLimitDefault int64 = 0
	// BuildCacheLimitNone lifts the limit; for a TTL it keeps entries until
	// they are evicted by size.
	BuildCacheLimitNone int64 = -1
)

// BuildCacheSettings are the cache limits an admin set for one org. They
// govern only entries kept in the server's storage; a bucket the org
// connected is limited by its owner.
type BuildCacheSettings struct {
	bun.BaseModel `bun:"table:build_cache_settings"`

	OrgID int64 `json:"org_id"     bun:"org_id,pk"`
	// MaxSize caps the size of the org's caches in bytes.
	MaxSize int64 `json:"max_size"   bun:"max_size,notnull,default:0"`
	// MaxEgress caps the bytes restored per calendar month (UTC).
	MaxEgress int64 `json:"max_egress" bun:"max_egress,notnull,default:0"`
	// TTL removes entries not restored for this many seconds.
	TTL     int64 `json:"ttl"        bun:"ttl,notnull,default:0"`
	Updated int64 `json:"updated"    bun:"updated,notnull,default:0"`
} //	@name	BuildCacheSettings

// TableName returns the database table name.
func (BuildCacheSettings) TableName() string {
	return "build_cache_settings"
}

// BuildCacheBucket is an S3-compatible bucket an org connected to keep its
// caches in instead of the server's storage. The cache step up- and downloads
// through presigned URLs, so the bucket must be reachable from pipeline steps.
type BuildCacheBucket struct {
	bun.BaseModel `bun:"table:build_cache_buckets"`

	ID        int64  `json:"id"         bun:"id,pk,autoincrement"`
	OrgID     int64  `json:"org_id"     bun:"org_id,notnull,unique"`
	Endpoint  string `json:"endpoint"   bun:"endpoint,type:varchar(255),notnull"`
	Bucket    string `json:"bucket"     bun:"bucket,type:varchar(255),notnull"`
	Region    string `json:"region"     bun:"region,type:varchar(255)"`
	Prefix    string `json:"prefix"     bun:"prefix,type:varchar(255)"`
	AccessKey string `json:"access_key" bun:"access_key,type:varchar(255)"`
	// SecretKey is never sent to clients.
	SecretKey string `json:"-"          bun:"secret_key,type:text"`
	// SSL has no bun default: bun would write the default in place of false.
	SSL       bool `json:"ssl"        bun:"use_ssl,notnull"`
	PathStyle bool `json:"path_style" bun:"path_style,notnull"`
	// MaxSize caps the size of the caches in the bucket and MaxEgress the
	// bytes restored from it per calendar month (UTC); 0 is unlimited. The
	// org sets them, the server's limits do not apply to its bucket.
	MaxSize   int64 `json:"max_size"   bun:"max_size,notnull"`
	MaxEgress int64 `json:"max_egress" bun:"max_egress,notnull"`
	// TTL removes entries not restored for this many seconds; 0 applies the
	// server default, -1 keeps them until removed by hand.
	TTL     int64 `json:"ttl"        bun:"ttl,notnull,default:0"`
	Created int64 `json:"created"    bun:"created,notnull,default:0"`
	Updated int64 `json:"updated"    bun:"updated,notnull,default:0"`
} //	@name	BuildCacheBucket

// TableName returns the database table name.
func (BuildCacheBucket) TableName() string {
	return "build_cache_buckets"
}

// BuildCacheUsage counts the cache traffic of one repo and storage on one day.
type BuildCacheUsage struct {
	bun.BaseModel `bun:"table:build_cache_usage"`

	ID     int64 `json:"-"            bun:"id,pk,autoincrement"`
	RepoID int64 `json:"repo_id"      bun:"repo_id,notnull,unique:build_cache_usage_day"`
	// OrgID is the org that owned the repo when the traffic happened.
	OrgID    int64 `json:"org_id"       bun:"org_id,notnull"`
	BucketID int64 `json:"bucket_id"    bun:"bucket_id,notnull,default:0,unique:build_cache_usage_day"`
	// Day is the start of the UTC day as a unix timestamp.
	Day int64 `json:"day"          bun:"day,notnull,unique:build_cache_usage_day"`

	BuildCacheUsageCounts
} //	@name	BuildCacheUsage

// TableName returns the database table name.
func (BuildCacheUsage) TableName() string {
	return "build_cache_usage"
}

// BuildCacheUsageCounts are the counters of BuildCacheUsage.
type BuildCacheUsageCounts struct {
	// Hits restored the requested key, PartialHits one of its restore keys.
	Hits        int64 `json:"hits"         bun:"hits,notnull,default:0"`
	PartialHits int64 `json:"partial_hits" bun:"partial_hits,notnull,default:0"`
	Misses      int64 `json:"misses"       bun:"misses,notnull,default:0"`
	// Denied restores found an entry but were refused by the egress limit.
	Denied int64 `json:"denied"       bun:"denied,notnull,default:0"`
	Saves  int64 `json:"saves"        bun:"saves,notnull,default:0"`
	// Egress is the bytes handed out to restore, Ingress the bytes saved.
	Egress  int64 `json:"egress"       bun:"egress,notnull,default:0"`
	Ingress int64 `json:"ingress"      bun:"ingress,notnull,default:0"`
}

// Add adds o to c.
func (c *BuildCacheUsageCounts) Add(o BuildCacheUsageCounts) {
	c.Hits += o.Hits
	c.PartialHits += o.PartialHits
	c.Misses += o.Misses
	c.Denied += o.Denied
	c.Saves += o.Saves
	c.Egress += o.Egress
	c.Ingress += o.Ingress
}

// BuildCacheUsageFilter narrows the usage a store query sums.
type BuildCacheUsageFilter struct {
	OrgID  int64
	RepoID int64
	// BucketID is a bucket ID or one of the BuildCache*Storage selectors.
	BucketID int64
	// Since is the first day counted, as a unix timestamp.
	Since int64
}

// BuildCacheUsageGroup is the usage summed per org, repo or day, depending on
// the query; the other keys are zero.
type BuildCacheUsageGroup struct {
	OrgID  int64 `json:"org_id,omitempty"  bun:"org_id"`
	RepoID int64 `json:"repo_id,omitempty" bun:"repo_id"`
	Day    int64 `json:"day,omitempty"     bun:"day"`
	// Own is set for the usage of a bucket the org connected.
	Own bool `json:"own"               bun:"own"`

	BuildCacheUsageCounts
} //	@name	BuildCacheUsageGroup
