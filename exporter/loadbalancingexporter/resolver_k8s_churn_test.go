// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// countSyncMap returns the number of entries currently held in a sync.Map.
func countSyncMap(m *sync.Map) int {
	n := 0
	m.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

// mkSlice builds an EndpointSlice for service "lb" in the "default" namespace.
func mkSlice(eps ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lb",
			Namespace: "default",
			Labels: map[string]string{
				"kubernetes.io/service-name": "lb",
			},
		},
		Endpoints: eps,
	}
}

// epNamed builds an endpoint with a populated hostname.
func epNamed(hostname, addr string) discoveryv1.Endpoint {
	h := hostname
	return discoveryv1.Endpoint{Addresses: []string{addr}, Hostname: &h}
}

// epNoHostname builds an endpoint that does not (yet) have a hostname populated,
// which is what Kubernetes routinely reports for a pod that has just started and
// been added to an EndpointSlice a moment before its Hostname field is set.
func epNoHostname(addr string) discoveryv1.Endpoint {
	return discoveryv1.Endpoint{Addresses: []string{addr}}
}

// TestK8sResolverReturnHostnamesPodChurnDoesNotLeak reproduces a memory leak in
// the Kubernetes resolver when return_hostnames is enabled.
//
// During a rolling update, a new pod frequently appears in an EndpointSlice a
// moment before its Hostname field is populated. In hostname mode the handler
// would then skip the update entirely (or diff against an empty old set),
// which meant the pods that churned OUT in that same update were never removed
// from the resolver's endpoint store. Over many pod rolls, dead pod hostnames
// accumulate without bound in endpointsStore -> endpoints -> the hash ring ->
// the load balancer's exporter map.
//
// This test drives the handler through many generations of pod churn and
// asserts that only the CURRENTLY-live pods remain tracked.
func TestK8sResolverReturnHostnamesPodChurnDoesNotLeak(t *testing.T) {
	_, tb := getTelemetryAssets(t)
	cl := fake.NewClientset()
	res, err := newK8sResolver(
		cl,
		zap.NewNop(),
		"lb",
		[]int32{4317},
		defaultListWatchTimeout,
		true, // return_hostnames
		tb,
	)
	require.NoError(t, err)

	h := res.handler

	const generations = 25
	const podsPerGen = 2 // each generation has this many live pods

	// Generation 0: two healthy pods with hostnames.
	prev := mkSlice(
		epNamed("pod-0a", "10.0.0.1"),
		epNamed("pod-0b", "10.0.0.2"),
	)
	h.OnAdd(prev, false)

	// Simulate many rolling updates. In each generation the two previous pods
	// are replaced by two brand new pods, and one of the new pods shows up
	// without its hostname first (as k8s does in practice) before settling.
	for g := 1; g <= generations; g++ {
		aName, aAddr := fmt.Sprintf("pod-%da", g), fmt.Sprintf("10.0.%d.1", g)
		bName, bAddr := fmt.Sprintf("pod-%db", g), fmt.Sprintf("10.0.%d.2", g)

		// Step 1: new pods scheduled; pod b is in the slice but has no hostname yet.
		transient := mkSlice(
			epNamed(aName, aAddr),
			epNoHostname(bAddr),
		)
		h.OnUpdate(prev, transient)

		// Step 2: pod b's hostname is now populated.
		settled := mkSlice(
			epNamed(aName, aAddr),
			epNamed(bName, bAddr),
		)
		h.OnUpdate(transient, settled)

		prev = settled
	}

	// Force a resolve so the resolver-visible endpoint list reflects the store.
	_, err = res.resolve(t.Context())
	require.NoError(t, err)

	// Only the final generation's pods should still be tracked. Anything more
	// means dead pod hostnames leaked.
	assert.Equalf(t, podsPerGen, countSyncMap(res.endpointsStore),
		"endpointsStore accumulated stale pod hostnames across %d generations of churn", generations)
	assert.Lenf(t, res.Endpoints(), podsPerGen,
		"resolver exposed backends for pods that no longer exist after %d generations of churn", generations)
}

// newTestHandler builds a handler backed by a fresh store with a no-op callback.
func newTestHandler(t *testing.T, returnNames bool) (handler, *sync.Map) {
	t.Helper()
	_, tb := getTelemetryAssets(t)
	store := &sync.Map{}
	h := handler{
		endpoints:   store,
		logger:      zap.NewNop(),
		telemetry:   tb,
		returnNames: returnNames,
		callback:    func(context.Context) ([]string, error) { return nil, nil },
	}
	return h, store
}

