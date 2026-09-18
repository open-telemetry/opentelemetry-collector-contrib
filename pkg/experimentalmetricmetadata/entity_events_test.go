// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package experimentalmetricmetadata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata/internal/metadata"
)

func Test_Entity_State(t *testing.T) {
	setEntityEventsSpecificationGate(t, true)

	slice := NewEntityEventsSlice()
	event := slice.AppendEmpty()

	event.ID().PutStr("k8s.pod.uid", "123")
	state := event.SetEntityState()
	state.SetEntityType("k8s.pod")
	state.SetInterval(1 * time.Hour)
	state.Attributes().PutStr("label1", "value1")
	state.Description().PutStr("label2", "value2")

	actual := slice.At(0)

	assert.Equal(t, EventTypeState, actual.EventType())

	v, ok := actual.ID().Get("k8s.pod.uid")
	assert.True(t, ok)
	assert.Equal(t, "123", v.Str())

	v, ok = actual.EntityStateDetails().Attributes().Get("label1")
	assert.True(t, ok)
	assert.Equal(t, "value1", v.Str())

	v, ok = actual.EntityStateDetails().Description().Get("label2")
	assert.True(t, ok)
	assert.Equal(t, "value2", v.Str())

	assert.Equal(t, "k8s.pod", actual.EntityStateDetails().EntityType())

	assert.Equal(t, 1*time.Hour, actual.EntityStateDetails().Interval())
}

func Test_Entity_Delete(t *testing.T) {
	setEntityEventsSpecificationGate(t, true)

	slice := NewEntityEventsSlice()

	event := slice.AppendEmpty()
	event.ID().PutStr("k8s.node.uid", "abc")
	deleteEvent := event.SetEntityDelete()
	deleteEvent.SetEntityType("k8s.node")
	deleteEvent.SetDeletionReason("terminated")

	actual := slice.At(0)

	assert.Equal(t, EventTypeDelete, actual.EventType())
	assert.Equal(t, "k8s.node", event.EntityDeleteDetails().EntityType())
	assert.Equal(t, "terminated", event.EntityDeleteDetails().DeletionReason())
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
	setEntityEventsSpecificationGate(t, true)

	// Prepare an event slice.
	slice := NewEntityEventsSlice()
	event := slice.AppendEmpty()

	event.ID().PutStr("k8s.pod.uid", "123")
	state := event.SetEntityState()
	state.SetEntityType("k8s.pod")
	state.SetInterval(90*time.Second + 500*time.Millisecond)
	state.Attributes().PutStr("label1", "value1")

	event = slice.AppendEmpty()
	event.ID().PutStr("k8s.node.uid", "abc")
	deleteEvent := event.SetEntityDelete()
	deleteEvent.SetEntityType("k8s.node")
	deleteEvent.SetDeletionReason("terminated")

	// Convert to logs.
	logs := slice.ConvertAndMoveToLogs()

	// Check that all 2 events are moved.
	assert.Equal(t, 0, slice.Len())
	assert.Equal(t, 2, logs.LogRecordCount())

	assert.Equal(t, 1, logs.ResourceLogs().Len())

	scopeLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0)

	// Check the Scope
	v, ok := scopeLogs.Scope().Attributes().Get(SemconvOtelEntityEventAsScope)
	assert.True(t, ok)
	assert.True(t, v.Bool())

	records := scopeLogs.LogRecords()
	assert.Equal(t, 2, records.Len())

	// Check the first event.
	assert.Equal(t, semconvEventEntityEventState, records.At(0).EventName())
	attrs := records.At(0).Attributes().AsRaw()
	assert.Equal(
		t, map[string]any{
			semconvOtelEntityType:       "k8s.pod",
			semconvOtelEntityID:         map[string]any{"k8s.pod.uid": "123"},
			semconvOtelEntityInterval:   int64(90),
			semconvOtelEntityAttributes: map[string]any{"label1": "value1"},
		}, attrs,
	)

	// Check the second event.
	assert.Equal(t, semconvEventEntityEventDelete, records.At(1).EventName())
	attrs = records.At(1).Attributes().AsRaw()
	assert.Equal(
		t, map[string]any{
			semconvOtelEntityType:           "k8s.node",
			semconvOtelEntityID:             map[string]any{"k8s.node.uid": "abc"},
			semconvOtelEntityDeletionReason: "terminated",
		}, attrs,
	)
}

