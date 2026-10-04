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
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// bootPollInterval is how often SetupWorkflow asks a booting VM whether
// PowerShell Direct is answering yet. A Windows guest takes tens of seconds to
// reach a logged-on runspace, so a two-second poll is already fine-grained.
const bootPollInterval = 2 * time.Second

// vmNamePrefix names every VM this backend creates. Stale-resource cleanup
// keys off it, so nothing outside this prefix is ever touched.
const vmNamePrefix = "crow-"

// vmNameUnsafe matches everything a Hyper-V VM name should not carry.
var vmNameUnsafe = regexp.MustCompile(`[^a-z0-9._-]+`)

// vmName derives this workflow's VM name from its task UUID.
func vmName(taskUUID string) string {
	name := vmNameUnsafe.ReplaceAllString(strings.ToLower(taskUUID), "-")
	return vmNamePrefix + strings.Trim(name, "-")
}

// diskPath is where this workflow's differencing disk lives.
func diskPath(vmPath, vm string) string {
	return winJoin(vmPath, vm+".vhdx")
}

// createVMScript builds the VM and its differencing disk.
//
// The disk is a differencing child rather than a copy, so creating it is
// instant and costs only the blocks the workflow actually writes. The parent
// is never opened for writing, which is also what makes it safe to share
// between concurrent VMs.
//
// Automatic checkpoints are turned off: they are on by default on desktop
// Windows and would silently retain a copy of every disposable VM's disk.
func createVMScript(cfg config, vm, disk string) string {
	var b strings.Builder
	b.WriteString(psPreamble)

	fmt.Fprintf(&b, "New-VHD -Path %s -ParentPath %s -Differencing | Out-Null\n",
		psQuote(disk), psQuote(cfg.BaseVHDX))

	fmt.Fprintf(&b, "$crowNew = @{ Name = %s; Generation = %d; VHDPath = %s }\n",
		psQuote(vm), cfg.Generation, psQuote(disk))
	if cfg.MemoryMB > 0 {
		fmt.Fprintf(&b, "$crowNew['MemoryStartupBytes'] = %dMB\n", cfg.MemoryMB)
	}
	if cfg.VMPath != "" {
		fmt.Fprintf(&b, "$crowNew['Path'] = %s\n", psQuote(cfg.VMPath))
	}
	if cfg.SwitchName != "" {
		fmt.Fprintf(&b, "$crowNew['SwitchName'] = %s\n", psQuote(cfg.SwitchName))
	}
	b.WriteString("New-VM @crowNew | Out-Null\n")

	if cfg.SwitchName == "" {
		// New-VM always attaches an adapter. An unconnected one would still
		// give the guest a device to probe, so it is removed outright.
		fmt.Fprintf(&b, "Get-VMNetworkAdapter -VMName %s | Remove-VMNetworkAdapter\n", psQuote(vm))
	}
	if cfg.CPU > 0 {
		fmt.Fprintf(&b, "Set-VMProcessor -VMName %s -Count %d\n", psQuote(vm), cfg.CPU)
	}
	if cfg.MemoryMB > 0 {
		fmt.Fprintf(&b, "Set-VMMemory -VMName %s -DynamicMemoryEnabled $false\n", psQuote(vm))
	}
	fmt.Fprintf(&b, "Set-VM -Name %s -AutomaticCheckpointsEnabled $false -CheckpointType Disabled\n", psQuote(vm))
	// Copy-VMFile, which the clone step uses to carry its binary in, needs
	// this integration component and it is off by default.
	fmt.Fprintf(&b, "Enable-VMIntegrationService -VMName %s -Name 'Guest Service Interface'\n", psQuote(vm))

	return b.String()
}

// startVMScript boots the VM.
func startVMScript(vm string) string {
	return psPreamble + fmt.Sprintf("Start-VM -Name %s\n", psQuote(vm))
}

