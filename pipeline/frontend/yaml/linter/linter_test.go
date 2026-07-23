// Copyright 2023 Woodpecker Authors
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

package linter_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline/errors"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/linter"
)

func TestLint(t *testing.T) {
	testdatas := []struct{ Title, Data string }{{
		Title: "map", Data: `
when:
  event: push

steps:
  build:
    image: docker
    volumes:
      - /tmp:/tmp
    commands:
      - go build
      - go test
  publish:
    image: woodpeckerci/plugin-kaniko
    settings:
      repo: foo/bar
      foo: bar
services:
  redis:
    image: redis
`,
	}, {
		Title: "list", Data: `
when:
  event: push

steps:
  - name: build
    image: docker
    volumes:
      - /tmp:/tmp
    commands:
      - go build
      - go test
  - name: publish
    image: woodpeckerci/plugin-kaniko
    settings:
      repo: foo/bar
      foo: bar
services:
  - name: redis
    image: redis
`,
	}, {
		Title: "merge maps", Data: `
when:
  event: push

variables:
  step_template: &base-step
    image: golang:1.19
    commands:
      - go version

steps:
  test base step:
    <<: *base-step
  test base step with latest image:
    <<: *base-step
    image: golang:latest
`,
	}}

	for _, testd := range testdatas {
		t.Run(testd.Title, func(t *testing.T) {
			conf, err := yaml.ParseString(testd.Data)
			assert.NoError(t, err)

			assert.NoError(t, linter.New(linter.WithTrusted(linter.TrustedConfiguration{
				Network:  true,
				Volumes:  true,
				Security: true,
			})).Lint([]*linter.WorkflowConfig{{
				File:      testd.Title,
				RawConfig: testd.Data,
				Workflow:  conf,
			}}), "expected lint returns no errors")
		})
	}
}

