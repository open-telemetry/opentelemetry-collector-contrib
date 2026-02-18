// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube

import (
	"errors"
	"maps"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	apps_v1 "k8s.io/api/apps/v1"
	batch_v1 "k8s.io/api/batch/v1"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

func newFakeAPIClientset(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
	return fake.NewClientset(), nil
}

func newPodIdentifier(from, name, value string) PodIdentifier {
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
	c, err := New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, []Association{}, Excludes{}, nil, InformersFactoryList{}, false, 10*time.Second)
	require.EqualError(t, err, "invalid authType for kubernetes: ")
	assert.Nil(t, c)

	c, err = New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, []Association{}, Excludes{}, newFakeAPIClientset, InformersFactoryList{}, false, 10*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestBadFilters(t *testing.T) {
	factory := InformersFactoryList{
		newInformer:           NewFakeInformer,
		newNamespaceInformer:  NewFakeNamespaceInformer,
		newReplicaSetInformer: NewFakeReplicaSetInformer,
	}
	c, err := New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{Fields: []FieldFilter{{Op: selection.Exists}}}, []Association{}, Excludes{}, newFakeAPIClientset, factory, false, 10*time.Second)
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
		factory := InformersFactoryList{
			newInformer:          NewFakeInformer,
			newNamespaceInformer: NewFakeNamespaceInformer,
		}
		c, err := New(componenttest.NewNopTelemetrySettings(), apiCfg, er, ff, []Association{}, Excludes{}, clientProvider, factory, false, 10*time.Second)
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
	pod.UID = "aaaaaaaa-bbbb-cccc-dddd"
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
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", deleteRequest.podUID)
	assert.False(t, deleteRequest.ts.Before(tsBeforeDelete))
	assert.False(t, deleteRequest.ts.After(time.Now()))
	deleteRequest = c.deleteQueue[1]
	assert.Equal(t, newPodIdentifier("resource_attribute", "k8s.pod.uid", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"), deleteRequest.id)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", deleteRequest.podUID)
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

	serviceRules := ExtractionRules{
		ServiceNamespace:  true,
		ServiceName:       true,
		ServiceVersion:    true,
		ServiceInstanceID: true,
		Annotations:       []FieldExtractionRule{OtelAnnotations()},
	}

	testCases := []struct {
		name                  string
		rules                 ExtractionRules
		additionalAnnotations map[string]string
		additionalLabels      map[string]string
		attributes            map[string]string
		singularFeatureGate   bool
	}{
		{
			name:       "no-rules",
			rules:      ExtractionRules{},
			attributes: map[string]string{},
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
			name: "deployment-from-replicaset-name",
			rules: ExtractionRules{
				DeploymentName: true,
			},
			attributes: map[string]string{
				"k8s.deployment.name": "auth-service",
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
			name: "nodeUID",
			rules: ExtractionRules{
				NodeUID: true,
			},
			attributes: map[string]string{},
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
						Regex: regexp.MustCompile(`k5=(?P<value>\S+)`),
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
						Regex: regexp.MustCompile(`k5=(?P<value>\S+)`),
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
			name: "all-labels singular",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:la.*)$"),
						From:     MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"k8s.pod.label.label1": "lv1",
				"k8s.pod.label.label2": "k1=v1 k5=v5 extra!",
			},
			singularFeatureGate: true,
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
			name: "all-annotations singular",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"k8s.pod.annotation.annotation1": "av1",
			},
			singularFeatureGate: true,
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
		{
			name: "captured-groups-no-tag-name",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"k8s.pod.labels.label1": "lv1",
				"k8s.pod.labels.label2": "k1=v1 k5=v5 extra!",
			},
		},
		{
			name: "captured-groups-no-tag-name singular",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromPod,
					},
				},
			},
			attributes: map[string]string{
				"k8s.pod.label.label1": "lv1",
				"k8s.pod.label.label2": "k1=v1 k5=v5 extra!",
			},
			singularFeatureGate: true,
		},
		{
			name:  "service-name",
			rules: serviceRules,
			attributes: map[string]string{
				"service.name":      "pi-statefulset",
				"service.namespace": "ns1",
			},
		},
		{
			name:  "service-attributes-label-values",
			rules: serviceRules,
			additionalLabels: map[string]string{
				"app.kubernetes.io/name":    "label-service",
				"app.kubernetes.io/version": "label-version",
			},
			attributes: map[string]string{
				"service.name":      "label-service",
				"service.version":   "label-version",
				"service.namespace": "ns1",
			},
		},
		{
			name:  "service-attributes-label-values-instance",
			rules: serviceRules,
			additionalLabels: map[string]string{
				"app.kubernetes.io/instance": "instance-service",
				"app.kubernetes.io/name":     "label-service",
				"app.kubernetes.io/version":  "label-version",
			},
			attributes: map[string]string{
				"service.name":      "instance-service",
				"service.version":   "label-version",
				"service.namespace": "ns1",
			},
		},
		{
			name:  "service-attributes-annotation-override",
			rules: serviceRules,
			additionalAnnotations: map[string]string{
				"resource.opentelemetry.io/service.instance.id": "annotation-id",
				"resource.opentelemetry.io/service.version":     "annotation-version",
				"resource.opentelemetry.io/service.name":        "annotation-service",
				"resource.opentelemetry.io/service.namespace":   "annotation-namespace",
			},
			attributes: map[string]string{
				"service.instance.id": "annotation-id",
				"service.name":        "annotation-service",
				"service.version":     "annotation-version",
				"service.namespace":   "annotation-namespace",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.singularFeatureGate {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), true))
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), true))
				defer func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), false))
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), false))
				}()
			}

			c.Rules = tc.rules

			// manually call the data removal functions here
			// normally the informer does this, but fully emulating the informer in this test is annoying
			podCopy := pod.DeepCopy()
			maps.Copy(podCopy.Annotations, tc.additionalAnnotations)
			maps.Copy(podCopy.Labels, tc.additionalLabels)
			transformedPod := removeUnnecessaryPodData(podCopy, c.Rules)

			if tc.rules.Node || tc.rules.NodeUID {
				assert.Equal(t, podCopy.Spec.NodeName, transformedPod.Spec.NodeName, "NodeName should be preserved when Node or NodeUID rule is enabled")
			}

			transformedReplicaset := removeUnnecessaryReplicaSetData(replicaset)
			c.handleReplicaSetAdd(transformedReplicaset)
			c.handlePodAdd(transformedPod)
			p, ok := c.GetPod(newPodIdentifier("connection", "", podCopy.Status.PodIP))
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
			attributes: map[string]string{},
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

