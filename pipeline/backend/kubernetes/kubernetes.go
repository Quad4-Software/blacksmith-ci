// Copyright 2022 Woodpecker Authors
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

package kubernetes

import (
	"context"
	std_errs "errors"
	"fmt"
	"io"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // To authenticate to GCP K8s clusters
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
	pipelineErrors "codefloe.com/crowci/crow/v6/pipeline/errors/types"
	"codefloe.com/crowci/crow/v6/shared/yaml"
)

const (
	EngineName = "kubernetes"
	// TODO: 5 seconds is against best practice, k3s didn't work otherwise
	defaultResyncDuration = 5 * time.Second
)

var defaultDeleteOptions = newDefaultDeleteOptions()

type kube struct {
	client kubernetes.Interface
	config *config
	goos   string
}

type config struct {
	Namespace                       string
	EnableNamespacePerOrg           bool
	StorageClass                    string
	VolumeSize                      string
	StorageRwx                      bool
	PodLabels                       map[string]string
	PodLabelsAllowFromStep          bool
	PodAnnotations                  map[string]string
	PodAnnotationsAllowFromStep     bool
	PodNodeSelector                 map[string]string
	PodTolerationsAllowFromStep     bool
	PodTolerations                  []Toleration
	ImagePullSecretNames            []string
	SecurityContext                 SecurityContextConfig
	NativeSecretsAllowFromStep      bool
	PriorityClassName               string
	ServiceAccountName              string
	ServiceAccountNameAllowFromStep bool
}

func (c *config) GetNamespace(orgID int64) string {
	if c.EnableNamespacePerOrg {
		return strings.ToLower(fmt.Sprintf("%s-%s", c.Namespace, strconv.FormatInt(orgID, 10)))
	}
	return c.Namespace
}

// workflowNamespace resolves the namespace for a workflow based on its first
// step's org. It falls back to the default namespace when the workflow has no
// stages/steps, avoiding an index-out-of-range panic.
func (e *kube) workflowNamespace(conf *types.Config) string {
	if len(conf.Stages) > 0 && len(conf.Stages[0].Steps) > 0 {
		return e.config.GetNamespace(conf.Stages[0].Steps[0].OrgID)
	}
	return e.config.Namespace
}

type SecurityContextConfig struct {
	RunAsNonRoot bool
	FSGroup      *int64
}

func newDefaultDeleteOptions() meta_v1.DeleteOptions {
	gracePeriodSeconds := int64(0) // immediately
	propagationPolicy := meta_v1.DeletePropagationBackground

	return meta_v1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
		PropagationPolicy:  &propagationPolicy,
	}
}

