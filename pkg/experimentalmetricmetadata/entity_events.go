// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package experimentalmetricmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata/internal/metadata"
)

// See entity event specification:
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/entities/entity-events.md

const (
	semconvEventEntityEventState  = "entity.state"
	semconvEventEntityEventDelete = "entity.delete"

	semconvOtelEntityID             = "entity.id"
	semconvOtelEntityType           = "entity.type"
	semconvOtelEntityInterval       = "entity.report.interval"
	semconvOtelEntityAttributes     = "entity.description"
	semconvOtelEntityRelationships  = "entity.relationships"
	semconvOtelRelationshipType     = "relationship.type"
	semconvOtelEntityDeletionReason = "entity.delete.reason"

	legacySemconvOtelEntityEventName    = "otel.entity.event.type"
	legacySemconvEventEntityEventState  = "entity_state"
	legacySemconvEventEntityEventDelete = "entity_delete"
	legacySemconvOtelEntityID           = "otel.entity.id"
	legacySemconvOtelEntityType         = "otel.entity.type"
	legacySemconvOtelEntityInterval     = "otel.entity.interval"
	legacySemconvOtelEntityAttributes   = "otel.entity.attributes"

	SemconvOtelEntityEventAsScope = "otel.entity.event_as_log"
)

// EntityEventsSlice is a slice of EntityEvent.
type EntityEventsSlice struct {
	orig plog.LogRecordSlice
}

// NewEntityEventsSlice creates an empty EntityEventsSlice.
func NewEntityEventsSlice() EntityEventsSlice {
	return EntityEventsSlice{orig: plog.NewLogRecordSlice()}
}

// NewEntityEventsSliceFromLogs creates an EntityEventsSlice from a plog.LogRecordSlice.
func NewEntityEventsSliceFromLogs(logs plog.LogRecordSlice) EntityEventsSlice {
	return EntityEventsSlice{orig: logs}
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
	scopeLogs.Scope().Attributes().PutBool(SemconvOtelEntityEventAsScope, true)

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
	m, ok := firstMap(e.orig.Attributes(), semconvOtelEntityID, legacySemconvOtelEntityID)
	if !ok {
		return e.orig.Attributes().PutEmptyMap(entityIDAttributeName())
	}
	return m
}

// SetEntityState makes this an EntityStateDetails event.
func (e EntityEvent) SetEntityState() EntityStateDetails {
	e.setEventName(semconvEventEntityEventState, legacySemconvEventEntityEventState)
	return e.EntityStateDetails()
}

// EntityStateDetails returns the entity state details of this event.
func (e EntityEvent) EntityStateDetails() EntityStateDetails {
	return EntityStateDetails(e)
}

// SetEntityDelete makes this an EntityDeleteDetails event.
func (e EntityEvent) SetEntityDelete() EntityDeleteDetails {
	e.setEventName(semconvEventEntityEventDelete, legacySemconvEventEntityEventDelete)
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
	switch e.orig.EventName() {
	case semconvEventEntityEventState:
		return EventTypeState
	case semconvEventEntityEventDelete:
		return EventTypeDelete
	}

	eventType, ok := e.orig.Attributes().Get(legacySemconvOtelEntityEventName)
	if !ok {
		return EventTypeNone
	}

	switch eventType.Str() {
	case legacySemconvEventEntityEventState:
		return EventTypeState
	case legacySemconvEventEntityEventDelete:
		return EventTypeDelete
	default:
		return EventTypeNone
	}
}

func (e EntityEvent) setEventName(eventName, legacyEventName string) {
	if useEntityEventsSpecification() {
		e.orig.SetEventName(eventName)
		e.orig.Attributes().Remove(legacySemconvOtelEntityEventName)
		return
	}

	e.orig.SetEventName("")
	e.orig.Attributes().PutStr(legacySemconvOtelEntityEventName, legacyEventName)
}

// EntityStateDetails represents the details of an EntityState event.
type EntityStateDetails struct {
	orig plog.LogRecord
}

// Attributes returns the attributes of the entity.
func (s EntityStateDetails) Attributes() pcommon.Map {
	m, ok := firstMap(s.orig.Attributes(), semconvOtelEntityAttributes, legacySemconvOtelEntityAttributes)
	if !ok {
		return s.orig.Attributes().PutEmptyMap(entityAttributesAttributeName())
	}
	return m
}

// Description returns the descriptive attributes of the entity.
func (s EntityStateDetails) Description() pcommon.Map {
	return s.Attributes()
}

// Relationships returns the relationships of the entity.
func (s EntityStateDetails) Relationships() EntityRelationshipsSlice {
	value, ok := s.orig.Attributes().Get(semconvOtelEntityRelationships)
	if !ok {
		return EntityRelationshipsSlice{orig: s.orig.Attributes().PutEmptySlice(semconvOtelEntityRelationships)}
	}
	return EntityRelationshipsSlice{orig: value.Slice()}
}

// EntityType returns the type of the entity.
func (s EntityStateDetails) EntityType() string {
	t, ok := firstString(s.orig.Attributes(), semconvOtelEntityType, legacySemconvOtelEntityType)
	if !ok {
		return ""
	}
	return t
}

// SetEntityType sets the type of the entity.
func (s EntityStateDetails) SetEntityType(t string) {
	s.orig.Attributes().PutStr(entityTypeAttributeName(), t)
}

