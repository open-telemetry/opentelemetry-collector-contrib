// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func newTestHandler() *handler {
	h := &handler{
		idNamespace: "test-1",
		endpoints:   &sync.Map{},
		now:         time.Now,
		logger:      zap.NewNop(),
	}
	return h
}

func TestPodEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(podWithNamedPorts, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "test-1/pod-2-UID/container-2",
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
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https",
				Pod: observer.Pod{
					Namespace: "default",
					UID:       "pod-2-UID",
					Name:      "pod-2",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:           443,
				Transport:      observer.ProtocolTCP,
				ContainerName:  "container-2",
				ContainerID:    "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				ContainerImage: "container-image-2",
			},
		},
	}, th.ListEndpoints())
}

func TestPodEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(podWithNamedPorts, true)
	th.OnDelete(podWithNamedPorts)
	assert.Empty(t, th.ListEndpoints())
}

func TestPodEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(podWithNamedPorts, podWithNamedPorts)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := podWithNamedPorts.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(podWithNamedPorts, changedLabels)

	endpoints := th.ListEndpoints()
	require.ElementsMatch(t,
		[]observer.EndpointID{"test-1/pod-2-UID", "test-1/pod-2-UID/container-2", "test-1/pod-2-UID/https(443)"},
		[]observer.EndpointID{endpoints[0].ID, endpoints[1].ID, endpoints[2].ID},
	)

	// Running state changed, one added and one removed.
	updatedPod := podWithNamedPorts.DeepCopy()
	updatedPod.Labels["updated-label"] = "true"
	th.OnUpdate(podWithNamedPorts, updatedPod)
	require.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod", "updated-label": "true"},
			},
		},
		{
			ID:     "test-1/pod-2-UID/container-2",
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
					Labels:    map[string]string{"env": "prod", "updated-label": "true"},
				},
			},
		},
		{
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https", Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod", "updated-label": "true"},
				},
				Port:           443,
				Transport:      observer.ProtocolTCP,
				ContainerName:  "container-2",
				ContainerID:    "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				ContainerImage: "container-image-2",
			},
		},
	}, th.ListEndpoints())
}

func TestServiceEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(serviceWithClusterIP, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/service-1-UID",
			Target: "service-1.default.svc.cluster.local",
			Details: &observer.K8sService{
				Name:        "service-1",
				Namespace:   "default",
				UID:         "service-1-UID",
				Labels:      map[string]string{"env": "prod"},
				ServiceType: "ClusterIP",
				ClusterIP:   "1.2.3.4",
			},
		},
	}, th.ListEndpoints())
}

func TestServiceEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(serviceWithClusterIP, true)
	th.OnDelete(serviceWithClusterIP)
	assert.Empty(t, th.ListEndpoints())
}

func TestServiceEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(serviceWithClusterIP, serviceWithClusterIP)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := serviceWithClusterIP.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(serviceWithClusterIP, changedLabels)

	endpoints := th.ListEndpoints()
	require.ElementsMatch(t,
		[]observer.EndpointID{"test-1/service-1-UID"},
		[]observer.EndpointID{endpoints[0].ID},
	)

	// Running state changed, one added and one removed.
	updatedService := serviceWithClusterIP.DeepCopy()
	updatedService.Labels["updated-label"] = "true"
	th.OnUpdate(serviceWithClusterIP, updatedService)
	require.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/service-1-UID",
			Target: "service-1.default.svc.cluster.local",
			Details: &observer.K8sService{
				Name:        "service-1",
				Namespace:   "default",
				UID:         "service-1-UID",
				Labels:      map[string]string{"env": "prod", "updated-label": "true"},
				ServiceType: "ClusterIP",
				ClusterIP:   "1.2.3.4",
			},
		},
	}, th.ListEndpoints())
}

func TestIngressEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(ingress, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/ingress-1-UID/host-1/",
			Target: "https://host-1/",
			Details: &observer.K8sIngress{
				Name:      "application-ingress",
				Namespace: "default",
				UID:       "test-1/ingress-1-UID/host-1/",
				Labels:    map[string]string{"env": "prod"},
				Scheme:    "https",
				Host:      "host-1",
				Path:      "/",
			},
		},
	}, th.ListEndpoints())
}

func TestIngressEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(ingress, true)
	th.OnDelete(ingress)
	assert.Empty(t, th.ListEndpoints())
}

func TestIngressEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(ingress, ingress)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := ingress.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(ingress, changedLabels)

	endpoints := th.ListEndpoints()
	require.ElementsMatch(t,
		[]observer.EndpointID{"test-1/ingress-1-UID/host-1/"},
		[]observer.EndpointID{endpoints[0].ID},
	)

	// Running state changed, one added and one removed.
	updatedIngress := ingress.DeepCopy()
	updatedIngress.Labels["updated-label"] = "true"
	th.OnUpdate(ingress, updatedIngress)
	require.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/ingress-1-UID/host-1/",
			Target: "https://host-1/",
			Details: &observer.K8sIngress{
				Name:      "application-ingress",
				Namespace: "default",
				UID:       "test-1/ingress-1-UID/host-1/",
				Labels:    map[string]string{"env": "prod", "updated-label": "true"},
				Scheme:    "https",
				Host:      "host-1",
				Path:      "/",
			},
		},
	}, th.ListEndpoints())
}

func TestNodeEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(node1V1, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/node1-uid",
			Target: "internalIP",
			Details: &observer.K8sNode{
				UID:                 "uid",
				Annotations:         map[string]string{"annotation-key": "annotation-value"},
				Labels:              map[string]string{"label-key": "label-value"},
				Name:                "node1",
				InternalIP:          "internalIP",
				InternalDNS:         "internalDNS",
				Hostname:            "localhost",
				ExternalIP:          "externalIP",
				ExternalDNS:         "externalDNS",
				KubeletEndpointPort: 1234,
			},
		},
	}, th.ListEndpoints())
}

func TestNodeEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(node1V1, true)
	th.OnDelete(node1V1)
	assert.Empty(t, th.ListEndpoints())
}

func TestNodeEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(node1V1, node1V1)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := node1V1.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(node1V1, changedLabels)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/node1-uid",
			Target: "internalIP",
			Details: &observer.K8sNode{
				UID:         "uid",
				Annotations: map[string]string{"annotation-key": "annotation-value"},
				Labels: map[string]string{
					"label-key": "label-value",
					"new-label": "value",
				},
				Name:                "node1",
				InternalIP:          "internalIP",
				InternalDNS:         "internalDNS",
				Hostname:            "localhost",
				ExternalIP:          "externalIP",
				ExternalDNS:         "externalDNS",
				KubeletEndpointPort: 1234,
			},
		},
	}, th.ListEndpoints())

	// Running state changed, one added and one removed.
	updatedNode := node1V1.DeepCopy()
	updatedNode.Labels["updated-label"] = "true"
	th.OnUpdate(node1V1, updatedNode)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/node1-uid",
			Target: "internalIP",
			Details: &observer.K8sNode{
				UID:         "uid",
				Annotations: map[string]string{"annotation-key": "annotation-value"},
				Labels: map[string]string{
					"label-key":     "label-value",
					"updated-label": "true",
				},
				Name:                "node1",
				InternalIP:          "internalIP",
				InternalDNS:         "internalDNS",
				Hostname:            "localhost",
				ExternalIP:          "externalIP",
				ExternalDNS:         "externalDNS",
				KubeletEndpointPort: 1234,
			},
		},
	}, th.ListEndpoints())
}

func TestPodEndpointsEphemeralContainerAdded(t *testing.T) {
	th := newTestHandler()
	th.podOptions = podEndpointOptions{ephemeralContainers: true}
	th.OnAdd(podWithNamedPorts, true)
	require.Len(t, th.ListEndpoints(), 3)

	// An ephemeral container is added to the running pod and starts.
	debugged := podWithNamedPorts.DeepCopy()
	debugged.Spec.EphemeralContainers = []v1.EphemeralContainer{
		{EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "debugger", Image: "debug-image"}},
	}
	debugged.Status.EphemeralContainerStatuses = []v1.ContainerStatus{
		{Name: "debugger", ContainerID: "containerd://debugger-id", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
	}
	th.OnUpdate(podWithNamedPorts, debugged)
	_, ok := th.endpoints.Load(observer.EndpointID("test-1/pod-2-UID/debugger"))
	require.True(t, ok)
	require.Len(t, th.ListEndpoints(), 4)

	// It exits: its endpoint goes, the others stay.
	exited := debugged.DeepCopy()
	exited.Status.EphemeralContainerStatuses[0].State = v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}}
	th.OnUpdate(debugged, exited)
	require.Len(t, th.ListEndpoints(), 3)

	th.OnDelete(exited)
	require.Empty(t, th.ListEndpoints())
}

