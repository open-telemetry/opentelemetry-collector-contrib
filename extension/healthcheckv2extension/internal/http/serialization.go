// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"

import (
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/status"
)

type healthyFunc func(status.Event) bool

func (f healthyFunc) isHealthy(ev status.Event) bool {
	if f != nil {
		return f(ev)
	}
	return true
}

type serializationOptions struct {
	includeStartTime bool
	startTimestamp   *time.Time
	healthyFunc      healthyFunc
}

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

func toSerializableEvent(ev status.Event, isHealthy bool) *SerializableEvent {
	se := &SerializableEvent{
		Healthy:      isHealthy,
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
	opts *serializationOptions,
) *serializableStatus {
	s := &serializableStatus{
		SerializableEvent: toSerializableEvent(
			st.Event,
			opts.healthyFunc.isHealthy(st.Event),
		),
		ComponentStatuses: make(map[string]*serializableStatus),
	}

	if opts.includeStartTime {
		s.StartTimestamp = opts.startTimestamp
		opts.includeStartTime = false
	}

	for k, cs := range st.ComponentStatusMap {
		s.ComponentStatuses[k] = toSerializableStatus(cs, opts)
	}

	return s
}
