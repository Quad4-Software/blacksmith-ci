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

// Package ldaptest provides an in-process LDAP directory for tests. It answers
// simple binds and searches over a fixed set of users and groups.
package ldaptest

import (
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	ber "github.com/go-asn1-ber/asn1-ber"
	goldap "github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/go-hclog"
	"github.com/jimlambrt/gldap"
	"github.com/stretchr/testify/require"
)

const (
	// ServiceDN is the bind DN of the service account.
	ServiceDN = "cn=svc,dc=example,dc=org"
	// ServicePassword is the password of the service account.
	ServicePassword = "svc-secret"

	readyTimeout = 5 * time.Second
	readyTick    = 10 * time.Millisecond
)

// User is a directory entry that can bind with Password.
type User struct {
	DN       string
	Password string
	// Attrs are the entry's attributes, e.g. "uid", "mail", "entryUUID" and
	// "memberOf". Binary values such as objectGUID can be stored as raw bytes
	// in a string.
	Attrs map[string][]string
}

// Group is a group entry with a "member" attribute.
type Group struct {
	DN      string
	Members []string // member DNs
}

// Server is a fake LDAP directory listening on 127.0.0.1.
type Server struct {
	// URL is ldap://127.0.0.1:<port>.
	URL string

	users   []User
	entries []entry

	mu    sync.Mutex
	binds []string
}

type entry struct {
	dn    string
	attrs map[string][]string
}

// New starts a directory on a free port with the service account
// ServiceDN/ServicePassword, the given users and groups, and stops it in
// t.Cleanup. Search supports equality, presence and AND filters over the
// stored attributes, the base DN scope and the size limit. Anonymous search is
// allowed.
func New(t *testing.T, users []User, groups []Group) *Server {
	t.Helper()

	srv := &Server{users: users}
	for _, u := range users {
		srv.entries = append(srv.entries, entry{dn: u.DN, attrs: u.Attrs})
	}
	for _, g := range groups {
		attrs := map[string][]string{"member": g.Members}
		if dn, err := goldap.ParseDN(g.DN); err == nil && len(dn.RDNs) > 0 && len(dn.RDNs[0].Attributes) > 0 {
			first := dn.RDNs[0].Attributes[0]
			attrs[first.Type] = []string{first.Value}
		}
		srv.entries = append(srv.entries, entry{dn: g.DN, attrs: attrs})
	}

	s, err := gldap.NewServer(gldap.WithLogger(hclog.NewNullLogger()))
	require.NoError(t, err)
	mux, err := gldap.NewMux()
	require.NoError(t, err)
	require.NoError(t, mux.Bind(srv.handleBind))
	require.NoError(t, mux.Search(srv.handleSearch))
	require.NoError(t, s.Router(mux))

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	go func() { _ = s.Run(addr) }()
	require.Eventually(t, s.Ready, readyTimeout, readyTick, "LDAP test server did not start")
	t.Cleanup(func() { _ = s.Stop() })

	srv.URL = "ldap://" + addr
	return srv
}

// Binds returns the DN of every bind attempt so far, successful or not.
func (s *Server) Binds() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.binds)
}

func (s *Server) handleBind(w *gldap.ResponseWriter, r *gldap.Request) {
	resp := r.NewBindResponse(gldap.WithResponseCode(gldap.ResultInvalidCredentials))
	defer func() { _ = w.Write(resp) }()

	m, err := r.GetSimpleBindMessage()
	if err != nil {
		resp.SetResultCode(gldap.ResultProtocolError)
		return
	}

	s.mu.Lock()
	s.binds = append(s.binds, m.UserName)
	s.mu.Unlock()

	if m.AuthChoice != gldap.SimpleAuthChoice || m.Password == "" {
		return
	}
	if equalDN(m.UserName, ServiceDN) && string(m.Password) == ServicePassword {
		resp.SetResultCode(gldap.ResultSuccess)
		return
	}
	for _, u := range s.users {
		if equalDN(m.UserName, u.DN) && string(m.Password) == u.Password {
			resp.SetResultCode(gldap.ResultSuccess)
			return
		}
	}
}

