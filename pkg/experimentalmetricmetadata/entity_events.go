// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package experimentalmetricmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// See entity event design document:
// https://docs.google.com/document/d/1Tg18sIck3Nakxtd3TFFcIjrmRO_0GLMdHXylVqBQmJA/edit#heading=h.pokdp8i2dmxy

const (
	semconvOtelEntityEventName    = "otel.entity.event.type"
	semconvEventEntityEventState  = "entity_state"
	semconvEventEntityEventDelete = "entity_delete"

	semconvOtelEntityID         = "otel.entity.id"
	semconvOtelEntityType       = "otel.entity.type"
	semconvOtelEntityInterval   = "otel.entity.interval"
	semconvOtelEntityAttributes = "otel.entity.attributes"

	semconvOtelEntityEventAsScope = "otel.entity.event_as_log"
)

// EntityEventsSlice is a slice of EntityEvent.
type EntityEventsSlice struct {
	orig plog.LogRecordSlice
}

// NewEntityEventsSlice creates an empty EntityEventsSlice.
func NewEntityEventsSlice() EntityEventsSlice {
	return EntityEventsSlice{orig: plog.NewLogRecordSlice()}
}

// AppendEmpty will append to the end of the slice an empty EntityEvent.
// It returns the newly added EntityEvent.
func (s EntityEventsSlice) AppendEmpty() EntityEvent {
	return EntityEvent{orig: s.orig.AppendEmpty()}
}

// Len returns the number of elements in the slice.
func (s EntityEventsSlice) Len() int {
	return s.orig.Len()
}

// EnsureCapacity is an operation that ensures the slice has at least the specified capacity.
func (s EntityEventsSlice) EnsureCapacity(newCap int) {
	s.orig.EnsureCapacity(newCap)
}

// At returns the element at the given index.
func (s EntityEventsSlice) At(i int) EntityEvent {
	return EntityEvent{orig: s.orig.At(i)}
}

// ConvertAndMoveToLogs converts entity events to log representation and moves them
// from this EntityEventsSlice into plog.Logs. This slice becomes empty after this call.
func (s EntityEventsSlice) ConvertAndMoveToLogs() plog.Logs {
	logs := plog.NewLogs()

	scopeLogs := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()

	// Set the scope marker.
	scopeLogs.Scope().Attributes().PutBool(semconvOtelEntityEventAsScope, true)

	// Move all events. Note that this remove all
	s.orig.MoveAndAppendTo(scopeLogs.LogRecords())

	return logs
}

// EntityEvent is an entity event.
type EntityEvent struct {
	orig plog.LogRecord
}

// Timestamp of the event.
func (e EntityEvent) Timestamp() pcommon.Timestamp {
	return e.orig.Timestamp()
}

// SetTimestamp sets the event timestamp.
func (e EntityEvent) SetTimestamp(timestamp pcommon.Timestamp) {
	e.orig.SetTimestamp(timestamp)
}

// ID of the entity.
func (e EntityEvent) ID() pcommon.Map {
	m, ok := e.orig.Attributes().Get(semconvOtelEntityID)
	if !ok {
		return e.orig.Attributes().PutEmptyMap(semconvOtelEntityID)
	}
	return m.Map()
}

// SetEntityState makes this an EntityStateDetails event.
func (e EntityEvent) SetEntityState() EntityStateDetails {
	e.orig.Attributes().PutStr(semconvOtelEntityEventName, semconvEventEntityEventState)
	return e.EntityStateDetails()
}

// EntityStateDetails returns the entity state details of this event.
func (e EntityEvent) EntityStateDetails() EntityStateDetails {
	return EntityStateDetails(e)
}

// SetEntityDelete makes this an EntityDeleteDetails event.
func (e EntityEvent) SetEntityDelete() EntityDeleteDetails {
	e.orig.Attributes().PutStr(semconvOtelEntityEventName, semconvEventEntityEventDelete)
	return e.EntityDeleteDetails()
}

// EntityDeleteDetails return the entity delete details of this event.
func (e EntityEvent) EntityDeleteDetails() EntityDeleteDetails {
	return EntityDeleteDetails(e)
}

// EventType is the type of the entity event.
type EventType int

const (
	// EventTypeNone indicates an invalid or unknown event type.
	EventTypeNone EventType = iota
	// EventTypeState is the "entity state" event.
	EventTypeState
	// EventTypeDelete is the "entity delete" event.
	EventTypeDelete
)

// EventType returns the type of the event.
func (e EntityEvent) EventType() EventType {
	eventType, ok := e.orig.Attributes().Get(semconvOtelEntityEventName)
	if !ok {
		return EventTypeNone
	}

	switch eventType.Str() {
	case semconvEventEntityEventState:
		return EventTypeState
	case semconvEventEntityEventDelete:
		return EventTypeDelete
	default:
		return EventTypeNone
	}
}

// EntityStateDetails represents the details of an EntityState event.
type EntityStateDetails struct {
	orig plog.LogRecord
}

// Attributes returns the attributes of the entity.
func (s EntityStateDetails) Attributes() pcommon.Map {
	m, ok := s.orig.Attributes().Get(semconvOtelEntityAttributes)
	if !ok {
		return s.orig.Attributes().PutEmptyMap(semconvOtelEntityAttributes)
	}
	return m.Map()
}

// EntityType returns the type of the entity.
func (s EntityStateDetails) EntityType() string {
	t, ok := s.orig.Attributes().Get(semconvOtelEntityType)
	if !ok {
		return ""
	}
	return t.Str()
}

// SetEntityType sets the type of the entity.
func (s EntityStateDetails) SetEntityType(t string) {
	s.orig.Attributes().PutStr(semconvOtelEntityType, t)
}

// SetInterval sets the reporting period
// i.e. how frequently the information about this entity is reported via EntityState events even if the entity does not change.
func (s EntityStateDetails) SetInterval(t time.Duration) {
	s.orig.Attributes().PutInt(semconvOtelEntityInterval, t.Milliseconds())
}

// Interval returns the reporting period
func (s EntityStateDetails) Interval() time.Duration {
	t, ok := s.orig.Attributes().Get(semconvOtelEntityInterval)
	if !ok {
		return 0
	}
	return time.Duration(t.Int()) * time.Millisecond
}

// EntityDeleteDetails represents the details of an EntityDelete event.
type EntityDeleteDetails struct {
	orig plog.LogRecord
}

// EntityType returns the type of the entity.
// TODO: Move the entity type methods to EntityEvent as they are needed for both EntityState and EntityDelete events.
func (d EntityDeleteDetails) EntityType() string {
	t, ok := d.orig.Attributes().Get(semconvOtelEntityType)
	if !ok {
		return ""
	}
	return t.Str()
}

// SetEntityType sets the type of the entity.
func (d EntityDeleteDetails) SetEntityType(t string) {
	d.orig.Attributes().PutStr(semconvOtelEntityType, t)
}
