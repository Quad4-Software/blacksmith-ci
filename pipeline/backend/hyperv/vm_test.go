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

package hyperv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVMName(t *testing.T) {
	assert.Equal(t, "crow-01jabc", vmName("01JABC"))
	assert.Equal(t, "crow-a-b", vmName("a/b"))
	assert.True(t, strings.HasPrefix(vmName("anything"), vmNamePrefix))
}

func TestDiskPath(t *testing.T) {
	assert.Equal(t, `D:\vms\crow-x.vhdx`, diskPath(`D:\vms`, "crow-x"))
	assert.Equal(t, "crow-x.vhdx", diskPath("", "crow-x"))
}

func TestCreateVMScriptUsesADifferencingChild(t *testing.T) {
	cfg := config{BaseVHDX: `C:\golden.vhdx`, Generation: 2, SwitchName: "crow-nat", CPU: 4, MemoryMB: 8192}
	got := createVMScript(cfg, "crow-x", `D:\vms\crow-x.vhdx`)

	// The golden disk is only ever a parent, never the VM's own disk.
	assert.Contains(t, got, "New-VHD -Path 'D:\\vms\\crow-x.vhdx' -ParentPath 'C:\\golden.vhdx' -Differencing")
	assert.Contains(t, got, "$crowNew['SwitchName'] = 'crow-nat'")
	assert.Contains(t, got, "Set-VMProcessor -VMName 'crow-x' -Count 4")
	assert.Contains(t, got, "$crowNew['MemoryStartupBytes'] = 8192MB")
	// Automatic checkpoints would retain a copy of every disposable disk.
	assert.Contains(t, got, "-AutomaticCheckpointsEnabled $false")
	// Copy-VMFile needs this and it is off by default.
	assert.Contains(t, got, "Enable-VMIntegrationService -VMName 'crow-x' -Name 'Guest Service Interface'")
	assert.NotContains(t, got, "Remove-VMNetworkAdapter")
}

func TestCreateVMScriptWithoutASwitchRemovesTheAdapter(t *testing.T) {
	got := createVMScript(config{BaseVHDX: `C:\g.vhdx`, Generation: 2}, "crow-x", `C:\crow-x.vhdx`)
	assert.NotContains(t, got, "SwitchName")
	// New-VM attaches an adapter regardless, so an air-gapped guest needs it
	// taken away rather than merely left disconnected.
	assert.Contains(t, got, "Get-VMNetworkAdapter -VMName 'crow-x' | Remove-VMNetworkAdapter")
}

func TestCreateVMScriptOmitsUnsetSizing(t *testing.T) {
	got := createVMScript(config{BaseVHDX: `C:\g.vhdx`, Generation: 2}, "crow-x", `C:\crow-x.vhdx`)
	assert.NotContains(t, got, "Set-VMProcessor")
	assert.NotContains(t, got, "MemoryStartupBytes")
}

func TestDestroyVMScriptNeverStopsEarly(t *testing.T) {
	got := destroyVMScript("crow-x", `C:\crow-x.vhdx`)
	// A failed setup leaves some of these missing, and the later ones still
	// have to run, so the script must not be in Stop mode.
	assert.Contains(t, got, "$ErrorActionPreference = 'SilentlyContinue'")
	assert.Contains(t, got, "Stop-VM -Name 'crow-x' -TurnOff -Force")
	assert.Contains(t, got, "Remove-VM -Name 'crow-x' -Force")
	assert.Contains(t, got, "Remove-Item -LiteralPath 'C:\\crow-x.vhdx' -Force")
	assert.True(t, strings.HasSuffix(got, "exit 0\n"))
}

func TestParseVMList(t *testing.T) {
	assert.Equal(t, []string{"crow-a", "crow-b"}, parseVMList([]byte("crow-a\r\ncrow-b\r\n\r\n")))
	assert.Nil(t, parseVMList([]byte("   \n\n")))
}

func TestStaleVMsLeavesActiveAndForeignVMsAlone(t *testing.T) {
	names := []string{"crow-01jabc", "crow-01jdef", "my-own-vm", "DC01"}
	active := map[string]struct{}{"01JABC": {}}

	assert.Equal(t, []string{"crow-01jdef"}, staleVMs(names, active))
}

func TestLeaseVMEnforcesTheLicenceCap(t *testing.T) {
	e := &hyperv{config: config{MaxVMs: 2}}

	assert.NoError(t, e.leaseVM())
	assert.NoError(t, e.leaseVM())
	assert.ErrorIs(t, e.leaseVM(), ErrVMLimitReached)

	e.releaseVM()
	assert.NoError(t, e.leaseVM())
}

func TestLeaseVMUnlimitedWhenMaxIsZero(t *testing.T) {
	e := &hyperv{config: config{MaxVMs: 0}}
	for range 50 {
		assert.NoError(t, e.leaseVM())
	}
	// Releasing an unlimited backend is a no-op rather than an underflow.
	e.releaseVM()
	assert.Equal(t, 0, e.leased)
}

func TestReleaseVMDoesNotUnderflow(t *testing.T) {
	e := &hyperv{config: config{MaxVMs: 1}}
	e.releaseVM()
	e.releaseVM()
	assert.Equal(t, 0, e.leased)
	assert.NoError(t, e.leaseVM())
}