func TestLintErrors(t *testing.T) {
	testdata := []struct {
		from string
		want string
	}{
		{
			from: "",
			want: "Invalid or missing `steps` section",
		},
		{
			from: "steps: { build: { image: '' }  }",
			want: "Invalid or missing image",
		},
		{
			from: "steps: { build: { image: golang, privileged: true }  }",
			want: "Insufficient `security` privileges. To allow, check the 'security' option in the 'Privileges & Security' repository settings.",
		},
		{
			from: "steps: { build: { image: golang, dns: [ 8.8.8.8 ] }  }",
			want: "Insufficient `network` privileges. To allow using 'dns' options, check the 'Network' option in the 'Privileges & Security' repository settings.",
		},

		{
			from: "steps: { build: { image: golang, dns_search: [ example.com ] }  }",
			want: "Insufficient `network` privileges. To allow using 'dns_search' options, check the 'Network' option in the 'Privileges & Security' repository settings.",
		},
		{
			from: "steps: { build: { image: golang, devices: [ '/dev/tty0:/dev/tty0' ] }  }",
			want: "Insufficient `volumes` privileges. To allow using 'devices' options, check the 'Volumes' option in the 'Privileges & Security' repository settings.",
		},
		{
			from: "steps: { build: { image: golang, extra_hosts: [ 'somehost:162.242.195.82' ] }  }",
			want: "Insufficient `network` privileges. To allow using 'extra_hosts' options, check the 'Network' option in the 'Privileges & Security' repository settings.",
		},
		{
			from: "steps: { build: { image: golang, network_mode: host }  }",
			want: "Insufficient `network` privileges. To allow using 'network_mode' options, check the 'Network' option in the 'Privileges & Security' repository settings.",
		},
		{
			from: "steps: { build: { image: golang, volumes: [ '/opt/data:/var/lib/mysql' ] }  }",
			want: "Insufficient `volumes` privileges. To allow using 'volumes' options, check the 'Volumes' option in the 'Privileges & Security' repository settings.",
		},
		{
			from: "steps: { build: { image: golang, network_mode: 'container:name' }  }",
			want: "Insufficient `network` privileges. To allow using 'network_mode' options, check the 'Network' option in the 'Privileges & Security' repository settings.",
		},
		{
			from: "steps: { build: { image: golang, settings: { test: 'true' }, commands: [ 'echo ja', 'echo nein' ] } }",
			want: "Using both `commands:` and `settings:` at the same time is not allowed.",
		},
		{
			from: "steps: { build: { image: golang, settings: { test: 'true' }, entrypoint: [ '/bin/fish' ] } }",
			want: "Using both `entrypoint:` and `settings:` at the same time is not allowed.",
		},
		{
			from: "steps: { build: { image: golang, settings: { test: 'true' }, environment: { 'TEST': 'true' } } }",
			want: "Using both `environment:` and `settings:` at the same time is not allowed.",
		},
		{
			from: "{pipeline: { build: { image: golang, settings: { test: 'true' } } }, when: { branch: main, event: push } }",
			want: "Unknown property `pipeline`. Did you mean `steps`?",
		},
		{
			from: "{steps: { build: { image: plugins/docker, settings: { test: 'true' } } }, when: { branch: main, event: push } }",
			want: "The formerly privileged plugin 'plugins/docker' is no longer privileged by default since v3. To use it, add it to (server) env var `CROW_PLUGINS_PRIVILEGED` (must be done by an instance administrator).",
		},
		{
			from: "{steps: { build: { image: golang, settings: { test: 'true' } } }, when: { branch: main, event: push }, clone: { git: { image: some-other/clone:v1.1.0 } } }",
			want: "The specified clone image does not match the images in the allow list (`CROW_PLUGINS_TRUSTED_CLONE`), not injecting `.netrc` credentials.",
		},
		{
			from: "steps: { build: { image: golang, secrets: [ { source: mysql_username, target: mysql_username } ] } }",
			want: "The usage of `secrets:` is deprecated since v2.8.x and was removed in >=3.x. Use `environment:` in combination with `from_secret:`.",
		},
		{
			from: "steps: { build: { image: golang, secrets: [ 'mysql_username' ] } }",
			want: "The usage of `secrets:` is deprecated since v2.8.x and was removed in >=3.x. Use `environment:` in combination with `from_secret:`.",
		},
		{
			from: "steps: { build: { image: golang }, publish: { image: golang, depends_on: [ binary ] } }",
			want: "One or more of the specified step dependencies do not exist. Double-check for typos.",
		},
	}

	for _, test := range testdata {
		conf, err := yaml.ParseString(test.from)
		assert.NoError(t, err)

		lerr := linter.New().Lint([]*linter.WorkflowConfig{{
			File:      test.from,
			RawConfig: test.from,
			Workflow:  conf,
		}})
		assert.Error(t, lerr, "expected lint error for configuration", test.from)

		lerrors := errors.GetPipelineErrors(lerr)
		found := false
		for _, lerr := range lerrors {
			if lerr.Message == test.want {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected error %q, got %q", test.want, lerrors)
	}
}

func TestLintTrustedErrorData(t *testing.T) {
	testdata := []struct {
		from        string
		wantTrusted string
	}{
		{
			from:        "steps: { build: { image: golang, privileged: true }  }",
			wantTrusted: "security",
		},
		{
			from:        "steps: { build: { image: golang, dns: [ 8.8.8.8 ] }  }",
			wantTrusted: "network",
		},
		{
			from:        "steps: { build: { image: golang, volumes: [ '/opt/data:/var/lib/mysql' ] }  }",
			wantTrusted: "volumes",
		},
	}

	for _, test := range testdata {
		conf, err := yaml.ParseString(test.from)
		assert.NoError(t, err)

		lerr := linter.New().Lint([]*linter.WorkflowConfig{{
			File:      test.from,
			RawConfig: test.from,
			Workflow:  conf,
		}})
		assert.Error(t, lerr, "expected lint error for configuration", test.from)

		found := false
		for _, lerr := range errors.GetPipelineErrors(lerr) {
			if data := errors.GetLinterData(lerr); data != nil && data.Trusted != "" {
				assert.Equal(t, test.wantTrusted, data.Trusted, "unexpected trusted setting for %q", test.from)
				found = true
				break
			}
		}
		assert.True(t, found, "Expected a linter error carrying trusted data for %q", test.from)
	}
}

func TestLintApprovalGate(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string // substring of expected error; "" = no error
	}{
		{
			name: "valid timeout 24h",
			yaml: `
when:
  event: push
steps:
  build:
    image: alpine
approval:
  required: true
  timeout: 24h
`,
			wantErr: "",
		},
		{
			name: "approvers without required is invalid",
			yaml: `
when:
  event: push
steps:
  build:
    image: alpine
approval:
  required: false
  approvers:
    users: [alice]
`,
			wantErr: "approvers without required",
		},
		{
			name: "zero timeout",
			yaml: `
when:
  event: push
steps:
  build:
    image: alpine
approval:
  required: true
  timeout: 0s
`,
			wantErr: "timeout must be positive",
		},
		{
			name: "negative timeout",
			yaml: `
when:
  event: push
steps:
  build:
    image: alpine
approval:
  required: true
  timeout: -1h
`,
			wantErr: "timeout must be positive",
		},
		{
			name: "unparseable timeout",
			yaml: `
when:
  event: push
steps:
  build:
    image: alpine
approval:
  required: true
  timeout: forever
`,
			wantErr: "invalid timeout",
		},
		{
			name: "timeout above max (default cap 168h)",
			yaml: `
when:
  event: push
steps:
  build:
    image: alpine
approval:
  required: true
  timeout: 200h
`,
			wantErr: "timeout exceeds max",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf, perr := yaml.ParseString(c.yaml)
			assert.NoError(t, perr)
			err := linter.New(linter.WithApprovalTimeoutMax(168 * time.Hour)).Lint([]*linter.WorkflowConfig{{
				File:      "x",
				RawConfig: c.yaml,
				Workflow:  conf,
			}})
			if c.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
			}
		})
	}
}

