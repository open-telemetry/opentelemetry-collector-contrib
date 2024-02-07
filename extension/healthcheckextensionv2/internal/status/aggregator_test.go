// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/testhelpers"
)

// func TestAggregateStatus(t *testing.T) {
// 	agg := status.NewAggregator()
// 	traces := testhelpers.NewPipelineMetadata("traces")

// 	t.Run("zero value", func(t *testing.T) {
// 		st, ok := agg.AggregateStatus(status.ScopeAll, status.ExcludeSubtrees)
// 		require.True(t, ok)
// 		assert.Equal(t, component.StatusNone, st.Status())
// 	})

// 	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

// 	t.Run("pipeline statuses all successful", func(t *testing.T) {
// 		st, ok := agg.AggregateStatus(status.ScopeAll, status.ExcludeSubtrees)
// 		require.True(t, ok)
// 		assert.Equal(t, component.StatusOK, st.Status())
// 	})

// 	agg.RecordStatus(
// 		traces.ExporterID,
// 		component.NewRecoverableErrorEvent(assert.AnError),
// 	)

// 	t.Run("pipeline with recoverable error", func(t *testing.T) {
// 		st, ok := agg.AggregateStatus(status.ScopeAll, status.ExcludeSubtrees)
// 		require.True(t, ok)
// 		assertErrorEventsMatch(t,
// 			component.StatusRecoverableError,
// 			assert.AnError,
// 			st,
// 		)
// 	})

// 	agg.RecordStatus(
// 		traces.ExporterID,
// 		component.NewPermanentErrorEvent(assert.AnError),
// 	)

// 	t.Run("pipeline with permanent error", func(t *testing.T) {
// 		st, ok := agg.AggregateStatus(status.ScopeAll, status.ExcludeSubtrees)
// 		require.True(t, ok)
// 		assertErrorEventsMatch(t,
// 			component.StatusPermanentError,
// 			assert.AnError,
// 			st,
// 		)
// 	})
// }

func TestAggregateStatusDetailed(t *testing.T) {
	agg := status.NewAggregator()
	traces := testhelpers.NewPipelineMetadata("traces")
	tracesKey := toPipelineKey(traces.PipelineID)

	t.Run("zero value", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll)
		require.True(t, ok)
		assertEventsMatch(t, component.StatusNone, st)
		assert.Empty(t, st.ComponentStatusMap)
	})

	// Seed aggregator with successful statuses for pipeline.
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll)
		require.True(t, ok)

		// The top-level status and pipeline status match.
		assertEventsMatch(t, component.StatusOK, st, st.ComponentStatusMap[tracesKey])

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectStatuses(st.ComponentStatusMap[tracesKey], traces.InstanceIDs()...)...,
		)
	})

	// Record an error in the traces exporter
	agg.RecordStatus(
		traces.ExporterID,
		component.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline with exporter error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.ScopeAll)
		require.True(t, ok)
		// The top-level status and pipeline status match.
		assertErrorEventsMatch(
			t,
			component.StatusRecoverableError,
			assert.AnError,
			st,
			st.ComponentStatusMap[tracesKey],
		)

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectStatuses(
				st.ComponentStatusMap[tracesKey], traces.ReceiverID, traces.ProcessorID,
			)...,
		)
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			st.ComponentStatusMap[tracesKey].ComponentStatusMap[toComponentKey(traces.ExporterID)],
		)
	})

}

// func TestPipelineAggregateStatus(t *testing.T) {
// 	agg := status.NewAggregator()
// 	traces := testhelpers.NewPipelineMetadata("traces")

// 	t.Run("non existent pipeline", func(t *testing.T) {
// 		st, ok := agg.AggregateStatus("doesnotexist", status.ExcludeSubtrees)
// 		require.Nil(t, st)
// 		require.False(t, ok)
// 	})

// 	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

// 	t.Run("pipeline exists / status successful", func(t *testing.T) {
// 		st, ok := agg.AggregateStatus(
// 			status.Scope(traces.PipelineID.String()),
// 			status.ExcludeSubtrees,
// 		)
// 		require.True(t, ok)
// 		assertEventsMatch(t, component.StatusOK, st)
// 	})

// 	agg.RecordStatus(
// 		traces.ExporterID,
// 		component.NewRecoverableErrorEvent(assert.AnError),
// 	)

// 	t.Run("pipeline exists / exporter error", func(t *testing.T) {
// 		st, ok := agg.AggregateStatus(
// 			status.Scope(traces.PipelineID.String()),
// 			status.ExcludeSubtrees,
// 		)
// 		require.True(t, ok)
// 		assertErrorEventsMatch(t, component.StatusRecoverableError, assert.AnError, st)
// 	})
// }

