// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"testing"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
)

// Ensure that before a prompb.WriteRequest is created, that the points per TimeSeries
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
