// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	apps_v1 "k8s.io/api/apps/v1"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func newFakeAPIClientset(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
	return fake.NewSimpleClientset(), nil
}

func newPodIdentifier(from string, name string, value string) PodIdentifier {
	if from == "connection" {
		name = ""
	}

	return PodIdentifier{
		{
			Source: AssociationSource{
				From: from,
				Name: name,
			},
			Value: value,
		},
	}
}

func podAddAndUpdateTest(t *testing.T, c *WatchClient, handler func(obj any)) {
	assert.Empty(t, c.Pods)

	// pod without IP
	pod := &api_v1.Pod{}
	handler(pod)
	assert.Empty(t, c.Pods)

	pod = &api_v1.Pod{}
	pod.Name = "podA"
	pod.Status.PodIP = "1.1.1.1"
	handler(pod)
	assert.Len(t, c.Pods, 2)
	got := c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")]
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Equal(t, "podA", got.Name)
	assert.Empty(t, got.PodUID)

	pod = &api_v1.Pod{}
	pod.Name = "podB"
	pod.Status.PodIP = "1.1.1.1"
	handler(pod)
	assert.Len(t, c.Pods, 2)
	got = c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")]
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Equal(t, "podB", got.Name)
	assert.Empty(t, got.PodUID)

	pod = &api_v1.Pod{}
	pod.Name = "podC"
	pod.Status.PodIP = "2.2.2.2"
	pod.UID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	handler(pod)
	assert.Len(t, c.Pods, 5)
	got = c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "2.2.2.2")]
	assert.Equal(t, "2.2.2.2", got.Address)
	assert.Equal(t, "podC", got.Name)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got.PodUID)
	got = c.Pods[newPodIdentifier("resource_attribute", "k8s.pod.uid", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")]
	assert.Equal(t, "2.2.2.2", got.Address)
	assert.Equal(t, "podC", got.Name)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got.PodUID)
}

func namespaceAddAndUpdateTest(t *testing.T, c *WatchClient, handler func(obj any)) {
	assert.Empty(t, c.Namespaces)

	namespace := &api_v1.Namespace{}
	handler(namespace)
	assert.Empty(t, c.Namespaces)

	namespace = &api_v1.Namespace{}
	namespace.Name = "namespaceA"
	handler(namespace)
	assert.Len(t, c.Namespaces, 1)
	got := c.Namespaces["namespaceA"]
	assert.Equal(t, "namespaceA", got.Name)
	assert.Empty(t, got.NamespaceUID)

	namespace = &api_v1.Namespace{}
	namespace.Name = "namespaceB"
	namespace.UID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	handler(namespace)
	assert.Len(t, c.Namespaces, 2)
	got = c.Namespaces["namespaceB"]
	assert.Equal(t, "namespaceB", got.Name)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got.NamespaceUID)
}

func nodeAddAndUpdateTest(t *testing.T, c *WatchClient, handler func(obj any)) {
	assert.Empty(t, c.Nodes)

	node := &api_v1.Node{}
	handler(node)
	assert.Empty(t, c.Nodes)

	node = &api_v1.Node{}
	node.Name = "nodeA"
	handler(node)
	assert.Len(t, c.Nodes, 1)
	got, ok := c.GetNode("nodeA")
	assert.True(t, ok)
	assert.Equal(t, "nodeA", got.Name)
	assert.Empty(t, got.NodeUID)

	node = &api_v1.Node{}
	node.Name = "nodeB"
	node.UID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	handler(node)
	assert.Len(t, c.Nodes, 2)
	got, ok = c.GetNode("nodeB")
	assert.True(t, ok)
	assert.Equal(t, "nodeB", got.Name)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got.NodeUID)
}

func TestDefaultClientset(t *testing.T) {
	c, err := New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, []Association{}, Excludes{}, nil, nil, nil, nil, false, 10*time.Second)
	require.EqualError(t, err, "invalid authType for kubernetes: ")
	assert.Nil(t, c)

	c, err = New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, []Association{}, Excludes{}, newFakeAPIClientset, nil, nil, nil, false, 10*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestBadFilters(t *testing.T) {
	c, err := New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{Fields: []FieldFilter{{Op: selection.Exists}}}, []Association{}, Excludes{}, newFakeAPIClientset, NewFakeInformer, NewFakeNamespaceInformer, NewFakeReplicaSetInformer, false, 10*time.Second)
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestClientStartStop(t *testing.T) {
	c, _ := newTestClient(t)
	ctr := c.informer.GetController()
	require.IsType(t, &FakeController{}, ctr)
	fctr := ctr.(*FakeController)
	require.NotNil(t, fctr)

	done := make(chan struct{})
	assert.False(t, fctr.HasStopped())
	go func() {
		assert.NoError(t, c.Start())
		close(done)
	}()
	c.Stop()
	<-done
	time.Sleep(time.Second)
	assert.True(t, fctr.HasStopped())
}

func TestConstructorErrors(t *testing.T) {
	er := ExtractionRules{}
	ff := Filters{}
	t.Run("client-provider-call", func(t *testing.T) {
		var gotAPIConfig k8sconfig.APIConfig
		apiCfg := k8sconfig.APIConfig{
			AuthType: "test-auth-type",
		}
		clientProvider := func(c k8sconfig.APIConfig) (kubernetes.Interface, error) {
			gotAPIConfig = c
			return nil, errors.New("error creating k8s client")
		}
		c, err := New(componenttest.NewNopTelemetrySettings(), apiCfg, er, ff, []Association{}, Excludes{}, clientProvider, NewFakeInformer, NewFakeNamespaceInformer, nil, false, 10*time.Second)
		assert.Nil(t, c)
		require.EqualError(t, err, "error creating k8s client")
		assert.Equal(t, apiCfg, gotAPIConfig)
	})
}

func TestPodAdd(t *testing.T) {
	c, _ := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
}

func TestNamespaceAdd(t *testing.T) {
	c, _ := newTestClient(t)
	namespaceAddAndUpdateTest(t, c, c.handleNamespaceAdd)
}

func TestNodeAdd(t *testing.T) {
	c, _ := newTestClient(t)
	nodeAddAndUpdateTest(t, c, c.handleNodeAdd)
}

