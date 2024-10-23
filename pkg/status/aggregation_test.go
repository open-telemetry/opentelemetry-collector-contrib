// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func TestAggregationFuncs(t *testing.T) {
	aggRecoverable := newAggregationFunc(PriorityRecoverable)
	aggPermanent := newAggregationFunc(PriorityPermanent)

	type statusExpectation struct {
		priorityPermanent   componentstatus.Status
		priorityRecoverable componentstatus.Status
	}

	for _, tc := range []struct {
		name            string
		aggregateStatus *AggregateStatus
		expectedStatus  *statusExpectation
	}{
		{
			name: "FatalError takes precedence over all",
			aggregateStatus: &AggregateStatus{
				ComponentStatusMap: map[string]*AggregateStatus{
					"c1": {
						Event: componentstatus.NewEvent(componentstatus.StatusFatalError),
					},
					"c2": {
						Event: componentstatus.NewEvent(componentstatus.StatusStarting),
					},
					"c3": {
						Event: componentstatus.NewEvent(componentstatus.StatusOK),
					},
					"c4": {
						Event: componentstatus.NewEvent(componentstatus.StatusRecoverableError),
					},
					"c5": {
						Event: componentstatus.NewEvent(componentstatus.StatusPermanentError),
					},
					"c6": {
						Event: componentstatus.NewEvent(componentstatus.StatusStopping),
					},
					"c7": {
						Event: componentstatus.NewEvent(componentstatus.StatusStopped),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   componentstatus.StatusFatalError,
				priorityRecoverable: componentstatus.StatusFatalError,
			},
		},
		{
			name: "Lifecycle: Starting takes precedence over non-fatal errors",
			aggregateStatus: &AggregateStatus{
				ComponentStatusMap: map[string]*AggregateStatus{
					"c1": {
						Event: componentstatus.NewEvent(componentstatus.StatusStarting),
					},
					"c2": {
						Event: componentstatus.NewEvent(componentstatus.StatusRecoverableError),
					},
					"c3": {
						Event: componentstatus.NewEvent(componentstatus.StatusPermanentError),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   componentstatus.StatusStarting,
				priorityRecoverable: componentstatus.StatusStarting,
			},
		},
		{
			name: "Lifecycle: Stopping takes precedence over non-fatal errors",
			aggregateStatus: &AggregateStatus{
				ComponentStatusMap: map[string]*AggregateStatus{
					"c1": {
						Event: componentstatus.NewEvent(componentstatus.StatusStopping),
					},
					"c2": {
						Event: componentstatus.NewEvent(componentstatus.StatusRecoverableError),
					},
					"c3": {
						Event: componentstatus.NewEvent(componentstatus.StatusPermanentError),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   componentstatus.StatusStopping,
				priorityRecoverable: componentstatus.StatusStopping,
			},
		},
		{
			name: "Prioritized error takes priority over OK",
			aggregateStatus: &AggregateStatus{
				ComponentStatusMap: map[string]*AggregateStatus{
					"c1": {
						Event: componentstatus.NewEvent(componentstatus.StatusOK),
					},
					"c2": {
						Event: componentstatus.NewEvent(componentstatus.StatusRecoverableError),
					},
					"c3": {
						Event: componentstatus.NewEvent(componentstatus.StatusPermanentError),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   componentstatus.StatusPermanentError,
				priorityRecoverable: componentstatus.StatusRecoverableError,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedStatus.priorityPermanent,
				aggPermanent(tc.aggregateStatus).Status())
			assert.Equal(t, tc.expectedStatus.priorityRecoverable,
				aggRecoverable(tc.aggregateStatus).Status())
		})
	}
}

func TestEventTemporalOrder(t *testing.T) {
	// Note: ErrorPriority does not affect temporal ordering
	aggFunc := newAggregationFunc(PriorityPermanent)
	st := &AggregateStatus{
		ComponentStatusMap: map[string]*AggregateStatus{
			"c1": {
				Event: componentstatus.NewEvent(componentstatus.StatusOK),
			},
		},
	}
	assert.Equal(t, st.ComponentStatusMap["c1"].Event, aggFunc(st))

	// Record first error
	st.ComponentStatusMap["c2"] = &AggregateStatus{
		Event: componentstatus.NewRecoverableErrorEvent(assert.AnError),
	}

	// Returns first error
	assert.Equal(t, st.ComponentStatusMap["c2"].Event, aggFunc(st))

	// Record second error
	st.ComponentStatusMap["c3"] = &AggregateStatus{
		Event: componentstatus.NewRecoverableErrorEvent(assert.AnError),
	}

	// Still returns first error
	assert.Equal(t, st.ComponentStatusMap["c2"].Event, aggFunc(st))

	// Replace first error with later error
	st.ComponentStatusMap["c2"] = &AggregateStatus{
		Event: componentstatus.NewRecoverableErrorEvent(assert.AnError),
	}

	// Returns second error now
	assert.Equal(t, st.ComponentStatusMap["c3"].Event, aggFunc(st))

	// Clear errors
	st.ComponentStatusMap["c2"] = &AggregateStatus{
		Event: componentstatus.NewEvent(componentstatus.StatusOK),
	}
	st.ComponentStatusMap["c3"] = &AggregateStatus{
		Event: componentstatus.NewEvent(componentstatus.StatusOK),
	}

	// Returns latest event
	assert.Equal(t, st.ComponentStatusMap["c3"].Event, aggFunc(st))
}
