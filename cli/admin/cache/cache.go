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

package cache

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v3"

	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

// Command exports the admin build cache command set.
var Command = &cli.Command{
	Name:  "cache",
	Usage: "manage the build caches and limits of users and organizations",
	Commands: []*cli.Command{
		cacheCleanupCmd,
		cacheListCmd,
		cachePurgeCmd,
		cacheSetCmd,
		cacheShowCmd,
	},
}

// Values of the limit flags besides an explicit size or duration.
const (
	limitDefault   = "default"
	limitUnlimited = "unlimited"
	limitNone      = "none"
)

const (
	day  = 24 * time.Hour
	week = 7 * day
)

var errMissingOrg = errors.New("missing required argument org-id / org-name / user-login")

// parseOrg resolves an org id, org name or user login to an org id. A user's
// caches belong to their personal org, which is named after their login.
func parseOrg(client crow.Client, orgIDOrName string) (int64, error) {
	if orgIDOrName == "" {
		return 0, errMissingOrg
	}
	if orgID, err := strconv.ParseInt(orgIDOrName, 10, 64); err == nil {
		return orgID, nil
	}
	org, err := client.OrgLookup(orgIDOrName)
	if err != nil {
		return 0, fmt.Errorf("could not find organization or user %q: %w", orgIDOrName, err)
	}
	return org.ID, nil
}

// parseSizeLimit parses a size limit flag: "default", "unlimited" or a size
// such as 10GiB or 500MB.
func parseSizeLimit(value string) (int64, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case limitDefault:
		return 0, nil
	case limitUnlimited, limitNone:
		return -1, nil
	}
	size, err := humanize.ParseBytes(value)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: use a size such as 10GiB, %q or %q", value, limitDefault, limitUnlimited)
	}
	if size == 0 || size > math.MaxInt64 {
		return 0, fmt.Errorf("invalid size %q: must be positive; use %q for the server default", value, limitDefault)
	}
	return int64(size), nil
}

// parseTTL parses a retention flag: "default", "unlimited" or a duration
// such as 7d, 2w or 36h.
func parseTTL(value string) (int64, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case limitDefault:
		return 0, nil
	case limitUnlimited, limitNone:
		return -1, nil
	}
	duration, err := parseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid retention %q: use a duration such as 7d, 2w or 36h, %q or %q", value, limitDefault, limitUnlimited)
	}
	if duration < time.Second {
		return 0, fmt.Errorf("invalid retention %q: must be at least one second; use %q for the server default", value, limitDefault)
	}
	return int64(duration / time.Second), nil
}

// parseDuration extends time.ParseDuration by whole days (d) and weeks (w).
func parseDuration(value string) (time.Duration, error) {
	for suffix, unit := range map[string]time.Duration{"d": day, "w": week} {
		if n, ok := strings.CutSuffix(value, suffix); ok {
			count, err := strconv.ParseInt(n, 10, 64)
			if err != nil || count < 0 || count > math.MaxInt64/int64(unit) {
				return 0, fmt.Errorf("invalid duration %q", value)
			}
			return time.Duration(count) * unit, nil
		}
	}
	return time.ParseDuration(value)
}

// formatBytes renders a byte count.
func formatBytes(size int64) string {
	return humanize.IBytes(uint64(max(size, 0)))
}

// formatLimit renders a size limit in effect, 0 being unlimited, and notes
// whether it is the server default.
func formatLimit(limit, setting int64) string {
	value := limitUnlimited
	if limit > 0 {
		value = formatBytes(limit)
	}
	return withOrigin(value, setting)
}

// formatRetention renders a retention in effect in seconds, 0 keeping entries
// until evicted by size, and notes whether it is the server default.
func formatRetention(ttl, setting int64) string {
	value := limitUnlimited
	if ttl > 0 {
		value = formatSeconds(ttl)
	}
	return withOrigin(value, setting)
}

func withOrigin(value string, setting int64) string {
	if setting == 0 {
		return value + " (default)"
	}
	return value
}

// formatSeconds renders seconds as whole weeks, days or hours where possible.
func formatSeconds(seconds int64) string {
	const (
		secondsPerHour = int64(time.Hour / time.Second)
		secondsPerDay  = int64(day / time.Second)
	)
	switch {
	case seconds%secondsPerDay == 0:
		return fmt.Sprintf("%dd", seconds/secondsPerDay)
	case seconds%secondsPerHour == 0:
		return fmt.Sprintf("%dh", seconds/secondsPerHour)
	default:
		return (time.Duration(seconds) * time.Second).String()
	}
}

// formatHitRate renders the share of restores that restored a cache, exact or
// by restore key, as the web UI does.
func formatHitRate(usage crow.BuildCacheUsage) string {
	restores := usage.Hits + usage.PartialHits + usage.Misses + usage.Denied
	if restores == 0 {
		return "-"
	}
	const percent = 100
	return fmt.Sprintf("%d%%", int64(math.Round(float64(usage.Hits+usage.PartialHits)/float64(restores)*percent)))
}

func ownerType(org *crow.BuildCacheOrgSummary) string {
	if org.IsUser {
		return "user"
	}
	return "org"
}