func TestPipelineAggregateStatusDetailed(t *testing.T) {
	agg := status.NewAggregator()
	traces := testhelpers.NewPipelineMetadata("traces")

	t.Run("non existent pipeline", func(t *testing.T) {
		st, ok := agg.AggregateStatus("doesnotexist")
		require.Nil(t, st)
		require.False(t, ok)
	})

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

	t.Run("pipeline exists / status successful", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.Scope(traces.PipelineID.String()))
		require.True(t, ok)

		// Top-level status matches
		assertEventsMatch(t, component.StatusOK, st)

		// Component statuses match
		assertEventsMatch(t, component.StatusOK, collectStatuses(st, traces.InstanceIDs()...)...)
	})

	agg.RecordStatus(traces.ExporterID, component.NewRecoverableErrorEvent(assert.AnError))

	t.Run("pipeline exists / exporter error", func(t *testing.T) {
		st, ok := agg.AggregateStatus(status.Scope(traces.PipelineID.String()))
		require.True(t, ok)

		// Top-level status matches
		assertErrorEventsMatch(t, component.StatusRecoverableError, assert.AnError, st)

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectStatuses(st, traces.ReceiverID, traces.ProcessorID)...,
		)
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			st.ComponentStatusMap[toComponentKey(traces.ExporterID)],
		)
	})
}

func TestStreaming(t *testing.T) {
	agg := status.NewAggregator()
	defer agg.Close()

	traces := testhelpers.NewPipelineMetadata("traces")
	metrics := testhelpers.NewPipelineMetadata("metrics")

	traceEvents := agg.Subscribe(status.Scope(traces.PipelineID.String()))
	metricEvents := agg.Subscribe(status.Scope(metrics.PipelineID.String()))
	allEvents := agg.Subscribe(status.ScopeAll)

	assert.Nil(t, <-traceEvents)
	assert.Nil(t, <-metricEvents)
	assert.NotNil(t, <-allEvents)

	// Start pipelines
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusStarting)
	assertEventsRecvdMatch(t, component.StatusStarting, traceEvents, allEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), component.StatusStarting)
	assertEventsRecvdMatch(t, component.StatusStarting, metricEvents, allEvents)

	// Successful start
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)
	assertEventsRecvdMatch(t, component.StatusOK, traceEvents)
	// All is still in StatusStarting until the metrics pipeline reports OK
	assertEventsRecvdMatch(t, component.StatusStarting, allEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), component.StatusOK)
	assertEventsRecvdMatch(t, component.StatusOK, metricEvents, allEvents)

	// Traces Pipeline RecoverableError
	agg.RecordStatus(traces.ExporterID, component.NewRecoverableErrorEvent(assert.AnError))
	assertErrorEventsRecvdMatch(t,
		component.StatusRecoverableError,
		assert.AnError,
		traceEvents,
		allEvents,
	)

	// Traces Pipeline Recover
	agg.RecordStatus(traces.ExporterID, component.NewStatusEvent(component.StatusOK))
	assertEventsRecvdMatch(t, component.StatusOK, traceEvents, allEvents)

	// Stopping
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusStopping)
	assertEventsRecvdMatch(t, component.StatusStopping, traceEvents, allEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), component.StatusStopping)
	assertEventsRecvdMatch(t, component.StatusStopping, metricEvents, allEvents)

	// Stopped
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusStopped)
	// All is not stopped until the metrics pipeline is stopped
	assertEventsRecvdMatch(t, component.StatusStopped, traceEvents)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), component.StatusStopped)
	assertEventsRecvdMatch(t, component.StatusStopped, metricEvents, allEvents)
}

func TestStreamingDetailed(t *testing.T) {
	agg := status.NewAggregator()
	defer agg.Close()

	traces := testhelpers.NewPipelineMetadata("traces")
	tracesKey := toPipelineKey(traces.PipelineID)

	allEvents := agg.Subscribe(status.ScopeAll)

	t.Run("zero value", func(t *testing.T) {
		st := <-allEvents
		assertEventsMatch(t, component.StatusNone, st)
		assert.Empty(t, st.ComponentStatusMap)
	})

	// Seed aggregator with successful statuses for pipeline.
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		st := <-allEvents
		// The top-level status matches the pipeline status.
		assertEventsMatch(t, component.StatusOK, st, st.ComponentStatusMap[tracesKey])

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectStatuses(st.ComponentStatusMap[tracesKey], traces.InstanceIDs()...)...,
		)
	})

	// Record an error in the traces exporter
	agg.RecordStatus(traces.ExporterID, component.NewRecoverableErrorEvent(assert.AnError))

	t.Run("pipeline with exporter error", func(t *testing.T) {
		st := <-allEvents

		// The top-level status and pipeline status match.
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			st,
			st.ComponentStatusMap[tracesKey],
		)

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectStatuses(
				st.ComponentStatusMap[tracesKey], traces.ReceiverID, traces.ProcessorID,
			)...,
		)
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			st.ComponentStatusMap[tracesKey].ComponentStatusMap[toComponentKey(traces.ExporterID)],
		)
	})
}

