// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pentity"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	semconvOtelEntityEventName    = "otel.entity.event.type"
	semconvEventEntityEventState  = "entity_state"
	semconvEventEntityEventDelete = "entity_delete"

	semconvOtelEntityID         = "otel.entity.id"
	semconvOtelEntityType       = "otel.entity.type"
	semconvOtelEntityAttributes = "otel.entity.attributes"

	semconvOtelEntityEventAsScope = "otel.entity.event_as_log"
)

// ConvertAndMoveToLogs converts entity events to log representation and moves them
// from this EntityEventsSlice into plog.Logs. This slice becomes empty after this call.
func ConvertAndMoveToLogs(s pentity.EntityEventSlice) plog.Logs {
	logs := plog.NewLogs()

	scopeLogs := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()

	// Set the scope marker.
	scopeLogs.Scope().Attributes().PutBool(semconvOtelEntityEventAsScope, true)

	// Move all events. Note that this remove all
	for i := 0; i < s.Len(); i++ {
		event := s.At(i)
		log := scopeLogs.LogRecords().AppendEmpty()
		convertEntityEvent2Log(event, log)
	}

	return logs
}

func convertEntityEvent2Log(event pentity.EntityEvent, log plog.LogRecord) {
	log.SetTimestamp(event.Timestamp())

	log.Attributes().PutStr(semconvOtelEntityType, event.EntityType())

	event.Id().CopyTo(log.Attributes().PutEmptyMap(semconvOtelEntityID))

	switch event.Type() {
	case pentity.EventTypeEntityState:
		log.Attributes().PutStr(semconvOtelEntityEventName, semconvEventEntityEventState)
		event.EntityState().Attributes().CopyTo(log.Attributes().PutEmptyMap(semconvOtelEntityAttributes))
	case pentity.EventTypeEntityDelete:
		log.Attributes().PutStr(semconvOtelEntityEventName, semconvEventEntityEventDelete)
	}
}