func TestRemoveUnnecessaryPodData_ClonesLabelsAndAnnotations(t *testing.T) {
	rules := ExtractionRules{
		Labels:      []FieldExtractionRule{{From: MetadataFromPod}},
		Annotations: []FieldExtractionRule{{From: MetadataFromPod}},
	}

	pod := &api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Labels: map[string]string{
				"app": "web",
			},
			Annotations: map[string]string{
				"anno": "value",
			},
		},
	}

	transformed := removeUnnecessaryPodData(pod, rules)

	pod.Labels["new"] = "label"
	pod.Annotations["new"] = "annotation"

	_, ok := transformed.Labels["new"]
	assert.False(t, ok)
	_, ok = transformed.Annotations["new"]
	assert.False(t, ok)
	assert.Equal(t, "web", transformed.Labels["app"])
	assert.Equal(t, "value", transformed.Annotations["anno"])
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
		name                string
		rules               ExtractionRules
		attributes          map[string]string
		singularFeatureGate bool
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
			name: "all-labels singular",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:la.*)$"),
						From:     MetadataFromNamespace,
					},
				},
			},
			attributes: map[string]string{
				"k8s.namespace.label.label1": "lv1",
			},
			singularFeatureGate: true,
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
		{
			name: "all-annotations singular",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromNamespace,
					},
				},
			},
			attributes: map[string]string{
				"k8s.namespace.annotation.annotation1": "av1",
			},
			singularFeatureGate: true,
		},
		{
			name: "captured-groups-no-tag-name",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromNamespace,
					},
				},
			},
			attributes: map[string]string{
				"k8s.namespace.labels.label1": "lv1",
			},
		},
		{
			name: "captured-groups-no-tag-name singular",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromNamespace,
					},
				},
			},
			attributes: map[string]string{
				"k8s.namespace.label.label1": "lv1",
			},
			singularFeatureGate: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.singularFeatureGate {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), true))
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), true))
				defer func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), false))
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), false))
				}()
			}

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

func TestDeleteQueue(t *testing.T) {
	makePodIdentifiers := func(pod *api_v1.Pod) []PodIdentifier {
		return []PodIdentifier{
			newPodIdentifier("resource_attribute", "k8s.pod.uid", string(pod.UID)),
			{
				{
					Source: AssociationSource{
						From: "resource_attribute",
						Name: "k8s.namespace.name",
					},
					Value: "ns",
				},
				{
					Source: AssociationSource{
						From: "resource_attribute",
						Name: "k8s.pod.name",
					},
					Value: "abc-0",
				},
			},
			newPodIdentifier("resource_attribute", "k8s.pod.ip", "10.0.0.1"),
			newPodIdentifier("connection", "", "10.0.0.1"),
		}
	}

	makePod := func(podUid string) *api_v1.Pod {
		pod1 := &api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      "abc-0",
				Namespace: "ns",
				UID:       types.UID(podUid),
			},
			Status: api_v1.PodStatus{
				PodIP: "10.0.0.1",
			},
		}
		return pod1
	}

	c, _ := newTestClient(t)
	doAssertions := func(pod *api_v1.Pod) {
		podIdentifiers := makePodIdentifiers(pod)
		for _, id := range podIdentifiers {
			foundPod, ok := c.Pods[id]
			assert.True(t, ok, "Pod should be present in c.Pods for identifier %v", id)
			assert.Equal(t, pod.UID, types.UID(foundPod.PodUID))
			assert.Equal(t, "ns", foundPod.Namespace)
			assert.Equal(t, "abc-0", foundPod.Name)
			assert.Equal(t, "10.0.0.1", foundPod.Address)
		}
	}

	// Clear the pods map to start fresh...
	c.Pods = make(map[PodIdentifier]*Pod)
	// Set associations to match what we have configured for our OpenTelemetry Collector.
	c.Associations = []Association{
		{
			Sources: []AssociationSource{
				{
					From: "resource_attribute",
					Name: "k8s.pod.uid",
				},
			},
		},
		{
			Sources: []AssociationSource{
				{
					From: "resource_attribute",
					Name: "k8s.namespace.name",
				},
				{
					From: "resource_attribute",
					Name: "k8s.pod.name",
				},
			},
		},
		{
			Sources: []AssociationSource{
				{
					From: "resource_attribute",
					Name: "k8s.pod.ip",
				},
			},
		},
		{
			Sources: []AssociationSource{
				{
					From: "connection",
				},
			},
		},
	}

	// Add a pod and verify that we can find it by all identifiers.
	pod1 := makePod("12345678-1234-1234-1234-123456789abc")
	c.handlePodAdd(pod1)
	doAssertions(pod1)
	assert.Len(t, c.Pods, 4)

	// Delete a pod and verify that we can still found it by all identifiers
	c.handlePodDelete(pod1)
	doAssertions(pod1)
	assert.Len(t, c.Pods, 4)

	// Add a pod with the same values as pod1 except for UID, and verify that we can find it by all identifiers.
	pod2 := makePod("87654321-4321-4321-4321-cba987654321")
	c.handlePodAdd(pod2)
	doAssertions(pod2)
	assert.Len(t, c.Pods, 5) // 4 from pod2 + 1 from pod1 (the pod UID identifier)

	c.deleteLoopProcessing(0 * time.Second)
	assert.Len(t, c.Pods, 4) // Only mappings for pod2 remain
	doAssertions(pod2)

	// Delete pod2 and verify that it gets removed after the next delete loop housekeeping.
	c.handlePodDelete(pod2)
	assert.Len(t, c.Pods, 4) // Only mappings for pod2 remain
	doAssertions(pod2)

	// Delete loop processing should remove mappings for pod2.
	c.deleteLoopProcessing(0 * time.Second)
	assert.Empty(t, c.Pods) // No more mappings
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
		name                string
		rules               ExtractionRules
		attributes          map[string]string
		singularFeatureGate bool
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
			name: "all-labels singular",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:la.*)$"),
						From:     MetadataFromNode,
					},
				},
			},
			attributes: map[string]string{
				"k8s.node.label.label1": "lv1",
			},
			singularFeatureGate: true,
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
		{
			name: "all-annotations singular",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromNode,
					},
				},
			},
			attributes: map[string]string{
				"k8s.node.annotation.annotation1": "av1",
			},
			singularFeatureGate: true,
		},
		{
			name: "captured-groups-no-tag-name",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromNode,
					},
				},
			},
			attributes: map[string]string{
				"k8s.node.labels.label1": "lv1",
			},
		},
		{
			name: "captured-groups-no-tag-name singular",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromNode,
					},
				},
			},
			attributes: map[string]string{
				"k8s.node.label.label1": "lv1",
			},
			singularFeatureGate: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.singularFeatureGate {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), true))
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), true))
				defer func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), false))
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), false))
				}()
			}

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

func TestDeploymentExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	deployment := &apps_v1.Deployment{
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
			name: "labels and annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromDeployment,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromDeployment,
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
						From:     MetadataFromDeployment,
					},
				},
			},
			attributes: map[string]string{
				"k8s.deployment.label.label1": "lv1",
			},
		},
		{
			name: "all-annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromDeployment,
					},
				},
			},
			attributes: map[string]string{
				"k8s.deployment.annotation.annotation1": "av1",
			},
		},
		{
			name: "captured-groups-no-tag-name",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromDeployment,
					},
				},
			},
			attributes: map[string]string{
				"k8s.deployment.label.label1": "lv1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			c.handleDeploymentAdd(deployment)
			n, ok := c.GetDeployment(string(deployment.UID))
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

func TestDeploymentNameFromReplicaSet(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	c.Rules = ExtractionRules{
		DeploymentName:               true,
		DeploymentNameFromReplicaSet: true,
	}

	pod := &api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "auth-service-abc12-xyz3",
			UID:       "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			Namespace: "ns1",
			OwnerReferences: []meta_v1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "auth-service-66f5996c7c",
					UID:        "207ea729-c779-401d-8347-008ecbc137e3",
				},
			},
		},
		Status: api_v1.PodStatus{
			PodIP: "1.1.1.1",
		},
	}

	c.handlePodAdd(pod)
	p, ok := c.GetPod(newPodIdentifier("connection", "", pod.Status.PodIP))
	require.True(t, ok)

	attributes := map[string]string{
		"k8s.deployment.name": "auth-service",
	}
	assert.Len(t, p.Attributes, len(attributes))
	for k, v := range attributes {
		got, ok := p.Attributes[k]
		assert.True(t, ok)
		assert.Equal(t, v, got)
	}
}

func TestStatefulSetExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	statefulset := &apps_v1.StatefulSet{
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
			name: "labels and annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromStatefulSet,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromStatefulSet,
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
						From:     MetadataFromStatefulSet,
					},
				},
			},
			attributes: map[string]string{
				"k8s.statefulset.label.label1": "lv1",
			},
		},
		{
			name: "all-annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromStatefulSet,
					},
				},
			},
			attributes: map[string]string{
				"k8s.statefulset.annotation.annotation1": "av1",
			},
		},
		{
			name: "captured-groups-no-tag-name",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromStatefulSet,
					},
				},
			},
			attributes: map[string]string{
				"k8s.statefulset.label.label1": "lv1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			c.handleStatefulSetAdd(statefulset)
			n, ok := c.GetStatefulSet(string(statefulset.UID))
			require.True(t, ok)

			assert.Len(t, tc.attributes, len(n.Attributes))
			for k, v := range tc.attributes {
				got, ok := n.Attributes[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestDaemonSetExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	daemonset := &apps_v1.DaemonSet{
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
			name: "labels and annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromDaemonSet,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromDaemonSet,
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
						From:     MetadataFromDaemonSet,
					},
				},
			},
			attributes: map[string]string{
				"k8s.daemonset.label.label1": "lv1",
			},
		},
		{
			name: "all-annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromDaemonSet,
					},
				},
			},
			attributes: map[string]string{
				"k8s.daemonset.annotation.annotation1": "av1",
			},
		},
		{
			name: "captured-groups-no-tag-name",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromDaemonSet,
					},
				},
			},
			attributes: map[string]string{
				"k8s.daemonset.label.label1": "lv1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			c.handleDaemonSetAdd(daemonset)
			n, ok := c.GetDaemonSet(string(daemonset.UID))
			require.True(t, ok)

			assert.Len(t, tc.attributes, len(n.Attributes))
			for k, v := range tc.attributes {
				got, ok := n.Attributes[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestJobExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	job := &batch_v1.Job{
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
			name: "labels and annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromJob,
					},
				},
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromJob,
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
						From:     MetadataFromJob,
					},
				},
			},
			attributes: map[string]string{
				"k8s.job.label.label1": "lv1",
			},
		},
		{
			name: "all-annotations",
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						KeyRegex: regexp.MustCompile("^(?:an.*)$"),
						From:     MetadataFromJob,
					},
				},
			},
			attributes: map[string]string{
				"k8s.job.annotation.annotation1": "av1",
			},
		},
		{
			name: "captured-groups-no-tag-name",
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						KeyRegex:             regexp.MustCompile(`^(?:(label\d+))$`),
						HasKeyRegexReference: true,
						From:                 MetadataFromJob,
					},
				},
			},
			attributes: map[string]string{
				"k8s.job.label.label1": "lv1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			c.handleJobAdd(job)
			n, ok := c.GetJob(string(job.UID))
			require.True(t, ok)

			assert.Len(t, tc.attributes, len(n.Attributes))
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
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
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
			name: "automatic-container-level-attributes",
			rules: ExtractionRules{
				ServiceNamespace:  true,
				ServiceName:       true,
				ServiceVersion:    true,
				ServiceInstanceID: true,
			},
			pod: &pod,
			want: PodContainers{
				ByID: map[string]*Container{
					"container1-id-123":     {ServiceInstanceID: "test-namespace.test-pod.container1", ServiceVersion: "0.1.0"},
					"container2-id-456":     {ServiceInstanceID: "test-namespace.test-pod.container2", ServiceVersion: "sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9"},
					"container3-id-abc":     {ServiceInstanceID: "test-namespace.test-pod.container3", ServiceVersion: "1.0@sha256:4b0b1b6f6cdd3e5b9e55f74a1e8d19ed93a3f5a04c6b6c3c57c4e6d19f6b7c4d"},
					"init-container-id-789": {ServiceInstanceID: "test-namespace.test-pod.init_container"},
				},
				ByName: map[string]*Container{
					"container1":     {ServiceInstanceID: "test-namespace.test-pod.container1", ServiceVersion: "0.1.0"},
					"container2":     {ServiceInstanceID: "test-namespace.test-pod.container2", ServiceVersion: "sha256:430ac608abaa332de4ce45d68534447c7a206edc5e98aaff9923ecc12f8a80d9"},
					"container3":     {ServiceInstanceID: "test-namespace.test-pod.container3", ServiceVersion: "1.0@sha256:4b0b1b6f6cdd3e5b9e55f74a1e8d19ed93a3f5a04c6b6c3c57c4e6d19f6b7c4d"},
					"init_container": {ServiceInstanceID: "test-namespace.test-pod.init_container"},
				},
			},
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

func Test_extractPodContainersAttributes_WithFeatureGates(t *testing.T) {
	pod := api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
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
					ContainerID:  "containerd://container1-id-123",
					ImageID:      "docker.io/otel/collector@sha256:55d008bc28344c3178645d40e7d07df30f9d90abe4b53c3fc4e5e9c0295533da",
					RestartCount: 0,
				},
				{
					Name:         "container2",
					ContainerID:  "containerd://container2-id-456",
					RestartCount: 2,
				},
				{
					Name:         "container3",
					ContainerID:  "containerd://container3-id-abc",
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
		want  PodContainers
	}{
		{
			name: "container-image-tags-stable-enabled",
			rules: ExtractionRules{
				ContainerImageTags: true,
			},
			want: PodContainers{
				ByID: map[string]*Container{
					"container1-id-123": {
						ImageTags: []string{"0.1.0"},
					},
					"container2-id-456": {
						ImageTags: []string{"latest"},
					},
					"container3-id-abc": {
						ImageTags: []string{"1.0"},
					},
					"init-container-id-789": {
						ImageTags: []string{"latest"},
					},
				},
				ByName: map[string]*Container{
					"container1": {
						ImageTags: []string{"0.1.0"},
					},
					"container2": {
						ImageTags: []string{"latest"},
					},
					"container3": {
						ImageTags: []string{"1.0"},
					},
					"init_container": {
						ImageTags: []string{"latest"},
					},
				},
			},
		},
		{
			name: "container-image-tag-and-tags-both-stable-enabled",
			rules: ExtractionRules{
				ContainerImageTag:  true,
				ContainerImageTags: true,
			},
			want: PodContainers{
				ByID: map[string]*Container{
					"container1-id-123": {
						ImageTag:  "0.1.0",
						ImageTags: []string{"0.1.0"},
					},
					"container2-id-456": {
						ImageTag:  "latest",
						ImageTags: []string{"latest"},
					},
					"container3-id-abc": {
						ImageTag:  "1.0",
						ImageTags: []string{"1.0"},
					},
					"init-container-id-789": {
						ImageTag:  "latest",
						ImageTags: []string{"latest"},
					},
				},
				ByName: map[string]*Container{
					"container1": {
						ImageTag:  "0.1.0",
						ImageTags: []string{"0.1.0"},
					},
					"container2": {
						ImageTag:  "latest",
						ImageTags: []string{"latest"},
					},
					"container3": {
						ImageTag:  "1.0",
						ImageTags: []string{"1.0"},
					},
					"init_container": {
						ImageTag:  "latest",
						ImageTags: []string{"latest"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Enable stable attributes for these tests
			require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), true))
			defer func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), false))
			}()

			c := WatchClient{Rules: tt.rules}
			transformedPod := removeUnnecessaryPodData(&pod, c.Rules)
			assert.Equal(t, tt.want, c.extractPodContainersAttributes(transformedPod))
		})
	}
}

