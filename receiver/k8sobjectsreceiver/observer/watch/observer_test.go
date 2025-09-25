package watch

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/observer"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"testing"
	"time"
)

func TestObserver(t *testing.T) {
	mockClient := newMockDynamicClient()
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	cfg := observer.Config{
		Gvr: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
		},
		Namespaces: []string{"default"},
	}

	receivedEventsChan := make(chan *watch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), func(event *watch.Event) {
		receivedEventsChan <- event
	})

	require.Nil(t, err)

	stopChan := obs.Start(t.Context())

	time.Sleep(time.Millisecond * 100)

	mockClient.createPods(
		generatePod("pod2", "default", map[string]any{
			"environment": "test",
		}, "2"),
		generatePod("pod3", "default_ignore", map[string]any{
			"environment": "production",
		}, "3"),
		generatePod("pod4", "default", map[string]any{
			"environment": "production",
		}, "4"),
	)

	verifyReceivedEvents(t, 2, receivedEventsChan, stopChan)
}

func TestObserverWithInitialState(t *testing.T) {
	mockClient := newMockDynamicClient()
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	cfg := observer.Config{
		Gvr: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
		},
		Namespaces:          []string{"default"},
		IncludeInitialState: true,
	}

	receivedEventsChan := make(chan *watch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), func(event *watch.Event) {
		receivedEventsChan <- event
	})

	require.Nil(t, err)

	stopChan := obs.Start(t.Context())

	verifyReceivedEvents(t, 1, receivedEventsChan, stopChan)
}

func verifyReceivedEvents(t *testing.T, numEvents int, receivedEventsChan chan *watch.Event, stopChan chan struct{}) {
	receivedEvents := 0

	exit := false
	for {
		select {
		case <-receivedEventsChan:
			receivedEvents++
			if receivedEvents == numEvents {
				exit = true
				break
			}
		case <-time.After(10 * time.Second):
			t.Log("timed out waiting for expected events")
			t.Fail()
			exit = true
			break
		}
		if exit {
			break
		}
	}

	stopChan <- struct{}{}
}

type mockDynamicClient struct {
	client dynamic.Interface
}

func (c mockDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return c.client.Resource(resource)
}

func newMockDynamicClient() mockDynamicClient {
	scheme := runtime.NewScheme()
	objs := []runtime.Object{}

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}

	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...)
	return mockDynamicClient{
		client: fakeClient,
	}
}

func (c mockDynamicClient) createPods(objects ...*unstructured.Unstructured) {
	pods := c.client.Resource(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "pods",
	})
	for _, pod := range objects {
		_, _ = pods.Namespace(pod.GetNamespace()).Create(context.Background(), pod, v1.CreateOptions{})
	}
}

func (c mockDynamicClient) deletePods(objects ...*unstructured.Unstructured) {
	pods := c.client.Resource(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "pods",
	})
	for _, pod := range objects {
		_ = pods.Namespace(pod.GetNamespace()).Delete(context.Background(), pod.GetName(), v1.DeleteOptions{})
	}
}

func generatePod(name, namespace string, labels map[string]any, resourceVersion string) *unstructured.Unstructured {
	pod := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pods",
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      name,
				"labels":    labels,
			},
		},
	}

	pod.SetResourceVersion(resourceVersion)
	return &pod
}