func configFromCliContext(ctx context.Context) (*config, error) {
	if ctx != nil {
		if c, ok := ctx.Value(types.CliCommand).(*cli.Command); ok {
			config := config{
				Namespace:                   c.String("backend-k8s-namespace"),
				EnableNamespacePerOrg:       c.Bool("backend-k8s-namespace-per-org"),
				StorageClass:                c.String("backend-k8s-storage-class"),
				VolumeSize:                  c.String("backend-k8s-volume-size"),
				StorageRwx:                  c.Bool("backend-k8s-storage-rwx"),
				PriorityClassName:           c.String("backend-k8s-priority-class"),
				PodLabels:                   make(map[string]string), // just init empty map to prevent nil panic
				PodLabelsAllowFromStep:      c.Bool("backend-k8s-pod-labels-allow-from-step"),
				PodAnnotations:              make(map[string]string), // just init empty map to prevent nil panic
				PodAnnotationsAllowFromStep: c.Bool("backend-k8s-pod-annotations-allow-from-step"),
				PodTolerationsAllowFromStep: c.Bool("backend-k8s-pod-tolerations-allow-from-step"),
				PodNodeSelector:             make(map[string]string), // just init empty map to prevent nil panic
				ImagePullSecretNames:        c.StringSlice("backend-k8s-pod-image-pull-secret-names"),
				SecurityContext: SecurityContextConfig{
					RunAsNonRoot: c.Bool("backend-k8s-secctx-nonroot"), // cspell:words secctx nonroot
					FSGroup:      newInt64(defaultFSGroup),
				},
				NativeSecretsAllowFromStep:      c.Bool("backend-k8s-allow-native-secrets"),
				ServiceAccountName:              c.String("backend-k8s-service-account-name"),
				ServiceAccountNameAllowFromStep: c.Bool("backend-k8s-service-account-name-allow-from-step"),
			}
			// Unmarshal label and annotation settings here to ensure they're valid on startup
			if labels := c.String("backend-k8s-pod-labels"); labels != "" {
				if err := yaml.Unmarshal([]byte(labels), &config.PodLabels); err != nil {
					log.Error().Err(err).Msgf("could not unmarshal pod labels '%s'", c.String("backend-k8s-pod-labels"))
					return nil, err
				}
			}
			if annotations := c.String("backend-k8s-pod-annotations"); annotations != "" {
				if err := yaml.Unmarshal([]byte(c.String("backend-k8s-pod-annotations")), &config.PodAnnotations); err != nil {
					log.Error().Err(err).Msgf("could not unmarshal pod annotations '%s'", c.String("backend-k8s-pod-annotations"))
					return nil, err
				}
			}
			if nodeSelector := c.String("backend-k8s-pod-node-selector"); nodeSelector != "" {
				if err := yaml.Unmarshal([]byte(nodeSelector), &config.PodNodeSelector); err != nil {
					log.Error().Err(err).Msgf("could not unmarshal pod node selector '%s'", nodeSelector)
					return nil, err
				}
			}
			if podTolerations := c.String("backend-k8s-pod-tolerations"); podTolerations != "" {
				if err := yaml.Unmarshal([]byte(podTolerations), &config.PodTolerations); err != nil {
					log.Error().Err(err).Msgf("could not unmarshal pod tolerations '%s'", podTolerations)
					return nil, err
				}
			}

			return &config, nil
		}
	}

	return nil, types.ErrNoCliContextFound
}

// New returns a new Kubernetes Backend.
func New() types.Backend {
	return &kube{}
}

func (e *kube) Name() string {
	return EngineName
}

func (e *kube) IsAvailable(context.Context) bool {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	return len(host) > 0
}

func (e *kube) Flags() []cli.Flag {
	return Flags
}

func (e *kube) Load(ctx context.Context) (*types.BackendInfo, error) {
	config, err := configFromCliContext(ctx)
	if err != nil {
		return nil, err
	}
	e.config = config

	var kubeClient kubernetes.Interface
	_, err = rest.InClusterConfig()
	if err != nil {
		kubeClient, err = getClientOutOfCluster()
	} else {
		kubeClient, err = getClientInsideOfCluster()
	}

	if err != nil {
		return nil, err
	}

	e.client = kubeClient

	// TODO(2693): use info resp of kubeClient to define platform var
	e.goos = runtime.GOOS
	return &types.BackendInfo{
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}, nil
}

func (e *kube) getConfig() *config {
	if e.config == nil {
		return nil
	}
	c := *e.config
	c.PodLabels = maps.Clone(e.config.PodLabels)
	c.PodAnnotations = maps.Clone(e.config.PodAnnotations)
	c.PodNodeSelector = maps.Clone(e.config.PodNodeSelector)
	c.ImagePullSecretNames = slices.Clone(e.config.ImagePullSecretNames)
	return &c
}

// SetupWorkflow sets up the pipeline environment.
func (e *kube) SetupWorkflow(ctx context.Context, conf *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msgf("Setting up Kubernetes primitives")

	if len(conf.TempVolumes) > 0 {
		return fmt.Errorf("the kubernetes backend does not support `temp_volumes`")
	}

	namespace := e.workflowNamespace(conf)

	if e.config.EnableNamespacePerOrg {
		err := mkNamespace(ctx, e.client.CoreV1().Namespaces(), namespace)
		if err != nil {
			return err
		}
	}

	_, err := startVolume(ctx, e, conf.Volume, namespace)
	if err != nil {
		return err
	}

	var extraHosts []types.HostAlias
	for _, stage := range conf.Stages {
		for _, step := range stage.Steps {
			if isService(step) {
				svc, err := startService(ctx, e, step)
				if err != nil {
					return &pipelineErrors.ErrInvalidWorkflowSetup{
						Err:  err,
						Step: step,
					}
				}
				hostAlias := types.HostAlias{Name: step.Networks[0].Aliases[0], IP: svc.Spec.ClusterIP}
				extraHosts = append(extraHosts, hostAlias)
			}
		}
	}
	log.Trace().Msgf("adding extra hosts: %v", extraHosts)
	for _, stage := range conf.Stages {
		for _, step := range stage.Steps {
			step.ExtraHosts = extraHosts
		}
	}

	return nil
}