func Test_extractPodContainersAttributes_NoTag(t *testing.T) {
	pod := api_v1.Pod{
		Spec: api_v1.PodSpec{
			Containers: []api_v1.Container{
				{
					Name:  "test-container",
					Image: "test/image",
				},
			},
		},
	}
	tests := []struct {
		name  string
		rules ExtractionRules
		want  PodContainers
	}{
		{
			name: "container-image-tags-no-tag",
			rules: ExtractionRules{
				ContainerImageTags: true,
			},
			want: PodContainers{
				ByID: map[string]*Container{},
				ByName: map[string]*Container{
					"test-container": {
						ImageTags: []string{"latest"}, // default tag when none specified
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Enable stable attributes for these tests
			require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), true))
			defer func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), false))
			}()

			c := WatchClient{Rules: tt.rules}
			transformedPod := removeUnnecessaryPodData(&pod, c.Rules)
			assert.Equal(t, tt.want, c.extractPodContainersAttributes(transformedPod))
		})
	}
}

func Test_extractPodContainersAttributes_FeatureGates(t *testing.T) {
	pod := api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
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
			},
		},
		Status: api_v1.PodStatus{
			ContainerStatuses: []api_v1.ContainerStatus{
				{
					Name:         "container1",
					ContainerID:  "containerd://container1-id-123",
					RestartCount: 0,
				},
				{
					Name:         "container2",
					ContainerID:  "containerd://container2-id-456",
					RestartCount: 2,
				},
			},
		},
	}

	tests := []struct {
		name               string
		rules              ExtractionRules
		enableStable       bool
		disableLegacy      bool
		wantImageTag       bool
		wantImageTags      bool
		expectedContainer1 *Container
		expectedContainer2 *Container
	}{
		{
			name: "legacy-only",
			rules: ExtractionRules{
				ContainerImageTag:  true,
				ContainerImageTags: true,
			},
			enableStable:  false,
			disableLegacy: false,
			wantImageTag:  true,
			wantImageTags: false,
			expectedContainer1: &Container{
				ImageTag: "0.1.0",
			},
			expectedContainer2: &Container{
				ImageTag: "latest",
			},
		},
		{
			name: "stable-only",
			rules: ExtractionRules{
				ContainerImageTag:  true,
				ContainerImageTags: true,
			},
			enableStable:  true,
			disableLegacy: true,
			wantImageTag:  false,
			wantImageTags: true,
			expectedContainer1: &Container{
				ImageTags: []string{"0.1.0"},
			},
			expectedContainer2: &Container{
				ImageTags: []string{"latest"},
			},
		},
		{
			name: "both-legacy-and-stable",
			rules: ExtractionRules{
				ContainerImageTag:  true,
				ContainerImageTags: true,
			},
			enableStable:  true,
			disableLegacy: false,
			wantImageTag:  true,
			wantImageTags: true,
			expectedContainer1: &Container{
				ImageTag:  "0.1.0",
				ImageTags: []string{"0.1.0"},
			},
			expectedContainer2: &Container{
				ImageTag:  "latest",
				ImageTags: []string{"latest"},
			},
		},
		{
			name: "only-imagetag-rule-stable",
			rules: ExtractionRules{
				ContainerImageTag: true,
			},
			enableStable:       true,
			disableLegacy:      true,
			wantImageTag:       false,
			wantImageTags:      false,
			expectedContainer1: &Container{},
			expectedContainer2: &Container{},
		},
		{
			name: "only-imagetags-rule-legacy",
			rules: ExtractionRules{
				ContainerImageTags: true,
			},
			enableStable:       false,
			disableLegacy:      false,
			wantImageTag:       false,
			wantImageTags:      false,
			expectedContainer1: &Container{},
			expectedContainer2: &Container{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enableStable {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), true))
				defer func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), false))
				}()
			}
			if tt.disableLegacy {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), true))
				defer func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), false))
				}()
			}

			c := WatchClient{Rules: tt.rules}
			transformedPod := removeUnnecessaryPodData(&pod, c.Rules)
			got := c.extractPodContainersAttributes(transformedPod)

			// Check container1
			assert.NotNil(t, got.ByName["container1"])
			if tt.wantImageTag {
				assert.Equal(t, tt.expectedContainer1.ImageTag, got.ByName["container1"].ImageTag)
			} else {
				assert.Empty(t, got.ByName["container1"].ImageTag)
			}
			if tt.wantImageTags {
				assert.Equal(t, tt.expectedContainer1.ImageTags, got.ByName["container1"].ImageTags)
			} else {
				assert.Nil(t, got.ByName["container1"].ImageTags)
			}

			// Check container2
			assert.NotNil(t, got.ByName["container2"])
			if tt.wantImageTag {
				assert.Equal(t, tt.expectedContainer2.ImageTag, got.ByName["container2"].ImageTag)
			} else {
				assert.Empty(t, got.ByName["container2"].ImageTag)
			}
			if tt.wantImageTags {
				assert.Equal(t, tt.expectedContainer2.ImageTags, got.ByName["container2"].ImageTags)
			} else {
				assert.Nil(t, got.ByName["container2"].ImageTags)
			}
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

