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

// Package ldap implements sign-in against an LDAP directory with
// search-then-bind: a service account (or an anonymous search) finds the user
// entry, a bind as that entry checks the password, and the entry's groups drive
// access checks. It does not depend on the server package.
package ldap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	goldap "github.com/go-ldap/ldap/v3"

	"codefloe.com/crowci/crow/v6/server/auth/groupmap"
)

// ProviderName is stored as user_identities.provider for LDAP identities.
const ProviderName = "ldap"

const (
	defaultUserFilter        = "(uid={username})"
	defaultUsernameAttribute = "uid"
	defaultEmailAttribute    = "mail"
	defaultIDAttribute       = "entryUUID"
	defaultGroupAttribute    = "memberOf"
	defaultDisplayName       = "LDAP"

	// The URL is the issuer, which is part of every stored identity key, so it
	// must fit the 255-character issuer column.
	maxURLLen = 255

	timeout = 10 * time.Second
	// A size limit of two is enough to tell a unique match from an ambiguous one.
	userSizeLimit = 2
	// The attribute list "1.1" asks the directory to return only the DN (RFC 4511 4.5.1.8).
	noAttributes = "1.1"
)

var (
	// ErrInvalidCredentials is returned for a wrong username or password, an
	// unknown user and an ambiguous username.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrDirectory wraps every other failure: network, TLS, the service bind,
	// searches and missing attributes.
	ErrDirectory = errors.New("LDAP directory error")
)

// Config configures the LDAP provider.
type Config struct {
	URL          string
	StartTLS     bool
	SkipVerify   bool
	CACertFile   string
	BindDN       string
	BindPassword string
	UserBaseDN   string
	// UserFilter finds the user entry; {username} is replaced by the escaped
	// username. Defaults to "(uid={username})".
	UserFilter        string
	UsernameAttribute string // default "uid"
	EmailAttribute    string // default "mail"
	IDAttribute       string // default "entryUUID"
	GroupAttribute    string // default "memberOf"
	GroupBaseDN       string // defaults to UserBaseDN
	// GroupSearchFilter finds the user's groups, e.g. "(member={dn})"; {dn} and
	// {username} are replaced by the escaped user DN and username. Empty reads
	// GroupAttribute from the user entry instead.
	GroupSearchFilter string
	DisplayName       string // default "LDAP"
	AllowedGroups     []string
	AdminGroups       []string
	// RoleGroups maps an instance role name to the groups that grant it.
	RoleGroups map[string][]string
}

// Enabled reports whether LDAP sign-in is configured.
func (c Config) Enabled() bool { return c.URL != "" }

// Validate reports configuration errors that must stop the server from starting.
func (c Config) Validate() error {
	if c.URL == "" {
		return nil
	}
	if c.UserBaseDN == "" {
		return errors.New("CROW_LDAP_URL requires CROW_LDAP_USER_BASE_DN")
	}
	scheme := urlScheme(c.URL)
	if scheme != "ldap" && scheme != "ldaps" {
		return errors.New("CROW_LDAP_URL must start with ldap:// or ldaps://")
	}
	if c.StartTLS && scheme == "ldaps" {
		return errors.New("CROW_LDAP_START_TLS cannot be used with an ldaps:// URL")
	}
	if c.UserFilter != "" && !strings.Contains(c.UserFilter, "{username}") {
		return errors.New("CROW_LDAP_USER_FILTER must contain {username}")
	}
	if c.GroupSearchFilter != "" && !strings.Contains(c.GroupSearchFilter, "{dn}") && !strings.Contains(c.GroupSearchFilter, "{username}") {
		return errors.New("CROW_LDAP_GROUP_SEARCH_FILTER must contain {dn} or {username}")
	}
	if len(c.URL) > maxURLLen {
		return errors.New("CROW_LDAP_URL must not be longer than 255 characters")
	}
	if c.BindDN != "" && c.BindPassword == "" {
		return errors.New("CROW_LDAP_BIND_DN requires CROW_LDAP_BIND_PASSWORD")
	}
	return nil
}

func urlScheme(raw string) string {
	scheme, _, ok := strings.Cut(raw, "://")
	if !ok {
		return ""
	}
	return strings.ToLower(scheme)
}

// Claims are the verified user attributes from a sign-in.
type Claims struct {
	Subject  string
	Email    string
	Username string
	Groups   []string // group DNs
}

