// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status/testhelpers"
)

func TestAggregateStatus(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)

	t.Run("zero value", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		assert.Equal(t, componentstatus.StatusNone, st.Status())
	})

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		assert.Equal(t, componentstatus.StatusOK, st.Status())
	})

	agg.RecordStatus(
		traces.ExporterID,
		componentstatus.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline with recoverable error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		assertErrorEventsMatch(t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st,
		)
	})

	agg.RecordStatus(
		traces.ExporterID,
		componentstatus.NewPermanentErrorEvent(assert.AnError),
	)

	t.Run("pipeline with permanent error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		assertErrorEventsMatch(t,
			componentstatus.StatusPermanentError,
			assert.AnError,
			st,
		)
	})
}

func TestAggregateStatusVerbose(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)
	tracesKey := toPipelineKey(traces.PipelineID)

	t.Run("zero value", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Verbose)
		require.True(t, ok)
		assertEventsMatch(t, componentstatus.StatusNone, st)
		assert.Empty(t, st.ComponentStatusMap)
	})

	// Seed aggregator with successful statuses for pipeline.
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Verbose)
		require.True(t, ok)

		// The top-level status and pipeline status match.
		assertEventsMatch(t, componentstatus.StatusOK, st, st.ComponentStatusMap[tracesKey])

		// Component statuses match
		assertEventsMatch(t,
			componentstatus.StatusOK,
			collectStatuses(st.ComponentStatusMap[tracesKey], traces.InstanceIDs()...)...,
		)
	})

	// Record an error in the traces exporter
	agg.RecordStatus(
		traces.ExporterID,
		componentstatus.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline with exporter error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Verbose)
		require.True(t, ok)
		// The top-level status and pipeline status match.
		assertErrorEventsMatch(
			t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st,
			st.ComponentStatusMap[tracesKey],
		)

		// Component statuses match
		assertEventsMatch(t,
			componentstatus.StatusOK,
			collectStatuses(
				st.ComponentStatusMap[tracesKey], traces.ReceiverID, traces.ProcessorID,
			)...,
		)
		assertErrorEventsMatch(t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st.ComponentStatusMap[tracesKey].ComponentStatusMap[toComponentKey(traces.ExporterID)],
		)
	})
}

func TestAggregateStatusPriorityRecoverable(t *testing.T) {
	agg := status.NewAggregator(status.PriorityRecoverable)
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		assert.Equal(t, componentstatus.StatusOK, st.Status())
	})

	agg.RecordStatus(
		traces.ProcessorID,
		componentstatus.NewPermanentErrorEvent(assert.AnError),
	)

	t.Run("pipeline with permanent error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		assertErrorEventsMatch(t,
			componentstatus.StatusPermanentError,
			assert.AnError,
			st,
		)
	})

	agg.RecordStatus(
		traces.ExporterID,
		componentstatus.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline with recoverable error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		assertErrorEventsMatch(t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st,
		)
	})
}

func TestPipelineAggregateStatus(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)

	t.Run("non existent pipeline", func(t *testing.T) {
		st, ok := agg.AggregateStatus("doesnotexist", status.Concise)
		require.Nil(t, st)
		require.False(t, ok)
	})

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)

	t.Run("pipeline exists / status successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(
			status.Scope(traces.PipelineID.String()),
			status.Concise,
		)
		require.True(t, ok)
		assertEventsMatch(t, componentstatus.StatusOK, st)
	})

	agg.RecordStatus(
		traces.ExporterID,
		componentstatus.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline exists / exporter error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(
			status.Scope(traces.PipelineID.String()),
			status.Concise,
		)
		require.True(t, ok)
		assertErrorEventsMatch(t, componentstatus.StatusRecoverableError, assert.AnError, st)
	})
}

func TestPipelineAggregateStatusVerbose(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)

	t.Run("non existent pipeline", func(t *testing.T) {
		st, ok := agg.AggregateStatus("doesnotexist", status.Verbose)
		require.Nil(t, st)
		require.False(t, ok)
	})

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)

	t.Run("pipeline exists / status successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.Scope(traces.PipelineID.String()), status.Verbose)
		require.True(t, ok)

		// Top-level status matches
		assertEventsMatch(t, componentstatus.StatusOK, st)

		// Component statuses match
		assertEventsMatch(t, componentstatus.StatusOK, collectStatuses(st, traces.InstanceIDs()...)...)
	})

	agg.RecordStatus(traces.ExporterID, componentstatus.NewRecoverableErrorEvent(assert.AnError))

	t.Run("pipeline exists / exporter error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.Scope(traces.PipelineID.String()), status.Verbose)
		require.True(t, ok)

		// Top-level status matches
		assertErrorEventsMatch(t, componentstatus.StatusRecoverableError, assert.AnError, st)

		// Component statuses match
		assertEventsMatch(t,
			componentstatus.StatusOK,
			collectStatuses(st, traces.ReceiverID, traces.ProcessorID)...,
		)
		assertErrorEventsMatch(t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st.ComponentStatusMap[toComponentKey(traces.ExporterID)],
		)
	})
}

