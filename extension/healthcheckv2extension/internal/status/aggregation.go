// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/status"

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

// statusEvent contains a status and timestamp, and can contain an error. Note:
// this is duplicated from core because we need to be able to "rewrite" the
// timestamps of some events during aggregation.
type statusEvent struct {
	status    component.Status
	err       error
	timestamp time.Time
}

var _ Event = (*statusEvent)(nil)

// Status returns the Status (enum) associated with the StatusEvent
func (ev *statusEvent) Status() component.Status {
	return ev.status
}

// Err returns the error associated with the StatusEvent.
func (ev *statusEvent) Err() error {
	return ev.err
}

// Timestamp returns the timestamp associated with the StatusEvent
func (ev *statusEvent) Timestamp() time.Time {
	return ev.timestamp
}

type ErrorPriority int

const (
	PriorityPermanent ErrorPriority = iota
	PriorityRecoverable
)

type aggregationFunc func(*AggregateStatus) Event

// The purpose of aggregation is to ensure that the most relevant status bubbles
// upwards in the aggregate status. This aggregation func prioritizes lifecycle
// events (including FatalError) over PermanentError and RecoverableError
// events. The priority argument determines the priority of PermanentError
// events vs RecoverableError events. Lifecycle events will have the timestamp
// of the most recent event and error events will have the timestamp of the
// first occurrence. We use the first occurrence of an error event as this marks
// the beginning of a possible failure. This is important for two reasons:
// recovery duration and causality. We expect a RecoverableError to recover
// before the RecoveryDuration elapses. We need to use the earliest timestamp so
// that a later RecoverableError does not shadow an earlier event in the
// aggregate status. Additionally, this makes sense in the case where a
// RecoverableError in one component cascades to other components; the earliest
// error event is likely to be correlated with the cause. For non-error stauses
// we use the latest event as it represents the last time a successful status was
// reported.
func newAggregationFunc(priority ErrorPriority) aggregationFunc {
	statusFunc := func(st *AggregateStatus) component.Status {
		seen := make(map[component.Status]struct{})
		for _, cs := range st.ComponentStatusMap {
			seen[cs.Status()] = struct{}{}
		}

		// All statuses are the same. Note, this will handle StatusOK and StatusStopped as these two
		// cases require all components be in the same state.
		if len(seen) == 1 {
			for st := range seen {
				return st
			}
		}

		// Handle mixed status cases
		if _, isFatal := seen[component.StatusFatalError]; isFatal {
			return component.StatusFatalError
		}

		if _, isStarting := seen[component.StatusStarting]; isStarting {
			return component.StatusStarting
		}

		if _, isStopping := seen[component.StatusStopping]; isStopping {
			return component.StatusStopping
		}

		if _, isStopped := seen[component.StatusStopped]; isStopped {
			return component.StatusStopping
		}

		if priority == PriorityPermanent {
			if _, isPermanent := seen[component.StatusPermanentError]; isPermanent {
				return component.StatusPermanentError
			}
			if _, isRecoverable := seen[component.StatusRecoverableError]; isRecoverable {
				return component.StatusRecoverableError
			}
		} else {
			if _, isRecoverable := seen[component.StatusRecoverableError]; isRecoverable {
				return component.StatusRecoverableError
			}
			if _, isPermanent := seen[component.StatusPermanentError]; isPermanent {
				return component.StatusPermanentError
			}
		}

		return component.StatusNone
	}

	return func(st *AggregateStatus) Event {
		var ev, lastEvent, matchingEvent Event
		status := statusFunc(st)
		isError := component.StatusIsError(status)

		for _, cs := range st.ComponentStatusMap {
			ev = cs.Event
			if lastEvent == nil || lastEvent.Timestamp().Before(ev.Timestamp()) {
				lastEvent = ev
			}
			if status == ev.Status() {
				switch {
				case matchingEvent == nil:
					matchingEvent = ev
				case isError:
					// Use earliest to mark beginning of a failure
					if ev.Timestamp().Before(matchingEvent.Timestamp()) {
						matchingEvent = ev
					}
				case ev.Timestamp().After(matchingEvent.Timestamp()):
					// Use most recent for last successful status
					matchingEvent = ev
				}
			}
		}

		// the error status will be the first matching event
		if isError {
			return matchingEvent
		}

		// the aggregate status matches an existing event
		if lastEvent.Status() == status {
			return lastEvent
		}

		// the aggregate status requires a synthetic event
		return &statusEvent{
			status:    status,
			timestamp: lastEvent.Timestamp(),
		}
	}
}
