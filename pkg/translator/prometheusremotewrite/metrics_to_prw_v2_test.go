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
	// TODO: Test 3 Conflict - different metrics with same hash
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
