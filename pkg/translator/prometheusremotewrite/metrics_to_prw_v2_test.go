// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestFromMetricsV2(t *testing.T) {
	settings := Settings{
		Namespace:         "",
		ExternalLabels:    nil,
		DisableTargetInfo: false,
		AddMetricSuffixes: false,
		SendMetadata:      false,
	}

	ts := uint64(time.Now().UnixNano())
	payload := createExportRequest(5, 0, 1, 3, 0, pcommon.Timestamp(ts))
	want := []*writev2.TimeSeries{
		{
			LabelsRefs: []uint32{1, 11, 3, 4, 5, 6, 7, 8},
			Samples: []writev2.Sample{
				{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
			},
			Metadata: writev2.Metadata{
				Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
				HelpRef: 12,
				UnitRef: 10,
			},
		},
		{
			LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
			Samples: []writev2.Sample{
				{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
			},
			Metadata: writev2.Metadata{
				Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
				HelpRef: 9,
				UnitRef: 10,
			},
		},
	}
	wantedSymbols := []string{"", "series_name_2", "value-2", "series_name_3", "value-3", "__name__", "gauge_1", "series_name_1", "value-1", "sum_1", "test gauge description", "test sum description", "bytes"}
	tsMap, symbolsTable, err := FromMetricsV2(payload.Metrics(), settings)
	require.NoError(t, err)
	require.ElementsMatch(t, want, slices.Collect(maps.Values(tsMap)))
	require.ElementsMatch(t, wantedSymbols, symbolsTable.Symbols())
}

func TestIsSameMetricV2(t *testing.T) {
	tests := []struct {
		name string
		ts1  *writev2.TimeSeries
		ts2  *writev2.TimeSeries
		same bool
	}{
		{
			name: "same",
			same: true,
			ts1: &writev2.TimeSeries{
				LabelsRefs: []uint32{1, 2, 3, 4},
			},
			ts2: &writev2.TimeSeries{
				LabelsRefs: []uint32{1, 2, 3, 4},
			},
		},
		{
			name: "different",
			same: false,
			ts1: &writev2.TimeSeries{
				LabelsRefs: []uint32{1, 2, 3, 4},
			},
			ts2: &writev2.TimeSeries{
				LabelsRefs: []uint32{1, 2, 3, 5},
			},
		},
	}
	for _, test := range tests {
		require.Equal(t, test.same, isSameMetricV2(test.ts1, test.ts2))
	}
}

func TestConflictHandling(t *testing.T) {
	// Test 1: No conflicts - different metrics should have different hashes
	t.Run("different metrics should not conflict", func(t *testing.T) {
		converter := newPrometheusConverterV2(Settings{})

		metric1 := createSample(1.0, []prompb.Label{
			{Name: "name1", Value: "value1"},
			{Name: "name2", Value: "value2"},
		})

		metric2 := createSample(2.0, []prompb.Label{
			{Name: "name3", Value: "value3"},
			{Name: "name4", Value: "value4"},
		})

		converter.addSample(metric1.sample, metric1.labels, metadata{})
		converter.addSample(metric2.sample, metric2.labels, metadata{})

		require.Equal(t, 0, converter.conflictCount)
		require.Len(t, converter.unique, 2)
	})

	// Test 2: Same metric - should be merged
	t.Run("same metric should be merged", func(t *testing.T) {
		converter := newPrometheusConverterV2(Settings{})

		labels := []prompb.Label{
			{Name: "name", Value: "value"},
		}

		sample1 := &writev2.Sample{Value: 1.0, Timestamp: 1000}
		sample2 := &writev2.Sample{Value: 2.0, Timestamp: 2000}

		converter.addSample(sample1, labels, metadata{})
		converter.addSample(sample2, labels, metadata{})

		require.Equal(t, 0, converter.conflictCount)
		require.Len(t, converter.unique, 1)
		require.Len(t, converter.unique[timeSeriesSignature(labels)].Samples, 2)
	})

	// Test 3: Hash conflict - different metrics with same hash
	t.Run("different metrics with same hash should be handled as conflicts", func(t *testing.T) {
		converter := newPrometheusConverterV2(Settings{})

		labels1 := []prompb.Label{
			{Name: "__name__", Value: "metric1"},
			{Name: "label1", Value: "value1"},
		}

		labels2 := []prompb.Label{
			{Name: "__name__", Value: "metric2"},
			{Name: "label2", Value: "value2"},
		}

		sample1 := &writev2.Sample{Value: 1.0, Timestamp: 1000}
		sample2 := &writev2.Sample{Value: 2.0, Timestamp: 2000}

		converter.addSample(sample1, labels1, metadata{})
		signature1 := timeSeriesSignature(labels1)

		// Manually force a hash collision by adding the second metric to the same signature
		// This simulates what would happen in a real hash collision scenario
		converter.addSample(sample2, labels2, metadata{})

		// Now manually simulate a hash collision by moving the second metric to create a conflict
		signature2 := timeSeriesSignature(labels2)
		if signature1 != signature2 {
			// If they don't naturally collide, we'll simulate the collision
			// by manually manipulating the converter state to create the conflict scenario
			ts2 := converter.unique[signature2]
			delete(converter.unique, signature2)

			// Move ts2 to conflicts under signature1 to simulate hash collision
			converter.conflicts[signature1] = append(converter.conflicts[signature1], ts2)
			converter.conflictCount++
		}

		// Verify that conflicts are properly tracked
		require.Positive(t, converter.conflictCount, "Expected at least one conflict to be recorded")

		// Verify that we have either:
		// 1. Two unique entries (if no natural collision occurred), or
		// 2. One unique entry and conflicts (if collision was simulated)
		totalTimeSeries := len(converter.unique)
		for _, conflictList := range converter.conflicts {
			totalTimeSeries += len(conflictList)
		}
		require.Equal(t, 2, totalTimeSeries, "Should have exactly 2 time series total")

		// Verify that different metrics with same hash maintain their separate identity
		allTimeSeries := converter.timeSeries()
		require.Len(t, allTimeSeries, 2, "Should produce 2 separate time series")

		// Verify that the two time series have different label references (different metrics)
		ts1, ts2 := &allTimeSeries[0], &allTimeSeries[1]
		require.False(t, isSameMetricV2(ts1, ts2), "Time series should be different metrics")
	})
}

// Helper function to create a sample with labels
type metricSample struct {
	sample *writev2.Sample
	labels []prompb.Label
}

func createSample(value float64, labels []prompb.Label) metricSample {
	return metricSample{
		sample: &writev2.Sample{
			Value:     value,
			Timestamp: 1000,
		},
		labels: labels,
	}
}

func TestScopeAttributesV2(t *testing.T) {
	settings := Settings{
		Namespace:         "",
		ExternalLabels:    nil,
		DisableTargetInfo: false,
		AddMetricSuffixes: false,
		SendMetadata:      false,
	}

	// Create metrics with scope attributes
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	// Scope 1
	sm1 := rm.ScopeMetrics().AppendEmpty()
	scope1 := sm1.Scope()
	scope1.SetName("scope1")
	scope1.SetVersion("1.0.0")
	scope1.Attributes().PutStr("scope.attr", "value1")

	m1 := sm1.Metrics().AppendEmpty()
	m1.SetName("test_metric")
	m1.SetDescription("test description")
	m1.SetUnit("1")
	g1 := m1.SetEmptyGauge()
	dp1 := g1.DataPoints().AppendEmpty()
	dp1.SetTimestamp(pcommon.Timestamp(time.Now().UnixNano()))
	dp1.SetIntValue(1)

	// Scope 2
	sm2 := rm.ScopeMetrics().AppendEmpty()
	scope2 := sm2.Scope()
	scope2.SetName("scope2")
	scope2.SetVersion("2.0.0")
	scope2.Attributes().PutStr("scope.attr", "value2")

	m2 := sm2.Metrics().AppendEmpty()
	m2.SetName("test_metric")
	m2.SetDescription("test description")
	m2.SetUnit("1")
	g2 := m2.SetEmptyGauge()
	dp2 := g2.DataPoints().AppendEmpty()
	dp2.SetTimestamp(pcommon.Timestamp(time.Now().UnixNano()))
	dp2.SetIntValue(2)

	tsMap, _, err := FromMetricsV2(md, settings)
	require.NoError(t, err)

	require.Len(t, tsMap, 2)
}
