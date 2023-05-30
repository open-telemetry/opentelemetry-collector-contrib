// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

type metricsTestCase struct {
	fn       func()        // function to test updating metrics
	v        *view.View    // view to reference
	m        stats.Measure // expected measure of the view
	calls    int           // number of times to call fn
	expected int           // expected value of reported metric at end of calls
}

func TestRecordMetrics(t *testing.T) {
	metrics := newTestMetrics(t)
	testCases := []metricsTestCase{
		{metrics.recordFailedReconnection, metrics.views.failedReconnections, metrics.stats.failedReconnections, 3, 3},
		{metrics.recordRecoverableUnmarshallingError, metrics.views.recoverableUnmarshallingErrors, metrics.stats.recoverableUnmarshallingErrors, 3, 3},
		{metrics.recordFatalUnmarshallingError, metrics.views.fatalUnmarshallingErrors, metrics.stats.fatalUnmarshallingErrors, 3, 3},
		{metrics.recordDroppedSpanMessages, metrics.views.droppedSpanMessages, metrics.stats.droppedSpanMessages, 3, 3},
		{metrics.recordReceivedSpanMessages, metrics.views.receivedSpanMessages, metrics.stats.receivedSpanMessages, 3, 3},
		{metrics.recordReportedSpans, metrics.views.reportedSpans, metrics.stats.reportedSpans, 3, 3},
		{func() {
			metrics.recordReceiverStatus(receiverStateTerminated)
		}, metrics.views.receiverStatus, metrics.stats.receiverStatus, 3, int(receiverStateTerminated)},
		{metrics.recordNeedUpgrade, metrics.views.needUpgrade, metrics.stats.needUpgrade, 3, 1},
		{func() {
			metrics.recordFlowControlStatus(flowControlStateControlled)
		}, metrics.views.flowControlStatus, metrics.stats.flowControlStatus, 3, 1},
		{func() {
			metrics.recordFlowControlRecentRetries(5)
		}, metrics.views.flowControlRecentRetries, metrics.stats.flowControlRecentRetries, 3, 5},
		{metrics.recordFlowControlTotal, metrics.views.flowControlTotal, metrics.stats.flowControlTotal, 3, 3},
		{metrics.recordFlowControlSingleSuccess, metrics.views.flowControlSingleSuccess, metrics.stats.flowControlSingleSuccess, 3, 3},
	}
	for _, tc := range testCases {
		t.Run(tc.m.Name(), func(t *testing.T) {
			for i := 0; i < tc.calls; i++ {
				tc.fn()
			}
			validateMetric(t, tc.v, tc.expected)
		})
	}
}

func validateMetric(t *testing.T, v *view.View, expected interface{}) {
	// hack to reset stats to 0
	defer func() {
		view.Unregister(v)
		err := view.Register(v)
		assert.NoError(t, err)
	}()
	rows, err := view.RetrieveData(v.Name)
	assert.NoError(t, err)
	if expected != nil {
		require.Len(t, rows, 1)
		value := reflect.Indirect(reflect.ValueOf(rows[0].Data)).FieldByName("Value").Interface()
		assert.EqualValues(t, expected, value)
	} else {
		assert.Len(t, rows, 0)
	}
}

// TestRegisterViewsExpectingFailure validates that if an error is returned from view.Register, we panic and don't continue with initialization
func TestRegisterViewsExpectingFailure(t *testing.T) {
	statName := "solacereceiver/" + t.Name() + "/failed_reconnections"
	stat := stats.Int64(statName, "", stats.UnitDimensionless)
	err := view.Register(&view.View{
		Name:        buildReceiverCustomMetricName(statName),
		Description: "some description",
		Measure:     stat,
		Aggregation: view.Sum(),
	})
	require.NoError(t, err)
	metrics, err := newOpenCensusMetrics(t.Name())
	assert.Error(t, err)
	assert.Nil(t, metrics)
}

// newTestMetrics builds a new metrics that will cleanup when testing.T completes
func newTestMetrics(t *testing.T) *opencensusMetrics {
	m, err := newOpenCensusMetrics(t.Name())
	require.NoError(t, err)
	t.Cleanup(func() {
		unregisterMetrics(m)
	})
	return m
}

// unregisterMetrics is used to unregister the metrics for testing purposes
func unregisterMetrics(metrics *opencensusMetrics) {
	view.Unregister(
		metrics.views.failedReconnections,
		metrics.views.recoverableUnmarshallingErrors,
		metrics.views.fatalUnmarshallingErrors,
		metrics.views.droppedSpanMessages,
		metrics.views.receivedSpanMessages,
		metrics.views.reportedSpans,
		metrics.views.receiverStatus,
		metrics.views.needUpgrade,
	)
}
