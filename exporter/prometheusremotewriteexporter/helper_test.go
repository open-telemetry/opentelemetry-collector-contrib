// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"testing"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
)

// Test_batchTimeSeries checks batchTimeSeries return the correct number of requests
// depending on byte size.
func Test_batchTimeSeries(t *testing.T) {
	// First we will instantiate a dummy TimeSeries instance to pass into both the export call and compare the http request
	labels := getPromLabels(label11, value11, label12, value12, label21, value21, label22, value22)
	sample1 := getSample(floatVal1, msTime1)
	sample2 := getSample(floatVal2, msTime2)
	sample3 := getSample(floatVal3, msTime3)
	ts1 := getTimeSeries(labels, sample1, sample2)
	ts2 := getTimeSeries(labels, sample1, sample2, sample3)

	tsMap1 := getTimeseriesMap([]*prompb.TimeSeries{})
	tsMap2 := getTimeseriesMap([]*prompb.TimeSeries{ts1})
	tsMap3 := getTimeseriesMap([]*prompb.TimeSeries{ts1, ts2})

	tests := []struct {
		name                string
		tsMap               map[string]*prompb.TimeSeries
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
			300,
			2,
			false,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, err := batchTimeSeries(tt.tsMap, tt.maxBatchByteSize, nil)
			if tt.returnErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.numExpectedRequests, len(requests))
		})
	}
}

// Ensure that before a prompb.WriteRequest is created, that the points per TimeSeries
// are sorted by Timestamp value, to prevent Prometheus from barfing when it gets poorly
// sorted values. See issues:
// * https://github.com/open-telemetry/wg-prometheus/issues/10
// * https://github.com/open-telemetry/opentelemetry-collector/issues/2315
func TestEnsureTimeseriesPointsAreSortedByTimestamp(t *testing.T) {
	outOfOrder := []prompb.TimeSeries{
		{
			Samples: []prompb.Sample{
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
			Samples: []prompb.Sample{
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
	got := convertTimeseriesToRequest(outOfOrder)

	// We must ensure that the resulting Timeseries' sample points are sorted by Timestamp.
	want := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Samples: []prompb.Sample{
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
				Samples: []prompb.Sample{
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
		},
	}
	assert.Equal(t, got, want)

	// For a full sanity/logical check, assert that EVERY
	// Sample has a Timestamp bigger than its prior values.
	for ti, ts := range got.Timeseries {
		for i := range ts.Samples {
			si := ts.Samples[i]
			for j := 0; j < i; j++ {
				sj := ts.Samples[j]
				if sj.Timestamp > si.Timestamp {
					t.Errorf("Timeseries[%d]: Sample[%d].Timestamp(%d) > Sample[%d].Timestamp(%d)",
						ti, j, sj.Timestamp, i, si.Timestamp)
				}
			}
		}
	}
}