// StartStep starts the pipeline step.
func (e *kube) StartStep(ctx context.Context, step *types.Step, taskUUID string) error {
	options, err := parseBackendOptions(step)
	if err != nil {
		log.Error().Err(err).Msg("could not parse backend options")
	}

	if needsRegistrySecret(step) {
		err = startRegistrySecret(ctx, e, step)
		if err != nil {
			return err
		}
	}

	if needsStepSecret(step) {
		err = startStepSecret(ctx, e, step)
		if err != nil {
			return err
		}
	}

	log.Trace().Str("taskUUID", taskUUID).Msgf("starting step: %s", step.Name)
	_, err = startPod(ctx, e, step, options, taskUUID)
	return err
}

// signalWhenPodGone calls signal if the step pod no longer exists once the
// informer's cache has synced.
//
// An informer only emits delete events for objects it saw in its initial list,
// and its resync only replays objects still held in the store. A pod deleted
// between StartStep and the initial list therefore produces no event at all,
// which would leave WaitStep and TailStep blocked until the workflow is
// canceled. Waiting for the sync before the lookup is what makes this
// airtight: it guarantees the lookup happens after the list, so a deletion is
// caught either by the delete handler or here, never by neither.
//
// It is meant to run in its own goroutine; both the cache sync and the lookup
// abort once stop is closed or ctx is done, so it cannot outlive its caller.
func (e *kube) signalWhenPodGone(ctx context.Context, step *types.Step, podName string, informer cache.SharedIndexInformer, stop <-chan struct{}, signal func()) {
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		return
	}

	_, err := e.client.CoreV1().Pods(e.config.GetNamespace(step.OrgID)).Get(ctx, podName, meta_v1.GetOptions{})
	if errors.IsNotFound(err) {
		log.Debug().Str("pod", podName).Msg("step pod is already gone, treating it as terminal")
		signal()
	}
}

// WaitStep waits for the pipeline step to complete and returns
// the completion results.
func (e *kube) WaitStep(ctx context.Context, step *types.Step, taskUUID string) (*types.State, error) {
	podName, err := stepToPodName(step)
	if err != nil {
		return nil, err
	}

	log.Trace().Str("taskUUID", taskUUID).Msgf("waiting for pod: %s", podName)

	finished := make(chan bool, 1)
	signalFinished := func() {
		select {
		case finished <- true:
		default:
		}
	}

	podUpdated := func(_, newPod any) {
		pod, ok := newPod.(*v1.Pod)
		if !ok {
			log.Error().Msgf("could not parse pod: %v", newPod)
			return
		}

		if pod.Name == podName {
			if isImagePullBackOffState(pod) || isInvalidImageName(pod) {
				signalFinished()
			}

			switch pod.Status.Phase {
			case v1.PodSucceeded, v1.PodFailed, v1.PodUnknown:
				signalFinished()
			}
		}
	}

	podDeleted := func(obj any) {
		pod, ok := obj.(*v1.Pod)
		if !ok {
			if unknown, isUnknown := obj.(cache.DeletedFinalStateUnknown); isUnknown {
				pod, ok = unknown.Obj.(*v1.Pod)
			}
			if !ok {
				log.Error().Msgf("could not parse deleted pod: %v", obj)
				return
			}
		}

		if pod.Name == podName {
			signalFinished()
		}
	}

	si := informers.NewSharedInformerFactoryWithOptions(e.client, defaultResyncDuration, informers.WithNamespace(e.config.GetNamespace(step.OrgID)))
	informer := si.Core().V1().Pods().Informer()
	if _, err := informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: podUpdated,
			DeleteFunc: podDeleted,
		},
	); err != nil {
		return nil, err
	}

	stop := make(chan struct{})
	si.Start(stop)
	defer close(stop)

	go e.signalWhenPodGone(ctx, step, podName, informer, stop, signalFinished)

	select {
	case <-finished:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	pod, err := e.client.CoreV1().Pods(e.config.GetNamespace(step.OrgID)).Get(ctx, podName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("pod %s was deleted before the step completed", podName)
		}
		return nil, err
	}

	if isImagePullBackOffState(pod) || isInvalidImageName(pod) {
		return nil, fmt.Errorf("could not pull image for pod %s", podName)
	}

	if len(pod.Status.ContainerStatuses) == 0 {
		return nil, fmt.Errorf("no container statuses found for pod %s", podName)
	}

	cs := pod.Status.ContainerStatuses[0]

	if cs.State.Terminated == nil {
		err := fmt.Errorf("no terminated state found for container %s/%s", podName, cs.Name)
		log.Error().Str("taskUUID", taskUUID).Str("pod", podName).Str("container", cs.Name).Interface("state", cs.State).Msg(err.Error())
		return nil, err
	}

	bs := &types.State{
		ExitCode:  int(cs.State.Terminated.ExitCode),
		Exited:    true,
		OOMKilled: false,
	}

	return bs, nil
}

