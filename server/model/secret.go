// Copyright 2021 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrSecretNameInvalid  = errors.New("invalid secret name")
	ErrSecretImageInvalid = errors.New("invalid secret image")
	ErrSecretValueInvalid = errors.New("invalid secret value")
	ErrSecretEventInvalid = errors.New("invalid secret event")
)

// SecretStore persists secret information to storage.
type SecretStore interface {
	SecretFind(*Repo, string) (*Secret, error)
	SecretList(*Repo, bool, *ListOptions) ([]*Secret, error)
	SecretCreate(*Secret) error
	SecretUpdate(*Secret) error
	SecretDelete(*Secret) error
	OrgSecretFind(int64, string) (*Secret, error)
	OrgSecretList(int64, *ListOptions) ([]*Secret, error)
	GlobalSecretFind(string) (*Secret, error)
	GlobalSecretList(*ListOptions) ([]*Secret, error)
	SecretListAll() ([]*Secret, error)
}

// Secret represents a secret variable, such as a password or token.
type Secret struct {
	ID        int64          `json:"id"              xorm:"pk autoincr 'id'"`
	OrgID     int64          `json:"org_id"          xorm:"NOT NULL DEFAULT 0 UNIQUE(s) INDEX 'org_id'"`
	RepoID    int64          `json:"repo_id"         xorm:"NOT NULL DEFAULT 0 UNIQUE(s) INDEX 'repo_id'"`
	Name      string         `json:"name"            xorm:"NOT NULL UNIQUE(s) INDEX 'name'"`
	Value     string         `json:"value,omitempty" xorm:"TEXT 'value'"`
	Images    []string       `json:"images"          xorm:"json 'images'"`
	Events    []WebhookEvent `json:"events"          xorm:"json 'events'"`
	Source    string         `json:"source"          xorm:"varchar(50) DEFAULT 'internal' 'source'"`
	CreatedAt int64          `json:"created_at"      xorm:"created"`
	UpdatedAt int64          `json:"updated_at"      xorm:"updated"`
} //	@name	Secret

// TableName return database table name for xorm.
func (Secret) TableName() string {
	return "secrets"
}

// BeforeInsert will sort events and set defaults before inserted into database.
func (s *Secret) BeforeInsert() {
	s.Events = sortEvents(s.Events)
	if s.Source == "" {
		s.Source = "internal"
	}
	s.Value = normalizePEMValue(s.Value)
}

// BeforeUpdate normalizes PEM values before updating in the database.
func (s *Secret) BeforeUpdate() {
	s.Value = normalizePEMValue(s.Value)
}

// normalizePEMValue ensures PEM-formatted values end with a trailing newline.
// PEM consumers (e.g. OpenSSH) require the final newline after the end marker.
// This is commonly lost when copy-pasting keys into a web form.
func normalizePEMValue(value string) string {
	trimmed := strings.TrimRight(value, " \t\r\n")
	if strings.Contains(trimmed, "-----BEGIN ") && strings.HasSuffix(trimmed, "-----") {
		return trimmed + "\n"
	}
	return value
}

// Global secret.
func (s Secret) IsGlobal() bool {
	return s.RepoID == 0 && s.OrgID == 0
}

// Organization secret.
func (s Secret) IsOrganization() bool {
	return s.RepoID == 0 && s.OrgID != 0
}

// Repository secret.
func (s Secret) IsRepository() bool {
	return s.RepoID != 0 && s.OrgID == 0
}

var validDockerImageString = regexp.MustCompile(
	`^(` +
		`[\w\d\-_\.]+` + // hostname
		`(:\d+)?` + // optional port
		`/)?` + // optional hostname + port
		`([\w\d\-_\.][\w\d\-_\.\/]*/)?` + // optional url prefix
		`([\w\d\-_]+)` + // image name
		`(:[\w\d\-_]+)?` + // optional image tag
		`$`,
)

