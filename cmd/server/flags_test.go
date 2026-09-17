package main

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
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