// destroyVMScript tears the VM down and reclaims its disk.
//
// Every step continues past a failure: a VM that never started, or a disk that
// was never created, is the normal case on a failed setup, and stopping there
// would leave the later resources behind forever.
func destroyVMScript(vm, disk string) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'SilentlyContinue'\n$ProgressPreference = 'SilentlyContinue'\n")
	// TurnOff rather than a clean shutdown: the disk is about to be deleted,
	// so there is nothing to leave consistent and a guest that ignores the
	// shutdown request must not hold the agent up.
	fmt.Fprintf(&b, "Stop-VM -Name %s -TurnOff -Force\n", psQuote(vm))
	fmt.Fprintf(&b, "Remove-VM -Name %s -Force\n", psQuote(vm))
	fmt.Fprintf(&b, "Remove-Item -LiteralPath %s -Force\n", psQuote(disk))
	b.WriteString("exit 0\n")
	return b.String()
}

// listVMsScript names this backend's VMs, one per line.
func listVMsScript() string {
	return psPreamble + fmt.Sprintf(
		"Get-VM | Where-Object { $_.Name -like %s } | ForEach-Object { $_.Name }\n",
		psQuote(vmNamePrefix+"*"))
}

// parseVMList reads the names listVMsScript printed.
func parseVMList(out []byte) []string {
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// createVM creates this workflow's VM and its differencing disk.
func (e *hyperv) createVM(ctx context.Context, vm, disk string) error {
	if out, err := e.run(ctx, createVMScript(e.config, vm, disk)); err != nil {
		return fmt.Errorf("create VM %s: %w: %s", vm, err, bytes.TrimSpace(out))
	}
	return nil
}

// startVM boots the VM. Unlike the tart backend's child process, a Hyper-V VM
// outlives the process that started it, which is why cleanup has to be able to
// find VMs an agent left behind.
func (e *hyperv) startVM(ctx context.Context, vm string) error {
	if out, err := e.run(ctx, startVMScript(vm)); err != nil {
		return fmt.Errorf("start VM %s: %w: %s", vm, err, bytes.TrimSpace(out))
	}
	return nil
}

// waitForGuest polls until PowerShell Direct reaches a logged-on runspace.
//
// Failures here are expected and frequent: the VM is booting, and every
// attempt before the guest reaches a usable state is an error. Only the last
// one is reported, because the earlier ones say nothing but "not yet".
func (e *hyperv) waitForGuest(ctx context.Context, vm string) error {
	deadline := time.Now().Add(e.config.StartupTimeout)
	ticker := time.NewTicker(bootPollInterval)
	defer ticker.Stop()

	var lastOut []byte
	var lastErr error
	for {
		out, err := e.execGuest(ctx, vm, guestReadyProbe)
		if err == nil {
			return nil
		}
		lastOut, lastErr = out, err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("VM %s did not answer PowerShell Direct within %s: %w: %s",
				vm, e.config.StartupTimeout, lastErr, bytes.TrimSpace(lastOut))
		}
	}
}

// destroyVM stops the VM, removes it and deletes its differencing disk.
func (e *hyperv) destroyVM(ctx context.Context, vm, disk string) error {
	out, err := e.run(ctx, destroyVMScript(vm, disk))
	if err != nil {
		return fmt.Errorf("destroy VM %s: %w: %s", vm, err, bytes.TrimSpace(out))
	}
	return nil
}

// leaseVM claims one of the configured VM slots.
//
// The cap is a license, not a queue: a workflow that arrives when every slot
// is taken is failed rather than held, because the agent's own concurrency
// limit is what is supposed to keep this from happening and silently blocking
// here would turn a misconfiguration into a stall nobody can see.
func (e *hyperv) leaseVM() error {
	if e.config.MaxVMs <= 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.leased >= e.config.MaxVMs {
		return fmt.Errorf("%w (%d in use); raise CROW_BACKEND_HYPERV_MAX_VMS or lower CROW_MAX_WORKFLOWS",
			ErrVMLimitReached, e.leased)
	}
	e.leased++
	return nil
}

// releaseVM gives a slot back.
func (e *hyperv) releaseVM() {
	if e.config.MaxVMs <= 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.leased > 0 {
		e.leased--
	}
}

// logDestroyFailure records a teardown that did not work.
//
// A VM that survives its workflow holds a differencing disk and, more
// importantly, one of the licensed slots, so this is worth more than a debug
// line even though nothing above can act on it.
func logDestroyFailure(vm string, err error) {
	log.Error().Err(err).Str("vm", vm).Msg("could not destroy Hyper-V VM; it will be reclaimed by stale-resource cleanup")
}