func TestAggregateStatusExtensions(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)

	extInstanceID1 := componentstatus.NewInstanceID(component.MustNewID("ext1"), component.KindExtension)
	extInstanceID2 := componentstatus.NewInstanceID(component.MustNewID("ext2"), component.KindExtension)
	extInstanceIDs := []*componentstatus.InstanceID{extInstanceID1, extInstanceID2}

	testhelpers.SeedAggregator(agg, extInstanceIDs, componentstatus.StatusOK)

	t.Run("extension statuses all successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeExtensions, status.Concise)
		require.True(t, ok)
		assert.Equal(t, componentstatus.StatusOK, st.Status())
	})

	agg.RecordStatus(
		extInstanceID1,
		componentstatus.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("extension with recoverable error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeExtensions, status.Concise)
		require.True(t, ok)
		assertErrorEventsMatch(t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st,
		)
	})

	agg.RecordStatus(
		extInstanceID1,
		componentstatus.NewEvent(componentstatus.StatusOK),
	)

	t.Run("extensions recovered", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeExtensions, status.Concise)
		require.True(t, ok)
		assertEventsMatch(t,
			componentstatus.StatusOK,
			st,
		)
	})
}

func TestStreaming(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)
	defer agg.Close()

	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)
	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)

	traceEvents, traceUnsub := agg.Subscribe(status.Scope(traces.PipelineID.String()), status.Concise)
	defer traceUnsub()

	metricEvents, metricUnsub := agg.Subscribe(status.Scope(metrics.PipelineID.String()), status.Concise)
	defer metricUnsub()

	allEvents, allUnsub := agg.Subscribe(status.ScopeAll, status.Concise)
	defer allUnsub()

	assert.Nil(t, <-traceEvents)
	assert.Nil(t, <-metricEvents)
	assert.NotNil(t, <-allEvents)

	// Start pipelines
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusStarting)
	assertEventsRecvdMatch(t, componentstatus.StatusStarting, traceEvents, allEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), componentstatus.StatusStarting)
	assertEventsRecvdMatch(t, componentstatus.StatusStarting, metricEvents, allEvents)

	// Successful start
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)
	assertEventsRecvdMatch(t, componentstatus.StatusOK, traceEvents)
	// All is still in StatusStarting until the metrics pipeline reports OK
	assertEventsRecvdMatch(t, componentstatus.StatusStarting, allEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), componentstatus.StatusOK)
	assertEventsRecvdMatch(t, componentstatus.StatusOK, metricEvents, allEvents)

	// Traces Pipeline RecoverableError
	agg.RecordStatus(traces.ExporterID, componentstatus.NewRecoverableErrorEvent(assert.AnError))
	assertErrorEventsRecvdMatch(t,
		componentstatus.StatusRecoverableError,
		assert.AnError,
		traceEvents,
		allEvents,
	)

	// Traces Pipeline Recover
	agg.RecordStatus(traces.ExporterID, componentstatus.NewEvent(componentstatus.StatusOK))
	assertEventsRecvdMatch(t, componentstatus.StatusOK, traceEvents, allEvents)

	// Stopping
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusStopping)
	assertEventsRecvdMatch(t, componentstatus.StatusStopping, traceEvents, allEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), componentstatus.StatusStopping)
	assertEventsRecvdMatch(t, componentstatus.StatusStopping, metricEvents, allEvents)

	// Stopped
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusStopped)
	// All is not stopped until the metrics pipeline is stopped
	assertEventsRecvdMatch(t, componentstatus.StatusStopped, traceEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), componentstatus.StatusStopped)
	assertEventsRecvdMatch(t, componentstatus.StatusStopped, metricEvents, allEvents)
}

