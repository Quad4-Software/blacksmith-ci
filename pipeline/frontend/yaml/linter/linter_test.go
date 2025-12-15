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
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v5/pipeline/errors"
	"codeberg.org/crowci/crow/v5/pipeline/frontend/yaml"
	"codeberg.org/crowci/crow/v5/pipeline/frontend/yaml/linter"
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
			want: "additional properties 'pipeline' not allowed",
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
