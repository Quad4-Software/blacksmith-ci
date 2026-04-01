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

// Package podman provides an experimental Podman backend for running pipeline steps.
// It uses the native Podman Go bindings SDK for direct communication with Podman.
//
// This backend is EXPERIMENTAL and may change or be removed in future releases.
package podman

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/bindings/network"
	"github.com/containers/podman/v5/pkg/bindings/system"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	nettypes "go.podman.io/common/libnetwork/types"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common"
	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// EngineName is the name of the Podman backend.
const EngineName = "podman"

const (
	networkDriverBridge = "bridge"
)

type podman struct {
	conn   context.Context // Podman connection context
	osType string          // OS type (linux, windows, etc.)
	config config
}

// New returns a new Podman Backend.
func New() backend.Backend {
	return &podman{}
}

// Name returns the name of the backend.
func (e *podman) Name() string {
	return EngineName
}

// IsAvailable checks if the Podman backend is available.
func (e *podman) IsAvailable(ctx context.Context) bool {
	if c, ok := ctx.Value(backend.CliCommand).(*cli.Command); ok {
		if c.IsSet("backend-podman-host") {
			return true
		}
	}

	// Check for Podman socket (rootful and rootless)
	// Rootful: /run/podman/podman.sock
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return true
	}

	// Check rootless socket
	xdgRuntime := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntime != "" {
		rootlessSocket := xdgRuntime + "/podman/podman.sock"
		if _, err := os.Stat(rootlessSocket); err == nil {
			return true
		}
	}

	// On systems using systemd socket activation, the socket file may not exist
	// until something connects to it. Fall back to checking for the podman binary.
	if _, err := exec.LookPath("podman"); err == nil {
		return true
	}

	return false
}

// Flags returns the CLI flags for the Podman backend.
func (e *podman) Flags() []cli.Flag {
	return Flags
}

// Load initializes the Podman backend using the native Podman SDK.
func (e *podman) Load(ctx context.Context) (*backend.BackendInfo, error) {
	c, ok := ctx.Value(backend.CliCommand).(*cli.Command)
	if !ok {
		return nil, backend.ErrNoCliContextFound
	}

	// Determine socket path
	podmanHost := c.String("backend-podman-host")
	if podmanHost == "" {
		podmanHost = detectPodmanSocket()
	}
	log.Debug().Str("socket", podmanHost).Msg("connecting to Podman")

	// Create Podman connection
	conn, err := bindings.NewConnection(ctx, podmanHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create Podman connection: %w", err)
	}
	e.conn = conn

	// Get system info
	info, err := system.Info(conn, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Podman info: %w", err)
	}
	e.osType = info.Host.OS

	// Parse config
	e.config, err = configFromCli(c)
	if err != nil {
		return nil, err
	}

	log.Warn().Msg("the Podman backend is EXPERIMENTAL — behavior may change in future releases")

	return &backend.BackendInfo{
		Platform: info.Host.OS + "/" + normalizeArchType(info.Host.Arch),
	}, nil
}

// detectPodmanSocket returns the path to the Podman socket.
// It checks for rootful socket first, then rootless.
func detectPodmanSocket() string {
	// Try rootful first
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		log.Debug().Msg("using rootful podman socket")
		return "unix:///run/podman/podman.sock"
	}

	// Try rootless
	xdgRuntime := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntime != "" {
		rootlessSocket := xdgRuntime + "/podman/podman.sock"
		if _, err := os.Stat(rootlessSocket); err == nil {
			log.Debug().Str("socket", rootlessSocket).Msg("using rootless podman socket")
			return "unix://" + rootlessSocket
		}
	}

	// Default to rootful
	log.Warn().Msg("no podman socket found, defaulting to rootful")
	return "unix:///run/podman/podman.sock"
}

