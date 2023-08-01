// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
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

		assert.Equal(t, logs.LogRecordCount(), 4)

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, resourceLogs.Len(), 2)

		namespaces = []string{"ns1", "ns2"}
		for i, namespace := range namespaces {
			rl := resourceLogs.At(i)
			resourceAttributes := rl.Resource().Attributes()
			ns, _ := resourceAttributes.Get(semconv.AttributeK8SNamespaceName)
			assert.Equal(t, ns.AsString(), namespace)
			assert.Equal(t, rl.ScopeLogs().Len(), 1)
			assert.Equal(t, rl.ScopeLogs().At(0).LogRecords().Len(), 2)
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

		assert.Equal(t, logs.LogRecordCount(), 3)

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, resourceLogs.Len(), 1)
		rl := resourceLogs.At(0)
		resourceAttributes := rl.Resource().Attributes()
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		_, ok := resourceAttributes.Get(semconv.AttributeK8SNamespaceName)
		assert.Equal(t, ok, false)
		assert.Equal(t, rl.ScopeLogs().Len(), 1)
		assert.Equal(t, logRecords.Len(), 3)

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
				Object: map[string]interface{}{
					"kind":       "Event",
					"apiVersion": "v1",
					"metadata": map[string]interface{}{
						"name": "generic-name",
					},
				},
			},
		}

		logs := watchObjectsToLogData(event, time.Now(), config)

		assert.Equal(t, logs.LogRecordCount(), 1)

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, resourceLogs.Len(), 1)
		rl := resourceLogs.At(0)
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		assert.Equal(t, rl.ScopeLogs().Len(), 1)
		assert.Equal(t, logRecords.Len(), 1)

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
				Object: map[string]interface{}{
					"kind":       "Event",
					"apiVersion": "v1",
					"metadata": map[string]interface{}{
						"name": "generic-name",
					},
				},
			},
		}

		observedAt := time.Now()
		logs := watchObjectsToLogData(event, observedAt, config)

		assert.Equal(t, logs.LogRecordCount(), 1)

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, resourceLogs.Len(), 1)
		rl := resourceLogs.At(0)
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		assert.Equal(t, rl.ScopeLogs().Len(), 1)
		assert.Equal(t, logRecords.Len(), 1)
		assert.Greater(t, logRecords.At(0).ObservedTimestamp().AsTime().Unix(), int64(0))
		assert.Equal(t, logRecords.At(0).ObservedTimestamp().AsTime().Unix(), observedAt.Unix())
	})

}
