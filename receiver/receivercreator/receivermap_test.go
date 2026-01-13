// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestReceiverMap(t *testing.T) {
	rm := receiverMap{}
	assert.Equal(t, 0, rm.Size())

	r1 := &nopWithEndpointReceiver{}
	r2 := &nopWithEndpointReceiver{}
	r3 := &nopWithEndpointReceiver{}

	e1 := receiverEntry{receiver: r1, id: component.MustNewID("test1")}
	e2 := receiverEntry{receiver: r2, id: component.MustNewID("test2")}
	e3 := receiverEntry{receiver: r3, id: component.MustNewID("test3")}

	rm.Put("a", e1)
	assert.Equal(t, 1, rm.Size())

	rm.Put("a", e2)
	assert.Equal(t, 2, rm.Size())

	rm.Put("b", e3)
	assert.Equal(t, 3, rm.Size())

	assert.Equal(t, []receiverEntry{e1, e2}, rm.Get("a"))
	assert.Nil(t, rm.Get("missing"))

	rm.RemoveAll("missing")
	assert.Equal(t, 3, rm.Size())

	rm.RemoveAll("b")
	assert.Equal(t, 2, rm.Size())

	rm.RemoveAll("a")
	assert.Equal(t, 0, rm.Size())

	rm.Put("a", e1)
	rm.Put("b", e2)
	assert.Equal(t, 2, rm.Size())
	assert.Equal(t, []component.Component{r1, r2}, rm.Values())
}

func TestReceiverEntryConfigsEqual(t *testing.T) {
	entry := receiverEntry{
		resolvedConfig:           userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
		resolvedDiscoveredConfig: userConfigMap{"endpoint": "localhost:8080"},
	}

	// Same configs should be equal
	assert.True(t, entry.configsEqual(
		userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
		userConfigMap{"endpoint": "localhost:8080"},
	))

	// Different resolved config
	assert.False(t, entry.configsEqual(
		userConfigMap{"key": "different"},
		userConfigMap{"endpoint": "localhost:8080"},
	))

	// Different discovered config
	assert.False(t, entry.configsEqual(
		userConfigMap{"key": "value", "nested": map[string]any{"a": 1}},
		userConfigMap{"endpoint": "localhost:9090"},
	))
}