func TestPodEndpointsInitContainerOfStartingPod(t *testing.T) {
	th := newTestHandler()
	th.podOptions = podEndpointOptions{initContainers: true}

	// The init container runs while the pod is pending.
	pending := podWithInitContainers()
	pending.Status.Phase = v1.PodPending
	pending.Status.InitContainerStatuses[0].State = stateRunning
	pending.Status.InitContainerStatuses[1] = containerStatus("sidecar", "", stateWaiting)
	pending.Status.ContainerStatuses[0] = containerStatus("app", "", stateWaiting)
	th.OnAdd(pending, true)
	require.Equal(t, []string{"test-1/pod-3-UID/init"}, endpointIDs(th.ListEndpoints()))

	// It finishes and the pod starts: its endpoint goes, the ones of the pod come.
	started := podWithInitContainers()
	th.OnUpdate(pending, started)
	require.ElementsMatch(t, []string{
		"test-1/pod-3-UID",
		"test-1/pod-3-UID/sidecar",
		"test-1/pod-3-UID/metrics(9090)",
		"test-1/pod-3-UID/app",
	}, endpointIDs(th.ListEndpoints()))

	th.OnDelete(started)
	require.Empty(t, th.ListEndpoints())
}

func TestPodEndpointsAppPortKeptOverInitContainerPort(t *testing.T) {
	th := newTestHandler()
	th.podOptions = podEndpointOptions{initContainers: true}

	// A native sidecar and the app container declare the same port.
	pod := podWithInitContainers()
	pod.Spec.Containers[0].Ports = pod.Spec.InitContainers[1].Ports
	th.OnAdd(pod, true)

	port, ok := th.endpoints.Load(observer.EndpointID("test-1/pod-3-UID/metrics(9090)"))
	require.True(t, ok)
	require.Equal(t, "app", port.(observer.Endpoint).Details.(*observer.Port).ContainerName)
}

// testClock is a clock that a test moves.
type testClock struct {
	now time.Time
}

func (c *testClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
}

// newTestHandlerWithClock returns a handler that observes terminated containers and the clock it reads.
func newTestHandlerWithClock(ttl time.Duration) (*handler, *testClock) {
	clock := &testClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	th := newTestHandler()
	th.podOptions = podEndpointOptions{terminatedContainers: true}
	th.terminatedContainerTTL = ttl
	th.now = func() time.Time { return clock.now }
	return th, clock
}

func terminatedAt(finishedAt time.Time) v1.ContainerState {
	return v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 1, FinishedAt: metav1.NewTime(finishedAt)}}
}

func podContainer(t *testing.T, th *handler, id observer.EndpointID) *observer.PodContainer {
	endpoint, ok := th.endpoints.Load(id)
	require.True(t, ok)
	return endpoint.(observer.Endpoint).Details.(*observer.PodContainer)
}

// storedEntries counts what the handler keeps, listed or not.
func storedEntries(th *handler) int {
	stored := 0
	count := func(_, _ any) bool {
		stored++
		return true
	}
	th.endpoints.Range(count)
	th.expiries.Range(count)
	return stored
}

// jobPod has one app container that runs.
func jobPod(name string) *v1.Pod {
	pod := newPod(name, "node-1")
	pod.Spec.Containers = []v1.Container{{Name: "work", Image: "image", Ports: []v1.ContainerPort{{Name: "http", ContainerPort: 80, Protocol: v1.ProtocolTCP}}}}
	pod.Status.ContainerStatuses = []v1.ContainerStatus{containerStatus("work", "containerd://work-id", stateRunning)}
	return pod
}