// Provider signs users in against an LDAP directory.
type Provider struct {
	cfg     Config
	tls     *tls.Config
	issuer  string
	mapping groupmap.Mapping
}

// New applies defaults to cfg and loads the CA file without contacting the
// directory.
func New(cfg Config) (*Provider, error) {
	if cfg.UserFilter == "" {
		cfg.UserFilter = defaultUserFilter
	}
	if cfg.UsernameAttribute == "" {
		cfg.UsernameAttribute = defaultUsernameAttribute
	}
	if cfg.EmailAttribute == "" {
		cfg.EmailAttribute = defaultEmailAttribute
	}
	if cfg.IDAttribute == "" {
		cfg.IDAttribute = defaultIDAttribute
	}
	if cfg.GroupAttribute == "" {
		cfg.GroupAttribute = defaultGroupAttribute
	}
	if cfg.GroupBaseDN == "" {
		cfg.GroupBaseDN = cfg.UserBaseDN
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = defaultDisplayName
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid CROW_LDAP_URL: %w", err)
	}
	tlsCfg, err := tlsConfig(cfg, u.Hostname())
	if err != nil {
		return nil, err
	}

	return &Provider{
		cfg:    cfg,
		tls:    tlsCfg,
		issuer: "ldap://" + strings.ToLower(u.Hostname()),
		mapping: groupmap.Mapping{
			AllowedGroups: cfg.AllowedGroups,
			AdminGroups:   cfg.AdminGroups,
			RoleGroups:    cfg.RoleGroups,
			Match:         MatchGroup,
		},
	}, nil
}

// tlsConfig trusts the system pool plus the optional CA file. ServerName is
// set explicitly because StartTLS, unlike ldaps://, does not derive it.
func tlsConfig(cfg Config, hostname string) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         hostname,
		InsecureSkipVerify: cfg.SkipVerify, //nolint:gosec // explicitly requested with CROW_LDAP_SKIP_VERIFY
	}
	if cfg.CACertFile == "" {
		return tlsCfg, nil
	}
	pem, err := os.ReadFile(cfg.CACertFile)
	if err != nil {
		return nil, fmt.Errorf("reading CROW_LDAP_CA_CERT_FILE: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("CROW_LDAP_CA_CERT_FILE %s contains no PEM certificates", cfg.CACertFile)
	}
	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}

// Authenticate checks username and password against the directory and returns
// the user's attributes. It returns ErrInvalidCredentials for credential
// problems and an error wrapping ErrDirectory for everything else.
func (p *Provider) Authenticate(ctx context.Context, username, password string) (*Claims, error) {
	// An empty password would be an unauthenticated bind, which many
	// directories accept, so reject it before connecting.
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	conn, err := goldap.DialURL(p.cfg.URL,
		goldap.DialWithTLSConfig(p.tls),
		goldap.DialWithDialer(&net.Dialer{Timeout: timeout}),
	)
	if err != nil {
		return nil, directoryError(ctx, "connecting", err)
	}
	defer conn.Close()
	conn.SetTimeout(timeout)

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()

	if p.cfg.StartTLS {
		if err := conn.StartTLS(p.tls); err != nil {
			return nil, directoryError(ctx, "StartTLS", err)
		}
	}

	if err := p.serviceBind(conn); err != nil {
		return nil, directoryError(ctx, "service account bind", err)
	}

	entry, err := p.findUser(conn, username)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return nil, err
		}
		return nil, directoryError(ctx, "searching user", err)
	}

	if err := conn.Bind(entry.DN, password); err != nil {
		if goldap.IsErrorWithCode(err, goldap.LDAPResultInvalidCredentials) {
			return nil, ErrInvalidCredentials
		}
		return nil, directoryError(ctx, "user bind", err)
	}

	groups, err := p.groups(conn, entry, username)
	if err != nil {
		return nil, directoryError(ctx, "searching groups", err)
	}

	raw := entry.GetEqualFoldRawAttributeValue(p.cfg.IDAttribute)
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: missing %s attribute", ErrDirectory, p.cfg.IDAttribute)
	}

	claims := &Claims{
		Subject:  subjectValue(p.cfg.IDAttribute, raw),
		Email:    entry.GetEqualFoldAttributeValue(p.cfg.EmailAttribute),
		Username: entry.GetEqualFoldAttributeValue(p.cfg.UsernameAttribute),
		Groups:   groups,
	}
	if claims.Username == "" {
		claims.Username = username
	}
	return claims, nil
}