// TailStep tails the pipeline step logs.
func (e *kube) TailStep(ctx context.Context, step *types.Step, taskUUID string) (io.ReadCloser, error) {
	podName, err := stepToPodName(step)
	if err != nil {
		return nil, err
	}

	log.Trace().Str("taskUUID", taskUUID).Msgf("tail logs of pod: %s", podName)

	up := make(chan bool, 1)
	signalUp := func() {
		select {
		case up <- true:
		default:
		}
	}

	podUpdated := func(_, newPod any) {
		pod, ok := newPod.(*v1.Pod)
		if !ok {
			log.Error().Msgf("could not parse pod: %v", newPod)
			return
		}

		if pod.Name == podName {
			if isImagePullBackOffState(pod) || isInvalidImageName(pod) {
				signalUp()
			}
			switch pod.Status.Phase {
			case v1.PodRunning, v1.PodSucceeded, v1.PodFailed:
				signalUp()
			}
		}
	}

	podDeleted := func(obj any) {
		pod, ok := obj.(*v1.Pod)
		if !ok {
			if unknown, isUnknown := obj.(cache.DeletedFinalStateUnknown); isUnknown {
				pod, ok = unknown.Obj.(*v1.Pod)
			}
			if !ok {
				log.Error().Msgf("could not parse deleted pod: %v", obj)
				return
			}
		}

		if pod.Name == podName {
			signalUp()
		}
	}

	si := informers.NewSharedInformerFactoryWithOptions(e.client, defaultResyncDuration, informers.WithNamespace(e.config.GetNamespace(step.OrgID)))
	informer := si.Core().V1().Pods().Informer()
	if _, err := informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: podUpdated,
			DeleteFunc: podDeleted,
		},
	); err != nil {
		return nil, err
	}

	stop := make(chan struct{})
	si.Start(stop)
	defer close(stop)

	go e.signalWhenPodGone(ctx, step, podName, informer, stop, signalUp)

	select {
	case <-up:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	opts := &v1.PodLogOptions{
		Follow:    true,
		Container: podName,
	}

	logs, err := e.client.CoreV1().RESTClient().Get().
		Namespace(e.config.GetNamespace(step.OrgID)).
		Name(podName).
		Resource("pods").
		SubResource("log").
		VersionedParams(opts, scheme.ParameterCodec).
		Stream(ctx)
	if err != nil {
		return nil, err
	}
	rc, wc := io.Pipe()

	go func() {
		defer logs.Close()
		defer wc.Close()

		_, err = io.Copy(wc, logs)
		if err != nil {
			return
		}
	}()
	return rc, nil
}