func TestReplicaSetHandler(t *testing.T) {
	c, _ := newTestClient(t)
	assert.Empty(t, c.ReplicaSets)

	replicaset := &apps_v1.ReplicaSet{}
	c.handleReplicaSetAdd(replicaset)
	assert.Empty(t, c.ReplicaSets)

	// test add replicaset
	replicaset = &apps_v1.ReplicaSet{}
	replicaset.Name = "deployment-aaa"
	replicaset.Namespace = "namespaceA"
	replicaset.UID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	replicaset.ResourceVersion = "333333"
	isController := true
	isNotController := false
	replicaset.OwnerReferences = []meta_v1.OwnerReference{
		{
			Kind:       "Deployment",
			Name:       "deployment",
			UID:        "ffffffff-gggg-hhhh-iiii-jjjjjjjjjjj",
			Controller: &isController,
		},
		{
			Kind:       "Deployment",
			Name:       "deploymentNotController",
			UID:        "kkkkkkkk-gggg-hhhh-iiii-jjjjjjjjjjj",
			Controller: &isNotController,
		},
	}
	c.handleReplicaSetAdd(replicaset)
	assert.Len(t, c.ReplicaSets, 1)
	got := c.ReplicaSets[string(replicaset.UID)]
	assert.Equal(t, "deployment-aaa", got.Name)
	assert.Equal(t, "namespaceA", got.Namespace)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got.UID)
	assert.Equal(t, Deployment{
		Name: "deployment",
		UID:  "ffffffff-gggg-hhhh-iiii-jjjjjjjjjjj",
	}, got.Deployment)

	// test update replicaset
	updatedReplicaset := replicaset
	updatedReplicaset.ResourceVersion = "444444"
	c.handleReplicaSetUpdate(replicaset, updatedReplicaset)
	assert.Len(t, c.ReplicaSets, 1)
	got = c.ReplicaSets[string(replicaset.UID)]
	assert.Equal(t, "deployment-aaa", got.Name)
	assert.Equal(t, "namespaceA", got.Namespace)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got.UID)
	assert.Equal(t, Deployment{
		Name: "deployment",
		UID:  "ffffffff-gggg-hhhh-iiii-jjjjjjjjjjj",
	}, got.Deployment)

	// test delete replicaset
	c.handleReplicaSetDelete(updatedReplicaset)
	assert.Empty(t, c.ReplicaSets)
	// test delete replicaset when DeletedFinalStateUnknown
	c.handleReplicaSetAdd(replicaset)
	require.Len(t, c.ReplicaSets, 1)
	c.handleReplicaSetDelete(cache.DeletedFinalStateUnknown{
		Obj: replicaset,
	})
	assert.Empty(t, c.ReplicaSets)
}

func TestPodHostNetwork(t *testing.T) {
	c, _ := newTestClient(t)
	assert.Empty(t, c.Pods)

	// pod will not be added if no rule matches
	pod := &api_v1.Pod{}
	pod.Name = "podA"
	pod.Status.PodIP = "1.1.1.1"
	pod.Spec.HostNetwork = true
	c.handlePodAdd(pod)
	assert.Empty(t, c.Pods)

	// pod will be added if rule matches
	pod.Name = "podB"
	pod.Status.PodIP = "2.2.2.2"
	pod.UID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	pod.Spec.HostNetwork = true
	c.handlePodAdd(pod)
	assert.Len(t, c.Pods, 1)
	got := c.Pods[newPodIdentifier("resource_attribute", "k8s.pod.uid", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")]
	assert.Equal(t, "2.2.2.2", got.Address)
	assert.Equal(t, "podB", got.Name)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got.PodUID)
	assert.False(t, got.Ignore)
}

// TestPodCreate tests that a new pod, created after otel-collector starts, has its attributes set
// correctly
func TestPodCreate(t *testing.T) {
	c, _ := newTestClient(t)
	assert.Empty(t, c.Pods)

	// pod is created in Pending phase. At this point it has a UID but no start time or pod IP address
	pod := &api_v1.Pod{}
	pod.Name = "podD"
	pod.UID = "11111111-2222-3333-4444-555555555555"
	c.handlePodAdd(pod)
	assert.Len(t, c.Pods, 1)
	got := c.Pods[newPodIdentifier("resource_attribute", "k8s.pod.uid", "11111111-2222-3333-4444-555555555555")]
	assert.Empty(t, got.Address)
	assert.Equal(t, "podD", got.Name)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got.PodUID)

	// pod is scheduled onto to a node (no changes relevant to this test happen in that event)
	// pod is started, and given a startTime but not an IP address - it's still Pending at this point
	startTime := meta_v1.NewTime(time.Now())
	pod.Status.StartTime = &startTime
	c.handlePodUpdate(&api_v1.Pod{}, pod)
	assert.Len(t, c.Pods, 1)
	got = c.Pods[newPodIdentifier("resource_attribute", "k8s.pod.uid", "11111111-2222-3333-4444-555555555555")]
	assert.Empty(t, got.Address)
	assert.Equal(t, "podD", got.Name)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got.PodUID)

	// pod is Running and has an IP address
	pod.Status.PodIP = "3.3.3.3"
	c.handlePodUpdate(&api_v1.Pod{}, pod)
	assert.Len(t, c.Pods, 3)
	got = c.Pods[newPodIdentifier("resource_attribute", "k8s.pod.uid", "11111111-2222-3333-4444-555555555555")]
	assert.Equal(t, "3.3.3.3", got.Address)
	assert.Equal(t, "podD", got.Name)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got.PodUID)
	got = c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "3.3.3.3")]
	assert.Equal(t, "3.3.3.3", got.Address)
	assert.Equal(t, "podD", got.Name)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got.PodUID)

	got = c.Pods[newPodIdentifier("resource_attribute", "k8s.pod.ip", "3.3.3.3")]
	assert.Equal(t, "3.3.3.3", got.Address)
	assert.Equal(t, "podD", got.Name)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got.PodUID)
}

func TestPodAddOutOfSync(t *testing.T) {
	c, _ := newTestClient(t)
	c.Associations = append(c.Associations, Association{
		Name: "name",
		Sources: []AssociationSource{
			{
				From: ResourceSource,
				Name: "k8s.pod.name",
			},
		},
	})
	assert.Empty(t, c.Pods)

	pod := &api_v1.Pod{}
	pod.Name = "podA"
	pod.Status.PodIP = "1.1.1.1"
	startTime := meta_v1.NewTime(time.Now())
	pod.Status.StartTime = &startTime
	c.handlePodAdd(pod)
	assert.Len(t, c.Pods, 3)
	got := c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")]
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Equal(t, "podA", got.Name)
	got = c.Pods[newPodIdentifier(ResourceSource, "k8s.pod.name", "podA")]
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Equal(t, "podA", got.Name)

	pod2 := &api_v1.Pod{}
	pod2.Name = "podB"
	pod2.Status.PodIP = "1.1.1.1"
	startTime2 := meta_v1.NewTime(time.Now().Add(-time.Second * 10))
	pod2.Status.StartTime = &startTime2
	c.handlePodAdd(pod2)
	assert.Len(t, c.Pods, 4)
	got = c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")]
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Equal(t, "podA", got.Name)
	got = c.Pods[newPodIdentifier(ResourceSource, "k8s.pod.name", "podB")]
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Equal(t, "podB", got.Name)
}

func TestPodUpdate(t *testing.T) {
	c, _ := newTestClient(t)
	podAddAndUpdateTest(t, c, func(obj any) {
		// first argument (old pod) is not used right now
		c.handlePodUpdate(&api_v1.Pod{}, obj)
	})
}