func TestExtractDeploymentLabelsAnnotations(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	testCases := []struct {
		name                    string
		shouldExtractDeployment bool
		rules                   ExtractionRules
	}{
		{
			name:                    "empty-rules",
			shouldExtractDeployment: false,
			rules:                   ExtractionRules{},
		}, {
			name:                    "pod-rules",
			shouldExtractDeployment: false,
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
			name:                    "deployment-rules-only-annotations",
			shouldExtractDeployment: true,
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromDeployment,
					},
				},
			},
		}, {
			name:                    "deployment-rules-only-labels",
			shouldExtractDeployment: true,
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromDeployment,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			assert.Equal(t, tc.shouldExtractDeployment, c.extractDeploymentLabelsAnnotations())
		})
	}
}

func TestExtractStatefulSetLabelsAnnotations(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	testCases := []struct {
		name                     string
		shouldExtractStatefulSet bool
		rules                    ExtractionRules
	}{
		{
			name:                     "empty-rules",
			shouldExtractStatefulSet: false,
			rules:                    ExtractionRules{},
		}, {
			name:                     "pod-rules",
			shouldExtractStatefulSet: false,
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
			name:                     "statefulset-rules-only-annotations",
			shouldExtractStatefulSet: true,
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromStatefulSet,
					},
				},
			},
		}, {
			name:                     "statefulset-rules-only-labels",
			shouldExtractStatefulSet: true,
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromStatefulSet,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			assert.Equal(t, tc.shouldExtractStatefulSet, c.extractStatefulSetLabelsAnnotations())
		})
	}
}

func TestExtractDaemonSetLabelsAnnotations(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	testCases := []struct {
		name                   string
		shouldExtractDaemonSet bool
		rules                  ExtractionRules
	}{
		{
			name:                   "empty-rules",
			shouldExtractDaemonSet: false,
			rules:                  ExtractionRules{},
		}, {
			name:                   "pod-rules",
			shouldExtractDaemonSet: false,
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
			name:                   "daemonset-rules-only-annotations",
			shouldExtractDaemonSet: true,
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromDaemonSet,
					},
				},
			},
		}, {
			name:                   "daemonset-rules-only-labels",
			shouldExtractDaemonSet: true,
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromDaemonSet,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			assert.Equal(t, tc.shouldExtractDaemonSet, c.extractDaemonSetLabelsAnnotations())
		})
	}
}

func TestExtractJobLabelsAnnotations(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	testCases := []struct {
		name             string
		shouldExtractJob bool
		rules            ExtractionRules
	}{
		{
			name:             "empty-rules",
			shouldExtractJob: false,
			rules:            ExtractionRules{},
		}, {
			name:             "pod-rules",
			shouldExtractJob: false,
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
			name:             "job-rules-only-annotations",
			shouldExtractJob: true,
			rules: ExtractionRules{
				Annotations: []FieldExtractionRule{
					{
						Name: "a1",
						Key:  "annotation1",
						From: MetadataFromJob,
					},
				},
			},
		}, {
			name:             "job-rules-only-labels",
			shouldExtractJob: true,
			rules: ExtractionRules{
				Labels: []FieldExtractionRule{
					{
						Name: "l1",
						Key:  "label1",
						From: MetadataFromJob,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			assert.Equal(t, tc.shouldExtractJob, c.extractJobLabelsAnnotations())
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
	factory := InformersFactoryList{
		newInformer:           NewFakeInformer,
		newNamespaceInformer:  NewFakeNamespaceInformer,
		newReplicaSetInformer: NewFakeReplicaSetInformer,
	}
	c, err := New(set, k8sconfig.APIConfig{}, ExtractionRules{}, f, associations, exclude, newFakeAPIClientset, factory, false, 10*time.Second)
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

func (*neverSyncedResourceEventHandlerRegistration) HasSynced() bool {
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
			c, err := New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, []Association{}, Excludes{}, newFakeAPIClientset, InformersFactoryList{newInformer: tc.informerProvider}, true, 1*time.Second)
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

func Test_parseServiceVersionFromImage(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{
			name:  "no version",
			image: "img",
		},
		{
			name:  "with tag",
			image: "img:1",
			want:  "1",
		},
		{
			name:  "with tag and digest",
			image: "img:1@sha256:232a180dbcbcfa7250917507f3827d88a9ae89bb1cdd8fe3ac4db7b764ebb25a",
			want:  "1@sha256:232a180dbcbcfa7250917507f3827d88a9ae89bb1cdd8fe3ac4db7b764ebb25a",
		},
		{
			name:  "with digest",
			image: "img@sha256:232a180dbcbcfa7250917507f3827d88a9ae89bb1cdd8fe3ac4db7b764ebb25a",
			want:  "sha256:232a180dbcbcfa7250917507f3827d88a9ae89bb1cdd8fe3ac4db7b764ebb25a",
		},
		{
			name:  "with registry",
			image: "registry.io/img",
		},
		{
			name:  "with port",
			image: "registry.io:8080/img",
		},
		{
			name:  "latest",
			image: "img:latest",
			want:  "latest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := parseServiceVersionFromImage(tt.image)
			// error is just for debugging
			assert.Equalf(t, tt.want, got, "parseServiceVersionFromImage(%v)", tt.image)
		})
	}
}

func TestGetIdentifiersFromAssoc(t *testing.T) {
	tests := map[string]struct {
		associations []Association
		pod          *Pod
		expected     []PodIdentifier
	}{
		"K8SPodUID": {
			associations: []Association{
				{
					Sources: []AssociationSource{
						{
							From: ResourceSource,
							Name: "k8s.pod.uid",
						},
					},
				},
			},
			pod: &Pod{
				PodUID: "myK8sPodUID",
			},
			expected: []PodIdentifier{
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "k8s.pod.uid"}, Value: "myK8sPodUID"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "k8s.pod.uid"}, Value: "myK8sPodUID"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
			},
		},
		"ContainerID": {
			associations: []Association{
				{
					Sources: []AssociationSource{
						{
							From: ResourceSource,
							Name: "container.id",
						},
					},
				},
			},
			pod: &Pod{
				PodUID: "myK8sPodUID",
				Containers: PodContainers{
					ByID: map[string]*Container{
						"id1": {
							Name: "id1",
						},
						"id2": {
							Name: "id2",
						},
					},
				},
			},
			expected: []PodIdentifier{
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "container.id"}, Value: "id1"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "container.id"}, Value: "id2"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "k8s.pod.uid"}, Value: "myK8sPodUID"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
			},
		},
		"multiple associations": {
			associations: []Association{
				{
					Sources: []AssociationSource{
						{
							From: ResourceSource,
							Name: "container.id",
						},
						{
							From: ConnectionSource,
						},
					},
				},
			},
			pod: &Pod{
				PodUID:  "myK8sPodUID",
				Address: "localhost",
				Containers: PodContainers{
					ByID: map[string]*Container{
						"id1": {
							Name: "id1",
						},
						"id2": {
							Name: "id2",
						},
					},
				},
			},
			expected: []PodIdentifier{
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "container.id"}, Value: "id1"},
					PodIdentifierAttribute{Source: AssociationSource{From: "connection", Name: ""}, Value: "localhost"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "container.id"}, Value: "id2"},
					PodIdentifierAttribute{Source: AssociationSource{From: "connection", Name: ""}, Value: "localhost"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "k8s.pod.uid"}, Value: "myK8sPodUID"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "connection", Name: ""}, Value: "localhost"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
				{
					PodIdentifierAttribute{Source: AssociationSource{From: "resource_attribute", Name: "k8s.pod.ip"}, Value: "localhost"},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
					PodIdentifierAttribute{Source: AssociationSource{From: "", Name: ""}, Value: ""},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wc, _ := newTestClient(t)
			wc.Associations = tc.associations
			actual := wc.getIdentifiersFromAssoc(tc.pod)
			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}