func TestTmpfsAllowedWithoutVolumesPrivilege(t *testing.T) {
	yamlData := `
when:
  event: push

steps:
  test:
    image: postgres:16
    tmpfs:
      - /var/lib/postgresql/data
    commands:
      - pg_isready
`
	conf, err := yaml.ParseString(yamlData)
	assert.NoError(t, err)

	// tmpfs should be allowed without any trusted privileges
	err = linter.New().Lint([]*linter.WorkflowConfig{{
		File:      "tmpfs-test",
		RawConfig: yamlData,
		Workflow:  conf,
	}})
	if err != nil {
		// Should not contain any tmpfs-related or volumes-related error
		lerrors := errors.GetPipelineErrors(err)
		for _, lerr := range lerrors {
			assert.NotContains(t, lerr.Message, "tmpfs", "tmpfs should not require any privilege")
			assert.NotContains(t, lerr.Message, "volumes", "tmpfs should not trigger volumes privilege check")
		}
	}
}

func TestTmpfsSyntaxValidation(t *testing.T) {
	tests := []struct {
		name    string
		tmpfs   string
		wantErr string // substring; "" means the entry should lint cleanly
	}{
		{name: "bare path", tmpfs: "/tmp", wantErr: ""},
		{name: "valid size", tmpfs: "/tmp:size=64m", wantErr: ""},
		{name: "valid mode", tmpfs: "/home/4000:mode=1777", wantErr: ""},
		{name: "size and mode", tmpfs: "/tmp:size=512Mi,mode=755", wantErr: ""},
		{name: "unknown option", tmpfs: "/tmp:foo=bar", wantErr: "Unsupported tmpfs option"},
		{name: "malformed option", tmpfs: "/tmp:noequals", wantErr: "expected `key=value`"},
		{name: "invalid mode", tmpfs: "/tmp:mode=9999", wantErr: "Invalid tmpfs mode"},
		{name: "invalid size", tmpfs: "/tmp:size=big", wantErr: "Invalid tmpfs size"},
		{name: "missing path", tmpfs: ":size=64m", wantErr: "missing mount path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlData := fmt.Sprintf(`
when:
  event: push

steps:
  test:
    image: alpine
    tmpfs:
      - %q
    commands:
      - echo ok
`, tt.tmpfs)
			conf, err := yaml.ParseString(yamlData)
			assert.NoError(t, err)

			err = linter.New().Lint([]*linter.WorkflowConfig{{
				File:      "tmpfs-syntax",
				RawConfig: yamlData,
				Workflow:  conf,
			}})

			if tt.wantErr == "" {
				if err != nil {
					for _, lerr := range errors.GetPipelineErrors(err) {
						assert.NotContains(t, lerr.Message, "tmpfs", "valid tmpfs entry should not raise a tmpfs error")
					}
				}
				return
			}

			require.Error(t, err)
			var found bool
			for _, lerr := range errors.GetPipelineErrors(err) {
				if strings.Contains(lerr.Message, tt.wantErr) {
					found = true
					break
				}
			}
			assert.True(t, found, "expected a lint error containing %q", tt.wantErr)
		})
	}
}

func TestTempVolumesAllowedWithoutVolumesPrivilege(t *testing.T) {
	yamlData := `
when:
  event: push

steps:
  test:
    image: docker
    temp_volumes:
      - dind:/var/run
    commands:
      - docker version
`
	conf, err := yaml.ParseString(yamlData)
	assert.NoError(t, err)

	err = linter.New().Lint([]*linter.WorkflowConfig{{
		File:      "temp-volumes-test",
		RawConfig: yamlData,
		Workflow:  conf,
	}})
	assert.NoError(t, err)
}

func TestLintDependsOnClone(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "depends_on implicit clone is valid",
			yaml: `
when:
  event: push
steps:
  build:
    image: golang
    depends_on: [clone]
    commands: [go build]
`,
			wantErr: false,
		},
		{
			name: "depends_on custom clone step is valid",
			yaml: `
when:
  event: push
clone:
  my-clone:
    image: some/clone
steps:
  build:
    image: golang
    depends_on: [my-clone]
    commands: [go build]
`,
			wantErr: false,
		},
		{
			name: "depends_on unknown step is flagged",
			yaml: `
when:
  event: push
steps:
  build:
    image: golang
    depends_on: [does-not-exist]
    commands: [go build]
`,
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf, perr := yaml.ParseString(c.yaml)
			assert.NoError(t, perr)
			err := linter.New().Lint([]*linter.WorkflowConfig{{
				File:      c.name,
				RawConfig: c.yaml,
				Workflow:  conf,
			}})

			depErr := false
			for _, lerr := range errors.GetPipelineErrors(err) {
				if lerr.Message == "One or more of the specified step dependencies do not exist. Double-check for typos." {
					depErr = true
				}
			}
			assert.Equal(t, c.wantErr, depErr, "unexpected depends_on lint result for %q", c.name)
		})
	}
}

func TestLintPluginRestrictions(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string // substring; "" = no plugin-restriction error expected
	}{
		{
			name: "plugin with settings and environment hard-errors",
			yaml: `
when:
  event: push
steps:
  publish:
    image: some/plugin
    settings:
      repo: foo/bar
    environment:
      FOO: bar
`,
			wantErr: "Plugin steps may not set `environment:`",
		},
		{
			name: "plugin with settings and secrets hard-errors",
			yaml: `
when:
  event: push
steps:
  publish:
    image: some/plugin
    settings:
      repo: foo/bar
    secrets: [mysql_password]
`,
			wantErr: "Plugin steps may not set `secrets:`",
		},
		{
			name: "plugin with only settings is fine",
			yaml: `
when:
  event: push
steps:
  publish:
    image: some/plugin
    settings:
      repo: foo/bar
`,
			wantErr: "",
		},
		{
			name: "regular step with environment is fine",
			yaml: `
when:
  event: push
steps:
  build:
    image: golang
    environment:
      FOO: bar
    commands: [go build]
`,
			wantErr: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf, perr := yaml.ParseString(c.yaml)
			assert.NoError(t, perr)
			err := linter.New().Lint([]*linter.WorkflowConfig{{
				File:      c.name,
				RawConfig: c.yaml,
				Workflow:  conf,
			}})

			found := ""
			for _, lerr := range errors.GetPipelineErrors(err) {
				if !lerr.IsWarning && strings.Contains(lerr.Message, "Plugin steps may not set") {
					found = lerr.Message
				}
			}
			if c.wantErr == "" {
				assert.Empty(t, found, "did not expect a plugin-restriction error for %q", c.name)
			} else {
				assert.Contains(t, found, c.wantErr, "expected plugin-restriction error for %q", c.name)
			}
		})
	}
}

func TestBadHabits(t *testing.T) {
	testdata := []struct {
		from string
		want string
	}{
		{
			from: "steps: { build: { image: golang } }",
			want: "Either set an event filter (`when:`) for all individual steps or for the entire workflow.",
		},
		{
			from: "when: [{branch: xyz}, {event: push}]\nsteps: { build: { image: golang } }",
			want: "Either set an event filter (`when:`) for all individual steps or for the entire workflow.",
		},
	}

	for _, test := range testdata {
		conf, err := yaml.ParseString(test.from)
		assert.NoError(t, err)

		lerr := linter.New().Lint([]*linter.WorkflowConfig{{
			File:      test.from,
			RawConfig: test.from,
			Workflow:  conf,
		}})
		assert.Error(t, lerr, "expected lint error for configuration", test.from)

		lerrors := errors.GetPipelineErrors(lerr)
		found := false
		for _, lerr := range lerrors {
			if lerr.Message == test.want {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected error %q, got %q", test.want, lerrors)
	}
}