// SetInterval sets the reporting period
// i.e. how frequently the information about this entity is reported via EntityState events even if the entity does not change.
func (s EntityStateDetails) SetInterval(t time.Duration) {
	if useEntityEventsSpecification() {
		s.orig.Attributes().PutInt(semconvOtelEntityInterval, int64(t/time.Second))
		return
	}
	s.orig.Attributes().PutInt(legacySemconvOtelEntityInterval, t.Milliseconds())
}

// Interval returns the reporting period
func (s EntityStateDetails) Interval() time.Duration {
	if t, ok := s.orig.Attributes().Get(semconvOtelEntityInterval); ok {
		return time.Duration(t.Int()) * time.Second
	}

	t, ok := s.orig.Attributes().Get(legacySemconvOtelEntityInterval)
	if !ok {
		return 0
	}

	return time.Duration(t.Int()) * time.Millisecond
}

// EntityRelationshipsSlice is a slice of entity relationships.
type EntityRelationshipsSlice struct {
	orig pcommon.Slice
}

// AppendEmpty will append to the end of the slice an empty EntityRelationship.
// It returns the newly added EntityRelationship.
func (s EntityRelationshipsSlice) AppendEmpty() EntityRelationship {
	return EntityRelationship{orig: s.orig.AppendEmpty().SetEmptyMap()}
}

// Len returns the number of elements in the slice.
func (s EntityRelationshipsSlice) Len() int {
	return s.orig.Len()
}

// EnsureCapacity is an operation that ensures the slice has at least the specified capacity.
func (s EntityRelationshipsSlice) EnsureCapacity(newCap int) {
	s.orig.EnsureCapacity(newCap)
}

// At returns the element at the given index.
func (s EntityRelationshipsSlice) At(i int) EntityRelationship {
	return EntityRelationship{orig: s.orig.At(i).Map()}
}

// EntityRelationship represents a relationship to another entity.
type EntityRelationship struct {
	orig pcommon.Map
}

// RelationshipType returns the type of relationship.
func (r EntityRelationship) RelationshipType() string {
	t, ok := firstString(r.orig, semconvOtelRelationshipType)
	if !ok {
		return ""
	}
	return t
}

// SetRelationshipType sets the type of relationship.
func (r EntityRelationship) SetRelationshipType(t string) {
	r.orig.PutStr(semconvOtelRelationshipType, t)
}

// EntityType returns the type of the related entity.
func (r EntityRelationship) EntityType() string {
	t, ok := firstString(r.orig, semconvOtelEntityType, legacySemconvOtelEntityType)
	if !ok {
		return ""
	}
	return t
}

// SetEntityType sets the type of the related entity.
func (r EntityRelationship) SetEntityType(t string) {
	r.orig.PutStr(semconvOtelEntityType, t)
}

// EntityID returns the identifying attributes of the related entity.
func (r EntityRelationship) EntityID() pcommon.Map {
	m, ok := firstMap(r.orig, semconvOtelEntityID, legacySemconvOtelEntityID)
	if !ok {
		return r.orig.PutEmptyMap(semconvOtelEntityID)
	}
	return m
}

// EntityDeleteDetails represents the details of an EntityDelete event.
type EntityDeleteDetails struct {
	orig plog.LogRecord
}

// EntityType returns the type of the entity.
// TODO: Move the entity type methods to EntityEvent as they are needed for both EntityState and EntityDelete events.
func (d EntityDeleteDetails) EntityType() string {
	t, ok := firstString(d.orig.Attributes(), semconvOtelEntityType, legacySemconvOtelEntityType)
	if !ok {
		return ""
	}
	return t
}

// SetEntityType sets the type of the entity.
func (d EntityDeleteDetails) SetEntityType(t string) {
	d.orig.Attributes().PutStr(entityTypeAttributeName(), t)
}

// DeletionReason returns the reason for entity deletion.
func (d EntityDeleteDetails) DeletionReason() string {
	t, ok := firstString(d.orig.Attributes(), semconvOtelEntityDeletionReason)
	if !ok {
		return ""
	}
	return t
}

// SetDeletionReason sets the reason for entity deletion.
func (d EntityDeleteDetails) SetDeletionReason(reason string) {
	d.orig.Attributes().PutStr(semconvOtelEntityDeletionReason, reason)
}

func useEntityEventsSpecification() bool {
	return metadata.PkgExperimentalmetricmetadataUseEntityEventsSpecificationFeatureGate.IsEnabled()
}

func entityIDAttributeName() string {
	if useEntityEventsSpecification() {
		return semconvOtelEntityID
	}
	return legacySemconvOtelEntityID
}

func entityTypeAttributeName() string {
	if useEntityEventsSpecification() {
		return semconvOtelEntityType
	}
	return legacySemconvOtelEntityType
}

func entityAttributesAttributeName() string {
	if useEntityEventsSpecification() {
		return semconvOtelEntityAttributes
	}
	return legacySemconvOtelEntityAttributes
}

func firstMap(attrs pcommon.Map, names ...string) (pcommon.Map, bool) {
	for _, name := range names {
		if v, ok := attrs.Get(name); ok {
			return v.Map(), true
		}
	}
	return pcommon.Map{}, false
}

func firstString(attrs pcommon.Map, names ...string) (string, bool) {
	for _, name := range names {
		if v, ok := attrs.Get(name); ok {
			return v.Str(), true
		}
	}
	return "", false
}
