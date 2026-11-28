// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestPodObjectToPortEndpoint(t *testing.T) {
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/pod-2-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "namespace/pod-2-UID/container-2",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:        "container-2",
				Image:       "container-image-2",
				ContainerID: "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				Kind:        "app",
				Running:     true,
				Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
		{
			ID:     "namespace/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https", Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:           443,
				Transport:      observer.ProtocolTCP,
				ContainerName:  "container-2",
				ContainerID:    "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				ContainerImage: "container-image-2",
			},
		},
	}

	endpoints := convertPodToEndpoints("namespace", podWithNamedPorts, podEndpointOptions{})
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectToPortEndpointIPv6(t *testing.T) {
	pod := podWithNamedPorts.DeepCopy()
	pod.Status.PodIP = "2001:db8::10"

	endpoints := convertPodToEndpoints("namespace", pod, podEndpointOptions{})
	require.Len(t, endpoints, 3)

	endpoint := endpoints[2]
	require.Equal(t, "[2001:db8::10]:443", endpoint.Target)

	env, err := endpoint.Env()
	require.NoError(t, err)
	require.Equal(t, "[2001:db8::10]:443", env["endpoint"])
	require.Equal(t, "2001:db8::10", env["host"])
	require.Equal(t, string(observer.PortType), env["type"])
	require.Equal(t, uint16(443), env["port"])
}

func TestPodEndpointsEphemeralContainers(t *testing.T) {
	pod := podWithNamedPorts.DeepCopy()
	pod.Spec.EphemeralContainers = []v1.EphemeralContainer{
		{EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "debugger", Image: "debug-image"}},
		{EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "old-debugger", Image: "debug-image"}},
	}
	pod.Status.EphemeralContainerStatuses = []v1.ContainerStatus{
		{Name: "debugger", ContainerID: "containerd://debugger-id", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
		{Name: "old-debugger", ContainerID: "containerd://old-debugger-id", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}}},
	}

	// Without the option the ephemeral containers get no endpoint.
	require.Len(t, convertPodToEndpoints("namespace", pod, podEndpointOptions{}), 3)

	// With it the one that runs gets a container endpoint.
	endpoints := convertPodToEndpoints("namespace", pod, podEndpointOptions{ephemeralContainers: true})
	require.Len(t, endpoints, 4)
	require.Equal(t, observer.Endpoint{
		ID:     "namespace/pod-2-UID/debugger",
		Target: "1.2.3.4",
		Details: &observer.PodContainer{
			Name:        "debugger",
			Image:       "debug-image",
			ContainerID: "debugger-id",
			Kind:        "ephemeral",
			Running:     true,
			Pod: observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
	}, endpoints[3])
	require.Equal(t, "app", endpoints[1].Details.(*observer.PodContainer).Kind)

	env, err := endpoints[3].Env()
	require.NoError(t, err)
	require.Equal(t, "ephemeral", env["container_kind"])

	// The spec of the pod is not changed.
	require.Len(t, pod.Spec.Containers, len(podWithNamedPorts.Spec.Containers))
}

func endpointIDs(endpoints []observer.Endpoint) []string {
	ids := make([]string, 0, len(endpoints))
	for _, e := range endpoints {
		ids = append(ids, string(e.ID))
	}
	return ids
}

func containerStatus(name, containerID string, state v1.ContainerState) v1.ContainerStatus {
	return v1.ContainerStatus{Name: name, ContainerID: containerID, State: state}
}

var (
	stateRunning    = v1.ContainerState{Running: &v1.ContainerStateRunning{}}
	stateTerminated = v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 1}}
	stateWaiting    = v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "PodInitializing"}}
)

// podWithInitContainers has an init container that finished, a native sidecar that runs and an
// app container that runs.
func podWithInitContainers() *v1.Pod {
	pod := newPod("pod-3", "localhost")
	pod.Spec.InitContainers = []v1.Container{
		{Name: "init", Image: "init-image"},
		{Name: "sidecar", Image: "sidecar-image", Ports: []v1.ContainerPort{{Name: "metrics", ContainerPort: 9090, Protocol: v1.ProtocolTCP}}},
	}
	pod.Spec.Containers = []v1.Container{{Name: "app", Image: "app-image"}}
	pod.Status.InitContainerStatuses = []v1.ContainerStatus{
		containerStatus("init", "containerd://init-id", stateTerminated),
		containerStatus("sidecar", "containerd://sidecar-id", stateRunning),
	}
	pod.Status.ContainerStatuses = []v1.ContainerStatus{
		containerStatus("app", "containerd://app-id", stateRunning),
	}
	return pod
}