func TestNamespaceUpdate(t *testing.T) {
	c, _ := newTestClient(t)
	namespaceAddAndUpdateTest(t, c, func(obj any) {
		// first argument (old namespace) is not used right now
		c.handleNamespaceUpdate(&api_v1.Namespace{}, obj)
	})
}

func TestNodeUpdate(t *testing.T) {
	c, _ := newTestClient(t)
	nodeAddAndUpdateTest(t, c, func(obj any) {
		// first argument (old node) is not used right now
		c.handleNodeUpdate(&api_v1.Node{}, obj)
	})
}

func TestPodDelete(t *testing.T) {
	c, _ := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
	assert.Len(t, c.Pods, 5)
	assert.Equal(t, "1.1.1.1", c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")].Address)

	// delete empty IP pod
	c.handlePodDelete(&api_v1.Pod{})

	// delete nonexistent IP
	c.deleteQueue = c.deleteQueue[:0]
	pod := &api_v1.Pod{}
	pod.Status.PodIP = "9.9.9.9"
	c.handlePodDelete(pod)
	assert.Len(t, c.Pods, 5)
	got := c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")]
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Empty(t, c.deleteQueue)

	// delete matching IP with wrong name/different pod
	c.deleteQueue = c.deleteQueue[:0]
	pod = &api_v1.Pod{}
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodDelete(pod)
	got = c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")]
	assert.Len(t, c.Pods, 5)
	assert.Equal(t, "1.1.1.1", got.Address)
	assert.Empty(t, c.deleteQueue)

	// delete matching IP and name
	c.deleteQueue = c.deleteQueue[:0]
	pod = &api_v1.Pod{}
	pod.Name = "podB"
	pod.Status.PodIP = "1.1.1.1"
	tsBeforeDelete := time.Now()
	c.handlePodDelete(pod)
	assert.Len(t, c.Pods, 5)
	assert.Len(t, c.deleteQueue, 3)
	deleteRequest := c.deleteQueue[0]
	assert.Equal(t, newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1"), deleteRequest.id)
	assert.Equal(t, "podB", deleteRequest.podName)
	assert.False(t, deleteRequest.ts.Before(tsBeforeDelete))
	assert.False(t, deleteRequest.ts.After(time.Now()))

	// delete when DeletedFinalStateUnknown
	c.deleteQueue = c.deleteQueue[:0]
	pod = &api_v1.Pod{}
	pod.Name = "podC"
	pod.Status.PodIP = "2.2.2.2"
	pod.UID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	tsBeforeDelete = time.Now()
	c.handlePodDelete(cache.DeletedFinalStateUnknown{Obj: pod})
	assert.Len(t, c.Pods, 5)
	assert.Len(t, c.deleteQueue, 5)
	deleteRequest = c.deleteQueue[0]
	assert.Equal(t, newPodIdentifier("connection", "k8s.pod.ip", "2.2.2.2"), deleteRequest.id)
	assert.Equal(t, "podC", deleteRequest.podName)
	assert.False(t, deleteRequest.ts.Before(tsBeforeDelete))
	assert.False(t, deleteRequest.ts.After(time.Now()))
	deleteRequest = c.deleteQueue[1]
	assert.Equal(t, newPodIdentifier("resource_attribute", "k8s.pod.uid", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"), deleteRequest.id)
	assert.Equal(t, "podC", deleteRequest.podName)
	assert.False(t, deleteRequest.ts.Before(tsBeforeDelete))
	assert.False(t, deleteRequest.ts.After(time.Now()))
}

func TestNamespaceDelete(t *testing.T) {
	c, _ := newTestClient(t)
	namespaceAddAndUpdateTest(t, c, c.handleNamespaceAdd)
	assert.Len(t, c.Namespaces, 2)
	assert.Equal(t, "namespaceA", c.Namespaces["namespaceA"].Name)

	// delete empty namespace
	c.handleNamespaceDelete(&api_v1.Namespace{})

	// delete nonexistent namespace
	namespace := &api_v1.Namespace{}
	namespace.Name = "namespaceC"
	c.handleNamespaceDelete(namespace)
	assert.Len(t, c.Namespaces, 2)
	got := c.Namespaces["namespaceA"]
	assert.Equal(t, "namespaceA", got.Name)
	// delete nonexistent namespace when DeletedFinalStateUnknown
	c.handleNamespaceDelete(cache.DeletedFinalStateUnknown{Obj: namespace})
	assert.Len(t, c.Namespaces, 2)
	got = c.Namespaces["namespaceA"]
	assert.Equal(t, "namespaceA", got.Name)

	// delete namespace A
	namespace.Name = "namespaceA"
	c.handleNamespaceDelete(namespace)
	assert.Len(t, c.Namespaces, 1)
	got = c.Namespaces["namespaceB"]
	assert.Equal(t, "namespaceB", got.Name)

	// delete namespace B when DeletedFinalStateUnknown
	namespace.Name = "namespaceB"
	c.handleNamespaceDelete(cache.DeletedFinalStateUnknown{Obj: namespace})
	assert.Empty(t, c.Namespaces)
}

func TestNodeDelete(t *testing.T) {
	c, _ := newTestClient(t)
	nodeAddAndUpdateTest(t, c, c.handleNodeAdd)
	assert.Len(t, c.Nodes, 2)
	assert.Equal(t, "nodeA", c.Nodes["nodeA"].Name)

	// delete empty node
	c.handleNodeDelete(&api_v1.Node{})

	// delete nonexistent node
	node := &api_v1.Node{}
	node.Name = "nodeC"
	c.handleNodeDelete(node)
	assert.Len(t, c.Nodes, 2)
	got := c.Nodes["nodeA"]
	assert.Equal(t, "nodeA", got.Name)
	// delete nonexistent namespace when DeletedFinalStateUnknown
	c.handleNodeDelete(cache.DeletedFinalStateUnknown{Obj: node})
	assert.Len(t, c.Nodes, 2)
	got = c.Nodes["nodeA"]
	assert.Equal(t, "nodeA", got.Name)

	// delete node A
	node.Name = "nodeA"
	c.handleNodeDelete(node)
	assert.Len(t, c.Nodes, 1)
	got = c.Nodes["nodeB"]
	assert.Equal(t, "nodeB", got.Name)

	// delete node B when DeletedFinalStateUnknown
	node.Name = "nodeB"
	c.handleNodeDelete(cache.DeletedFinalStateUnknown{Obj: node})
	assert.Empty(t, c.Nodes)
}

func TestDeleteQueue(t *testing.T) {
	c, _ := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
	assert.Len(t, c.Pods, 5)
	assert.Equal(t, "1.1.1.1", c.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")].Address)

	// delete pod
	pod := &api_v1.Pod{}
	pod.Name = "podB"
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodDelete(pod)
	assert.Len(t, c.Pods, 5)
	assert.Len(t, c.deleteQueue, 3)
}

func TestDeleteLoop(t *testing.T) {
	// go c.deleteLoop(time.Second * 1)
	c, _ := newTestClient(t)

	pod := &api_v1.Pod{}
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodAdd(pod)
	assert.Len(t, c.Pods, 2)
	assert.Empty(t, c.deleteQueue)

	c.handlePodDelete(pod)
	assert.Len(t, c.Pods, 2)
	assert.Len(t, c.deleteQueue, 3)

	gracePeriod := time.Millisecond * 500
	go c.deleteLoop(time.Millisecond, gracePeriod)
	go func() {
		time.Sleep(time.Millisecond * 50)
		c.m.Lock()
		assert.Len(t, c.Pods, 2)
		c.m.Unlock()
		c.deleteMut.Lock()
		assert.Len(t, c.deleteQueue, 3)
		c.deleteMut.Unlock()

		time.Sleep(gracePeriod + (time.Millisecond * 50))
		c.m.Lock()
		assert.Empty(t, c.Pods)
		c.m.Unlock()
		c.deleteMut.Lock()
		assert.Empty(t, c.deleteQueue)
		c.deleteMut.Unlock()
		close(c.stopCh)
	}()
	<-c.stopCh
}

func TestGetIgnoredPod(t *testing.T) {
	c, _ := newTestClient(t)
	pod := &api_v1.Pod{}
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodAdd(pod)
	c.Pods[newPodIdentifier("connection", "k8s.pod.ip", pod.Status.PodIP)].Ignore = true
	got, ok := c.GetPod(newPodIdentifier("connection", "k8s.pod.ip", pod.Status.PodIP))
	assert.Nil(t, got)
	assert.False(t, ok)
}

func TestHandlerWrongType(t *testing.T) {
	c, logs := newTestClientWithRulesAndFilters(t, Filters{})
	assert.Equal(t, 0, logs.Len())
	c.handlePodAdd(1)
	c.handlePodDelete(1)
	c.handlePodUpdate(1, 2)
	assert.Equal(t, 3, logs.Len())
	for _, l := range logs.All() {
		assert.Equal(t, "object received was not of type api_v1.Pod", l.Message)
	}
}

func TestExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	// Disable saving ip into k8s.pod.ip
	c.Associations[0].Sources[0].Name = ""

	pod := &api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "auth-service-abc12-xyz3",
			UID:               "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			Namespace:         "ns1",
			CreationTimestamp: meta_v1.Now(),
			Labels: map[string]string{
				"label1": "lv1",
				"label2": "k1=v1 k5=v5 extra!",
			},
			Annotations: map[string]string{
				"annotation1": "av1",
			},
			OwnerReferences: []meta_v1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "auth-service-66f5996c7c",
					UID:        "207ea729-c779-401d-8347-008ecbc137e3",
				},
				{
					APIVersion: "apps/v1",
					Kind:       "DaemonSet",
					Name:       "auth-daemonset",
					UID:        "c94d3814-2253-427a-ab13-2cf609e4dafa",
				},
				{
					APIVersion: "batch/v1",
					Kind:       "Job",
					Name:       "auth-cronjob-27667920",
					UID:        "59f27ac1-5c71-42e5-abe9-2c499d603706",
				},
				{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       "pi-statefulset",
					UID:        "03755eb1-6175-47d5-afd5-05cfc30244d7",
				},
			},
		},
		Spec: api_v1.PodSpec{
			NodeName: "node1",
			Hostname: "host1",
		},
		Status: api_v1.PodStatus{
			PodIP: "1.1.1.1",
		},
	}

	isController := true
	replicaset := &apps_v1.ReplicaSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "auth-service-66f5996c7c",
			Namespace: "ns1",
			UID:       "207ea729-c779-401d-8347-008ecbc137e3",
			OwnerReferences: []meta_v1.OwnerReference{
				{
					Name:       "auth-service",
					Kind:       "Deployment",
					UID:        "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
					Controller: &isController,
				},
			},
		},
	}

	testCases := []struct {
		name       string
		rules      ExtractionRules
		attributes map[string]string
	}{
		{
			name:       "no-rules",
			rules:      ExtractionRules{},
			attributes: nil,
		},
		{
			name: "deployment",
			rules: ExtractionRules{
				DeploymentName: true,
				DeploymentUID:  true,
			},
			attributes: map[string]string{
				"k8s.deployment.name": "auth-service",
				"k8s.deployment.uid":  "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
			},
		},
		{
			name: "replicasetId",
			rules: ExtractionRules{
				ReplicaSetID: true,
			},
			attributes: map[string]string{
				"k8s.replicaset.uid": "207ea729-c779-401d-8347-008ecbc137e3",
			},
		},
		{
			name: "replicasetName",
			rules: ExtractionRules{
				ReplicaSetName: true,
			},
			attributes: map[string]string{
				"k8s.replicaset.name": "auth-service-66f5996c7c",
			},
		},
		{
			name: "daemonsetUID",
			rules: ExtractionRules{
				DaemonSetUID: true,
			},
			attributes: map[string]string{
				"k8s.daemonset.uid": "c94d3814-2253-427a-ab13-2cf609e4dafa",
			},
		},
		{
			name: "daemonsetName",
			rules: ExtractionRules{
				DaemonSetName: true,
			},
			attributes: map[string]string{
				"k8s.daemonset.name": "auth-daemonset",
			},
		},
		{
			name: "jobUID",
			rules: ExtractionRules{
				JobUID: true,
			},
			attributes: map[string]string{
				"k8s.job.uid": "59f27ac1-5c71-42e5-abe9-2c499d603706",
			},
		},
		{
			name: "jobName",
			rules: ExtractionRules{
				JobName: true,
			},
			attributes: map[string]string{
				"k8s.job.name": "auth-cronjob-27667920",
			},
		},
		{
			name: "cronJob",
			rules: ExtractionRules{
				CronJobName: true,
			},
			attributes: map[string]string{
				"k8s.cronjob.name": "auth-cronjob",
			},
		},
		{
			name: "statefulsetUID",
			rules: ExtractionRules{
				StatefulSetUID: true,
			},
			attributes: map[string]string{
				"k8s.statefulset.uid": "03755eb1-6175-47d5-afd5-05cfc30244d7",
			},
		},
		{
			name: "jobName",
			rules: ExtractionRules{
				StatefulSetName: true,
			},
			attributes: map[string]string{
				"k8s.statefulset.name": "pi-statefulset",
			},
		},
		{
			name: "metadata",
			rules: ExtractionRules{
				DeploymentName: true,
				DeploymentUID:  true,
				Namespace:      true,
				PodName:        true,
				PodUID:         true,
				PodHostName:    true,
				PodIP:          true,
				Node:           true,
				StartTime:      true,
			},
			attributes: map[string]string{
				"k8s.deployment.name": "auth-service",
				"k8s.deployment.uid":  "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
				"k8s.namespace.name":  "ns1",
				"k8s.node.name":       "node1",
				"k8s.pod.name":        "auth-service-abc12-xyz3",
				"k8s.pod.hostname":    "host1",
				"k8s.pod.uid":         "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"k8s.pod.ip":          "1.1.1.1",
				"k8s.pod.start_time": func() string {
					b, err := pod.GetCreationTimestamp().MarshalText()
					require.NoError(t, err)
					return string(b)
				}(),
			},
		},
		{
			name: "labels",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromPod,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromPod,
					}, {
						Name:  "l2",
						Key:   "label2",
						Regex: regexp.MustCompile(`k5=(?P<value>[^\s]+)`),
						From:  MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"l1": "lv1",
				"l2": "v5",
				"a1": "av1",
			},
		},
		{
			// By default if the From field is not set for labels and annotations we want to extract them from pod
			name: "labels-annotations-default-pod",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
					}, {
						Name:  "l2",
						Key:   "label2",
						Regex: regexp.MustCompile(`k5=(?P<value>[^\s]+)`),
					},
				},
			},
			attributes: map[string]string{
				"l1": "lv1",
				"l2": "v5",
				"a1": "av1",
			},
		},
		{
			name: "all-labels",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:la.*)$"),
						From:     MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"k8s.pod.labels.label1": "lv1",
				"k8s.pod.labels.label2": "k1=v1 k5=v5 extra!",
			},
		},
		{
			name: "all-annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"k8s.pod.annotations.annotation1": "av1",
			},
		},
		{
			name: "all-annotations-not-match",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an*)$"),
						From:     MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{},
		},
		{
			name: "captured-groups",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name:                 "$1",
						KeyRegex:             regexp.MustCompile(`^(?:annotation(\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"1": "av1",
			},
		},
		{
			name: "captured-groups-$0",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name:                 "prefix-$0",
						KeyRegex:             regexp.MustCompile(`^(?:annotation(\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"prefix-annotation1": "av1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules

			// manually call the data removal functions here
			// normally the informer does this, but fully emulating the informer in this test is annoying
			transformedPod := removeUnnecessaryPodData(pod, c.Rules)
			transformedReplicaset := removeUnnecessaryReplicaSetData(replicaset)
			c.handleReplicaSetAdd(transformedReplicaset)
			c.handlePodAdd(transformedPod)
			p, ok := c.GetPod(newPodIdentifier("connection", "", pod.Status.PodIP))
			require.True(t, ok)

			assert.Len(t, p.Attributes, len(tc.attributes))
			for k, v := range tc.attributes {
				got, ok := p.Attributes[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestReplicaSetExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	// Disable saving ip into k8s.pod.ip
	c.Associations[0].Sources[0].Name = ""

	pod := &api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "auth-service-abc12-xyz3",
			UID:               "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			Namespace:         "ns1",
			CreationTimestamp: meta_v1.Now(),
			Labels: map[string]string{
				"label1": "lv1",
				"label2": "k1=v1 k5=v5 extra!",
			},
			Annotations: map[string]string{
				"annotation1": "av1",
			},
			OwnerReferences: []meta_v1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "auth-service-66f5996c7c",
					UID:        "207ea729-c779-401d-8347-008ecbc137e3",
				},
			},
		},
		Spec: api_v1.PodSpec{
			NodeName: "node1",
		},
		Status: api_v1.PodStatus{
			PodIP: "1.1.1.1",
		},
	}

	replicaset := &apps_v1.ReplicaSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "auth-service-66f5996c7c",
			Namespace: "ns1",
			UID:       "207ea729-c779-401d-8347-008ecbc137e3",
		},
	}

	isController := true
	isNotController := false
	testCases := []struct {
		name            string
		rules           ExtractionRules
		ownerReferences []meta_v1.OwnerReference
		attributes      map[string]string
	}{
		{
			name:       "no-rules",
			rules:      ExtractionRules{},
			attributes: nil,
		}, {
			name: "one_deployment_is_controller",
			ownerReferences: []meta_v1.OwnerReference{
				{
					Name:       "auth-service",
					Kind:       "Deployment",
					UID:        "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
					Controller: &isController,
				},
			},
			rules: ExtractionRules{
				DeploymentName: true,
				DeploymentUID:  true,
				ReplicaSetID:   true,
			},
			attributes: map[string]string{
				"k8s.replicaset.uid":  "207ea729-c779-401d-8347-008ecbc137e3",
				"k8s.deployment.name": "auth-service",
				"k8s.deployment.uid":  "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
			},
		}, {
			name: "one_deployment_is_controller_another_deployment_is_not_controller",
			ownerReferences: []meta_v1.OwnerReference{
				{
					Name:       "auth-service",
					Kind:       "Deployment",
					UID:        "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
					Controller: &isController,
				},
				{
					Name:       "auth-service-not-controller",
					Kind:       "Deployment",
					UID:        "kkkk-gggg-hhhh-iiii-eeeeeeeeeeee",
					Controller: &isNotController,
				},
			},
			rules: ExtractionRules{
				ReplicaSetID:   true,
				DeploymentName: true,
				DeploymentUID:  true,
			},
			attributes: map[string]string{
				"k8s.replicaset.uid":  "207ea729-c779-401d-8347-008ecbc137e3",
				"k8s.deployment.name": "auth-service",
				"k8s.deployment.uid":  "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
			},
		}, {
			name: "one_deployment_is_not_controller",
			ownerReferences: []meta_v1.OwnerReference{
				{
					Name:       "auth-service",
					Kind:       "Deployment",
					UID:        "ffff-gggg-hhhh-iiii-eeeeeeeeeeee",
					Controller: &isNotController,
				},
			},
			rules: ExtractionRules{
				ReplicaSetID:   true,
				DeploymentName: true,
				DeploymentUID:  true,
			},
			attributes: map[string]string{
				"k8s.replicaset.uid": "207ea729-c779-401d-8347-008ecbc137e3",
			},
		}, {
			name:            "no_deployment",
			ownerReferences: []meta_v1.OwnerReference{},
			rules: ExtractionRules{
				ReplicaSetID:   true,
				DeploymentName: true,
				DeploymentUID:  true,
			},
			attributes: map[string]string{
				"k8s.replicaset.uid": "207ea729-c779-401d-8347-008ecbc137e3",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			replicaset.OwnerReferences = tc.ownerReferences

			// manually call the data removal functions here
			// normally the informer does this, but fully emulating the informer in this test is annoying
			transformedPod := removeUnnecessaryPodData(pod, c.Rules)
			transformedReplicaset := removeUnnecessaryReplicaSetData(replicaset)
			c.handleReplicaSetAdd(transformedReplicaset)
			c.handlePodAdd(transformedPod)
			p, ok := c.GetPod(newPodIdentifier("connection", "", pod.Status.PodIP))
			require.True(t, ok)

			assert.Len(t, p.Attributes, len(tc.attributes))
			for k, v := range tc.attributes {
				got, ok := p.Attributes[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestNamespaceExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	namespace := &api_v1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "auth-service-namespace-abc12-xyz3",
			UID:               "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			CreationTimestamp: meta_v1.Now(),
			Labels: map[string]string{
				"label1": "lv1",
			},
			Annotations: map[string]string{
				"annotation1": "av1",
			},
		},
	}

	testCases := []struct {
		name       string
		rules      ExtractionRules
		attributes map[string]string
	}{
		{
			name:       "no-rules",
			rules:      ExtractionRules{},
			attributes: nil,
		},
		{
			name: "labels",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromNamespace,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromNamespace,
					},
				},
			},
			attributes: map[string]string{
				"l1": "lv1",
				"a1": "av1",
			},
		},
		{
			name: "all-labels",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:la.*)$"),
						From:     MetadataFromNamespace,
					},
				},
			},
			attributes: map[string]string{
				"k8s.namespace.labels.label1": "lv1",
			},
		},
		{
			name: "all-annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromNamespace,
					},
				},
			},
			attributes: map[string]string{
				"k8s.namespace.annotations.annotation1": "av1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			c.handleNamespaceAdd(namespace)
			p, ok := c.GetNamespace(namespace.Name)
			require.True(t, ok)

			assert.Len(t, p.Attributes, len(tc.attributes))
			for k, v := range tc.attributes {
				got, ok := p.Attributes[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestNodeExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	node := &api_v1.Node{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "k8s-node-example",
			UID:               "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			CreationTimestamp: meta_v1.Now(),
			Labels: map[string]string{
				"label1": "lv1",
			},
			Annotations: map[string]string{
				"annotation1": "av1",
			},
		},
	}

	testCases := []struct {
		name       string
		rules      ExtractionRules
		attributes map[string]string
	}{
		{
			name:       "no-rules",
			rules:      ExtractionRules{},
			attributes: nil,
		},
		{
			name: "labels",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromNode,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromNode,
					},
				},
			},
			attributes: map[string]string{
				"l1": "lv1",
				"a1": "av1",
			},
		},
		{
			name: "all-labels",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:la.*)$"),
						From:     MetadataFromNode,
					},
				},
			},
			attributes: map[string]string{
				"k8s.node.labels.label1": "lv1",
			},
		},
		{
			name: "all-annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromNode,
					},
				},
			},
			attributes: map[string]string{
				"k8s.node.annotations.annotation1": "av1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			c.handleNodeAdd(node)
			n, ok := c.GetNode(node.Name)
			require.True(t, ok)

			assert.Len(t, n.Attributes, len(tc.attributes))
			for k, v := range tc.attributes {
				got, ok := n.Attributes[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestFilters(t *testing.T) {
	testCases := []struct {
		name    string
		filters Filters
		labels  string
		fields  string
	}{
		{
			name:    "no-filters",
			filters: Filters{},
		}, {
			name: "namespace",
			filters: Filters{
				Namespace: "default",
			},
		}, {
			name: "node",
			filters: Filters{
				Node: "ec2-test",
			},
			fields: "spec.nodeName=ec2-test",
		}, {
			name: "labels-and-fields",
			filters: Filters{
				Labels: []LabelFilter{
					{
						Key:   "k1",
						Value: "v1",
						Op:    selection.Equals,
					},
					{
						Key:   "k2",
						Value: "v2",
						Op:    selection.NotEquals,
					},
				},
				Fields: []FieldFilter{
					{
						Key:   "k1",
						Value: "v1",
						Op:    selection.Equals,
					},
					{
						Key:   "k2",
						Value: "v2",
						Op:    selection.NotEquals,
					},
				},
			},
			labels: "k1=v1,k2!=v2",
			fields: "k1=v1,k2!=v2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClientWithRulesAndFilters(t, tc.filters)
			inf := c.informer.(*FakeInformer)
			assert.Equal(t, tc.filters.Namespace, inf.namespace)
			assert.Equal(t, tc.labels, inf.labelSelector.String())
			assert.Equal(t, tc.fields, inf.fieldSelector.String())
		})
	}
}

func TestPodIgnorePatterns(t *testing.T) {
	testCases := []struct {
		ignore bool
		pod    *api_v1.Pod
	}{
		{
			ignore: false,
			pod:    &api_v1.Pod{},
		}, {
			ignore: false,
			pod: &api_v1.Pod{
				Spec: api_v1.PodSpec{
					HostNetwork: true,
				},
			},
		}, {
			ignore: true,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Annotations: map[string]string{
						"opentelemetry.io/k8s-processor/ignore": "True ",
					},
				},
			},
		}, {
			ignore: true,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Annotations: map[string]string{
						"opentelemetry.io/k8s-processor/ignore": "true",
					},
				},
			},
		}, {
			ignore: false,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Annotations: map[string]string{
						"opentelemetry.io/k8s-processor/ignore": "false",
					},
				},
			},
		}, {
			ignore: false,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Annotations: map[string]string{
						"opentelemetry.io/k8s-processor/ignore": "",
					},
				},
			},
		}, {
			ignore: true,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "jaeger-agent",
				},
			},
		}, {
			ignore: true,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "jaeger-collector",
				},
			},
		}, {
			ignore: true,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "jaeger-agent-b2zdv",
				},
			},
		}, {
			ignore: false,
			pod: &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "test-pod-name",
				},
			},
		},
	}

	c, _ := newTestClient(t)
	for _, tc := range testCases {
		assert.Equal(t, tc.ignore, c.shouldIgnorePod(tc.pod))
	}
}