func TestCronJobExtractionRules_FromJobOwner(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	// Disable saving ip into k8s.pod.ip so attributes length assertions stay predictable
	c.Associations[0].Sources[0].Name = ""

	// Pod owned by a Job
	pod := &api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "my-cronjob-27667920-pod",
			UID:               "pod-uid-1",
			Namespace:         "ns1",
			CreationTimestamp: meta_v1.Now(),
			OwnerReferences: []meta_v1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "Job",
					Name:       "my-cronjob-27667920",
					UID:        "job-uid-123",
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

	// The Job object the pod points to (we'll mutate OwnerReferences per test case)
	job := &batch_v1.Job{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "my-cronjob-27667920",
			Namespace: "ns1",
			UID:       "job-uid-123",
		},
	}

	isController := true
	isNotController := false

	testCases := []struct {
		name      string
		rules     ExtractionRules
		jobOwners []meta_v1.OwnerReference
		want      map[string]string
	}{
		{
			name:  "no-rules",
			rules: ExtractionRules{},
			jobOwners: []meta_v1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "my-cronjob",
					UID:        "cron-uid-999",
					Controller: &isController,
				},
			},
			want: map[string]string{},
		},
		{
			name: "cronjob-is-controller_emit_uid_only",
			rules: ExtractionRules{
				CronJobUID: true,
			},
			jobOwners: []meta_v1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "my-cronjob",
					UID:        "cron-uid-999",
					Controller: &isController,
				},
			},
			want: map[string]string{
				"k8s.cronjob.uid": "cron-uid-999",
			},
		},
		{
			name: "cronjob-is-controller_emit_name_and_uid",
			rules: ExtractionRules{
				CronJobName: true,
				CronJobUID:  true,
			},
			jobOwners: []meta_v1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "my-cronjob",
					UID:        "cron-uid-999",
					Controller: &isController,
				},
			},
			want: map[string]string{
				"k8s.cronjob.name": "my-cronjob",
				"k8s.cronjob.uid":  "cron-uid-999",
			},
		},
		{
			name: "cronjob-is-not-controller_do_not_emit",
			rules: ExtractionRules{
				CronJobName: true,
				CronJobUID:  true,
			},
			jobOwners: []meta_v1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "my-cronjob",
					UID:        "cron-uid-999",
					Controller: &isNotController,
				},
			},
			want: map[string]string{
				"k8s.cronjob.name": "my-cronjob",
			},
		},
		{
			name: "multiple_owners_only_controller_counts",
			rules: ExtractionRules{
				CronJobName: true,
				CronJobUID:  true,
			},
			jobOwners: []meta_v1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "cj-not-controller",
					UID:        "cron-uid-111",
					Controller: &isNotController,
				},
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "cj-controller",
					UID:        "cron-uid-222",
					Controller: &isController,
				},
			},
			want: map[string]string{
				"k8s.cronjob.name": "my-cronjob",
				"k8s.cronjob.uid":  "cron-uid-222",
			},
		},
		{
			name: "no_cronjob_owner",
			rules: ExtractionRules{
				CronJobName: true,
				CronJobUID:  true,
			},
			jobOwners: []meta_v1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "SomethingElse",
					Name:       "not-a-cronjob",
					UID:        "whatever",
					Controller: &isController,
				},
			},
			want: map[string]string{
				"k8s.cronjob.name": "my-cronjob",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules

			// Set owners on Job according to case
			job.OwnerReferences = tc.jobOwners

			// Emulate informer transforms (like other tests do)
			transformedPod := removeUnnecessaryPodData(pod, c.Rules)

			// Feed caches
			c.handleJobAdd(job)
			c.handlePodAdd(transformedPod)

			// Fetch enriched pod by connection id
			p, ok := c.GetPod(newPodIdentifier("connection", "", pod.Status.PodIP))
			require.True(t, ok)

			assert.Len(t, p.Attributes, len(tc.want))
			for k, v := range tc.want {
				got, ok := p.Attributes[k]
				assert.True(t, ok, "expected attribute %s", k)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestExtractDeploymentNameFromReplicaSet(t *testing.T) {
	tests := []struct {
		name           string
		replicaSetName string
		expected       string
	}{
		{
			name:           "valid replicaset name with pod template hash",
			replicaSetName: "my-deployment-7b9f4c8d5e",
			expected:       "my-deployment",
		},
		{
			name:           "complex deployment name with dashes",
			replicaSetName: "my-complex-deployment-name-7b9f4c8d5e",
			expected:       "my-complex-deployment-name",
		},
		{
			name:           "single word deployment",
			replicaSetName: "nginx-7b9f4c8d5e",
			expected:       "nginx",
		},
		{
			name:           "replicaset name without valid pod template hash",
			replicaSetName: "my-deployment-invalidhash",
			expected:       "",
		},
		{
			name:           "replicaset name with short hash",
			replicaSetName: "my-deployment-7b9f4c",
			expected:       "",
		},
		{
			name:           "replicaset name with long hash",
			replicaSetName: "my-deployment-7b9f4c8d5e123",
			expected:       "",
		},
		{
			name:           "replicaset name with uppercase in hash",
			replicaSetName: "my-deployment-7B9F4C8D5E",
			expected:       "",
		},
		{
			name:           "replicaset name with special characters in hash",
			replicaSetName: "my-deployment-7b9f4c8d5_",
			expected:       "",
		},
		{
			name:           "empty replicaset name",
			replicaSetName: "",
			expected:       "",
		},
		{
			name:           "replicaset name without dashes",
			replicaSetName: "deployment7b9f4c8d5e",
			expected:       "",
		},
		{
			name:           "replicaset name with only hash part",
			replicaSetName: "7b9f4c8d5e",
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDeploymentNameFromReplicaSet(tt.replicaSetName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// trackableInformer is a mock informer that tracks if Run has been called.
type trackableInformer struct {
	cache.SharedInformer
	runCalled bool
	mutex     sync.Mutex
}

func (i *trackableInformer) Run(stopCh <-chan struct{}) {
	i.mutex.Lock()
	i.runCalled = true
	i.mutex.Unlock()
	i.SharedInformer.Run(stopCh)
}

func (i *trackableInformer) hasRun() bool {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	return i.runCalled
}

func newTrackableInformer(client kubernetes.Interface, namespace string, labelSelector labels.Selector, fieldSelector fields.Selector) cache.SharedInformer {
	return &trackableInformer{
		SharedInformer: NewFakeInformer(client, namespace, labelSelector, fieldSelector),
	}
}

func TestReplicaSetInformerConditionalStart(t *testing.T) {
	tests := []struct {
		name      string
		rules     ExtractionRules
		expectRun bool
	}{
		{
			name:      "start informer if deployment UID is requested",
			rules:     ExtractionRules{DeploymentUID: true},
			expectRun: true,
		},
		{
			name:      "start informer if deployment name is requested",
			rules:     ExtractionRules{DeploymentName: true},
			expectRun: true,
		},
		{
			name:      "don't start informer if deployment name from replicaset is requested",
			rules:     ExtractionRules{DeploymentName: true, DeploymentNameFromReplicaSet: true},
			expectRun: false,
		},
		{
			name:      "start informer if deployment UID and name are requested",
			rules:     ExtractionRules{DeploymentName: true, DeploymentUID: true},
			expectRun: true,
		},
		{
			name:      "start informer if deployment UID and name from replicaset are requested",
			rules:     ExtractionRules{DeploymentName: true, DeploymentUID: true, DeploymentNameFromReplicaSet: true},
			expectRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := InformersFactoryList{
				newInformer:          NewFakeInformer,
				newNamespaceInformer: NewFakeNamespaceInformer,
				newReplicaSetInformer: func(kc kubernetes.Interface, ns string) cache.SharedInformer {
					return newTrackableInformer(kc, ns, labels.Everything(), fields.Everything())
				},
			}

			c, err := New(componenttest.NewNopTelemetrySettings(), k8sconfig.APIConfig{}, tt.rules, Filters{}, []Association{}, Excludes{}, newFakeAPIClientset, factory, false, 10*time.Second)
			require.NoError(t, err)
			wc := c.(*WatchClient)

			err = wc.Start()
			require.NoError(t, err)
			defer wc.Stop()

			// Allow time for the informer goroutine to start
			time.Sleep(100 * time.Millisecond)

			informer := wc.replicasetInformer.(*trackableInformer)
			assert.Equal(t, tt.expectRun, informer.hasRun())
		})
	}
}

func TestDeploymentNameFromReplicaSetFeature(t *testing.T) {
	// Test the DeploymentNameFromReplicaSet flag functionality with extractPodAttributes

	tests := []struct {
		name                                string
		deploymentNameFromReplicaSetEnabled bool
		replicaSetInCache                   bool
		deploymentInRS                      bool
		replicaSetName                      string
		expectedDeploymentName              string
	}{
		{
			name:                                "flag disabled - no deployment name extraction from replicaset name",
			deploymentNameFromReplicaSetEnabled: false,
			replicaSetInCache:                   false,
			deploymentInRS:                      false,
			replicaSetName:                      "my-deployment-7b9f4c8d5e",
			expectedDeploymentName:              "",
		},
		{
			name:                                "flag enabled - replicaset not in cache",
			deploymentNameFromReplicaSetEnabled: true,
			replicaSetInCache:                   false,
			deploymentInRS:                      false,
			replicaSetName:                      "my-deployment-7b9f4c8d5e",
			expectedDeploymentName:              "my-deployment",
		},
		{
			name:                                "flag enabled - replicaset in cache but no deployment",
			deploymentNameFromReplicaSetEnabled: true,
			replicaSetInCache:                   true,
			deploymentInRS:                      false,
			replicaSetName:                      "my-deployment-7b9f4c8d5e",
			expectedDeploymentName:              "my-deployment",
		},
		{
			name:                                "flag enabled - replicaset in cache with deployment (should prefer existing)",
			deploymentNameFromReplicaSetEnabled: true,
			replicaSetInCache:                   true,
			deploymentInRS:                      true,
			replicaSetName:                      "my-deployment-7b9f4c8d5e",
			expectedDeploymentName:              "my-deployment",
		},
		{
			name:                                "flag enabled - invalid replicaset name",
			deploymentNameFromReplicaSetEnabled: true,
			replicaSetInCache:                   false,
			deploymentInRS:                      false,
			replicaSetName:                      "invalid-name",
			expectedDeploymentName:              "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestClientWithRulesAndFilters(t, Filters{})
			c.Rules.DeploymentName = true
			if tt.deploymentNameFromReplicaSetEnabled {
				c.Rules.DeploymentNameFromReplicaSet = true
			}

			// Create a replicaset if needed
			if tt.replicaSetInCache {
				replicaset := &apps_v1.ReplicaSet{
					ObjectMeta: meta_v1.ObjectMeta{
						Name:      tt.replicaSetName,
						Namespace: "default",
						UID:       "rs-uid-123",
					},
				}

				if tt.deploymentInRS {
					isController := true
					replicaset.OwnerReferences = []meta_v1.OwnerReference{
						{
							Kind:       "Deployment",
							Name:       "real-deployment-name",
							UID:        "deploy-uid-123",
							Controller: &isController,
						},
					}
				}

				c.handleReplicaSetAdd(replicaset)
			}

			// Create a pod with replicaset owner reference
			pod := &api_v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					UID:       "pod-uid-123",
					OwnerReferences: []meta_v1.OwnerReference{
						{
							Kind: "ReplicaSet",
							Name: tt.replicaSetName,
							UID:  "rs-uid-123",
						},
					},
				},
				Status: api_v1.PodStatus{
					PodIP: "1.2.3.4",
				},
			}

			// Extract attributes
			attributes := c.extractPodAttributes(pod)

			// Check the result
			if tt.expectedDeploymentName != "" {
				deploymentName, exists := attributes["k8s.deployment.name"]
				assert.True(t, exists, "Expected deployment name to be extracted")
				assert.Equal(t, tt.expectedDeploymentName, deploymentName)
			} else {
				_, exists := attributes["k8s.deployment.name"]
				assert.False(t, exists, "Expected no deployment name to be extracted")
			}
		})
	}
}

func TestDeploymentHashSuffixPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid pod template hash",
			input:    "7b9f4c8d5e",
			expected: true,
		},
		{
			name:     "valid all digits",
			input:    "1234567890",
			expected: true,
		},
		{
			name:     "valid all letters",
			input:    "abcdefghij",
			expected: true,
		},
		{
			name:     "too short",
			input:    "7b9f4c8d5",
			expected: false,
		},
		{
			name:     "too long",
			input:    "7b9f4c8d5e1",
			expected: false,
		},
		{
			name:     "contains uppercase",
			input:    "7B9F4C8D5E",
			expected: false,
		},
		{
			name:     "contains special character",
			input:    "7b9f4c8d5-",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deploymentHashSuffixPattern.MatchString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleDeploymentUpdate(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	c.Rules = ExtractionRules{
		DeploymentName: true,
	}

	deployment := &apps_v1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			UID:       "deployment-uid-123",
		},
	}

	// Add initial deployment
	c.handleDeploymentAdd(deployment)
	assert.Len(t, c.Deployments, 1)

	// Update deployment
	updatedDeployment := &apps_v1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-deployment-updated",
			Namespace: "default",
			UID:       "deployment-uid-123",
		},
	}
	c.handleDeploymentUpdate(deployment, updatedDeployment)

	// Verify update
	d, ok := c.GetDeployment(string(updatedDeployment.UID))
	require.True(t, ok)
	assert.Equal(t, "test-deployment-updated", d.Name)
}

