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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// waitStepTimeout bounds how long a test lets WaitStep/TailStep run before
// declaring it hung. It is generously above the informer resync period so a
// slow CI machine cannot turn a pass into a flake.
const waitStepTimeout = 30 * time.Second

func runningPod(name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{Name: name, Namespace: "crow"},
		Status: v1.PodStatus{
			Phase:             v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{{Name: name}},
		},
	}
}

func testEngine(objects ...*v1.Pod) (*kube, *fake.Clientset) {
	client := fake.NewSimpleClientset()
	for _, pod := range objects {
		_, _ = client.CoreV1().Pods("crow").Create(context.Background(), pod, meta_v1.CreateOptions{})
	}
	return &kube{client: client, config: &config{Namespace: "crow"}}, client
}

// A pod deleted while WaitStep is watching must be reported through the
// informer's delete handler rather than blocking forever.
func TestWaitStepReturnsWhenPodIsDeleted(t *testing.T) {
	step := &types.Step{UUID: "abc123", Name: "build"}
	podName, err := stepToPodName(step)
	require.NoError(t, err)

	engine, client := testEngine(runningPod(podName))

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	done := make(chan error, 1)
	go func() {
		_, err := engine.WaitStep(ctx, step, "task-uuid")
		done <- err
	}()

	// Give the informer a moment to sync so the deletion is seen as an event
	// rather than by the already-gone lookup, which is the path under test.
	time.Sleep(500 * time.Millisecond)
	require.NoError(t, client.CoreV1().Pods("crow").Delete(ctx, podName, meta_v1.DeleteOptions{}))

	select {
	case err := <-done:
		assert.ErrorContains(t, err, "was deleted before the step completed")
	case <-time.After(waitStepTimeout):
		t.Fatal("WaitStep did not return after the pod was deleted")
	}
}

// A pod deleted before the informer's initial list produces no event at all, so
// WaitStep has to notice it is already gone on its own.
func TestWaitStepReturnsWhenPodIsAlreadyGone(t *testing.T) {
	step := &types.Step{UUID: "abc123", Name: "build"}
	engine, _ := testEngine()

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	done := make(chan error, 1)
	go func() {
		_, err := engine.WaitStep(ctx, step, "task-uuid")
		done <- err
	}()

	select {
	case err := <-done:
		assert.ErrorContains(t, err, "was deleted before the step completed")
	case <-time.After(waitStepTimeout):
		t.Fatal("WaitStep did not return for an already deleted pod")
	}
}

// Canceling the workflow must unblock WaitStep so the agent can release the
// workflow slot instead of leaking it.
func TestWaitStepReturnsOnContextCancel(t *testing.T) {
	step := &types.Step{UUID: "abc123", Name: "build"}
	podName, err := stepToPodName(step)
	require.NoError(t, err)

	engine, _ := testEngine(runningPod(podName))

	ctx, cancel := context.WithCancelCause(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := engine.WaitStep(ctx, step, "task-uuid")
		done <- err
	}()

	time.Sleep(500 * time.Millisecond)
	cancel(nil)

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(waitStepTimeout):
		t.Fatal("WaitStep did not return after the context was canceled")
	}
}

// signalWhenPodGone is what unblocks both WaitStep and TailStep when the pod
// vanished before the informer could list it, so it is tested on its own. The
// fake clientset cannot serve the log stream TailStep opens afterwards, which
// is why TailStep itself is only exercised on the cancellation path below.
func TestSignalWhenPodGone(t *testing.T) {
	step := &types.Step{UUID: "abc123", Name: "build"}
	podName, err := stepToPodName(step)
	require.NoError(t, err)

	tests := []struct {
		name       string
		pods       []*v1.Pod
		wantSignal bool
	}{
		{
			name:       "signals for a pod that no longer exists",
			wantSignal: true,
		},
		{
			name:       "stays quiet while the pod is still there",
			pods:       []*v1.Pod{runningPod(podName)},
			wantSignal: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, _ := testEngine(tc.pods...)

			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)

			si := informers.NewSharedInformerFactoryWithOptions(engine.client, defaultResyncDuration, informers.WithNamespace("crow"))
			informer := si.Core().V1().Pods().Informer()
			stop := make(chan struct{})
			si.Start(stop)
			defer close(stop)

			signaled := make(chan struct{}, 1)
			engine.signalWhenPodGone(ctx, step, podName, informer, stop, func() { signaled <- struct{}{} })

			select {
			case <-signaled:
				assert.True(t, tc.wantSignal, "unexpected signal for a pod that is still running")
			default:
				assert.False(t, tc.wantSignal, "expected a signal for a pod that is gone")
			}
		})
	}
}

// TailStep waits for the pod to come up, so canceling the workflow has to
// unblock it too.
func TestTailStepReturnsOnContextCancel(t *testing.T) {
	step := &types.Step{UUID: "abc123", Name: "build"}
	podName, err := stepToPodName(step)
	require.NoError(t, err)

	pod := runningPod(podName)
	pod.Status.Phase = v1.PodPending
	engine, _ := testEngine(pod)

	ctx, cancel := context.WithCancelCause(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := engine.TailStep(ctx, step, "task-uuid")
		done <- err
	}()

	time.Sleep(500 * time.Millisecond)
	cancel(nil)

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(waitStepTimeout):
		t.Fatal("TailStep did not return after the context was canceled")
	}
}