func TestPodEndpointsWithoutOptionsSkipInitContainers(t *testing.T) {
	endpoints := convertPodToEndpoints("namespace", podWithInitContainers(), podEndpointOptions{})
	require.Equal(t, []string{
		"namespace/pod-3-UID",
		"namespace/pod-3-UID/app",
	}, endpointIDs(endpoints))
}

func TestPodEndpointsInitContainers(t *testing.T) {
	opts := podEndpointOptions{initContainers: true}

	// The init container that finished is not reported, as with the other containers.
	endpoints := convertPodToEndpoints("namespace", podWithInitContainers(), opts)
	require.Equal(t, []string{
		"namespace/pod-3-UID",
		"namespace/pod-3-UID/sidecar",
		"namespace/pod-3-UID/metrics(9090)",
		"namespace/pod-3-UID/app",
	}, endpointIDs(endpoints))
	require.Equal(t, &observer.PodContainer{
		Name:        "sidecar",
		Image:       "sidecar-image",
		ContainerID: "sidecar-id",
		Kind:        "init",
		Running:     true,
		Pod: observer.Pod{
			Name:      "pod-3",
			Namespace: "default",
			UID:       "pod-3-UID",
			Labels:    map[string]string{"env": "prod"},
		},
	}, endpoints[1].Details)
	require.Equal(t, "app", endpoints[3].Details.(*observer.PodContainer).Kind)

	env, err := endpoints[1].Env()
	require.NoError(t, err)
	require.Equal(t, "init", env["container_kind"])
}

func TestPodEndpointsInitContainersOfPendingPod(t *testing.T) {
	// While the init containers run the pod is still pending.
	pending := podWithInitContainers()
	pending.Status.Phase = v1.PodPending
	pending.Status.InitContainerStatuses[0].State = stateRunning
	pending.Status.InitContainerStatuses[1] = containerStatus("sidecar", "", stateWaiting)
	pending.Status.ContainerStatuses[0] = containerStatus("app", "", stateWaiting)

	require.Empty(t, convertPodToEndpoints("namespace", pending, podEndpointOptions{}))
	require.Equal(t, []string{"namespace/pod-3-UID/init"},
		endpointIDs(convertPodToEndpoints("namespace", pending, podEndpointOptions{initContainers: true})))
}

func TestPodEndpointsInitContainersOfFinishedPod(t *testing.T) {
	// The status of a native sidecar can still say running for a moment after the pod finished.
	for _, phase := range []v1.PodPhase{v1.PodSucceeded, v1.PodFailed} {
		finished := podWithInitContainers()
		finished.Status.Phase = phase
		require.Empty(t, convertPodToEndpoints("namespace", finished, podEndpointOptions{initContainers: true}), phase)
	}
}

func TestPodEndpointsPortsNeedPodIP(t *testing.T) {
	// A native sidecar runs while the pod is pending. Its port gets an endpoint only once the
	// pod has an address.
	pod := podWithInitContainers()
	pod.Status.Phase = v1.PodPending
	pod.Status.ContainerStatuses[0] = containerStatus("app", "", stateWaiting)
	opts := podEndpointOptions{initContainers: true}

	require.Equal(t, []string{
		"namespace/pod-3-UID/sidecar",
		"namespace/pod-3-UID/metrics(9090)",
	}, endpointIDs(convertPodToEndpoints("namespace", pod, opts)))

	pod.Status.PodIP = ""
	endpoints := convertPodToEndpoints("namespace", pod, opts)
	require.Equal(t, []string{"namespace/pod-3-UID/sidecar"}, endpointIDs(endpoints))
	require.Empty(t, endpoints[0].Target)
}