func TestHandleDeploymentDelete(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	deployment := &apps_v1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			UID:       "deployment-uid-123",
		},
	}

	// Add deployment
	c.handleDeploymentAdd(deployment)
	assert.Len(t, c.Deployments, 1)

	// Delete deployment
	c.handleDeploymentDelete(deployment)

	// Verify deletion
	_, ok := c.GetDeployment(string(deployment.UID))
	assert.False(t, ok)
	assert.Empty(t, c.Deployments)
}

func TestHandleStatefulSetUpdate(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	c.Rules = ExtractionRules{
		StatefulSetName: true,
	}

	statefulset := &apps_v1.StatefulSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
			UID:       "statefulset-uid-123",
		},
	}

	// Add initial statefulset
	c.handleStatefulSetAdd(statefulset)
	assert.Len(t, c.StatefulSets, 1)

	// Update statefulset
	updatedStatefulSet := &apps_v1.StatefulSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-statefulset-updated",
			Namespace: "default",
			UID:       "statefulset-uid-123",
		},
	}
	c.handleStatefulSetUpdate(statefulset, updatedStatefulSet)

	// Verify update
	s, ok := c.GetStatefulSet(string(updatedStatefulSet.UID))
	require.True(t, ok)
	assert.Equal(t, "test-statefulset-updated", s.Name)
}