func TestStreamingVerbose(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)
	defer agg.Close()

	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)
	tracesKey := toPipelineKey(traces.PipelineID)

	allEvents, unsub := agg.Subscribe(status.ScopeAll, status.Verbose)
	defer unsub()

	t.Run("zero value", func(t *testing.T) {
		st := <-allEvents
		assertEventsMatch(t, componentstatus.StatusNone, st)
		assert.Empty(t, st.ComponentStatusMap)
	})

	// Seed aggregator with successful statuses for pipeline.
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		st := <-allEvents
		// The top-level status matches the pipeline status.
		assertEventsMatch(t, componentstatus.StatusOK, st, st.ComponentStatusMap[tracesKey])

		// Component statuses match
		assertEventsMatch(t,
			componentstatus.StatusOK,
			collectStatuses(st.ComponentStatusMap[tracesKey], traces.InstanceIDs()...)...,
		)
	})

	// Record an error in the traces exporter
	agg.RecordStatus(traces.ExporterID, componentstatus.NewRecoverableErrorEvent(assert.AnError))

	t.Run("pipeline with exporter error", func(t *testing.T) {
		st := <-allEvents

		// The top-level status and pipeline status match.
		assertErrorEventsMatch(t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st,
			st.ComponentStatusMap[tracesKey],
		)

		// Component statuses match
		assertEventsMatch(t,
			componentstatus.StatusOK,
			collectStatuses(
				st.ComponentStatusMap[tracesKey], traces.ReceiverID, traces.ProcessorID,
			)...,
		)
		assertErrorEventsMatch(t,
			componentstatus.StatusRecoverableError,
			assert.AnError,
			st.ComponentStatusMap[tracesKey].ComponentStatusMap[toComponentKey(traces.ExporterID)],
		)
	})
}

func TestUnsubscribe(t *testing.T) {
	agg := status.NewAggregator(status.PriorityPermanent)
	defer agg.Close()

	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)

	traceEvents, traceUnsub := agg.Subscribe(status.Scope(traces.PipelineID.String()), status.Concise)
	allEvents, allUnsub := agg.Subscribe(status.ScopeAll, status.Concise)

	assert.Nil(t, <-traceEvents)
	assert.NotNil(t, <-allEvents)

	// Start pipeline
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusStarting)
	assertEventsRecvdMatch(t, componentstatus.StatusStarting, traceEvents, allEvents)

	traceUnsub()

	// Pipeline OK
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusOK)
	assertNoEventsRecvd(t, traceEvents)
	assertEventsRecvdMatch(t, componentstatus.StatusOK, allEvents)

	allUnsub()

	// Stop pipeline
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), componentstatus.StatusStopping)

	assertNoEventsRecvd(t, traceEvents, allEvents)
}

// assertEventMatches ensures one or more events share the expected status and are
// otherwise equal, ignoring timestamp.
func assertEventsMatch(
	t *testing.T,
	expectedStatus componentstatus.Status,
	statuses ...*status.AggregateStatus,
) {
	err0 := statuses[0].Err()
	for _, st := range statuses {
		ev := st.Event
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, err0, ev.Err())
	}
}

// assertErrorEventMatches compares one or more status events with the expected
// status and expected error.
func assertErrorEventsMatch(
	t *testing.T,
	expectedStatus componentstatus.Status,
	expectedErr error,
	statuses ...*status.AggregateStatus,
) {
	assert.True(t, componentstatus.StatusIsError(expectedStatus))
	for _, st := range statuses {
		ev := st.Event
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, expectedErr, ev.Err())
	}
}

func collectStatuses(
	aggregateStatus *status.AggregateStatus,
	instanceIDs ...*componentstatus.InstanceID,
) (result []*status.AggregateStatus) {
	for _, id := range instanceIDs {
		key := toComponentKey(id)
		result = append(result, aggregateStatus.ComponentStatusMap[key])
	}
	return
}

func assertEventsRecvdMatch(t *testing.T,
	expectedStatus componentstatus.Status,
	chans ...<-chan *status.AggregateStatus,
) {
	var err0 error
	for i, stCh := range chans {
		st := <-stCh
		ev := st.Event
		if i == 0 {
			err0 = ev.Err()
		}
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, err0, ev.Err())
	}
}

func assertErrorEventsRecvdMatch(t *testing.T,
	expectedStatus componentstatus.Status,
	expectedErr error,
	chans ...<-chan *status.AggregateStatus,
) {
	assert.True(t, componentstatus.StatusIsError(expectedStatus))
	for _, stCh := range chans {
		st := <-stCh
		ev := st.Event
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, expectedErr, ev.Err())
	}
}

func toComponentKey(id *componentstatus.InstanceID) string {
	return fmt.Sprintf("%s:%s", strings.ToLower(id.Kind().String()), id.ComponentID())
}

func toPipelineKey(id pipeline.ID) string {
	return fmt.Sprintf("pipeline:%s", id.String())
}

func assertNoEventsRecvd(t *testing.T, chans ...<-chan *status.AggregateStatus) {
	for _, stCh := range chans {
		select {
		case <-stCh:
			require.Fail(t, "Found unexpected event")
		default:
		}
	}
}