// Validate validates the required fields and formats.
func (s *Secret) Validate() error {
	for _, event := range s.Events {
		if err := event.Validate(); err != nil {
			return errors.Join(err, ErrSecretEventInvalid)
		}
	}
	if len(s.Events) == 0 {
		return fmt.Errorf("%w: no event specified", ErrSecretEventInvalid)
	}

	for _, image := range s.Images {
		if len(image) == 0 {
			return fmt.Errorf("%w: empty image in images", ErrSecretImageInvalid)
		}
		if !validDockerImageString.MatchString(image) {
			return fmt.Errorf("%w: image '%s' do not match regexp '%s'", ErrSecretImageInvalid, image, validDockerImageString.String())
		}
	}

	switch {
	case len(s.Name) == 0:
		return fmt.Errorf("%w: empty name", ErrSecretNameInvalid)
	case len(s.Value) == 0:
		return fmt.Errorf("%w: empty value", ErrSecretValueInvalid)
	default:
		return nil
	}
}

// Copy makes a copy of the secret without the value.
func (s *Secret) Copy() *Secret {
	return &Secret{
		ID:     s.ID,
		OrgID:  s.OrgID,
		RepoID: s.RepoID,
		Name:   s.Name,
		Images: s.Images,
		Events: sortEvents(s.Events),
		Source: s.Source,
	}
}

// CopyWithAbbreviatedValue makes a copy of the secret with an abbreviated value.
// The abbreviated value shows the first 6 and last 6 characters for verification.
func (s *Secret) CopyWithAbbreviatedValue() *Secret {
	return &Secret{
		ID:        s.ID,
		OrgID:     s.OrgID,
		RepoID:    s.RepoID,
		Name:      s.Name,
		Value:     abbreviateValue(s.Value),
		Images:    s.Images,
		Events:    sortEvents(s.Events),
		Source:    s.Source,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// Constants for abbreviated value display.
const (
	abbreviatedPrefixLen = 6  // Number of characters to show at start
	abbreviatedSuffixLen = 6  // Number of characters to show at end
	abbreviatedMinLen    = 12 // Minimum length before showing full abbreviation
	abbreviatedShortLen  = 3  // Minimum length before showing any chars
)

// abbreviateValue returns an abbreviated version of the secret value.
// Shows first 6 and last 6 characters with "..." in between.
func abbreviateValue(value string) string {
	runes := []rune(value)
	length := len(runes)

	if length <= abbreviatedMinLen {
		if length <= abbreviatedShortLen {
			return "***"
		}
		// Show first 2 chars for short values
		return string(runes[:2]) + "..."
	}

	// Normal case: first 6 + ... + last 6
	prefix := string(runes[:abbreviatedPrefixLen])
	suffix := string(runes[length-abbreviatedSuffixLen:])
	return prefix + "..." + suffix
}

func sortEvents(wel WebhookEventList) WebhookEventList {
	sort.Sort(wel)
	return wel
}

// SecretWithOrg extends Secret with organization and repository context for user views.
type SecretWithOrg struct {
	*Secret
	OrgName   string `json:"org_name"`
	RepoName  string `json:"repo_name,omitempty"`
	ScopeType string `json:"scope_type"`           // "user", "org", "repo", or "global"
	ForgeID   int64  `json:"forge_id,omitempty"`   // forge ID for multi-forge display
	ForgeName string `json:"forge_name,omitempty"` // forge hostname for multi-forge display
	ForgeIcon string `json:"forge_icon,omitempty"` // forge icon name/URL for multi-forge display
	ForgeType string `json:"forge_type,omitempty"` // forge type (github, gitlab, etc.) for icon fallback
} //	@name	SecretWithOrg

// WithOrg creates a SecretWithOrg from a Secret with the given org name.
func (s *Secret) WithOrg(orgName string, isPersonalOrg bool, forgeID int64, forgeName, forgeIcon, forgeType string) *SecretWithOrg {
	scopeType := "org"
	if s.IsGlobal() {
		scopeType = "global"
	} else if isPersonalOrg {
		scopeType = "user"
	}
	return &SecretWithOrg{
		Secret:    s,
		OrgName:   orgName,
		ScopeType: scopeType,
		ForgeID:   forgeID,
		ForgeName: forgeName,
		ForgeIcon: forgeIcon,
		ForgeType: forgeType,
	}
}

// WithRepo creates a SecretWithOrg from a Secret with the given repo name.
func (s *Secret) WithRepo(repoName string, forgeID int64, forgeName, forgeIcon, forgeType string) *SecretWithOrg {
	return &SecretWithOrg{
		Secret:    s,
		RepoName:  repoName,
		ScopeType: "repo",
		ForgeID:   forgeID,
		ForgeName: forgeName,
		ForgeIcon: forgeIcon,
		ForgeType: forgeType,
	}
}
