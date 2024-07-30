// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package experimentalmetricmetadata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func Test_Entity_State(t *testing.T) {
	slice := NewEntityEventsSlice()
	event := slice.AppendEmpty()

	event.ID().PutStr("k8s.pod.uid", "123")
	state := event.SetEntityState()
	state.SetEntityType("k8s.pod")
	state.SetInterval(1 * time.Hour)
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

	assert.Equal(t, 1*time.Hour, actual.EntityStateDetails().Interval())
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

func Test_EntityEventsSlice_ConvertAndMoveToLogs(t *testing.T) {
	// Prepare an event slice.
	slice := NewEntityEventsSlice()
	event := slice.AppendEmpty()

	event.ID().PutStr("k8s.pod.uid", "123")
	state := event.SetEntityState()
	state.SetEntityType("k8s.pod")
	state.Attributes().PutStr("label1", "value1")

	event = slice.AppendEmpty()
	event.ID().PutStr("k8s.node.uid", "abc")
	event.SetEntityDelete()

	// Convert to logs.
	logs := slice.ConvertAndMoveToLogs()

	// Check that all 2 events are moved.
	assert.Equal(t, 0, slice.Len())
	assert.Equal(t, 2, logs.LogRecordCount())

	assert.Equal(t, 1, logs.ResourceLogs().Len())

	scopeLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0)

	// Check the Scope
	v, ok := scopeLogs.Scope().Attributes().Get(semconvOtelEntityEventAsScope)
	assert.True(t, ok)
	assert.Equal(t, true, v.Bool())

	records := scopeLogs.LogRecords()
	assert.Equal(t, 2, records.Len())

	// Check the first event.
	attrs := records.At(0).Attributes().AsRaw()
	assert.EqualValues(
		t, map[string]any{
			semconvOtelEntityEventName:  semconvEventEntityEventState,
			semconvOtelEntityType:       "k8s.pod",
			semconvOtelEntityID:         map[string]any{"k8s.pod.uid": "123"},
			semconvOtelEntityAttributes: map[string]any{"label1": "value1"},
		}, attrs,
	)

	// Check the second event.
	attrs = records.At(1).Attributes().AsRaw()
	assert.EqualValues(
		t, map[string]any{
			semconvOtelEntityEventName: semconvEventEntityEventDelete,
			semconvOtelEntityID:        map[string]any{"k8s.node.uid": "abc"},
		}, attrs,
	)
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

func Test_EntityEventTimestamp(t *testing.T) {
	lr := plog.NewLogRecord()
	e := EntityEvent{lr}
	ts := pcommon.NewTimestampFromTime(time.Now())
	e.SetTimestamp(ts)
	assert.EqualValues(t, ts, e.Timestamp())
	assert.EqualValues(t, ts, lr.Timestamp())
}