func TestPodEndpointsContainerKindAndState(t *testing.T) {
	pod := podWithInitContainers()
	pod.Spec.EphemeralContainers = []v1.EphemeralContainer{
		{EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "debugger", Image: "debug-image"}},
		{EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "old-debugger", Image: "debug-image"}},
	}
	pod.Status.EphemeralContainerStatuses = []v1.ContainerStatus{
		containerStatus("debugger", "containerd://debugger-id", stateRunning),
		containerStatus("old-debugger", "containerd://old-debugger-id", stateTerminated),
	}

	type container struct {
		name    string
		kind    string
		running bool
	}
	containers := func(opts podEndpointOptions) []container {
		var found []container
		for _, e := range convertPodToEndpoints("namespace", pod, opts) {
			if details, ok := e.Details.(*observer.PodContainer); ok {
				found = append(found, container{details.Name, details.Kind, details.Running})
			}
		}
		return found
	}

	require.Equal(t, []container{{"app", "app", true}}, containers(podEndpointOptions{}))
	require.Equal(t, []container{
		{"app", "app", true},
		{"debugger", "ephemeral", true},
	}, containers(podEndpointOptions{ephemeralContainers: true}))
	require.Equal(t, []container{
		{"init", "init", false},
		{"sidecar", "init", true},
		{"app", "app", true},
		{"debugger", "ephemeral", true},
		{"old-debugger", "ephemeral", false},
	}, containers(podEndpointOptions{initContainers: true, ephemeralContainers: true, terminatedContainers: true}))

	env, err := convertPodToEndpoints("namespace", pod, podEndpointOptions{initContainers: true, terminatedContainers: true})[1].Env()
	require.NoError(t, err)
	require.Equal(t, "init", env["container_kind"])
	require.Equal(t, false, env["container_running"])
}

func TestContainerFinishTimes(t *testing.T) {
	finishedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	terminated := v1.ContainerState{Terminated: &v1.ContainerStateTerminated{FinishedAt: metav1.NewTime(finishedAt)}}

	pod := newPod("pod-5", "localhost")
	pod.Status.InitContainerStatuses = []v1.ContainerStatus{containerStatus("init", "containerd://init-id", terminated)}
	pod.Status.ContainerStatuses = []v1.ContainerStatus{
		containerStatus("running", "containerd://running-id", stateRunning),
		// Restarted after a crash: the crash does not count.
		{Name: "restarted", State: stateRunning, LastTerminationState: terminated},
		{Name: "backoff", State: stateWaiting, LastTerminationState: terminated},
		containerStatus("never-started", "", stateWaiting),
		containerStatus("no-finish-time", "containerd://no-finish-time-id", stateTerminated),
	}
	pod.Status.EphemeralContainerStatuses = []v1.ContainerStatus{containerStatus("debugger", "containerd://debugger-id", terminated)}

	require.Equal(t, map[observer.EndpointID]time.Time{
		"namespace/pod-5-UID/init":     finishedAt,
		"namespace/pod-5-UID/backoff":  finishedAt,
		"namespace/pod-5-UID/debugger": finishedAt,
	}, containerFinishTimes("namespace", pod))
}

func TestPodEndpointsTerminatedContainers(t *testing.T) {
	opts := podEndpointOptions{terminatedContainers: true}

	pod := newPod("pod-4", "localhost")
	pod.Spec.Containers = []v1.Container{
		{Name: "crashed", Image: "image", Ports: []v1.ContainerPort{{Name: "http", ContainerPort: 80, Protocol: v1.ProtocolTCP}}},
		{Name: "backoff", Image: "image"},
		{Name: "never-started", Image: "image"},
		{Name: "running", Image: "image"},
	}
	pod.Status.ContainerStatuses = []v1.ContainerStatus{
		containerStatus("crashed", "containerd://crashed-id", stateTerminated),
		containerStatus("backoff", "containerd://backoff-id", stateWaiting),
		containerStatus("never-started", "", stateWaiting),
		containerStatus("running", "containerd://running-id", stateRunning),
	}

	require.Equal(t, []string{
		"namespace/pod-4-UID",
		"namespace/pod-4-UID/running",
	}, endpointIDs(convertPodToEndpoints("namespace", pod, podEndpointOptions{})))

	// Containers that ran get a container endpoint, without port endpoints.
	endpoints := convertPodToEndpoints("namespace", pod, opts)
	require.Equal(t, []string{
		"namespace/pod-4-UID",
		"namespace/pod-4-UID/crashed",
		"namespace/pod-4-UID/backoff",
		"namespace/pod-4-UID/running",
	}, endpointIDs(endpoints))
	require.Equal(t, "crashed-id", endpoints[1].Details.(*observer.PodContainer).ContainerID)

	// A pod that finished keeps its container endpoints and has no pod endpoint.
	for _, phase := range []v1.PodPhase{v1.PodSucceeded, v1.PodFailed} {
		finished := pod.DeepCopy()
		finished.Status.Phase = phase
		finished.Status.ContainerStatuses[3].State = stateTerminated
		require.Empty(t, convertPodToEndpoints("namespace", finished, podEndpointOptions{}))
		require.Equal(t, []string{
			"namespace/pod-4-UID/crashed",
			"namespace/pod-4-UID/backoff",
			"namespace/pod-4-UID/running",
		}, endpointIDs(convertPodToEndpoints("namespace", finished, opts)))
	}
}

