// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
)

type serializableStatus struct {
	StartTimestamp *time.Time `json:"start_time,omitempty"`
	*SerializableEvent
	ComponentStatuses map[string]*serializableStatus `json:"components,omitempty"`
}

// SerializableEvent is exported for json.Unmarshal
type SerializableEvent struct {
	Healthy      bool      `json:"healthy"`
	StatusString string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	Timestamp    time.Time `json:"status_time"`
}

var stringToStatusMap = map[string]component.Status{
	"StatusNone":             component.StatusNone,
	"StatusStarting":         component.StatusStarting,
	"StatusOK":               component.StatusOK,
	"StatusRecoverableError": component.StatusRecoverableError,
	"StatusPermanentError":   component.StatusPermanentError,
	"StatusFatalError":       component.StatusFatalError,
	"StatusStopping":         component.StatusStopping,
	"StatusStopped":          component.StatusStopped,
}

func (ev *SerializableEvent) Status() component.Status {
	if st, ok := stringToStatusMap[ev.StatusString]; ok {
		return st
	}
	return component.StatusNone
}

func toSerializableEvent(
	ev *component.StatusEvent,
	now time.Time,
	recoveryDuration time.Duration,
) *SerializableEvent {
	se := &SerializableEvent{
		Healthy:      isHealthy(ev, now, recoveryDuration),
		StatusString: ev.Status().String(),
		Timestamp:    ev.Timestamp(),
	}
	if ev.Err() != nil {
		se.Error = ev.Err().Error()
	}
	return se
}

var extsKey = "extensions"

func toSerializableStatus(
	ev *component.StatusEvent,
	startTimestamp time.Time,
	recoveryDuration time.Duration,
) *serializableStatus {
	return &serializableStatus{
		StartTimestamp:    &startTimestamp,
		SerializableEvent: toSerializableEvent(ev, time.Now(), recoveryDuration),
	}
}

func toCollectorSerializableStatus(
	details *status.CollectorStatusDetails,
	startTimestamp time.Time,
	recoveryDuration time.Duration,
) *serializableStatus {
	now := time.Now()
	s := &serializableStatus{
		StartTimestamp:    &startTimestamp,
		SerializableEvent: toSerializableEvent(details.OverallStatus, now, recoveryDuration),
		ComponentStatuses: make(map[string]*serializableStatus),
	}

	for compID, ev := range details.PipelineStatusMap {
		key := compID.String()
		if key != extsKey {
			key = "pipeline:" + key
		}
		cs := &serializableStatus{
			SerializableEvent: toSerializableEvent(ev, now, recoveryDuration),
			ComponentStatuses: make(map[string]*serializableStatus),
		}
		s.ComponentStatuses[key] = cs
		for instance, ev := range details.ComponentStatusMap[compID] {
			key := fmt.Sprintf("%s:%s", kindToString(instance.Kind), instance.ID)
			cs.ComponentStatuses[key] = &serializableStatus{
				SerializableEvent: toSerializableEvent(ev, now, recoveryDuration),
			}
		}
	}

	return s
}

func toPipelineSerializableStatus(
	details *status.PipelineStatusDetails,
	startTimestamp time.Time,
	recoveryDuration time.Duration,
) *serializableStatus {
	now := time.Now()
	s := &serializableStatus{
		StartTimestamp:    &startTimestamp,
		SerializableEvent: toSerializableEvent(details.OverallStatus, now, recoveryDuration),
		ComponentStatuses: make(map[string]*serializableStatus),
	}

	for instance, ev := range details.ComponentStatusMap {
		key := fmt.Sprintf("%s:%s", kindToString(instance.Kind), instance.ID)
		s.ComponentStatuses[key] = &serializableStatus{
			SerializableEvent: toSerializableEvent(ev, now, recoveryDuration),
		}
	}

	return s
}

func isHealthy(ev *component.StatusEvent, now time.Time, recoveryDuration time.Duration) bool {
	if ev.Status() == component.StatusRecoverableError &&
		now.Compare(ev.Timestamp().Add(recoveryDuration)) == -1 {
		return true
	}

	return !component.StatusIsError(ev.Status())
}

// TODO: implemnent Stringer on Kind in core
func kindToString(k component.Kind) string {
	switch k {
	case component.KindReceiver:
		return "receiver"
	case component.KindProcessor:
		return "processor"
	case component.KindExporter:
		return "exporter"
	case component.KindExtension:
		return "extension"
	case component.KindConnector:
		return "connector"
	}
	return ""
}
