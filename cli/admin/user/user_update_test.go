// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package user

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

func newTestRoot(sub *cli.Command) *cli.Command {
	return &cli.Command{
		Name:   "root",
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server"},
			&cli.StringFlag{Name: "token"},
		},
		Commands: []*cli.Command{sub},
	}
}

func TestUserUpdateValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing login", args: []string{"update", "--role", "operator"}, wantErr: "missing username"},
		{name: "neither role nor no-roles", args: []string{"update", "alice"}, wantErr: "set --role or --no-roles"},
		{name: "role and no-roles", args: []string{"update", "alice", "--role", "operator", "--no-roles"}, wantErr: "cannot be used together"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validation must fail before any API call is made.
			err := newTestRoot(newUserUpdateCmd()).Run(t.Context(), append([]string{"root"}, tt.args...))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestUserUpdateKeepsProfileFields(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantRoles string
	}{
		{name: "role", args: []string{"--role", "operator", "--role", "auditor"}, wantRoles: `["operator","auditor"]`},
		{name: "no-roles", args: []string{"--no-roles"}, wantRoles: `[]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patched map[string]json.RawMessage
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/users/alice", r.URL.Path)
				switch r.Method {
				case http.MethodGet:
					_, _ = io.WriteString(w, `{"id":7,"login":"alice","email":"a@example.com","avatar_url":"http://x/a.png","admin":true,"description":"bio","timezone":"Europe/Berlin","roles":["secrets-manager"]}`)
				case http.MethodPatch:
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
					_, _ = io.WriteString(w, `{"id":7,"login":"alice"}`)
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer ts.Close()

			args := append([]string{"root", "--server", ts.URL, "--token", "t", "update", "alice"}, tt.args...)
			assert.NoError(t, newTestRoot(newUserUpdateCmd()).Run(t.Context(), args))

			str := func(k string) string { return string(patched[k]) }
			assert.Equal(t, `"a@example.com"`, str("email"))
			assert.Equal(t, `"http://x/a.png"`, str("avatar_url"))
			assert.Equal(t, `true`, str("admin"))
			assert.Equal(t, `"bio"`, str("description"))
			assert.Equal(t, `"Europe/Berlin"`, str("timezone"))
			assert.Equal(t, tt.wantRoles, str("roles"))
		})
	}
}

func TestUserOutputRoles(t *testing.T) {
	command := &cli.Command{Name: "root"}
	roles := []string{"operator", "auditor"}
	users := []*crow.User{
		{ID: 1, Login: "alice", Roles: &roles},
		{ID: 2, Login: "bob"},
	}

	var out bytes.Buffer
	assert.NoError(t, userOutput(command, users, &out))
	assert.Contains(t, out.String(), "ROLES")
	assert.Contains(t, out.String(), "operator, auditor")
	assert.Contains(t, out.String(), "bob")
}
