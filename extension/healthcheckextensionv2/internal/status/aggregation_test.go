// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestAggregationFuncs(t *testing.T) {
	aggRecoverable := newAggregationFunc(PriorityRecoverable)
	aggPermanent := newAggregationFunc(PriorityPermanent)

	type statusExpectation struct {
		priorityPermanent   component.Status
		priorityRecoverable component.Status
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
						Event: component.NewStatusEvent(component.StatusFatalError),
					},
					"c2": {
						Event: component.NewStatusEvent(component.StatusStarting),
					},
					"c3": {
						Event: component.NewStatusEvent(component.StatusOK),
					},
					"c4": {
						Event: component.NewStatusEvent(component.StatusRecoverableError),
					},
					"c5": {
						Event: component.NewStatusEvent(component.StatusPermanentError),
					},
					"c6": {
						Event: component.NewStatusEvent(component.StatusStopping),
					},
					"c7": {
						Event: component.NewStatusEvent(component.StatusStopped),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   component.StatusFatalError,
				priorityRecoverable: component.StatusFatalError,
			},
		},
		{
			name: "Lifecycle: Starting takes precedence over non-fatal errors",
			aggregateStatus: &AggregateStatus{
				ComponentStatusMap: map[string]*AggregateStatus{
					"c1": {
						Event: component.NewStatusEvent(component.StatusStarting),
					},
					"c2": {
						Event: component.NewStatusEvent(component.StatusRecoverableError),
					},
					"c3": {
						Event: component.NewStatusEvent(component.StatusPermanentError),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   component.StatusStarting,
				priorityRecoverable: component.StatusStarting,
			},
		},
		{
			name: "Lifecycle: Stopping takes precedence over non-fatal errors",
			aggregateStatus: &AggregateStatus{
				ComponentStatusMap: map[string]*AggregateStatus{
					"c1": {
						Event: component.NewStatusEvent(component.StatusStopping),
					},
					"c2": {
						Event: component.NewStatusEvent(component.StatusRecoverableError),
					},
					"c3": {
						Event: component.NewStatusEvent(component.StatusPermanentError),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   component.StatusStopping,
				priorityRecoverable: component.StatusStopping,
			},
		},
		{
			name: "Prioritized error takes priority over OK",
			aggregateStatus: &AggregateStatus{
				ComponentStatusMap: map[string]*AggregateStatus{
					"c1": {
						Event: component.NewStatusEvent(component.StatusOK),
					},
					"c2": {
						Event: component.NewStatusEvent(component.StatusRecoverableError),
					},
					"c3": {
						Event: component.NewStatusEvent(component.StatusPermanentError),
					},
				},
			},
			expectedStatus: &statusExpectation{
				priorityPermanent:   component.StatusPermanentError,
				priorityRecoverable: component.StatusRecoverableError,
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
