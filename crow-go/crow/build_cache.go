package crow

import (
	"fmt"
	"net/http"
)

const (
	pathAdminCaches       = "%s/api/v1/admin/caches"
	pathAdminCacheCleanup = "%s/api/v1/admin/caches/cleanup"
	pathAdminCacheOrg     = "%s/api/v1/admin/caches/orgs/%d"
)

// AdminCacheList returns the build caches of all users and orgs.
func (c *client) AdminCacheList() (*AdminCaches, error) {
	out := new(AdminCaches)
	uri := fmt.Sprintf(pathAdminCaches, c.addr)
	return out, c.get(uri, out)
}

// AdminCacheOrg returns the build caches and limits of a user or org.
func (c *client) AdminCacheOrg(orgID int64) (*BuildCacheOrg, error) {
	out := new(BuildCacheOrg)
	uri := fmt.Sprintf(pathAdminCacheOrg, c.addr, orgID)
	return out, c.get(uri, out)
}

// AdminCacheOrgUpdate sets the build cache limits of a user or org.
func (c *client) AdminCacheOrgUpdate(orgID int64, limits *BuildCacheOrgLimits) (*BuildCacheOrg, error) {
	out := new(BuildCacheOrg)
	uri := fmt.Sprintf(pathAdminCacheOrg, c.addr, orgID)
	return out, c.put(uri, limits, out)
}

// AdminCacheOrgPurge deletes all build caches of a user or org.
func (c *client) AdminCacheOrgPurge(orgID int64) (*BuildCacheCleanup, error) {
	out := new(BuildCacheCleanup)
	uri := fmt.Sprintf(pathAdminCacheOrg, c.addr, orgID)
	return out, c.do(uri, http.MethodDelete, nil, out)
}

// AdminCacheCleanup removes expired build caches and enforces the size limits
// at once.
func (c *client) AdminCacheCleanup() (*BuildCacheCleanup, error) {
	out := new(BuildCacheCleanup)
	uri := fmt.Sprintf(pathAdminCacheCleanup, c.addr)
	return out, c.post(uri, nil, out)
}