func storeKeys(m *sync.Map) []string {
	var keys []string
	m.Range(func(k, _ any) bool {
		keys = append(keys, k.(string))
		return true
	})
	return keys
}

// TestK8sHandlerOnUpdateRemovesChurnedPodsDespiteMissingHostname verifies that a
// single OnUpdate still removes pods that churned out even when a newly added
// pod in the same update has not yet been assigned a hostname.
func TestK8sHandlerOnUpdateRemovesChurnedPodsDespiteMissingHostname(t *testing.T) {
	h, store := newTestHandler(t, true)

	// Two live pods.
	old := mkSlice(epNamed("pod-x", "10.0.0.1"), epNamed("pod-y", "10.0.0.2"))
	h.OnAdd(old, false)
	require.ElementsMatch(t, []string{"pod-x", "pod-y"}, storeKeys(store))

	// pod-y is replaced by pod-z, which appears without a hostname yet.
	transient := mkSlice(epNamed("pod-x", "10.0.0.1"), epNoHostname("10.0.0.3"))
	h.OnUpdate(old, transient)

	// pod-y must be gone; pod-z is not routable yet so it is not added.
	assert.ElementsMatch(t, []string{"pod-x"}, storeKeys(store),
		"churned-out pod-y should be removed even though pod-z has no hostname yet")

	// Once pod-z's hostname is populated, it is added.
	settled := mkSlice(epNamed("pod-x", "10.0.0.1"), epNamed("pod-z", "10.0.0.3"))
	h.OnUpdate(transient, settled)
	assert.ElementsMatch(t, []string{"pod-x", "pod-z"}, storeKeys(store))
}

// TestK8sHandlerOnAddSkipsHostnamelessButAddsRest verifies OnAdd still records
// the pods that do have hostnames when others in the slice do not.
func TestK8sHandlerOnAddSkipsHostnamelessButAddsRest(t *testing.T) {
	h, store := newTestHandler(t, true)

	slice := mkSlice(epNamed("pod-a", "10.0.0.1"), epNoHostname("10.0.0.2"))
	h.OnAdd(slice, false)

	assert.ElementsMatch(t, []string{"pod-a"}, storeKeys(store),
		"pod-a should be added even though the other endpoint lacks a hostname")
}

// TestK8sHandlerOnDeleteRemovesValidSubset verifies OnDelete removes the
// endpoints it can resolve even if some in the slice lack hostnames.
func TestK8sHandlerOnDeleteRemovesValidSubset(t *testing.T) {
	h, store := newTestHandler(t, true)

	h.OnAdd(mkSlice(epNamed("pod-a", "10.0.0.1"), epNamed("pod-b", "10.0.0.2")), false)
	require.ElementsMatch(t, []string{"pod-a", "pod-b"}, storeKeys(store))

	// The slice is deleted; pod-b happens to have lost its hostname in the
	// tombstone. pod-a must still be removed.
	h.OnDelete(mkSlice(epNamed("pod-a", "10.0.0.1"), epNoHostname("10.0.0.2")))
	assert.ElementsMatch(t, []string{"pod-b"}, storeKeys(store),
		"pod-a should be removed on delete even though the tombstone slice had a hostnameless endpoint")
}

// TestK8sResolverIPChurnDoesNotLeak is a regression guard confirming the
// default IP mode (return_hostnames=false) stays bounded across pod churn.
func TestK8sResolverIPChurnDoesNotLeak(t *testing.T) {
	_, tb := getTelemetryAssets(t)
	res, err := newK8sResolver(fake.NewClientset(), zap.NewNop(), "lb", []int32{4317}, defaultListWatchTimeout, false, tb)
	require.NoError(t, err)
	h := res.handler

	prev := mkSlice(epNoHostname("10.0.0.1"), epNoHostname("10.0.0.2"))
	h.OnAdd(prev, false)

	const generations = 25
	for g := 1; g <= generations; g++ {
		next := mkSlice(
			epNoHostname(fmt.Sprintf("10.0.%d.1", g)),
			epNoHostname(fmt.Sprintf("10.0.%d.2", g)),
		)
		h.OnUpdate(prev, next)
		prev = next
	}

	assert.Equal(t, 2, countSyncMap(res.endpointsStore),
		"IP-mode endpointsStore should stay bounded across churn")
}