func Test_extractPodContainersAttributes(t *testing.T) {
	pod := api_v1.Pod{
		Spec: api_v1.PodSpec{
			Containers: []api_v1.Container{
				{
					Name:  "container1",
					Image: "example.com:5000/test/image1:0.1.0",
				},
				{
					Name:  "container2",
					Image: "example.com:81/image2@sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9",
				},
				{
					Name:  "container3",
					Image: "example-website.com/image3:1.0@sha256:4b0b1b6f6cdd3e5b9e55f74a1e8d19ed93a3f5a04c6b6c3c57c4e6d19f6b7c4d",
				},
			},
			InitContainers: []api_v1.Container{
				{
					Name:  "init_container",
					Image: "test/init-image",
				},
			},
		},
		Status: api_v1.PodStatus{
			ContainerStatuses: []api_v1.ContainerStatus{
				{
					Name:         "container1",
					ContainerID:  "docker://container1-id-123",
					ImageID:      "docker.io/otel/collector@sha256:55d008bc28344c3178645d40e7d07df30f9d90abe4b53c3fc4e5e9c0295533da",
					RestartCount: 0,
				},
				{
					Name:         "container2",
					ContainerID:  "docker://container2-id-456",
					ImageID:      "sha256:4b0b1b6f6cdd3e5b9e55f74a1e8d19ed93a3f5a04c6b6c3c57c4e6d19f6b7c4d",
					RestartCount: 2,
				},
				{
					Name:         "container3",
					ContainerID:  "docker://container3-id-abc",
					ImageID:      "docker.io/otel/collector:2.0.0@sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9",
					RestartCount: 2,
				},
			},
			InitContainerStatuses: []api_v1.ContainerStatus{
				{
					Name:         "init_container",
					ContainerID:  "containerd://init-container-id-789",
					ImageID:      "ghcr.io/initimage1@sha256:42e8ba40f9f70d604684c3a2a0ed321206b7e2e3509fdb2c8836d34f2edfb57b",
					RestartCount: 0,
				},
			},
		},
	}
	tests := []struct {
		name  string
		rules ExtractionRules
		pod   *api_v1.Pod
		want  PodContainers
	}{
		{
			name: "no-data",
			rules: ExtractionRules{
				ContainerImageName: true,
				ContainerImageTag:  true,
				ContainerID:        true,
			},
			pod:  &api_v1.Pod{},
			want: PodContainers{ByID: map[string]*Container{}, ByName: map[string]*Container{}},
		},
		{
			name:  "no-rules",
			rules: ExtractionRules{},
			pod:   &pod,
			want:  PodContainers{ByID: map[string]*Container{}, ByName: map[string]*Container{}},
		},
		{
			name: "image-name-only",
			rules: ExtractionRules{
				ContainerImageName: true,
			},
			pod: &pod,
			want: PodContainers{
				ByID: map[string]*Container{
					"container1-id-123":     {ImageName: "example.com:5000/test/image1"},
					"container2-id-456":     {ImageName: "example.com:81/image2"},
					"container3-id-abc":     {ImageName: "example-website.com/image3"},
					"init-container-id-789": {ImageName: "test/init-image"},
				},
				ByName: map[string]*Container{
					"container1":     {ImageName: "example.com:5000/test/image1"},
					"container2":     {ImageName: "example.com:81/image2"},
					"container3":     {ImageName: "example-website.com/image3"},
					"init_container": {ImageName: "test/init-image"},
				},
			},
		},
		{
			name: "no-image-tag-available",
			rules: ExtractionRules{
				ContainerImageName: true,
			},
			pod: &api_v1.Pod{
				Spec: api_v1.PodSpec{
					Containers: []api_v1.Container{
						{
							Name:  "test-container",
							Image: "test/image",
						},
					},
				},
			},
			want: PodContainers{
				ByID: map[string]*Container{},
				ByName: map[string]*Container{
					"test-container": {ImageName: "test/image"},
				},
			},
		},
		{
			name: "container-id-only",
			rules: ExtractionRules{
				ContainerID: true,
			},
			pod: &pod,
			want: PodContainers{
				ByID: map[string]*Container{
					"container1-id-123": {
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "container1-id-123"},
						},
					},
					"container2-id-456": {
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container2-id-456"},
						},
					},
					"container3-id-abc": {
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container3-id-abc"},
						},
					},
					"init-container-id-789": {
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "init-container-id-789"},
						},
					},
				},
				ByName: map[string]*Container{
					"container1": {
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "container1-id-123"},
						},
					},
					"container2": {
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container2-id-456"},
						},
					},
					"container3": {
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container3-id-abc"},
						},
					},
					"init_container": {
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "init-container-id-789"},
						},
					},
				},
			},
		},
		{
			name: "container-image-repo-digest-only",
			rules: ExtractionRules{
				ContainerImageRepoDigests: true,
			},
			pod: &pod,
			want: PodContainers{
				ByID: map[string]*Container{
					"container1-id-123": {
						Statuses: map[int]ContainerStatus{
							0: {ImageRepoDigest: "docker.io/otel/collector@sha256:55d008bc28344c3178645d40e7d07df30f9d90abe4b53c3fc4e5e9c0295533da"},
						},
					},
					"container2-id-456": {
						Statuses: map[int]ContainerStatus{
							2: {},
						},
					},
					"container3-id-abc": {
						Statuses: map[int]ContainerStatus{
							2: {ImageRepoDigest: "docker.io/otel/collector:2.0.0@sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9"},
						},
					},
					"init-container-id-789": {
						Statuses: map[int]ContainerStatus{
							0: {ImageRepoDigest: "ghcr.io/initimage1@sha256:42e8ba40f9f70d604684c3a2a0ed321206b7e2e3509fdb2c8836d34f2edfb57b"},
						},
					},
				},
				ByName: map[string]*Container{
					"container1": {
						Statuses: map[int]ContainerStatus{
							0: {ImageRepoDigest: "docker.io/otel/collector@sha256:55d008bc28344c3178645d40e7d07df30f9d90abe4b53c3fc4e5e9c0295533da"},
						},
					},
					"container2": {
						Statuses: map[int]ContainerStatus{
							2: {},
						},
					},
					"container3": {
						Statuses: map[int]ContainerStatus{
							2: {ImageRepoDigest: "docker.io/otel/collector:2.0.0@sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9"},
						},
					},
					"init_container": {
						Statuses: map[int]ContainerStatus{
							0: {ImageRepoDigest: "ghcr.io/initimage1@sha256:42e8ba40f9f70d604684c3a2a0ed321206b7e2e3509fdb2c8836d34f2edfb57b"},
						},
					},
				},
			},
		},
		{
			name: "all-container-attributes",
			rules: ExtractionRules{
				ContainerImageName:        true,
				ContainerImageTag:         true,
				ContainerID:               true,
				ContainerImageRepoDigests: true,
			},
			pod: &pod,
			want: PodContainers{
				ByID: map[string]*Container{
					"container1-id-123": {
						ImageName: "example.com:5000/test/image1",
						ImageTag:  "0.1.0",
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "container1-id-123", ImageRepoDigest: "docker.io/otel/collector@sha256:55d008bc28344c3178645d40e7d07df30f9d90abe4b53c3fc4e5e9c0295533da"},
						},
					},
					"container2-id-456": {
						ImageName: "example.com:81/image2",
						ImageTag:  "latest",
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container2-id-456"},
						},
					},
					"container3-id-abc": {
						ImageName: "example-website.com/image3",
						ImageTag:  "1.0",
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container3-id-abc", ImageRepoDigest: "docker.io/otel/collector:2.0.0@sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9"},
						},
					},
					"init-container-id-789": {
						ImageName: "test/init-image",
						ImageTag:  "latest",
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "init-container-id-789", ImageRepoDigest: "ghcr.io/initimage1@sha256:42e8ba40f9f70d604684c3a2a0ed321206b7e2e3509fdb2c8836d34f2edfb57b"},
						},
					},
				},
				ByName: map[string]*Container{
					"container1": {
						ImageName: "example.com:5000/test/image1",
						ImageTag:  "0.1.0",
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "container1-id-123", ImageRepoDigest: "docker.io/otel/collector@sha256:55d008bc28344c3178645d40e7d07df30f9d90abe4b53c3fc4e5e9c0295533da"},
						},
					},
					"container2": {
						ImageName: "example.com:81/image2",
						ImageTag:  "latest",
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container2-id-456"},
						},
					},
					"container3": {
						ImageName: "example-website.com/image3",
						ImageTag:  "1.0",
						Statuses: map[int]ContainerStatus{
							2: {ContainerID: "container3-id-abc", ImageRepoDigest: "docker.io/otel/collector:2.0.0@sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9"},
						},
					},
					"init_container": {
						ImageName: "test/init-image",
						ImageTag:  "latest",
						Statuses: map[int]ContainerStatus{
							0: {ContainerID: "init-container-id-789", ImageRepoDigest: "ghcr.io/initimage1@sha256:42e8ba40f9f70d604684c3a2a0ed321206b7e2e3509fdb2c8836d34f2edfb57b"},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := WatchClient{Rules: tt.rules}
			// manually call the data removal function here
			// normally the informer does this, but fully emulating the informer in this test is annoying
			transformedPod := removeUnnecessaryPodData(tt.pod, c.Rules)
			assert.Equal(t, tt.want, c.extractPodContainersAttributes(transformedPod))
		})
	}
}

