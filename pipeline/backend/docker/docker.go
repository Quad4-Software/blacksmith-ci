// Copyright 2022 Woodpecker Authors
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

package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	dockerFilters "github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	tls_config "github.com/docker/go-connections/tlsconfig"
	std_copy "github.com/moby/moby/api/pkg/stdcopy"
	json_message "github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common"
	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
	"codefloe.com/crowci/crow/v5/shared/utils"
)

type docker struct {
	client client.APIClient
	info   system.Info
	config config
}

const (
	EngineName          = "docker"
	networkDriverNAT    = "nat"
	networkDriverBridge = "bridge"
	volumeDriver        = "local"
)

// New returns a new Docker Backend.
func New() backend.Backend {
	return &docker{
		client: nil,
	}
}

func (e *docker) Name() string {
	return EngineName
}

// defaultDockerEndpoint returns the local Docker daemon endpoint path the
// agent should probe when DOCKER_HOST / --backend-docker-host isn't set.
// Matches the Docker SDK's default-host selection: Unix domain socket on
// Linux/macOS, named pipe on Windows.
func defaultDockerEndpoint() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\docker_engine`
	}
	return "/var/run/docker.sock"
}

// isAvailableTimeout caps the Ping attempt in IsAvailable. The local Docker
// daemon usually responds in <10ms; if it doesn't (e.g. crashed mid-startup),
// blocking longer just delays the agent picking another backend.
const isAvailableTimeout = 2 * time.Second

func (e *docker) IsAvailable(ctx context.Context) bool {
	if c, ok := ctx.Value(backend.CliCommand).(*cli.Command); ok {
		if c.IsSet("backend-docker-host") {
			return true
		}
	}
	// On Linux the Unix socket exists as a real file and os.Stat is fast and
	// reliable. On Windows the Docker daemon listens on a named pipe; Go's
	// os.Stat behavior on \\.\pipe\* paths is conservative and returns
	// errors even when the pipe is present (Test-Path / CreateFile succeed
	// against the same path). Use a real client ping there — slower in the
	// missing-daemon case but the only signal that's actually reliable.
	if runtime.GOOS == "windows" {
		cl, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
		if err != nil {
			return false
		}
		defer cl.Close()
		pingCtx, cancel := context.WithTimeout(ctx, isAvailableTimeout)
		defer cancel()
		_, err = cl.Ping(pingCtx)
		return err == nil
	}
	_, err := os.Stat(defaultDockerEndpoint())
	return err == nil
}

// Ping checks that the Docker daemon is reachable through the live API
// client. This detects post-startup failures that IsAvailable cannot — for
// example, when the host's dockerd is restarted (e.g. by an unattended
// package upgrade) and the agent container's bind-mounted socket still
// references the unlinked inode of the previous socket file.
func (e *docker) Ping(ctx context.Context) error {
	if e.client == nil {
		return errors.New("docker client not initialized")
	}
	_, err := e.client.Ping(ctx)
	return err
}

func httpClientOfOpts(dockerCertPath string, verifyTLS bool) *http.Client {
	if dockerCertPath == "" {
		return nil
	}

	options := tls_config.Options{
		CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
		CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
		KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
		InsecureSkipVerify: !verifyTLS,
	}
	tlsConf, err := tls_config.Client(options)
	if err != nil {
		log.Error().Err(err).Msg("could not create http client out of docker backend options")
		return nil
	}

	return &http.Client{
		Transport:     &http.Transport{TLSClientConfig: tlsConf},
		CheckRedirect: client.CheckRedirect,
	}
}

func (e *docker) Flags() []cli.Flag {
	return Flags
}

// Load new client for Docker Backend using environment variables.
func (e *docker) Load(ctx context.Context) (*backend.BackendInfo, error) {
	c, ok := ctx.Value(backend.CliCommand).(*cli.Command)
	if !ok {
		return nil, backend.ErrNoCliContextFound
	}

	var dockerClientOpts []client.Opt
	if httpClient := httpClientOfOpts(c.String("backend-docker-cert"), c.Bool("backend-docker-tls-verify")); httpClient != nil {
		dockerClientOpts = append(dockerClientOpts, client.WithHTTPClient(httpClient))
	}
	if dockerHost := c.String("backend-docker-host"); dockerHost != "" {
		dockerClientOpts = append(dockerClientOpts, client.WithHost(dockerHost))
	}
	if dockerAPIVersion := c.String("backend-docker-api-version"); dockerAPIVersion != "" {
		dockerClientOpts = append(dockerClientOpts, client.WithVersion(dockerAPIVersion))
	} else {
		dockerClientOpts = append(dockerClientOpts, client.WithAPIVersionNegotiation())
	}

	cl, err := client.NewClientWithOpts(dockerClientOpts...)
	if err != nil {
		return nil, err
	}
	e.client = cl

	e.info, err = cl.Info(ctx)
	if err != nil {
		return nil, err
	}

	e.config, err = configFromCli(c)
	if err != nil {
		return nil, err
	}

	return &backend.BackendInfo{
		Platform: e.info.OSType + "/" + normalizeArchType(e.info.Architecture),
	}, nil
}

func (e *docker) SetupWorkflow(ctx context.Context, conf *backend.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("create workflow environment")

	_, err := e.client.VolumeCreate(ctx, volume.CreateOptions{
		Name:   conf.Volume,
		Driver: volumeDriver,
	})
	if err != nil {
		return err
	}

	networkDriver := networkDriverBridge
	if e.info.OSType == "windows" {
		networkDriver = networkDriverNAT
	}
	_, err = e.client.NetworkCreate(ctx, conf.Network, network.CreateOptions{
		Driver:     networkDriver,
		EnableIPv6: &e.config.enableIPv6,
	})
	// Ignore "already exists" errors as this can happen in concurrent workflows
	if err != nil && strings.Contains(err.Error(), "already exists") {
		log.Debug().Str("network", conf.Network).Msg("Network already exists, continuing")
		return nil
	}
	return err
}

func (e *docker) StartStep(ctx context.Context, step *backend.Step, taskUUID string) error {
	// On Windows the Linux clone plugin image cannot run, and writing the clone
	// to the workflow volume's host mountpoint does not propagate into container
	// steps. Run the clone as a normal container using the Windows variant of
	// the plugin image (tag + "-" + windowsCloneTagSuffix); it writes through
	// the shared workspace volume mount like any other step.
	if step.Type == backend.StepTypeClone && e.info.OSType == "windows" {
		step.Image = appendImageTagSuffix(step.Image, windowsCloneTagSuffix)
		log.Trace().Str("step", step.Name).Str("image", step.Image).Msg("routing Windows clone step to containerized plugin-clone")
	}

	options, err := parseBackendOptions(step)
	if err != nil {
		log.Error().Err(err).Msg("could not parse backend options")
	}

	log.Trace().Str("taskUUID", taskUUID).Msgf("start step %s", step.Name)

	config := e.toConfig(step, options, taskUUID)
	hostConfig := toHostConfig(step, &e.config)
	containerName := toContainerName(step)

	// create pull options with encoded authorization credentials.
	pullOpts := image.PullOptions{}
	if step.AuthConfig.Username != "" && step.AuthConfig.Password != "" {
		pullOpts.RegistryAuth, _ = encodeAuthToBase64(step.AuthConfig)
	}

	// automatically pull the latest version of the image if requested
	// by the process configuration.
	if step.Pull {
		responseBody, pErr := e.client.ImagePull(ctx, config.Image, pullOpts)
		if pErr == nil {
			// TODO(1936): show image pull progress in web-ui
			fd := os.Stdout.Fd()
			isTerminal := term.IsTerminal(int(fd))
			if err := json_message.DisplayJSONMessagesStream(responseBody, os.Stdout, fd, isTerminal, nil); err != nil {
				log.Error().Err(err).Msg("DisplayJSONMessagesStream")
			}
			responseBody.Close()
		}
		// Fix "Show warning when fail to auth to docker registry"
		// (https://web.archive.org/web/20201023145804/https://github.com/drone/drone/issues/1917)
		if pErr != nil && step.AuthConfig.Password != "" {
			return pErr
		}
	}

	// add default volumes to the host configuration
	hostConfig.Binds = utils.DeduplicateStrings(append(hostConfig.Binds, e.config.volumes...))

	_, err = e.client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if errdefs.IsNotFound(err) {
		// automatically pull and try to re-create the image if the
		// failure is caused because the image does not exist.
		responseBody, pErr := e.client.ImagePull(ctx, config.Image, pullOpts)
		if pErr != nil {
			return pErr
		}
		// TODO(1936): show image pull progress in web-ui
		fd := os.Stdout.Fd()
		isTerminal := term.IsTerminal(int(fd))
		if err := json_message.DisplayJSONMessagesStream(responseBody, os.Stdout, fd, isTerminal, nil); err != nil {
			log.Error().Err(err).Msg("DisplayJSONMessagesStream")
		}
		responseBody.Close()

		_, err = e.client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	}
	if err != nil {
		return wrapDaemonError("create container", containerName, err)
	}

	if len(step.NetworkMode) == 0 {
		for _, net := range step.Networks {
			err = e.client.NetworkConnect(ctx, net.Name, containerName, &network.EndpointSettings{
				Aliases: net.Aliases,
			})
			if err != nil {
				return wrapDaemonError("connect network", containerName, err)
			}
		}

		// join the container to an existing network
		if e.config.network != "" {
			err = e.client.NetworkConnect(ctx, e.config.network, containerName, &network.EndpointSettings{})
			if err != nil {
				return wrapDaemonError("connect network", containerName, err)
			}
		}
	}

	return wrapDaemonError("start container", containerName, e.client.ContainerStart(ctx, containerName, container.StartOptions{}))
}

func (e *docker) WaitStep(ctx context.Context, step *backend.Step, taskUUID string) (*backend.State, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("wait for step %s", step.Name)

	containerName := toContainerName(step)

	wait, errC := e.client.ContainerWait(ctx, containerName, "")
	var waitErr error
	select {
	case <-wait:
	case waitErr = <-errC:
	}

	info, err := e.client.ContainerInspect(ctx, containerName)
	if err != nil {
		// If both calls failed, the inspect error is usually the more
		// informative one (it carries the actual transport failure), but we
		// fall back to the wait-channel error if inspect happens to succeed
		// later in some retry path.
		return nil, wrapDaemonError("inspect", containerName, err)
	}
	if waitErr != nil {
		return nil, wrapDaemonError("wait", containerName, waitErr)
	}

	return &backend.State{
		Exited:    true,
		ExitCode:  info.State.ExitCode,
		OOMKilled: info.State.OOMKilled,
	}, nil
}

func (e *docker) TailStep(ctx context.Context, step *backend.Step, taskUUID string) (io.ReadCloser, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("tail logs of step %s", step.Name)

	containerName := toContainerName(step)
	logs, err := e.client.ContainerLogs(ctx, containerName, container.LogsOptions{
		Follow:     true,
		ShowStdout: true,
		ShowStderr: true,
		Details:    false,
		Timestamps: false,
	})
	if err != nil {
		return nil, wrapDaemonError("tail logs", containerName, err)
	}
	rc, wc := io.Pipe()

	// demultiplex 'logs' which contains two streams (stdout and stderr),
	// writing them to separate tagged writers that preserve stream identity
	go func() {
		stdoutWriter := &taggedWriter{w: wc, tag: '\x01'} // Tag stdout with 0x01
		stderrWriter := &taggedWriter{w: wc, tag: '\x02'} // Tag stderr with 0x02
		_, _ = std_copy.StdCopy(stdoutWriter, stderrWriter, logs)
		_ = logs.Close()
		_ = wc.Close()
	}()
	return rc, nil
}

// taggedWriter wraps an io.Writer and prepends each line with a tag byte
// to indicate the stream type (stdout or stderr).
type taggedWriter struct {
	w      io.Writer
	tag    byte
	buffer []byte
}

func (tw *taggedWriter) Write(p []byte) (n int, err error) {
	// Accumulate data in buffer
	tw.buffer = append(tw.buffer, p...)

	// Process complete lines
	written := 0
	for {
		idx := bytes.IndexByte(tw.buffer, '\n')
		if idx == -1 {
			break // No complete line yet
		}

		// Extract line including newline
		line := tw.buffer[:idx+1]
		tw.buffer = tw.buffer[idx+1:]

		// Prepend tag byte and write
		tagged := make([]byte, len(line)+1)
		tagged[0] = tw.tag
		copy(tagged[1:], line)

		_, err := tw.w.Write(tagged)
		if err != nil {
			return written, err
		}
		written += len(line)
	}

	return len(p), nil
}

func (e *docker) DestroyStep(ctx context.Context, step *backend.Step, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msgf("stop step %s", step.Name)

	containerName := toContainerName(step)

	if err := e.client.ContainerKill(ctx, containerName, "9"); err != nil && !isErrContainerNotFoundOrNotRunning(err) {
		return err
	}

	if err := e.client.ContainerRemove(ctx, containerName, removeOpts); err != nil && !isErrContainerNotFoundOrNotRunning(err) {
		return err
	}

	return nil
}

func (e *docker) DestroyWorkflow(ctx context.Context, conf *backend.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msgf("delete workflow environment")

	for _, stage := range conf.Stages {
		for _, step := range stage.Steps {
			containerName := toContainerName(step)
			if err := e.client.ContainerKill(ctx, containerName, "9"); err != nil && !isErrContainerNotFoundOrNotRunning(err) {
				log.Error().Err(err).Msgf("could not kill container '%s'", step.Name)
			}
			if err := e.client.ContainerRemove(ctx, containerName, removeOpts); err != nil && !isErrContainerNotFoundOrNotRunning(err) {
				log.Error().Err(err).Msgf("could not remove container '%s'", step.Name)
			}
		}
	}
	if err := e.client.VolumeRemove(ctx, conf.Volume, true); err != nil {
		log.Error().Err(err).Msgf("could not remove volume '%s'", conf.Volume)
	}
	if err := e.client.NetworkRemove(ctx, conf.Network); err != nil {
		// Ignore "not found" errors as the network may have already been removed
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "No such network") {
			log.Debug().Str("network", conf.Network).Msg("Network already removed or not found")
		} else {
			log.Error().Err(err).Msgf("could not remove network '%s'", conf.Network)
		}
	}
	return nil
}

// DiscoverWorkflows finds orphaned workflow containers that can be adopted after agent restart.
// It searches for running containers with the crowci/task-uuid label.
func (e *docker) DiscoverWorkflows(ctx context.Context) ([]*backend.DiscoveredWorkflow, error) {
	log.Trace().Msg("discovering orphaned workflows")

	// List all running containers with our task UUID label
	containers, err := e.client.ContainerList(ctx, container.ListOptions{
		Filters: dockerFilters.NewArgs(
			dockerFilters.Arg("label", LabelTaskUUID),
			dockerFilters.Arg("status", "running"),
		),
	})
	if err != nil {
		return nil, err
	}

	// Group containers by task UUID
	workflowMap := make(map[string]*backend.DiscoveredWorkflow)
	for _, c := range containers {
		taskUUID := c.Labels[LabelTaskUUID]
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
			UUID:        c.Labels[LabelStepUUID],
			Name:        c.Labels[LabelStepName],
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
// Unlike StartStep, this does not create a new container - it verifies the container exists and is running.
func (e *docker) ResumeStep(ctx context.Context, step *backend.Step, taskUUID string) error {
	containerName := toContainerName(step)
	log.Trace().Str("taskUUID", taskUUID).Str("container", containerName).Msgf("resuming step %s", step.Name)

	// Verify the container exists and is running
	info, err := e.client.ContainerInspect(ctx, containerName)
	if err != nil {
		return wrapDaemonError("inspect", containerName, err)
	}

	if !info.State.Running {
		// Container exists but is not running - it may have completed while we were restarting
		log.Debug().
			Str("container", containerName).
			Str("status", info.State.Status).
			Int("exit_code", info.State.ExitCode).
			Msg("container is not running, may have completed during agent restart")
	}

	log.Info().
		Str("taskUUID", taskUUID).
		Str("container", containerName).
		Str("step", step.Name).
		Msg("resumed monitoring of adopted step")

	return nil
}

var removeOpts = container.RemoveOptions{
	RemoveVolumes: true,
	RemoveLinks:   false,
	Force:         false,
}

func isErrContainerNotFoundOrNotRunning(err error) bool {
	return common.IsContainerNotFoundOrNotRunning(err)
}

// normalizeArchType converts the arch type reported by docker info into
// the runtime.GOARCH format
// TODO: find out if we we need to convert other arch types too
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
