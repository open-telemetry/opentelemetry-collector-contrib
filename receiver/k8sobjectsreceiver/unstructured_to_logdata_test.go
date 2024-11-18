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
						"name":      "generic-name",
						"namespace": "my-namespace",
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
		namespaceAttr, ok := rl.Resource().Attributes().Get(semconv.AttributeK8SNamespaceName)
		assert.True(t, ok)
		assert.Equal(t, "my-namespace", namespaceAttr.Str())
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		assert.Equal(t, 1, rl.ScopeLogs().Len())
		assert.Equal(t, 1, logRecords.Len())

		attrs := logRecords.At(0).Attributes()
		eventName, ok := attrs.Get("event.name")
		require.True(t, ok)
		assert.EqualValues(t, "generic-name", eventName.AsRaw())

		eventType, ok := attrs.Get("event.type")
		require.True(t, ok)
		assert.Equal(t, string(watch.Added), eventType.AsString())
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

	t.Run("Test pull and watch objects result in the same log entry body", func(t *testing.T) {
		observedTimestamp := time.Now()
		config := &K8sObjectsConfig{
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "events",
			},
		}
		watchedEvent := &watch.Event{
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

		pulledEvent := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{{
				Object: map[string]any{
					"kind":       "Event",
					"apiVersion": "v1",
					"metadata": map[string]any{
						"name": "generic-name",
					},
				},
			}},
		}

		logEntryFromWatchEvent, err := watchObjectsToLogData(watchedEvent, observedTimestamp, config)
		assert.Nil(t, err)
		assert.NotNil(t, logEntryFromWatchEvent)

		logEntryFromPulledEvent := unstructuredListToLogData(pulledEvent, observedTimestamp, config)
		assert.NotNil(t, logEntryFromPulledEvent)

		assert.Equal(t,
			logEntryFromWatchEvent.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body(),
			logEntryFromPulledEvent.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body(),
		)
	})
}