func TestPodEndpointsKeptForTerminatedContainers(t *testing.T) {
	th, clock := newTestHandlerWithClock(0)
	const workID = observer.EndpointID("test-1/job-1-UID/work")

	job := jobPod("job-1")
	th.OnAdd(job, true)
	require.ElementsMatch(t, []string{"test-1/job-1-UID", "test-1/job-1-UID/work", "test-1/job-1-UID/http(80)"}, endpointIDs(th.ListEndpoints()))
	require.True(t, podContainer(t, th, workID).Running)

	// The container exits and the pod succeeds: the pod and port endpoints go, the container
	// endpoint stays and tells that the container does not run.
	finished := job.DeepCopy()
	finished.Status.Phase = v1.PodSucceeded
	finished.Status.ContainerStatuses[0].State = terminatedAt(clock.now)
	th.OnUpdate(job, finished)
	require.Equal(t, []string{"test-1/job-1-UID/work"}, endpointIDs(th.ListEndpoints()))
	require.False(t, podContainer(t, th, workID).Running)
	require.Equal(t, "work-id", podContainer(t, th, workID).ContainerID)

	// Without a TTL it stays for as long as the pod exists.
	clock.advance(24 * time.Hour)
	require.Equal(t, []string{"test-1/job-1-UID/work"}, endpointIDs(th.ListEndpoints()))

	// After the pod is deleted it stays for deletedPodLinger and is dropped then.
	th.OnDelete(finished)
	clock.advance(deletedPodLinger - time.Second)
	require.Equal(t, []string{"test-1/job-1-UID/work"}, endpointIDs(th.ListEndpoints()))
	clock.advance(time.Second)
	require.Empty(t, th.ListEndpoints())
	require.Zero(t, storedEntries(th))
}

func TestPodEndpointsOfPodGoneBetweenListings(t *testing.T) {
	th, clock := newTestHandlerWithClock(time.Minute)

	// A pod that is created, runs, finishes and is deleted before the endpoints are listed once.
	created := jobPod("job-2")
	created.Status.Phase = v1.PodPending
	created.Status.PodIP = ""
	created.Status.ContainerStatuses[0] = containerStatus("work", "", stateWaiting)
	th.OnAdd(created, false)

	finished := jobPod("job-2")
	finished.Status.Phase = v1.PodSucceeded
	finished.Status.ContainerStatuses[0].State = terminatedAt(clock.now)
	th.OnUpdate(created, finished)
	th.OnDelete(finished)

	require.Equal(t, []string{"test-1/job-2-UID/work"}, endpointIDs(th.ListEndpoints()))
	clock.advance(deletedPodLinger)
	require.Empty(t, th.ListEndpoints())
	require.Zero(t, storedEntries(th))
}

func TestPodEndpointsOfDeletedRunningPod(t *testing.T) {
	// A pod deleted while its container runs, as in a rollout.
	th, clock := newTestHandlerWithClock(time.Minute)
	pod := jobPod("pod-5")
	th.OnAdd(pod, true)
	th.OnDelete(pod)
	require.Equal(t, []string{"test-1/pod-5-UID/work"}, endpointIDs(th.ListEndpoints()))
	clock.advance(deletedPodLinger)
	require.Empty(t, th.ListEndpoints())
	require.Zero(t, storedEntries(th))

	// The final state comes wrapped when the delete event was missed.
	th.OnAdd(pod, true)
	th.OnDelete(&cache.DeletedFinalStateUnknown{Key: "default/pod-5", Obj: pod})
	clock.advance(deletedPodLinger)
	require.Empty(t, th.ListEndpoints())
	require.Zero(t, storedEntries(th))

	// Without the option the endpoints go at once.
	th = newTestHandler()
	th.OnAdd(pod, true)
	th.OnDelete(pod)
	require.Empty(t, th.ListEndpoints())
	require.Zero(t, storedEntries(th))
}

