// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

// TestReceiverMap verifies the multimap behavior of receiverMap.
// receiverMap maps one endpoint ID to multiple receiver entries because
// multiple receiver templates can match the same endpoint (e.g., a pod
// might have both a prometheus scraper and a filelog receiver configured).
func TestReceiverMap(t *testing.T) {
	r1 := &nopWithEndpointReceiver{}
	r2 := &nopWithEndpointReceiver{}
	r3 := &nopWithEndpointReceiver{}

	e1 := receiverEntry{receiver: r1, id: component.MustNewID("test1")}
	e2 := receiverEntry{receiver: r2, id: component.MustNewID("test2")}
	e3 := receiverEntry{receiver: r3, id: component.MustNewID("test3")}

	t.Run("multimap stores multiple entries under same key", func(t *testing.T) {
		rm := receiverMap{}
		assert.Equal(t, 0, rm.Size())

		rm.Put("a", e1)
		assert.Equal(t, 1, rm.Size())

		rm.Put("a", e2) // Second entry under same key "a"
		assert.Equal(t, 2, rm.Size())

		rm.Put("b", e3)
		assert.Equal(t, 3, rm.Size())
	})

	t.Run("Get returns all entries for key in insertion order", func(t *testing.T) {
		rm := receiverMap{}
		rm.Put("a", e1)
		rm.Put("a", e2)

		assert.Equal(t, []receiverEntry{e1, e2}, rm.Get("a"))
		assert.Nil(t, rm.Get("missing"))
	})

	t.Run("RemoveAll removes all entries for key", func(t *testing.T) {
		rm := receiverMap{}
		rm.Put("a", e1)
		rm.Put("a", e2)
		rm.Put("b", e3)

		rm.RemoveAll("missing") // no-op for missing keys
		assert.Equal(t, 3, rm.Size())

		rm.RemoveAll("b")
		assert.Equal(t, 2, rm.Size())

		rm.RemoveAll("a")
		assert.Equal(t, 0, rm.Size())
	})

	t.Run("Values returns all receiver components across all endpoints", func(t *testing.T) {
		rm := receiverMap{}
		rm.Put("a", e1)
		rm.Put("b", e2)

		assert.Equal(t, 2, rm.Size())
		assert.Equal(t, []component.Component{r1, r2}, rm.Values())
	})
}

func TestReceiverEntryConfigsEqual(t *testing.T) {
	entry := receiverEntry{
		resolvedConfig:           userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
		resolvedDiscoveredConfig: userConfigMap{"endpoint": "localhost:8080"},
		computedResourceAttrs:    map[string]string{"k8s.pod.name": "my-pod", "app": "v1"},
	}

	tests := []struct {
		name                     string
		resolvedConfig           userConfigMap
		resolvedDiscoveredConfig userConfigMap
		computedAttrs            map[string]string
		expected                 bool
	}{
		{
			name:                     "all configs and attrs equal",
			resolvedConfig:           userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
			resolvedDiscoveredConfig: userConfigMap{"endpoint": "localhost:8080"},
			computedAttrs:            map[string]string{"k8s.pod.name": "my-pod", "app": "v1"},
			expected:                 true,
		},
		{
			name:                     "different resolved config",
			resolvedConfig:           userConfigMap{"key": "different"},
			resolvedDiscoveredConfig: userConfigMap{"endpoint": "localhost:8080"},
			computedAttrs:            map[string]string{"k8s.pod.name": "my-pod", "app": "v1"},
			expected:                 false,
		},
		{
			name:                     "different discovered config",
			resolvedConfig:           userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
			resolvedDiscoveredConfig: userConfigMap{"endpoint": "localhost:9090"},
			computedAttrs:            map[string]string{"k8s.pod.name": "my-pod", "app": "v1"},
			expected:                 false,
		},
		{
			name:                     "different computed resource attrs",
			resolvedConfig:           userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
			resolvedDiscoveredConfig: userConfigMap{"endpoint": "localhost:8080"},
			computedAttrs:            map[string]string{"k8s.pod.name": "my-pod", "app": "v2"},
			expected:                 false,
		},
		{
			name:                     "nil vs non-empty computed attrs are not equal",
			resolvedConfig:           userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
			resolvedDiscoveredConfig: userConfigMap{"endpoint": "localhost:8080"},
			computedAttrs:            nil,
			expected:                 false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, entry.configsEqual(tt.resolvedConfig, tt.resolvedDiscoveredConfig, tt.computedAttrs))
		})
	}
}
