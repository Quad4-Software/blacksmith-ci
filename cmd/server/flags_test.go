package main

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/server"
)

// The --logs-duration-keep flag (WOODPECKER_DEFAULT_LOGS_KEEP_DURATION /
// CROW_DEFAULT_LOGS_KEEP_DURATION) is documented as "The default time (as
// duration) for which pipeline logs of historic runs are preserved" and is
// consumed in setup.go as a duration string (server.Config.Pipeline.LogsDurationKeep
// is a string later parsed with time.ParseDuration). But it is registered in
// flags.go as an cli.Int64Flag, so the server refuses to start on any duration
// value, and setup.go reads it back with c.String(), which always returns "".
//
// These tests exercise the real `flags` slice used by cmd/server/main.go,
// with a no-op Action so no server/database/forge setup is required.
func newTestCommand(action cli.ActionFunc) *cli.Command {
	return &cli.Command{
		Name:      "crow-server",
		Flags:     flags,
		Action:    action,
		Writer:    io.Discard, // suppress the full --help dump urfave/cli prints on a parse error
		ErrWriter: io.Discard,
	}
}

func TestLogsDurationKeepFlag_AcceptsDurationStrings(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "hours", value: "720h"},
		{name: "minutes", value: "30m"},
		{name: "zero", value: "0h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestCommand(func(_ context.Context, _ *cli.Command) error { return nil })

			err := cmd.Run(context.Background(), []string{"crow-server", "--logs-duration-keep", tt.value})

			require.NoErrorf(t, err, "server should accept a duration value for --logs-duration-keep, matching its own usage text")
		})
	}
}

func TestLogsDurationKeepFlag_StringGetterReturnsSetValue(t *testing.T) {
	// "5000" is a value the current Int64Flag definition can parse, isolating
	// the second bug from the parse failure covered above: setup.go reads
	// this flag with c.String(), but c.String() type-asserts the stored value
	// to string and silently returns "" whenever the flag was registered as
	// an Int64Flag, regardless of what was passed on the CLI.
	const want = "5000"
	var got string

	cmd := newTestCommand(func(_ context.Context, c *cli.Command) error {
		// This is exactly how setup.go reads the flag today:
		// server.Config.Pipeline.LogsDurationKeep = c.String("logs-duration-keep")
		got = c.String("logs-duration-keep")
		return nil
	})

	err := cmd.Run(context.Background(), []string{"crow-server", "--logs-duration-keep", want})
	require.NoError(t, err)

	assert.Equalf(t, want, got, "c.String(\"logs-duration-keep\") should return the value passed on the CLI")
}

func TestSetupIdentityProviders_ForgeLoginDisabledRequiresProvider(t *testing.T) {
	var setupErr error
	cmd := newTestCommand(func(ctx context.Context, c *cli.Command) error {
		setupErr = setupIdentityProviders(ctx, c)
		return nil
	})

	err := cmd.Run(context.Background(), []string{"crow-server", "--forge-login-disabled"})
	require.NoError(t, err)

	assert.EqualError(t, setupErr, "CROW_FORGE_LOGIN_DISABLED requires OIDC or LDAP sign-in")
}

func TestSetupIdentityProviders_ForgeLoginDisabledWithLDAP(t *testing.T) {
	t.Cleanup(func() {
		server.Config.Services.LDAP = nil
		server.Config.Server.ForgeLoginDisabled = false
	})
	var setupErr error
	cmd := newTestCommand(func(ctx context.Context, c *cli.Command) error {
		setupErr = setupIdentityProviders(ctx, c)
		return nil
	})

	// ldap.New does not contact the directory, so any valid URL works.
	err := cmd.Run(context.Background(), []string{
		"crow-server", "--forge-login-disabled",
		"--ldap-url", "ldap://ldap.example:389", "--ldap-user-base-dn", "dc=example,dc=org",
	})
	require.NoError(t, err)

	require.NoError(t, setupErr)
	require.NotNil(t, server.Config.Services.LDAP)
	assert.Equal(t, "LDAP", server.Config.Services.LDAP.DisplayName())
	assert.True(t, server.Config.Server.ForgeLoginDisabled)
}

func TestSetupIdentityProviders_InvalidLDAPConfig(t *testing.T) {
	var setupErr error
	cmd := newTestCommand(func(ctx context.Context, c *cli.Command) error {
		setupErr = setupIdentityProviders(ctx, c)
		return nil
	})

	err := cmd.Run(context.Background(), []string{"crow-server", "--ldap-url", "ldap://ldap.example:389"})
	require.NoError(t, err)

	assert.EqualError(t, setupErr, "CROW_LDAP_URL requires CROW_LDAP_USER_BASE_DN")
	assert.Nil(t, server.Config.Services.LDAP)
}

func TestSetupTrustedProxies(t *testing.T) {
	t.Cleanup(func() { server.Config.Server.TrustedProxies = nil })
	for _, tc := range []struct {
		name    string
		value   string
		want    []string
		wantErr string
	}{
		{name: "unset", want: []string{}},
		{name: "ips and ranges", value: "10.0.0.1,fd00::/8,192.168.0.0/16", want: []string{"10.0.0.1", "fd00::/8", "192.168.0.0/16"}},
		{name: "invalid entry", value: "10.0.0.1,proxy.example", wantErr: `CROW_TRUSTED_PROXIES: invalid entry "proxy.example"`},
		{name: "invalid range", value: "10.0.0.0/33", wantErr: `CROW_TRUSTED_PROXIES: invalid entry "10.0.0.0/33"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server.Config.Server.TrustedProxies = nil
			var setupErr error
			cmd := newTestCommand(func(_ context.Context, c *cli.Command) error {
				setupErr = setupTrustedProxies(c)
				return nil
			})
			args := []string{"crow-server"}
			if tc.value != "" {
				args = append(args, "--trusted-proxies", tc.value)
			}
			require.NoError(t, cmd.Run(context.Background(), args))

			if tc.wantErr != "" {
				assert.EqualError(t, setupErr, tc.wantErr)
				assert.Nil(t, server.Config.Server.TrustedProxies)
				return
			}
			require.NoError(t, setupErr)
			assert.Equal(t, tc.want, server.Config.Server.TrustedProxies)
		})
	}
}
