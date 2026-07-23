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

package kubernetes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestTmpfsVolumesAndMounts(t *testing.T) {
	t.Run("bare path becomes unbounded memory emptyDir when no memory limit", func(t *testing.T) {
		parsed, err := parseTmpfsMounts([]string{"/tmp"}, nil)
		require.NoError(t, err)

		vols := tmpfsVolumes(parsed)
		require.Len(t, vols, 1)
		assert.Equal(t, "tmpfs-0", vols[0].Name)
		require.NotNil(t, vols[0].EmptyDir)
		assert.Equal(t, v1.StorageMediumMemory, vols[0].EmptyDir.Medium)
		assert.Nil(t, vols[0].EmptyDir.SizeLimit)

		mounts := tmpfsVolumeMounts(parsed)
		require.Len(t, mounts, 1)
		assert.Equal(t, "tmpfs-0", mounts[0].Name)
		assert.Equal(t, "/tmp", mounts[0].MountPath)
	})

	t.Run("explicit size sets the emptyDir size limit", func(t *testing.T) {
		parsed, err := parseTmpfsMounts([]string{"/scratch:size=64m"}, nil)
		require.NoError(t, err)
		vols := tmpfsVolumes(parsed)
		require.Len(t, vols, 1)
		require.NotNil(t, vols[0].EmptyDir.SizeLimit)
		// NOTE: common.ParseMemory reads "64m" (uppercased to "64M") as decimal
		// megabytes, whereas Docker reads lowercase "m" as mebibytes. This unit
		// mismatch is a known cross-backend gap.
		want := resource.NewQuantity(64*1000*1000, resource.BinarySI)
		assert.True(t, vols[0].EmptyDir.SizeLimit.Equal(*want),
			"want %s, got %s", want.String(), vols[0].EmptyDir.SizeLimit.String())
	})

	t.Run("missing size defaults to the step memory limit", func(t *testing.T) {
		memLimit := resource.NewQuantity(256*1024*1024, resource.BinarySI)
		parsed, err := parseTmpfsMounts([]string{"/tmp"}, memLimit)
		require.NoError(t, err)
		vols := tmpfsVolumes(parsed)
		require.Len(t, vols, 1)
		require.NotNil(t, vols[0].EmptyDir.SizeLimit)
		assert.True(t, vols[0].EmptyDir.SizeLimit.Equal(*memLimit))
	})

	t.Run("explicit size wins over the step memory limit", func(t *testing.T) {
		memLimit := resource.NewQuantity(256*1024*1024, resource.BinarySI)
		parsed, err := parseTmpfsMounts([]string{"/tmp:size=8m"}, memLimit)
		require.NoError(t, err)
		vols := tmpfsVolumes(parsed)
		require.NotNil(t, vols[0].EmptyDir.SizeLimit)
		assert.False(t, vols[0].EmptyDir.SizeLimit.Equal(*memLimit))
	})

	t.Run("unknown option is rejected with a clear error", func(t *testing.T) {
		_, err := parseTmpfsMounts([]string{"/tmp:foo=bar"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "foo=bar")
	})

	t.Run("invalid mode is rejected", func(t *testing.T) {
		_, err := parseTmpfsMounts([]string{"/tmp:mode=9999"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "octal")
	})

	t.Run("volume and mount names stay aligned across entries", func(t *testing.T) {
		parsed, err := parseTmpfsMounts([]string{"/a:size=8m", "/b:size=16m"}, nil)
		require.NoError(t, err)
		vols := tmpfsVolumes(parsed)
		mounts := tmpfsVolumeMounts(parsed)
		require.Len(t, vols, 2)
		require.Len(t, mounts, 2)
		for i := range vols {
			assert.Equal(t, vols[i].Name, mounts[i].Name)
		}
	})
}

func TestTmpfsInitContainer(t *testing.T) {
	t.Run("mode produces a chmod init container", func(t *testing.T) {
		// This is the reporter's exact input from issue #1367.
		parsed, err := parseTmpfsMounts([]string{"/home/4000:mode=1777", "/home/8000:mode=1777"}, nil)
		require.NoError(t, err)

		init, err := tmpfsInitContainer(parsed, "alpine:latest", true)
		require.NoError(t, err)
		require.NotNil(t, init)
		assert.Equal(t, tmpfsInitContainerName, init.Name)
		assert.Equal(t, "alpine:latest", init.Image)
		require.NotNil(t, init.SecurityContext.RunAsUser)
		assert.Equal(t, int64(0), *init.SecurityContext.RunAsUser)

		require.Len(t, init.Command, 3)
		script := init.Command[2]
		assert.Contains(t, script, "chmod 1777 '/home/4000'")
		assert.Contains(t, script, "chmod 1777 '/home/8000'")

		// The init container mounts the same tmpfs volumes it chmods.
		require.Len(t, init.VolumeMounts, 2)
		assert.Equal(t, "/home/4000", init.VolumeMounts[0].MountPath)
	})

	t.Run("no mode means no init container", func(t *testing.T) {
		parsed, err := parseTmpfsMounts([]string{"/tmp:size=8m"}, nil)
		require.NoError(t, err)
		init, err := tmpfsInitContainer(parsed, "alpine:latest", true)
		require.NoError(t, err)
		assert.Nil(t, init)
	})

	t.Run("mode is rejected when root is disallowed", func(t *testing.T) {
		parsed, err := parseTmpfsMounts([]string{"/home/4000:mode=1777"}, nil)
		require.NoError(t, err)
		_, err = tmpfsInitContainer(parsed, "alpine:latest", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "runAsNonRoot")
	})

	t.Run("path with a single quote is escaped safely", func(t *testing.T) {
		parsed, err := parseTmpfsMounts([]string{"/od'd:mode=700"}, nil)
		require.NoError(t, err)
		init, err := tmpfsInitContainer(parsed, "alpine:latest", true)
		require.NoError(t, err)
		require.NotNil(t, init)
		assert.Contains(t, init.Command[2], `'/od'\''d'`)
		assert.False(t, strings.Contains(init.Command[2], `/od'd `))
	})
}

func TestTmpfsModeAllowed(t *testing.T) {
	tests := []struct {
		name       string
		privileged bool
		cfgNonRoot bool
		optNonRoot *bool
		want       bool
	}{
		{name: "default allows root init", want: true},
		{name: "privileged step always allowed", privileged: true, cfgNonRoot: true, want: true},
		{name: "agent enforces non-root", cfgNonRoot: true, want: false},
		{name: "step options enforce non-root", optNonRoot: newBool(true), want: false},
		{name: "step options explicitly allow root", optNonRoot: newBool(false), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &types.Step{Privileged: tt.privileged}
			cfg := &config{SecurityContext: SecurityContextConfig{RunAsNonRoot: tt.cfgNonRoot}}
			options := BackendOptions{}
			if tt.optNonRoot != nil {
				options.SecurityContext = &SecurityContext{RunAsNonRoot: tt.optNonRoot}
			}
			assert.Equal(t, tt.want, tmpfsModeAllowed(step, cfg, options))
		})
	}
}