func (e *kube) DestroyStep(ctx context.Context, step *types.Step, taskUUID string) error {
	var errs []error
	log.Trace().Str("taskUUID", taskUUID).Msgf("Stopping step: %s", step.Name)
	if needsRegistrySecret(step) {
		err := stopRegistrySecret(ctx, e, step, defaultDeleteOptions)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if needsStepSecret(step) {
		err := stopStepSecret(ctx, e, step, defaultDeleteOptions)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err := stopPod(ctx, e, step, defaultDeleteOptions)
	if err != nil {
		errs = append(errs, err)
	}
	return std_errs.Join(errs...)
}

// DestroyWorkflow destroys the pipeline environment.
func (e *kube) DestroyWorkflow(ctx context.Context, conf *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("deleting Kubernetes primitives")

	var errs []error
	for _, stage := range conf.Stages {
		for _, step := range stage.Steps {
			if err := stopPod(ctx, e, step, defaultDeleteOptions); err != nil {
				errs = append(errs, err)
			}

			if isService(step) {
				if err := stopService(ctx, e, step, defaultDeleteOptions); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if err := stopVolume(ctx, e, conf.Volume, e.workflowNamespace(conf), defaultDeleteOptions); err != nil {
		errs = append(errs, err)
	}

	return std_errs.Join(errs...)
}

// DiscoverWorkflows finds orphaned workflow pods that can be adopted after agent restart.
// It searches for running pods with the crowci/task-uuid label.
func (e *kube) DiscoverWorkflows(ctx context.Context) ([]*types.DiscoveredWorkflow, error) {
	log.Trace().Msg("discovering orphaned workflows")

	// List all running pods with our task UUID label
	pods, err := e.client.CoreV1().Pods(e.config.Namespace).List(ctx, meta_v1.ListOptions{
		LabelSelector: TaskUUIDLabel,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, err
	}

	// Group pods by task UUID
	workflowMap := make(map[string]*types.DiscoveredWorkflow)
	for _, pod := range pods.Items {
		taskUUID := pod.Labels[TaskUUIDLabel]
		if taskUUID == "" {
			continue
		}

		workflow, exists := workflowMap[taskUUID]
		if !exists {
			workflow = &types.DiscoveredWorkflow{
				TaskUUID: taskUUID,
				Steps:    []*types.DiscoveredStep{},
			}
			workflowMap[taskUUID] = workflow
		}

		workflow.Steps = append(workflow.Steps, &types.DiscoveredStep{
			UUID:        pod.Labels["wp_uuid"], // legacy label for step UUID
			Name:        pod.Labels[StepLabel],
			ContainerID: string(pod.UID),
		})
	}

	// Convert map to slice
	workflows := make([]*types.DiscoveredWorkflow, 0, len(workflowMap))
	for _, workflow := range workflowMap {
		workflows = append(workflows, workflow)
	}

	log.Debug().Int("count", len(workflows)).Msg("discovered orphaned workflows")
	return workflows, nil
}

// ResumeStep resumes monitoring of an already-running step (for adopted workflows).
// Unlike StartStep, this does not create a new pod - it verifies the pod exists and is running.
func (e *kube) ResumeStep(ctx context.Context, step *types.Step, taskUUID string) error {
	podName, err := stepToPodName(step)
	if err != nil {
		return err
	}

	log.Trace().Str("taskUUID", taskUUID).Str("pod", podName).Msgf("resuming step %s", step.Name)

	// Verify the pod exists and is running
	pod, err := e.client.CoreV1().Pods(e.config.GetNamespace(step.OrgID)).Get(ctx, podName, meta_v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s: %w", podName, err)
	}

	if pod.Status.Phase != v1.PodRunning {
		// Pod exists but is not running - it may have completed while we were restarting
		log.Debug().
			Str("pod", podName).
			Str("phase", string(pod.Status.Phase)).
			Msg("pod is not running, may have completed during agent restart")
	}

	log.Info().
		Str("taskUUID", taskUUID).
		Str("pod", podName).
		Str("step", step.Name).
		Msg("resumed monitoring of adopted step")

	return nil
}
