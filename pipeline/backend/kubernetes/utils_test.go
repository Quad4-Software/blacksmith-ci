// Copyright 2024 Woodpecker Authors
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

package kubernetes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDNSName(t *testing.T) {
	name, err := dnsName("crow_01he8bebctabr3kgk0qj36d2me_0_services_0")
	assert.NoError(t, err)
	assert.Equal(t, "crow-01he8bebctabr3kgk0qj36d2me-0-services-0", name)

	name, err = dnsName("a.0-AA")
	assert.NoError(t, err)
	assert.Equal(t, "a.0-aa", name)

	name, err = dnsName("crow-01he8bebctabr3kgk0qj36d2me-0-services-0.crow-runtime.svc.cluster.local")
	assert.NoError(t, err)
	assert.Equal(t, "crow-01he8bebctabr3kgk0qj36d2me-0-services-0.crow-runtime.svc.cluster.local", name)

	_, err = dnsName(".0-a")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)

	_, err = dnsName("ABC..DEF")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)

	_, err = dnsName("0.-a")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)

	_, err = dnsName("test-")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)

	_, err = dnsName("-test")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)

	_, err = dnsName("0-a.")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)

	_, err = dnsName("abc\\def")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)
}

func TestToDnsName(t *testing.T) {
	name, err := toDNSName("BUILD_AND_DEPLOY_0")
	assert.NoError(t, err)
	assert.Equal(t, "build-and-deploy-0", name)

	name, err = toDNSName("build and deploy")
	assert.NoError(t, err)
	assert.Equal(t, "build-and-deploy", name)

	name, err = toDNSName("build & deploy")
	assert.NoError(t, err)
	assert.Equal(t, "build--deploy", name)

	_, err = toDNSName("-build-and-deploy")
	assert.ErrorIs(t, err, ErrDNSPatternInvalid)
}

func TestSanitizeLabelValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain alphanumeric is unchanged", in: "go-test", want: "go-test"},
		{name: "underscores and dots are preserved", in: "a_b.c", want: "a_b.c"},
		{name: "slashes in repo full name are replaced", in: "foo-team/baz-arch", want: "foo-team-baz-arch"},
		{name: "curly-brace wrapped uuid is trimmed", in: "{00000000-0000-0000-0000-000000000000}", want: "00000000-0000-0000-0000-000000000000"},
		{name: "leading and trailing non-alphanumeric are trimmed", in: "--foo--", want: "foo"},
		{name: "spaces are replaced with dashes", in: "build and deploy", want: "build-and-deploy"},
		{name: "value over 63 chars is truncated and trimmed", in: strings.Repeat("a", 70), want: strings.Repeat("a", 63)},
		{name: "truncated value with trailing dash is re-trimmed", in: strings.Repeat("a", 62) + "-bbbb", want: strings.Repeat("a", 62)},
		{name: "all-invalid input becomes empty", in: "///", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeLabelValue(tt.in))
		})
	}
}