func TestHandleStatefulSetDelete(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	statefulset := &apps_v1.StatefulSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
			UID:       "statefulset-uid-123",
		},
	}

	// Add statefulset
	c.handleStatefulSetAdd(statefulset)
	assert.Len(t, c.StatefulSets, 1)

	// Delete statefulset
	c.handleStatefulSetDelete(statefulset)

	// Verify deletion
	_, ok := c.GetStatefulSet(string(statefulset.UID))
	assert.False(t, ok)
	assert.Empty(t, c.StatefulSets)
}

func TestHandleDaemonSetUpdate(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	c.Rules = ExtractionRules{
		DaemonSetName: true,
	}

	daemonset := &apps_v1.DaemonSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: "default",
			UID:       "daemonset-uid-123",
		},
	}

	// Add initial daemonset
	c.handleDaemonSetAdd(daemonset)
	assert.Len(t, c.DaemonSets, 1)

	// Update daemonset
	updatedDaemonSet := &apps_v1.DaemonSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-daemonset-updated",
			Namespace: "default",
			UID:       "daemonset-uid-123",
		},
	}
	c.handleDaemonSetUpdate(daemonset, updatedDaemonSet)

	// Verify update
	d, ok := c.GetDaemonSet(string(updatedDaemonSet.UID))
	require.True(t, ok)
	assert.Equal(t, "test-daemonset-updated", d.Name)
}

func TestHandleDaemonSetDelete(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	daemonset := &apps_v1.DaemonSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: "default",
			UID:       "daemonset-uid-123",
		},
	}

	// Add daemonset
	c.handleDaemonSetAdd(daemonset)
	assert.Len(t, c.DaemonSets, 1)

	// Delete daemonset
	c.handleDaemonSetDelete(daemonset)

	// Verify deletion
	_, ok := c.GetDaemonSet(string(daemonset.UID))
	assert.False(t, ok)
	assert.Empty(t, c.DaemonSets)
}

func TestHandleJobUpdate(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})
	c.Rules = ExtractionRules{
		JobName: true,
	}

	job := &batch_v1.Job{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			UID:       "job-uid-123",
		},
	}

	// Add initial job
	c.handleJobAdd(job)
	assert.Len(t, c.Jobs, 1)

	// Update job
	updatedJob := &batch_v1.Job{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-job-updated",
			Namespace: "default",
			UID:       "job-uid-123",
		},
	}
	c.handleJobUpdate(job, updatedJob)

	// Verify update
	j, ok := c.GetJob(string(updatedJob.UID))
	require.True(t, ok)
	assert.Equal(t, "test-job-updated", j.Name)
}

func TestHandleJobDelete(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, Filters{})

	job := &batch_v1.Job{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			UID:       "job-uid-123",
		},
	}

	// Add job
	c.handleJobAdd(job)
	assert.Len(t, c.Jobs, 1)

	// Delete job
	c.handleJobDelete(job)

	// Verify deletion
	_, ok := c.GetJob(string(job.UID))
	assert.False(t, ok)
	assert.Empty(t, c.Jobs)
}