func TestPodEndpointsTerminatedContainerTTL(t *testing.T) {
	const ttl = 15 * time.Minute
	th, clock := newTestHandlerWithClock(ttl)

	running := jobPod("job-3")
	th.OnAdd(running, true)

	finished := running.DeepCopy()
	finished.Status.Phase = v1.PodSucceeded
	finished.Status.ContainerStatuses[0].State = terminatedAt(clock.now)
	th.OnUpdate(running, finished)

	clock.advance(ttl - time.Second)
	require.Equal(t, []string{"test-1/job-3-UID/work"}, endpointIDs(th.ListEndpoints()))
	clock.advance(time.Second)
	require.Empty(t, th.ListEndpoints())

	// A change to the pod does not bring the endpoint back.
	relabeled := finished.DeepCopy()
	relabeled.Labels["new-label"] = "value"
	th.OnUpdate(finished, relabeled)
	require.Empty(t, th.ListEndpoints())

	// An endpoint that is not listed any more does not linger when the pod is deleted.
	th.OnDelete(relabeled)
	require.Empty(t, th.ListEndpoints())
	require.Zero(t, storedEntries(th))
}

func TestPodEndpointsOfPodFinishedBeforeStart(t *testing.T) {
	th, clock := newTestHandlerWithClock(15 * time.Minute)

	// Pods that finished before the collector started: only the recent one is listed.
	recent := jobPod("job-4")
	recent.Status.Phase = v1.PodSucceeded
	recent.Status.ContainerStatuses[0].State = terminatedAt(clock.now.Add(-time.Minute))
	old := jobPod("job-5")
	old.Status.Phase = v1.PodFailed
	old.Status.ContainerStatuses[0].State = terminatedAt(clock.now.Add(-time.Hour))
	th.OnAdd(recent, true)
	th.OnAdd(old, true)

	require.Equal(t, []string{"test-1/job-4-UID/work"}, endpointIDs(th.ListEndpoints()))
}

func TestPodEndpointsContainerRestart(t *testing.T) {
	const ttl = 15 * time.Minute
	const workID = observer.EndpointID("test-1/pod-6-UID/work")
	allIDs := []string{"test-1/pod-6-UID", "test-1/pod-6-UID/work", "test-1/pod-6-UID/http(80)"}
	th, clock := newTestHandlerWithClock(ttl)

	running := jobPod("pod-6")
	th.OnAdd(running, true)
	require.ElementsMatch(t, allIDs, endpointIDs(th.ListEndpoints()))

	// The container crashes: the port endpoint goes, the container endpoint stays.
	crashed := running.DeepCopy()
	crashed.Status.ContainerStatuses[0].State = terminatedAt(clock.now)
	th.OnUpdate(running, crashed)
	require.ElementsMatch(t, []string{"test-1/pod-6-UID", "test-1/pod-6-UID/work"}, endpointIDs(th.ListEndpoints()))
	require.False(t, podContainer(t, th, workID).Running)

	// It waits for its next start in a crash loop and still has the ID of the container that crashed.
	backoff := crashed.DeepCopy()
	backoff.Status.ContainerStatuses[0].State = stateWaiting
	backoff.Status.ContainerStatuses[0].LastTerminationState = terminatedAt(clock.now)
	th.OnUpdate(crashed, backoff)
	require.ElementsMatch(t, []string{"test-1/pod-6-UID", "test-1/pod-6-UID/work"}, endpointIDs(th.ListEndpoints()))
	require.Equal(t, "work-id", podContainer(t, th, workID).ContainerID)

	// The wait is longer than the TTL: the endpoint is not listed.
	clock.advance(ttl)
	require.ElementsMatch(t, []string{"test-1/pod-6-UID"}, endpointIDs(th.ListEndpoints()))

	// It starts again as a new container: the same endpoint is listed with the new container ID.
	restarted := backoff.DeepCopy()
	restarted.Status.ContainerStatuses[0].State = stateRunning
	restarted.Status.ContainerStatuses[0].ContainerID = "containerd://work-id-2"
	restarted.Status.ContainerStatuses[0].RestartCount = 1
	th.OnUpdate(backoff, restarted)
	require.ElementsMatch(t, allIDs, endpointIDs(th.ListEndpoints()))
	require.Equal(t, "work-id-2", podContainer(t, th, workID).ContainerID)
	require.True(t, podContainer(t, th, workID).Running)

	// It keeps running for longer than the TTL of its last crash.
	clock.advance(2 * ttl)
	require.ElementsMatch(t, allIDs, endpointIDs(th.ListEndpoints()))
}