func (s *Server) handleSearch(w *gldap.ResponseWriter, r *gldap.Request) {
	done := r.NewSearchDoneResponse(gldap.WithResponseCode(gldap.ResultSuccess))
	defer func() { _ = w.Write(done) }()

	m, err := r.GetSearchMessage()
	if err != nil {
		done.SetResultCode(gldap.ResultProtocolError)
		return
	}
	filter, err := goldap.CompileFilter(m.Filter)
	if err != nil {
		done.SetResultCode(gldap.ResultProtocolError)
		return
	}

	var matches []entry
	for _, e := range s.entries {
		if !inScope(e.dn, m.BaseDN, m.Scope) {
			continue
		}
		ok, supported := match(filter, e.attrs)
		if !supported {
			done.SetResultCode(gldap.ResultUnwillingToPerform)
			return
		}
		if ok {
			matches = append(matches, e)
		}
	}

	if m.SizeLimit > 0 && int64(len(matches)) > m.SizeLimit {
		matches = matches[:m.SizeLimit]
		done.SetResultCode(gldap.ResultSizeLimitExceeded)
	}
	for _, e := range matches {
		_ = w.Write(r.NewSearchResponseEntry(e.dn, gldap.WithAttributes(selectAttrs(e.attrs, m.Attributes))))
	}
}

// match evaluates a compiled filter against attrs. It reports supported=false for
// filter types the test server does not implement.
func match(f *ber.Packet, attrs map[string][]string) (ok, supported bool) {
	switch f.Tag {
	case goldap.FilterAnd:
		for _, child := range f.Children {
			ok, supported := match(child, attrs)
			if !supported || !ok {
				return ok, supported
			}
		}
		return true, true
	case goldap.FilterEqualityMatch:
		if len(f.Children) != 2 { //nolint:mnd
			return false, false
		}
		name, value := packetString(f.Children[0]), packetString(f.Children[1])
		return slices.ContainsFunc(values(attrs, name), func(v string) bool { return strings.EqualFold(v, value) }), true
	case goldap.FilterPresent:
		name := packetString(f)
		return strings.EqualFold(name, "objectClass") || len(values(attrs, name)) > 0, true
	default:
		return false, false
	}
}

func packetString(p *ber.Packet) string {
	if p.Data != nil {
		return p.Data.String()
	}
	if s, ok := p.Value.(string); ok {
		return s
	}
	return ""
}

func values(attrs map[string][]string, name string) []string {
	for k, v := range attrs {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return nil
}

func selectAttrs(attrs map[string][]string, requested []string) map[string][]string {
	if len(requested) == 0 || slices.Contains(requested, "*") {
		return attrs
	}
	out := make(map[string][]string, len(requested))
	for k, v := range attrs {
		if slices.ContainsFunc(requested, func(r string) bool { return strings.EqualFold(r, k) }) {
			out[k] = v
		}
	}
	return out
}

func inScope(dn, base string, scope gldap.Scope) bool {
	dn, base = normalizeDN(dn), normalizeDN(base)
	switch scope {
	case gldap.BaseObject:
		return dn == base
	case gldap.SingleLevel:
		i := strings.IndexByte(dn, ',')
		return i >= 0 && dn[i+1:] == base
	default:
		return base == "" || dn == base || strings.HasSuffix(dn, ","+base)
	}
}

func equalDN(a, b string) bool { return normalizeDN(a) == normalizeDN(b) }

func normalizeDN(dn string) string {
	parsed, err := goldap.ParseDN(dn)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(dn))
	}
	rdns := make([]string, 0, len(parsed.RDNs))
	for _, rdn := range parsed.RDNs {
		parts := make([]string, 0, len(rdn.Attributes))
		for _, a := range rdn.Attributes {
			parts = append(parts, strings.ToLower(a.Type)+"="+strings.ToLower(a.Value))
		}
		rdns = append(rdns, strings.Join(parts, "+"))
	}
	return strings.Join(rdns, ",")
}
