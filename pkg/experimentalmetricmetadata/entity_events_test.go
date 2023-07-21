// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package experimentalmetricmetadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func Test_Entity_State(t *testing.T) {
	slice := NewEntityEventsSlice()
	event := slice.AppendEmpty()

	event.ID().PutStr("k8s.pod.uid", "123")
	state := event.SetEntityState()
	state.SetEntityType("k8s.pod")
	state.Attributes().PutStr("label1", "value1")

	actual := slice.At(0)

	assert.Equal(t, EventTypeState, actual.EventType())

	v, ok := actual.ID().Get("k8s.pod.uid")
	assert.True(t, ok)
	assert.Equal(t, "123", v.Str())

	v, ok = actual.EntityStateDetails().Attributes().Get("label1")
	assert.True(t, ok)
	assert.Equal(t, "value1", v.Str())

	assert.Equal(t, "k8s.pod", actual.EntityStateDetails().EntityType())
}

func Test_Entity_Delete(t *testing.T) {
	slice := NewEntityEventsSlice()

	event := slice.AppendEmpty()
	event.ID().PutStr("k8s.node.uid", "abc")
	event.SetEntityDelete()

	actual := slice.At(0)

	assert.Equal(t, EventTypeDelete, actual.EventType())
	v, ok := actual.ID().Get("k8s.node.uid")
	assert.True(t, ok)
	assert.Equal(t, "abc", v.Str())
}

func Test_EntityEventsSlice(t *testing.T) {
	slice := NewEntityEventsSlice()
	slice.AppendEmpty()
	assert.Equal(t, 1, slice.Len())

	slice.EnsureCapacity(10)
	assert.Equal(t, 1, slice.Len())
}

func Test_EntityEventType(t *testing.T) {
	lr := plog.NewLogRecord()
	e := EntityEvent{lr}
	assert.Equal(t, EventTypeNone, e.EventType())

	lr.Attributes().PutStr(semconvOtelEntityEventName, "invalidtype")
	assert.Equal(t, EventTypeNone, e.EventType())
}

func Test_EntityTypeEmpty(t *testing.T) {
	lr := plog.NewLogRecord()
	e := EntityStateDetails{lr}
	assert.Equal(t, "", e.EntityType())
}