// serviceBind binds as the service account; without a bind DN the searches
// run anonymously.
func (p *Provider) serviceBind(conn *goldap.Conn) error {
	if p.cfg.BindDN == "" {
		return nil
	}
	return conn.Bind(p.cfg.BindDN, p.cfg.BindPassword)
}

// findUser returns the single entry matching the user filter. No match and
// more than one match both return ErrInvalidCredentials.
func (p *Provider) findUser(conn *goldap.Conn, username string) (*goldap.Entry, error) {
	attrs := []string{p.cfg.UsernameAttribute, p.cfg.EmailAttribute, p.cfg.IDAttribute}
	if p.cfg.GroupSearchFilter == "" {
		attrs = append(attrs, p.cfg.GroupAttribute)
	}
	res, err := conn.Search(goldap.NewSearchRequest(
		p.cfg.UserBaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases,
		userSizeLimit, 0, false,
		strings.ReplaceAll(p.cfg.UserFilter, "{username}", goldap.EscapeFilter(username)),
		attrs, nil,
	))
	if err != nil {
		if goldap.IsErrorWithCode(err, goldap.LDAPResultSizeLimitExceeded) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if len(res.Entries) != 1 {
		return nil, ErrInvalidCredentials
	}
	return res.Entries[0], nil
}

// groups returns the DNs of the user's groups, read from GroupAttribute or
// found with GroupSearchFilter.
func (p *Provider) groups(conn *goldap.Conn, entry *goldap.Entry, username string) ([]string, error) {
	if p.cfg.GroupSearchFilter == "" {
		return entry.GetEqualFoldAttributeValues(p.cfg.GroupAttribute), nil
	}
	if err := p.serviceBind(conn); err != nil {
		return nil, fmt.Errorf("service account rebind: %w", err)
	}
	filter := strings.NewReplacer(
		"{dn}", goldap.EscapeFilter(entry.DN),
		"{username}", goldap.EscapeFilter(username),
	).Replace(p.cfg.GroupSearchFilter)
	res, err := conn.Search(goldap.NewSearchRequest(
		p.cfg.GroupBaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases,
		0, 0, false, filter, []string{noAttributes}, nil,
	))
	if err != nil {
		return nil, err
	}
	groups := make([]string, 0, len(res.Entries))
	for _, e := range res.Entries {
		groups = append(groups, e.DN)
	}
	return groups, nil
}

// directoryError wraps err as ErrDirectory, reporting a canceled context
// instead of the resulting connection error.
func directoryError(ctx context.Context, what string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = ctxErr
	}
	return fmt.Errorf("%w: %s: %v", ErrDirectory, what, err) //nolint:errorlint // only ErrDirectory is part of the API
}

// Name is stored as user_identities.provider.
func (p *Provider) Name() string { return ProviderName }

// Issuer is "ldap://" plus the lower-cased directory hostname, without scheme
// or port, so switching to ldaps:// or another port keeps every identity.
// Identities are keyed by issuer and subject, because a subject is only unique
// within its directory.
func (p *Provider) Issuer() string { return p.issuer }

// DisplayName is the label of the login form.
func (p *Provider) DisplayName() string { return p.cfg.DisplayName }

// IsAllowed reports whether a user with these groups may sign in.
func (p *Provider) IsAllowed(groups []string) bool { return p.mapping.IsAllowed(groups) }

// RestrictsGroups reports whether sign-in is limited to CROW_LDAP_ALLOWED_GROUPS.
func (p *Provider) RestrictsGroups() bool { return p.mapping.RestrictsGroups() }

// ManagesAdmin reports whether admin status is derived from CROW_LDAP_ADMIN_GROUPS.
func (p *Provider) ManagesAdmin() bool { return p.mapping.ManagesAdmin() }

// IsAdmin reports whether these groups grant instance admin.
func (p *Provider) IsAdmin(groups []string) bool { return p.mapping.IsAdmin(groups) }

// ManagesGroups reports whether any check needs the user's groups.
func (p *Provider) ManagesGroups() bool { return p.mapping.ManagesGroups() }

// ManagedRoles returns the roles that CROW_LDAP_<ROLE>_GROUPS maps, sorted.
func (p *Provider) ManagedRoles() []string { return p.mapping.ManagedRoles() }

// RolesFor returns the managed roles these groups grant, sorted.
func (p *Provider) RolesFor(groups []string) []string { return p.mapping.RolesFor(groups) }