func Test_extractField(t *testing.T) {
	type args struct {
		v string
		r FieldExtractionRule
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"no-regex",
			args{
				"str",
				FieldExtractionRule{Regex: nil},
			},
			"str",
		},
		{
			"basic",
			args{
				"str",
				FieldExtractionRule{Regex: regexp.MustCompile("field=(?P<value>.+)")},
			},
			"",
		},
		{
			"basic",
			args{
				"field=val1",
				FieldExtractionRule{Regex: regexp.MustCompile("field=(?P<value>.+)")},
			},
			"val1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.args.r.extractField(tt.args.v), "extractField()")
		})
	}
}

func TestErrorSelectorsFromFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters Filters
		wantErr bool
	}{
		{
			name: "label/invalid-op",
			filters: Filters{
				Labels: []LabelFilter{{Op: "invalid-op"}},
			},
			wantErr: true,
		},
		{
			name: "label/equals",
			filters: Filters{
				Labels: []LabelFilter{{Key: "app", Op: selection.Equals, Value: "test"}},
			},
		},
		{
			name: "label/not-equals",
			filters: Filters{
				Labels: []LabelFilter{{Key: "app", Op: selection.NotEquals, Value: "test"}},
			},
		},
		{
			name: "label/exists",
			filters: Filters{
				Labels: []LabelFilter{{Key: "app", Op: selection.Exists}},
			},
		},
		{
			name: "label/does-not-exist",
			filters: Filters{
				Labels: []LabelFilter{{Key: "app", Op: selection.DoesNotExist}},
			},
		},
		{
			name: "label/in",
			filters: Filters{
				Labels: []LabelFilter{{Key: "app", Op: selection.In}},
			},
			wantErr: true,
		},
		{
			name: "label/not-in",
			filters: Filters{
				Labels: []LabelFilter{{Key: "app", Op: selection.NotIn}},
			},
			wantErr: true,
		},
		{
			name: "fields/invalid-op",
			filters: Filters{
				Fields: []FieldFilter{{Op: selection.Exists}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := selectorsFromFilters(tt.filters)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractNamespaceLabelsAnnotations(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	testCases := []struct {
		name                   string
		shouldExtractNamespace bool
		rules                  ExtractionRules
	}{
		{
			name:                   "empty-rules",
			shouldExtractNamespace: false,
			rules:                  ExtractionRules{},
		}, {
			name:                   "pod-rules",
			shouldExtractNamespace: false,
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromPod,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromPod,
					},
				},
			},
		}, {
			name:                   "namespace-rules-only-annotations",
			shouldExtractNamespace: true,
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromNamespace,
					},
				},
			},
		}, {
			name:                   "namespace-rules-only-labels",
			shouldExtractNamespace: true,
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromNamespace,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			assert.Equal(t, tc.shouldExtractNamespace, c.extractNamespaceLabelsAnnotations())
		})
	}
}

