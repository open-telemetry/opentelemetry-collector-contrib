// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import (
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

func toSerializableStatus(
	st *status.AggregateStatus,
	startTimestamp time.Time,
	recoveryDuration time.Duration,
) *serializableStatus {
	now := time.Now()
	s := &serializableStatus{
		StartTimestamp:    &startTimestamp,
		SerializableEvent: toSerializableEvent(st.StatusEvent, now, recoveryDuration),
		ComponentStatuses: make(map[string]*serializableStatus),
	}

	for k, cs := range st.ComponentStatusMap {
		s.ComponentStatuses[k] = toComponentSerializableStatus(cs, now, recoveryDuration)
	}

	return s
}

func toComponentSerializableStatus(
	st *status.AggregateStatus,
	now time.Time,
	recoveryDuration time.Duration,
) *serializableStatus {
	s := &serializableStatus{
		SerializableEvent: toSerializableEvent(st.StatusEvent, now, recoveryDuration),
		ComponentStatuses: make(map[string]*serializableStatus),
	}

	for k, cs := range st.ComponentStatusMap {
		s.ComponentStatuses[k] = toComponentSerializableStatus(cs, now, recoveryDuration)
	}

	return s
}

func isHealthy(ev *component.StatusEvent, now time.Time, recoveryDuration time.Duration) bool {
	if ev.Status() == component.StatusRecoverableError &&
		now.Before(ev.Timestamp().Add(recoveryDuration)) {
		return true
	}

	return !component.StatusIsError(ev.Status())
}