// assertEventMatches ensures one or more events share the expected status and are
// otherwise equal, ignoring timestamp.
func assertEventsMatch(
	t *testing.T,
	expectedStatus component.Status,
	statuses ...*status.AggregateStatus,
) {
	err0 := statuses[0].StatusEvent.Err()
	for _, st := range statuses {
		ev := st.StatusEvent
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, err0, ev.Err())
	}
}

// assertErrorEventMatches compares one or more status events with the expected
// status and expected error.
func assertErrorEventsMatch(
	t *testing.T,
	expectedStatus component.Status,
	expectedErr error,
	statuses ...*status.AggregateStatus,
) {
	assert.True(t, component.StatusIsError(expectedStatus))
	for _, st := range statuses {
		ev := st.StatusEvent
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, expectedErr, ev.Err())
	}
}

func collectStatuses(
	aggregateStatus *status.AggregateStatus,
	instanceIDs ...*component.InstanceID,
) (result []*status.AggregateStatus) {
	for _, id := range instanceIDs {
		key := toComponentKey(id)
		result = append(result, aggregateStatus.ComponentStatusMap[key])
	}
	return
}

func assertEventsRecvdMatch(t *testing.T,
	expectedStatus component.Status,
	chans ...<-chan *status.AggregateStatus,
) {
	var err0 error
	for i, stCh := range chans {
		st := <-stCh
		ev := st.StatusEvent
		if i == 0 {
			err0 = ev.Err()
		}
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, err0, ev.Err())
	}
}

func assertErrorEventsRecvdMatch(t *testing.T,
	expectedStatus component.Status,
	expectedErr error,
	chans ...<-chan *status.AggregateStatus,
) {
	assert.True(t, component.StatusIsError(expectedStatus))
	for _, stCh := range chans {
		st := <-stCh
		ev := st.StatusEvent
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, expectedErr, ev.Err())
	}
}

func toComponentKey(id *component.InstanceID) string {
	return fmt.Sprintf("%s:%s", strings.ToLower(id.Kind.String()), id.ID)
}

func toPipelineKey(id component.ID) string {
	return fmt.Sprintf("pipeline:%s", id.String())
}

func TestSearch(t *testing.T) {
	agg := status.NewAggregator()
	defer agg.Close()

	traces := testhelpers.NewPipelineMetadata("traces")
	metrics := testhelpers.NewPipelineMetadata("metrics")

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)
	testhelpers.SeedAggregator(agg, metrics.InstanceIDs(), component.StatusStarting)

	agg.RecordStatus(
		traces.ExporterID,
		component.NewPermanentErrorEvent(errors.New("it's perm")),
	)
	agg.RecordStatus(
		traces.ReceiverID,
		component.NewRecoverableErrorEvent(errors.New("first recoverable")),
	)
	time.Sleep(20 * time.Millisecond)
	agg.RecordStatus(
		traces.ProcessorID,
		component.NewRecoverableErrorEvent(errors.New("second recoverable")),
	)

	stAll, _ := agg.AggregateStatus(status.ScopeAll)
	assert.False(t, stAll.Ready())

	reAll, found := stAll.ActiveRecoverable()
	assert.NotNil(t, reAll)
	assert.True(t, found)
	assert.Equal(t, reAll.Err().Error(), "first recoverable")

	stTraces, _ := agg.AggregateStatus(status.Scope("traces"))
	assert.True(t, stTraces.Ready())

	reTr, found := stTraces.ActiveRecoverable()
	assert.NotNil(t, reTr)
	assert.True(t, found)
	assert.Equal(t, reTr.Err().Error(), "first recoverable")

	stMetrics, _ := agg.AggregateStatus(status.Scope("metrics"))
	assert.False(t, stMetrics.Ready())

	reMt, found := stMetrics.ActiveRecoverable()
	assert.Nil(t, reMt)
	assert.False(t, found)
}