func newTestClientWithRulesAndFilters(t *testing.T, f Filters) (*WatchClient, *observer.ObservedLogs) {
	set := componenttest.NewNopTelemetrySettings()
	observedLogger, logs := observer.New(zapcore.WarnLevel)
	set.Logger = zap.New(observedLogger)
	exclude := Excludes{
		Pods: []ExcludePods{
			{Name: regexp.MustCompile(`jaeger-agent`)},
			{Name: regexp.MustCompile(`jaeger-collector`)},
		},
	}
	associations := []Association{
		{
			Sources: []AssociationSource{
				{
					From: "connection",
				},
			},
		},
		{
			Sources: []AssociationSource{
				{
					From: "resource_attribute",
					Name: "k8s.pod.uid",
				},
			},
		},
	}
	c, err := New(set, k8sconfig.APIConfig{}, ExtractionRules{}, f, associations, exclude, newFakeAPIClientset, NewFakeInformer, NewFakeNamespaceInformer, NewFakeReplicaSetInformer, false, 10*time.Second)
	require.NoError(t, err)
	return c.(*WatchClient), logs
}

func newTestClient(t *testing.T) (*WatchClient, *observer.ObservedLogs) {
	return newTestClientWithRulesAndFilters(t, Filters{})
}

type neverSyncedFakeClient struct {
	cache.SharedInformer
}

type neverSyncedResourceEventHandlerRegistration struct {
	cache.ResourceEventHandlerRegistration
}

func (n *neverSyncedResourceEventHandlerRegistration) HasSynced() bool {
	return false
}

func (n *neverSyncedFakeClient) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	delegate, err := n.SharedInformer.AddEventHandler(handler)
	if err != nil {
		return nil, err
	}
	return &neverSyncedResourceEventHandlerRegistration{ResourceEventHandlerRegistration: delegate}, nil
}

func TestWaitForMetadata(t *testing.T) {
	testCases := []struct {
		name             string
		informerProvider InformerProvider
		err              bool
	}{{
		name:             "no wait",
		informerProvider: NewFakeInformer,
		err:              false,
	}, {
		name: "wait but never synced",
		informerProvider: func(client kubernetes.Interface, namespace string, labelSelector labels.Selector, fieldSelector fields.Selector) cache.SharedInformer {
			return &neverSyncedFakeClient{NewFakeInformer(client, namespace, labelSelector, fieldSelector)}
		},
		err: true,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, []Association{}, Excludes{}, newFakeAPIClientset, tc.informerProvider, nil, nil, true, 1*time.Second)
			require.NoError(t, err)

			err = c.Start()
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
