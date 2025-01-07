// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

func TestUnstructuredListToLogData(t *testing.T) {
	t.Parallel()

	t.Run("Test namespaced objects", func(t *testing.T) {
		objects := unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{},
		}
		namespaces := []string{"ns1", "ns1", "ns2", "ns2"}
		for i, namespace := range namespaces {
			object := unstructured.Unstructured{}
			object.SetKind("Pod")
			object.SetNamespace(namespace)
			object.SetName(fmt.Sprintf("pod-%d", i))
			objects.Items = append(objects.Items, object)
		}

		config := &K8sObjectsConfig{
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		}
		logs := pullObjectsToLogData(&objects, time.Now(), config)

		assert.Equal(t, 4, logs.LogRecordCount())

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, 2, resourceLogs.Len())

		namespaces = []string{"ns1", "ns2"}
		for i, namespace := range namespaces {
			rl := resourceLogs.At(i)
			resourceAttributes := rl.Resource().Attributes()
			ns, _ := resourceAttributes.Get(semconv.AttributeK8SNamespaceName)
			assert.Equal(t, ns.AsString(), namespace)
			assert.Equal(t, 1, rl.ScopeLogs().Len())
			assert.Equal(t, 2, rl.ScopeLogs().At(0).LogRecords().Len())
		}
	})

	t.Run("Test non-namespaced objects", func(t *testing.T) {
		objects := unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{},
		}
		for i := 0; i < 3; i++ {
			object := unstructured.Unstructured{}
			object.SetKind("Node")
			object.SetName(fmt.Sprintf("node-%d", i))
			objects.Items = append(objects.Items, object)
		}

		config := &K8sObjectsConfig{
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "nodes",
			},
		}

		logs := pullObjectsToLogData(&objects, time.Now(), config)

		assert.Equal(t, 3, logs.LogRecordCount())

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, 1, resourceLogs.Len())
		rl := resourceLogs.At(0)
		resourceAttributes := rl.Resource().Attributes()
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		_, ok := resourceAttributes.Get(semconv.AttributeK8SNamespaceName)
		assert.False(t, ok)
		assert.Equal(t, 1, rl.ScopeLogs().Len())
		assert.Equal(t, 3, logRecords.Len())
	})

	t.Run("Test event.name in watch events", func(t *testing.T) {
		config := &K8sObjectsConfig{
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "events",
			},
		}
		event := &watch.Event{
			Type: watch.Added,
			Object: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Event",
					"apiVersion": "v1",
					"metadata": map[string]any{
						"name": "generic-name",
					},
				},
			},
		}

		logs, err := watchObjectsToLogData(event, time.Now(), config)
		assert.NoError(t, err)

		assert.Equal(t, 1, logs.LogRecordCount())

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, 1, resourceLogs.Len())
		rl := resourceLogs.At(0)
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		assert.Equal(t, 1, rl.ScopeLogs().Len())
		assert.Equal(t, 1, logRecords.Len())

		attrs := logRecords.At(0).Attributes()
		eventName, ok := attrs.Get("event.name")
		require.True(t, ok)
		assert.EqualValues(t, "generic-name", eventName.AsRaw())
	})

	t.Run("Test event observed timestamp is present", func(t *testing.T) {
		config := &K8sObjectsConfig{
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "events",
			},
		}
		event := &watch.Event{
			Type: watch.Added,
			Object: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Event",
					"apiVersion": "v1",
					"metadata": map[string]any{
						"name": "generic-name",
					},
				},
			},
		}

		observedAt := time.Now()
		logs, err := watchObjectsToLogData(event, observedAt, config)
		assert.NoError(t, err)

		assert.Equal(t, 1, logs.LogRecordCount())

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, 1, resourceLogs.Len())
		rl := resourceLogs.At(0)
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		assert.Equal(t, 1, rl.ScopeLogs().Len())
		assert.Equal(t, 1, logRecords.Len())
		assert.Positive(t, logRecords.At(0).ObservedTimestamp().AsTime().Unix())
		assert.Equal(t, logRecords.At(0).ObservedTimestamp().AsTime().Unix(), observedAt.Unix())
	})
}
