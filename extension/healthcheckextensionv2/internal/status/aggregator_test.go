// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/testhelpers"
)

func TestCollectorStatus(t *testing.T) {
	agg := status.NewAggregator()
	traces := testhelpers.NewPipelineMetadata("traces")

	t.Run("zero value", func(t *testing.T) {
		assert.Equal(t, component.StatusNone, agg.CollectorStatus().Status())
	})

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		assert.Equal(t, component.StatusOK, agg.CollectorStatus().Status())
	})

	agg.RecordStatus(
		traces.ExporterID,
		component.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline with recoverable error", func(t *testing.T) {
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			agg.CollectorStatus(),
		)
	})

	agg.RecordStatus(
		traces.ExporterID,
		component.NewPermanentErrorEvent(assert.AnError),
	)

	t.Run("pipeline with permanent error", func(t *testing.T) {
		assertErrorEventsMatch(t,
			component.StatusPermanentError,
			assert.AnError,
			agg.CollectorStatus(),
		)
	})
}

func TestCollectorStatusDetailed(t *testing.T) {
	agg := status.NewAggregator()
	traces := testhelpers.NewPipelineMetadata("traces")

	t.Run("zero value", func(t *testing.T) {
		dst := agg.CollectorStatusDetailed()
		assertEventsMatch(t, component.StatusNone, agg.CollectorStatus(), dst.OverallStatus)
		assert.Empty(t, dst.PipelineStatusMap)
		assert.Empty(t, dst.ComponentStatusMap)
	})

	// Seed aggregator with successful statuses for pipeline.
	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

	t.Run("pipeline statuses all successful", func(t *testing.T) {
		dst := agg.CollectorStatusDetailed()

		// CollectorStatus, OverAllStatus, and PipelineStatus match.
		assertEventsMatch(t,
			component.StatusOK,
			agg.CollectorStatus(),
			dst.OverallStatus,
			dst.PipelineStatusMap[traces.PipelineID],
		)

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectEvents(dst.ComponentStatusMap[traces.PipelineID], traces.InstanceIDs()...)...,
		)
	})

	// Record an error in the traces exporter
	agg.RecordStatus(
		traces.ExporterID,
		component.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline with exporter error", func(t *testing.T) {
		dst := agg.CollectorStatusDetailed()

		// CollectorStatus, OverAllStatus, and PipelineStatus match.
		assertErrorEventsMatch(
			t,
			component.StatusRecoverableError,
			assert.AnError,
			agg.CollectorStatus(),
			dst.OverallStatus,
			dst.PipelineStatusMap[traces.PipelineID],
		)

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectEvents(
				dst.ComponentStatusMap[traces.PipelineID], traces.ReceiverID, traces.ProcessorID,
			)...,
		)
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			dst.ComponentStatusMap[traces.PipelineID][traces.ExporterID],
		)
	})

}

func TestPipelineStatus(t *testing.T) {
	agg := status.NewAggregator()
	traces := testhelpers.NewPipelineMetadata("traces")

	t.Run("non existent pipeline", func(t *testing.T) {
		st, err := agg.PipelineStatus("doesnotexist")
		assert.Nil(t, st)
		assert.Error(t, err)
	})

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

	t.Run("pipeline exists / status successful", func(t *testing.T) {
		st, err := agg.PipelineStatus(traces.PipelineID.String())
		require.NoError(t, err)
		assertEventsMatch(t, component.StatusOK, agg.CollectorStatus(), st)
	})

	agg.RecordStatus(
		traces.ExporterID,
		component.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline exists / exporter error", func(t *testing.T) {
		st, err := agg.PipelineStatus(traces.PipelineID.String())
		require.NoError(t, err)
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			agg.CollectorStatus(),
			st,
		)
	})
}

func TestPipelineStatusDetailed(t *testing.T) {
	agg := status.NewAggregator()
	traces := testhelpers.NewPipelineMetadata("traces")

	t.Run("non existent pipeline", func(t *testing.T) {
		dst, err := agg.PipelineStatusDetailed("doesnotexist")
		assert.Nil(t, dst)
		assert.Error(t, err)
	})

	testhelpers.SeedAggregator(agg, traces.InstanceIDs(), component.StatusOK)

	t.Run("pipeline exists / status successful", func(t *testing.T) {
		dst, err := agg.PipelineStatusDetailed(traces.PipelineID.String())
		require.NoError(t, err)

		// CollectorStatus, OverAllStatus, match.
		assertEventsMatch(t,
			component.StatusOK,
			agg.CollectorStatus(),
			dst.OverallStatus,
		)

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectEvents(dst.ComponentStatusMap, traces.InstanceIDs()...)...,
		)
	})

	agg.RecordStatus(
		traces.ExporterID,
		component.NewRecoverableErrorEvent(assert.AnError),
	)

	t.Run("pipeline exists / exporter error", func(t *testing.T) {
		dst, err := agg.PipelineStatusDetailed(traces.PipelineID.String())
		require.NoError(t, err)

		// CollectorStatus, OverAllStatus, match.
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			agg.CollectorStatus(),
			dst.OverallStatus,
		)

		// Component statuses match
		assertEventsMatch(t,
			component.StatusOK,
			collectEvents(dst.ComponentStatusMap, traces.ReceiverID, traces.ProcessorID)...,
		)
		assertErrorEventsMatch(t,
			component.StatusRecoverableError,
			assert.AnError,
			dst.ComponentStatusMap[traces.ExporterID],
		)

	})
}

func TestStreaming(t *testing.T) {
	agg := status.NewAggregator()
	defer agg.Close()

	traces := testhelpers.NewPipelineMetadata("traces")
	metrics := testhelpers.NewPipelineMetadata("metrics")

	traceEvents, err := agg.Subscribe(traces.PipelineID.String())
	require.NoError(t, err)
	metricEvents, err := agg.Subscribe(metrics.PipelineID.String())
	require.NoError(t, err)
	allEvents, err := agg.Subscribe("")
	require.NoError(t, err)

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
	assertEventsRecvdMatch(t,
		component.StatusOK,
		traceEvents,
		allEvents,
	)

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

// assertEventMatches ensures one or more events share the expected status and are
// otherwise equal, ignoring timestamp.
func assertEventsMatch(
	t *testing.T,
	expectedStatus component.Status,
	events ...*component.StatusEvent,
) {
	err0 := events[0].Err()
	for _, ev := range events {
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
	events ...*component.StatusEvent,
) {
	assert.True(t, component.StatusIsError(expectedStatus))
	for _, ev := range events {
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, expectedErr, ev.Err())
	}
}

// collectEvents returns a slice of events collected from the componentMap using
// the provided instanceIDs
func collectEvents(
	componentMap map[*component.InstanceID]*component.StatusEvent,
	instanceIDs ...*component.InstanceID,
) (result []*component.StatusEvent) {
	for _, id := range instanceIDs {
		result = append(result, componentMap[id])
	}
	return
}

func assertEventsRecvdMatch(t *testing.T,
	expectedStatus component.Status,
	chans ...<-chan *component.StatusEvent,
) {
	var err0 error
	for i, evCh := range chans {
		ev := <-evCh
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
	chans ...<-chan *component.StatusEvent,
) {
	assert.True(t, component.StatusIsError(expectedStatus))
	for _, evCh := range chans {
		ev := <-evCh
		assert.Equal(t, expectedStatus, ev.Status())
		assert.Equal(t, expectedErr, ev.Err())
	}
}