func TestPodEndpointsTerminatedInitContainers(t *testing.T) {
	endpoints := convertPodToEndpoints("namespace", podWithInitContainers(), podEndpointOptions{initContainers: true, terminatedContainers: true})
	require.Equal(t, []string{
		"namespace/pod-3-UID",
		"namespace/pod-3-UID/init",
		"namespace/pod-3-UID/sidecar",
		"namespace/pod-3-UID/metrics(9090)",
		"namespace/pod-3-UID/app",
	}, endpointIDs(endpoints))
}

func TestPodEndpointsTerminatedContainersKeepPortsOfRunningPodsOnly(t *testing.T) {
	opts := podEndpointOptions{initContainers: true, terminatedContainers: true}

	pod := podWithInitContainers()
	pod.Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "http", ContainerPort: 80, Protocol: v1.ProtocolTCP}}
	pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{Name: "pulling", Image: "image"})
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, containerStatus("pulling", "", stateWaiting))

	// A pending pod of which an app container already runs gets no port endpoint for it, as
	// without the option. The port of a native sidecar is reported as without the option too.
	pending := pod.DeepCopy()
	pending.Status.Phase = v1.PodPending
	endpoints := convertPodToEndpoints("namespace", pending, opts)
	require.Equal(t, []string{
		"namespace/pod-3-UID/init",
		"namespace/pod-3-UID/sidecar",
		"namespace/pod-3-UID/metrics(9090)",
		"namespace/pod-3-UID/app",
	}, endpointIDs(endpoints))
	require.True(t, endpoints[3].Details.(*observer.PodContainer).Running)

	unknown := pod.DeepCopy()
	unknown.Status.Phase = v1.PodUnknown
	require.Equal(t, []string{
		"namespace/pod-3-UID/init",
		"namespace/pod-3-UID/sidecar",
		"namespace/pod-3-UID/app",
	}, endpointIDs(convertPodToEndpoints("namespace", unknown, opts)))

	// The status of a container can still say running after the pod finished, as when the pod
	// was evicted. Its containers do not run then.
	for _, phase := range []v1.PodPhase{v1.PodSucceeded, v1.PodFailed} {
		finished := pod.DeepCopy()
		finished.Status.Phase = phase
		endpoints = convertPodToEndpoints("namespace", finished, opts)
		require.Equal(t, []string{
			"namespace/pod-3-UID/init",
			"namespace/pod-3-UID/sidecar",
			"namespace/pod-3-UID/app",
		}, endpointIDs(endpoints), phase)
		for _, endpoint := range endpoints {
			require.False(t, endpoint.Details.(*observer.PodContainer).Running, "%s of a pod that is %s", endpoint.ID, phase)
		}
	}
}

func TestPodEndpointsPortsOfRunningPodWithoutAddress(t *testing.T) {
	// The port of a container in a running pod is reported whether the pod has an address or not.
	pod := podWithInitContainers()
	pod.Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "http", ContainerPort: 80, Protocol: v1.ProtocolTCP}}
	pod.Status.PodIP = ""
	require.Equal(t, []string{
		"namespace/pod-3-UID",
		"namespace/pod-3-UID/app",
		"namespace/pod-3-UID/http(80)",
	}, endpointIDs(convertPodToEndpoints("namespace", pod, podEndpointOptions{})))
}
