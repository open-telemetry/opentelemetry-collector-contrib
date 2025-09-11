// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"math"
	"testing"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
)

// Test_batchTimeSeriesV2 checks batchTimeSeriesV2 return the correct number of requests
// depending on byte size.
func Test_batchTimeSeriesV2(t *testing.T) {
	// First we will instantiate a dummy TimeSeries instance to pass into both the export call and compare the http request
	labels := getPromLabels(label11, value11, label12, value12, label21, value21, label22, value22)
	sample1 := getSampleV2(floatVal1, msTime1)
	sample2 := getSampleV2(floatVal2, msTime2)
	sample3 := getSampleV2(floatVal3, msTime3)
	ts1, smb1 := getTimeSeriesV2(labels, sample1, sample2)
	ts2, _ := getTimeSeriesV2(labels, sample1, sample2, sample3)

	tsMap1 := getTimeseriesMapV2([]*writev2.TimeSeries{})
	tsMap2 := getTimeseriesMapV2([]*writev2.TimeSeries{ts1})
	tsMap3 := getTimeseriesMapV2([]*writev2.TimeSeries{ts1, ts2})

	tests := []struct {
		name                string
		tsMap               map[string]*writev2.TimeSeries
		maxBatchByteSize    int
		numExpectedRequests int
		returnErr           bool
	}{
		{
			"no_timeseries",
			tsMap1,
			100,
			-1,
			true,
		},
		{
			"normal_case",
			tsMap2,
			300,
			1,
			false,
		},
		{
			"two_requests",
			tsMap3,
			170,
			2,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newBatchTimeServicesState()
			requests, err := batchTimeSeriesV2(tt.tsMap, smb1, tt.maxBatchByteSize, state)
			if tt.returnErr {
				assert.Error(t, err)
				return
			}

			for r := range requests {
				assert.Equal(t, smb1.Symbols(), requests[r].Symbols)
			}

			assert.NoError(t, err)
			assert.Len(t, requests, tt.numExpectedRequests)
			if tt.numExpectedRequests <= 1 {
				assert.Equal(t, math.MaxInt, state.nextTimeSeriesBufferSize)
				assert.Equal(t, math.MaxInt, state.nextMetricMetadataBufferSize)
				assert.Equal(t, 2*len(requests), state.nextRequestBufferSize)
			} else {
				assert.Equal(t, max(10, len(requests[len(requests)-2].Timeseries)*2), state.nextTimeSeriesBufferSize)
				assert.Equal(t, math.MaxInt, state.nextMetricMetadataBufferSize)
				assert.Equal(t, 2*len(requests), state.nextRequestBufferSize)
			}
		})
	}
}

func Test_batchTimeSeriesV2UpdatesStateForLargeBatches(t *testing.T) {
	labels := getPromLabels(label11, value11, label12, value12, label21, value21, label22, value22)
	sample1 := getSampleV2(floatVal1, msTime1)
	sample2 := getSampleV2(floatVal2, msTime2)
	sample3 := getSampleV2(floatVal3, msTime3)

	// Benchmark for large data sizes
	// First allocate 100k time series
	tsArray := make([]*writev2.TimeSeries, 0, 100000)
	var smb writev2.SymbolsTable
	var ts *writev2.TimeSeries
	for i := 0; i < 100000; i++ {
		ts, smb = getTimeSeriesV2(labels, sample1, sample2, sample3)
		tsArray = append(tsArray, ts)
	}

	tsMap1 := getTimeseriesMapV2(tsArray)
	state := newBatchTimeServicesState()
	requests, err := batchTimeSeriesV2(tsMap1, smb, 1000000, state)

	assert.NoError(t, err)
	assert.Len(t, requests, 7)
	assert.Equal(t, len(requests[len(requests)-2].Timeseries)*2, state.nextTimeSeriesBufferSize)
	assert.Equal(t, math.MaxInt, state.nextMetricMetadataBufferSize)
	assert.Equal(t, 14, state.nextRequestBufferSize)
}

// Ensure that before a writev2.Request is created, that the points per TimeSeries
// are sorted by Timestamp value, to prevent Prometheus from barfing when it gets poorly
// sorted values. See issues:
// * https://github.com/open-telemetry/wg-prometheus/issues/10
// * https://github.com/open-telemetry/opentelemetry-collector/issues/2315
func TestEnsureTimeseriesPointsAreSortedByTimestampV2(t *testing.T) {
	outOfOrder := []writev2.TimeSeries{
		{
			Samples: []writev2.Sample{
				{
					Value:     10.11,
					Timestamp: 1000,
				},
				{
					Value:     7.81,
					Timestamp: 2,
				},
				{
					Value:     987.81,
					Timestamp: 1,
				},
				{
					Value:     18.22,
					Timestamp: 999,
				},
			},
		},
		{
			Samples: []writev2.Sample{
				{
					Value:     99.91,
					Timestamp: 5,
				},
				{
					Value:     4.33,
					Timestamp: 3,
				},
				{
					Value:     47.81,
					Timestamp: 4,
				},
				{
					Value:     18.22,
					Timestamp: 8,
				},
			},
		},
	}
	got := orderBySampleTimestampV2(outOfOrder)

	// We must ensure that the resulting Timeseries' sample points are sorted by Timestamp.
	want := []writev2.TimeSeries{
		{
			Samples: []writev2.Sample{
				{
					Value:     987.81,
					Timestamp: 1,
				},
				{
					Value:     7.81,
					Timestamp: 2,
				},
				{
					Value:     18.22,
					Timestamp: 999,
				},
				{
					Value:     10.11,
					Timestamp: 1000,
				},
			},
		},
		{
			Samples: []writev2.Sample{
				{
					Value:     4.33,
					Timestamp: 3,
				},
				{
					Value:     47.81,
					Timestamp: 4,
				},
				{
					Value:     99.91,
					Timestamp: 5,
				},
				{
					Value:     18.22,
					Timestamp: 8,
				},
			},
		},
	}
	assert.Equal(t, want, got)

	// For a full sanity/logical check, assert that EVERY
	// Sample has a Timestamp bigger than its prior values.
	for ti, ts := range got {
		for i := range ts.Samples {
			si := ts.Samples[i]
			for j := 0; j < i; j++ {
				sj := ts.Samples[j]
				assert.LessOrEqual(t, sj.Timestamp, si.Timestamp, "Timeseries[%d]: Sample[%d].Timestamp(%d) > Sample[%d].Timestamp(%d)",
					ti, j, sj.Timestamp, i, si.Timestamp)
			}
		}
	}
}