// SetupWorkflow sets up the workflow environment (volume and network).
func (e *podman) SetupWorkflow(ctx context.Context, conf *backend.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("create workflow environment")

	// Create volume
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	_, err := volumes.Create(e.conn, newVolumeCreateOptions(conf.Volume), nil)
	if err != nil {
		return fmt.Errorf("failed to create volume %s: %w", conf.Volume, err)
	}

	// Create network
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	_, err = network.Create(e.conn, newNetworkCreateOptions(conf.Network, e.config.enableIPv6))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create network %s: %w", conf.Network, err)
	}
	if err != nil {
		log.Debug().Str("network", conf.Network).Msg("Network already exists, continuing")
	}

	return nil
}

// StartStep starts a container for the given step.
func (e *podman) StartStep(ctx context.Context, step *backend.Step, taskUUID string) error {
	options, err := parseBackendOptions(step)
	if err != nil {
		log.Error().Err(err).Msg("could not parse backend options")
	}

	log.Trace().Str("taskUUID", taskUUID).Msgf("start step %s", step.Name)

	spec := e.toSpecGenerator(step, options, taskUUID)
	containerName := common.ToContainerName(step)
	spec.Name = containerName

	// Add default volumes
	spec.Volumes = append(spec.Volumes, toNamedVolumes(e.config.volumes)...)

	// Automatically pull the latest version of the image if requested
	if step.Pull {
		pullOpts := new(images.PullOptions)
		if step.AuthConfig.Username != "" && step.AuthConfig.Password != "" {
			pullOpts = pullOpts.WithUsername(step.AuthConfig.Username).WithPassword(step.AuthConfig.Password)
		}
		//nolint:contextcheck // Podman SDK uses context embedded in connection
		_, pErr := images.Pull(e.conn, step.Image, pullOpts)
		// Only return pull error if credentials were provided
		if pErr != nil && step.AuthConfig.Password != "" {
			return pErr
		}
	}

	// Create container
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	response, err := containers.CreateWithSpec(e.conn, spec, nil)
	if err != nil {
		// Image not found, try to pull
		if strings.Contains(err.Error(), "image not known") || strings.Contains(err.Error(), "no such image") {
			pullOpts := new(images.PullOptions)
			if step.AuthConfig.Username != "" && step.AuthConfig.Password != "" {
				pullOpts = pullOpts.WithUsername(step.AuthConfig.Username).WithPassword(step.AuthConfig.Password)
			}
			//nolint:contextcheck // Podman SDK uses context embedded in connection
			_, pErr := images.Pull(e.conn, step.Image, pullOpts)
			if pErr != nil {
				return fmt.Errorf("failed to pull image %s: %w", step.Image, pErr)
			}
			//nolint:contextcheck // Podman SDK uses context embedded in connection
			response, err = containers.CreateWithSpec(e.conn, spec, nil)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Connect to networks (if not using custom network mode)
	if len(step.NetworkMode) == 0 {
		for _, n := range step.Networks {
			opts := &nettypes.PerNetworkOptions{
				Aliases: n.Aliases,
			}
			//nolint:contextcheck // Podman SDK uses context embedded in connection
			err = network.Connect(e.conn, n.Name, response.ID, opts)
			if err != nil {
				return fmt.Errorf("failed to connect to network %s: %w", n.Name, err)
			}
		}

		// Join the container to an existing network if configured
		if e.config.network != "" {
			//nolint:contextcheck // Podman SDK uses context embedded in connection
			err = network.Connect(e.conn, e.config.network, response.ID, nil)
			if err != nil {
				return fmt.Errorf("failed to connect to configured network %s: %w", e.config.network, err)
			}
		}
	}

	//nolint:contextcheck // Podman SDK uses context embedded in connection
	return containers.Start(e.conn, response.ID, nil)
}

// WaitStep waits for the container to complete.
func (e *podman) WaitStep(ctx context.Context, step *backend.Step, taskUUID string) (*backend.State, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("wait for step %s", step.Name)

	containerName := common.ToContainerName(step)

	// Wait for container to stop
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	exitCode, err := containers.Wait(e.conn, containerName, new(containers.WaitOptions).WithCondition([]define.ContainerStatus{define.ContainerStateStopped, define.ContainerStateExited}))
	if err != nil {
		return nil, fmt.Errorf("failed to wait for container: %w", err)
	}

	// Inspect container for OOMKilled status
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	info, err := containers.Inspect(e.conn, containerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	return &backend.State{
		Exited:    true,
		ExitCode:  int(exitCode),
		OOMKilled: info.State.OOMKilled,
	}, nil
}

// TailStep streams the container logs.
func (e *podman) TailStep(ctx context.Context, step *backend.Step, taskUUID string) (io.ReadCloser, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("tail logs of step %s", step.Name)

	containerName := common.ToContainerName(step)

	// Create channels for stdout and stderr
	stdoutChan := make(chan string, 100)
	stderrChan := make(chan string, 100)

	// Create pipe for output
	rc, wc := io.Pipe()

	// Start log streaming in goroutine
	go func() {
		defer close(stdoutChan)
		defer close(stderrChan)

		logOpts := new(containers.LogOptions).
			WithFollow(true).
			WithStdout(true).
			WithStderr(true)

		err := containers.Logs(e.conn, containerName, logOpts, stdoutChan, stderrChan)
		if err != nil {
			log.Error().Err(err).Msg("error streaming logs")
		}
	}()

	// Merge stdout and stderr into the pipe with tags
	go func() {
		defer wc.Close()

		for {
			select {
			case line, ok := <-stdoutChan:
				if !ok {
					stdoutChan = nil
				} else {
					// Tag stdout with 0x01
					tagged := append([]byte{'\x01'}, []byte(line+"\n")...)
					_, _ = wc.Write(tagged)
				}
			case line, ok := <-stderrChan:
				if !ok {
					stderrChan = nil
				} else {
					// Tag stderr with 0x02
					tagged := append([]byte{'\x02'}, []byte(line+"\n")...)
					_, _ = wc.Write(tagged)
				}
			}

			if stdoutChan == nil && stderrChan == nil {
				return
			}
		}
	}()

	return rc, nil
}

// DestroyStep stops and removes the container.
func (e *podman) DestroyStep(ctx context.Context, step *backend.Step, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msgf("stop step %s", step.Name)

	containerName := common.ToContainerName(step)

	// Kill container
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	if err := containers.Kill(e.conn, containerName, new(containers.KillOptions).WithSignal("SIGKILL")); err != nil && !isContainerNotFoundOrNotRunning(err) {
		return err
	}

	// Remove container
	force := true
	removeVolumes := true
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	_, err := containers.Remove(e.conn, containerName, new(containers.RemoveOptions).WithForce(force).WithVolumes(removeVolumes))
	if err != nil && !isContainerNotFoundOrNotRunning(err) {
		return err
	}

	return nil
}

// DestroyWorkflow destroys the workflow environment.
func (e *podman) DestroyWorkflow(ctx context.Context, conf *backend.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("delete workflow environment")

	for _, stage := range conf.Stages {
		for _, step := range stage.Steps {
			containerName := common.ToContainerName(step)
			//nolint:contextcheck // Podman SDK uses context embedded in connection
			if err := containers.Kill(e.conn, containerName, new(containers.KillOptions).WithSignal("SIGKILL")); err != nil && !isContainerNotFoundOrNotRunning(err) {
				log.Error().Err(err).Msgf("could not kill container '%s'", step.Name)
			}
			force := true
			removeVolumes := true
			//nolint:contextcheck // Podman SDK uses context embedded in connection
			if _, err := containers.Remove(e.conn, containerName, new(containers.RemoveOptions).WithForce(force).WithVolumes(removeVolumes)); err != nil && !isContainerNotFoundOrNotRunning(err) {
				log.Error().Err(err).Msgf("could not remove container '%s'", step.Name)
			}
		}
	}

	//nolint:contextcheck // Podman SDK uses context embedded in connection
	if err := volumes.Remove(e.conn, conf.Volume, new(volumes.RemoveOptions).WithForce(true)); err != nil {
		log.Error().Err(err).Msgf("could not remove volume '%s'", conf.Volume)
	}

	//nolint:contextcheck // Podman SDK uses context embedded in connection
	if _, err := network.Remove(e.conn, conf.Network, nil); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no such network") {
			log.Debug().Str("network", conf.Network).Msg("Network already removed or not found")
		} else {
			log.Error().Err(err).Msgf("could not remove network '%s'", conf.Network)
		}
	}

	return nil
}

// DiscoverWorkflows finds orphaned workflow containers that can be adopted after agent restart.
func (e *podman) DiscoverWorkflows(ctx context.Context) ([]*backend.DiscoveredWorkflow, error) {
	log.Trace().Msg("discovering orphaned workflows")

	// List all running containers with our task UUID label
	filters := map[string][]string{
		"label":  {common.LabelTaskUUID},
		"status": {"running"},
	}
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	containerList, err := containers.List(e.conn, new(containers.ListOptions).WithFilters(filters))
	if err != nil {
		return nil, err
	}

	// Group containers by task UUID
	workflowMap := make(map[string]*backend.DiscoveredWorkflow)
	for _, c := range containerList {
		taskUUID := c.Labels[common.LabelTaskUUID]
		if taskUUID == "" {
			continue
		}

		workflow, exists := workflowMap[taskUUID]
		if !exists {
			workflow = &backend.DiscoveredWorkflow{
				TaskUUID: taskUUID,
				Steps:    []*backend.DiscoveredStep{},
			}
			workflowMap[taskUUID] = workflow
		}

		workflow.Steps = append(workflow.Steps, &backend.DiscoveredStep{
			UUID:        c.Labels[common.LabelStepUUID],
			Name:        c.Labels[common.LabelStepName],
			ContainerID: c.ID,
		})
	}

	// Convert map to slice
	workflows := make([]*backend.DiscoveredWorkflow, 0, len(workflowMap))
	for _, workflow := range workflowMap {
		workflows = append(workflows, workflow)
	}

	log.Debug().Int("count", len(workflows)).Msg("discovered orphaned workflows")
	return workflows, nil
}

// ResumeStep resumes monitoring of an already-running step (for adopted workflows).
func (e *podman) ResumeStep(ctx context.Context, step *backend.Step, taskUUID string) error {
	containerName := common.ToContainerName(step)
	log.Trace().Str("taskUUID", taskUUID).Str("container", containerName).Msgf("resuming step %s", step.Name)

	// Verify the container exists and is running
	//nolint:contextcheck // Podman SDK uses context embedded in connection
	info, err := containers.Inspect(e.conn, containerName, nil)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}

	if !info.State.Running {
		log.Debug().
			Str("container", containerName).
			Str("status", info.State.Status).
			Int32("exit_code", info.State.ExitCode).
			Msg("container is not running, may have completed during agent restart")
	}

	log.Info().
		Str("taskUUID", taskUUID).
		Str("container", containerName).
		Str("step", step.Name).
		Msg("resumed monitoring of adopted step")

	return nil
}

// isContainerNotFoundOrNotRunning checks if the error indicates the container
// was not found or is not running.
func isContainerNotFoundOrNotRunning(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "no such container") ||
		strings.Contains(errStr, "container not found") ||
		strings.Contains(errStr, "is not running") ||
		strings.Contains(errStr, "no container with") ||
		strings.Contains(errStr, "can only kill running containers") ||
		strings.Contains(errStr, "container state improper")
}

// normalizeArchType converts the arch type reported by podman info into
// the runtime.GOARCH format.
func normalizeArchType(s string) string {
	switch s {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return s
	}
}

// Ensure podman implements the Backend interface.
var _ backend.Backend = (*podman)(nil)

// Ensure podman implements the WorkflowDiscoverer interface.
var _ backend.WorkflowDiscoverer = (*podman)(nil)

// Ensure podman implements the WorkflowResumer interface.
var _ backend.WorkflowResumer = (*podman)(nil)
