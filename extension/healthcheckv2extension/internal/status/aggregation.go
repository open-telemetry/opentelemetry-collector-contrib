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
// first occurrence.
func newAggregationFunc(priority ErrorPriority) aggregationFunc {
	permanentPriorityFunc := func(seen map[component.Status]struct{}) component.Status {
		if _, isPermanent := seen[component.StatusPermanentError]; isPermanent {
			return component.StatusPermanentError
		}
		if _, isRecoverable := seen[component.StatusRecoverableError]; isRecoverable {
			return component.StatusRecoverableError
		}
		return component.StatusNone
	}

	recoverablePriorityFunc := func(seen map[component.Status]struct{}) component.Status {
		if _, isRecoverable := seen[component.StatusRecoverableError]; isRecoverable {
			return component.StatusRecoverableError
		}
		if _, isPermanent := seen[component.StatusPermanentError]; isPermanent {
			return component.StatusPermanentError
		}
		return component.StatusNone
	}

	errPriorityFunc := permanentPriorityFunc
	if priority == PriorityRecoverable {
		errPriorityFunc = recoverablePriorityFunc
	}

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

		return errPriorityFunc(seen)
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
					if ev.Timestamp().Before(matchingEvent.Timestamp()) {
						matchingEvent = ev
					}
				case ev.Timestamp().After(matchingEvent.Timestamp()):
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