func TestPodEndpointsContainerRestartWithoutOptions(t *testing.T) {
	th := newTestHandler()
	allIDs := []string{"test-1/pod-7-UID", "test-1/pod-7-UID/work", "test-1/pod-7-UID/http(80)"}

	running := jobPod("pod-7")
	th.OnAdd(running, true)
	require.ElementsMatch(t, allIDs, endpointIDs(th.ListEndpoints()))

	backoff := running.DeepCopy()
	backoff.Status.ContainerStatuses[0].State = stateWaiting
	backoff.Status.ContainerStatuses[0].LastTerminationState = terminatedAt(time.Now())
	th.OnUpdate(running, backoff)
	require.ElementsMatch(t, []string{"test-1/pod-7-UID"}, endpointIDs(th.ListEndpoints()))

	restarted := backoff.DeepCopy()
	restarted.Status.ContainerStatuses[0].State = stateRunning
	restarted.Status.ContainerStatuses[0].ContainerID = "containerd://work-id-2"
	th.OnUpdate(backoff, restarted)
	require.ElementsMatch(t, allIDs, endpointIDs(th.ListEndpoints()))
	require.Equal(t, len(allIDs), storedEntries(th))
}

func TestPodEndpointsOfEvictedPod(t *testing.T) {
	th, clock := newTestHandlerWithClock(15 * time.Minute)

	running := jobPod("pod-8")
	th.OnAdd(running, true)

	evicted := running.DeepCopy()
	evicted.Status.Phase = v1.PodFailed
	evicted.Status.Reason = "Evicted"
	evicted.Status.ContainerStatuses[0].State = terminatedAt(clock.now)
	th.OnUpdate(running, evicted)
	require.Equal(t, []string{"test-1/pod-8-UID/work"}, endpointIDs(th.ListEndpoints()))

	// The pod object of an evicted pod stays until something deletes it.
	clock.advance(time.Hour)
	require.Empty(t, th.ListEndpoints())

	// Without the option nothing of an evicted pod is listed.
	th = newTestHandler()
	th.OnAdd(running, true)
	th.OnUpdate(running, evicted)
	require.Empty(t, th.ListEndpoints())
}

func TestPodEndpointsExpiredContainerNeverListed(t *testing.T) {
	// A pod that finished long ago is added and deleted while the endpoints are listed, as when
	// the collector starts on a node with many finished pods and when old Jobs are cleaned up.
	th, clock := newTestHandlerWithClock(15 * time.Minute)
	finished := jobPod("old-job")
	finished.Status.Phase = v1.PodSucceeded
	finished.Status.ContainerStatuses[0].State = terminatedAt(clock.now.Add(-time.Hour))

	var listed atomic.Int64
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			default:
				listed.Add(int64(len(th.ListEndpoints())))
			}
		}
	}()

	for i := range 50000 {
		pod := finished.DeepCopy()
		pod.UID = types.UID(fmt.Sprintf("old-job-%d", i))
		th.OnAdd(pod, true)
		th.OnDelete(pod)
	}
	close(done)
	<-stopped
	require.Zero(t, listed.Load(), "the endpoint of a container that finished before its time to live was listed")
}

func TestPodEndpointsOfDeletedPodsRemovedWithoutListing(t *testing.T) {
	// Nothing lists the endpoints when no receiver creator watches the observer.
	th, clock := newTestHandlerWithClock(15 * time.Minute)
	for i := range 100 {
		pod := jobPod(fmt.Sprintf("pod-%d", i))
		th.OnAdd(pod, true)
		th.OnDelete(pod)
		clock.advance(deletedPodLinger)
	}
	// What is left is the container endpoint of the last pod and the time it goes.
	require.Equal(t, 2, storedEntries(th))

	th.OnAdd(jobPod("next"), true)
	require.ElementsMatch(t, []string{"test-1/next-UID", "test-1/next-UID/work", "test-1/next-UID/http(80)"}, endpointIDs(th.ListEndpoints()))
	require.Equal(t, 3, storedEntries(th))
}