func Test_EntityEventsSlice_ConvertAndMoveToLogs_LegacyFeatureGate(t *testing.T) {
	setEntityEventsSpecificationGate(t, false)

	slice := NewEntityEventsSlice()
	event := slice.AppendEmpty()

	event.ID().PutStr("k8s.pod.uid", "123")
	state := event.SetEntityState()
	state.SetEntityType("k8s.pod")
	state.SetInterval(1 * time.Hour)
	state.Attributes().PutStr("label1", "value1")

	event = slice.AppendEmpty()
	event.ID().PutStr("k8s.node.uid", "abc")
	deleteEvent := event.SetEntityDelete()
	deleteEvent.SetEntityType("k8s.node")

	logs := slice.ConvertAndMoveToLogs()
	records := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 2, records.Len())

	assert.Empty(t, records.At(0).EventName())
	attrs := records.At(0).Attributes().AsRaw()
	assert.Equal(
		t, map[string]any{
			legacySemconvOtelEntityEventName:  legacySemconvEventEntityEventState,
			legacySemconvOtelEntityType:       "k8s.pod",
			legacySemconvOtelEntityID:         map[string]any{"k8s.pod.uid": "123"},
			legacySemconvOtelEntityInterval:   (1 * time.Hour).Milliseconds(),
			legacySemconvOtelEntityAttributes: map[string]any{"label1": "value1"},
		}, attrs,
	)

	actual := NewEntityEventsSliceFromLogs(records).At(0)
	assert.Equal(t, EventTypeState, actual.EventType())
	assert.Equal(t, "k8s.pod", actual.EntityStateDetails().EntityType())
	assert.Equal(t, 1*time.Hour, actual.EntityStateDetails().Interval())

	assert.Empty(t, records.At(1).EventName())
	attrs = records.At(1).Attributes().AsRaw()
	assert.Equal(
		t, map[string]any{
			legacySemconvOtelEntityEventName: legacySemconvEventEntityEventDelete,
			legacySemconvOtelEntityType:      "k8s.node",
			legacySemconvOtelEntityID:        map[string]any{"k8s.node.uid": "abc"},
		}, attrs,
	)

	actual = NewEntityEventsSliceFromLogs(records).At(1)
	assert.Equal(t, EventTypeDelete, actual.EventType())
	assert.Equal(t, "k8s.node", actual.EntityDeleteDetails().EntityType())
}

func Test_EntityEventType(t *testing.T) {
	setEntityEventsSpecificationGate(t, true)

	lr := plog.NewLogRecord()
	e := EntityEvent{lr}
	assert.Equal(t, EventTypeNone, e.EventType())

	lr.SetEventName("invalidtype")
	assert.Equal(t, EventTypeNone, e.EventType())

	lr.SetEventName("")
	lr.Attributes().PutStr(legacySemconvOtelEntityEventName, "invalidtype")
	assert.Equal(t, EventTypeNone, e.EventType())
}

func Test_EntityEventType_LegacyFeatureGate(t *testing.T) {
	setEntityEventsSpecificationGate(t, false)

	lr := plog.NewLogRecord()
	e := EntityEvent{lr}
	assert.Equal(t, EventTypeNone, e.EventType())

	lr.SetEventName(semconvEventEntityEventState)
	assert.Equal(t, EventTypeNone, e.EventType())

	lr.SetEventName("")
	lr.Attributes().PutStr(legacySemconvOtelEntityEventName, "invalidtype")
	assert.Equal(t, EventTypeNone, e.EventType())
}

func Test_EntityEventsSpecificationGate_DefaultDisabled(t *testing.T) {
	assert.False(t, metadata.PkgExperimentalmetricmetadataUseEntityEventsSpecificationFeatureGate.IsEnabled())
}

func Test_EntityStateDetails_EntityType_StrictFeatureGate(t *testing.T) {
	t.Run("specification", func(t *testing.T) {
		setEntityEventsSpecificationGate(t, true)

		lr := plog.NewLogRecord()
		lr.Attributes().PutStr(legacySemconvOtelEntityType, "legacy")
		state := EntityStateDetails{orig: lr}
		assert.Empty(t, state.EntityType())

		lr.Attributes().PutStr(semconvOtelEntityType, "specification")
		assert.Equal(t, "specification", state.EntityType())
	})

	t.Run("legacy", func(t *testing.T) {
		setEntityEventsSpecificationGate(t, false)

		lr := plog.NewLogRecord()
		lr.Attributes().PutStr(semconvOtelEntityType, "specification")
		state := EntityStateDetails{orig: lr}
		assert.Empty(t, state.EntityType())

		lr.Attributes().PutStr(legacySemconvOtelEntityType, "legacy")
		assert.Equal(t, "legacy", state.EntityType())
	})
}

func Test_EntityTypeEmpty(t *testing.T) {
	lr := plog.NewLogRecord()
	e := EntityStateDetails{lr}
	assert.Empty(t, e.EntityType())
}

func Test_EntityEventTimestamp(t *testing.T) {
	lr := plog.NewLogRecord()
	e := EntityEvent{lr}
	ts := pcommon.NewTimestampFromTime(time.Now())
	e.SetTimestamp(ts)
	assert.Equal(t, ts, e.Timestamp())
	assert.Equal(t, ts, lr.Timestamp())
}

func setEntityEventsSpecificationGate(t *testing.T, enabled bool) {
	t.Helper()

	gate := metadata.PkgExperimentalmetricmetadataUseEntityEventsSpecificationFeatureGate
	previous := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), previous))
	})
}
