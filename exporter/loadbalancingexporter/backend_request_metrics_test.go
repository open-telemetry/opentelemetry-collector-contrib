// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadatatest"
)

var (
	backendRequestBytesBounds = []float64{1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216}
	backendRequestItemsBounds = []float64{1, 10, 50, 100, 500, 1000, 5000, 10000, 50000}
)

func assertBackendRequestMetrics(t *testing.T, telemetry *componenttest.Telemetry, signal, endpoint string, bytes, items int64) {
	t.Helper()

	require.Positive(t, bytes)
	require.Positive(t, items)

	signalAttrs := attribute.NewSet(attribute.String("signal", signal))
	endpointAttrs := attribute.NewSet(attribute.String("endpoint", endpoint), attribute.String("signal", signal))
	metadatatest.AssertEqualLoadbalancerBackendRequestTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: endpointAttrs,
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualLoadbalancerBackendRequestBytes(t, telemetry, []metricdata.HistogramDataPoint[int64]{
		backendRequestHistogramPoint(signalAttrs, bytes, backendRequestBytesBounds),
	}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
	metadatatest.AssertEqualLoadbalancerBackendRequestItems(t, telemetry, []metricdata.HistogramDataPoint[int64]{
		backendRequestHistogramPoint(signalAttrs, items, backendRequestItemsBounds),
	}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
}

func backendRequestHistogramPoint(attrs attribute.Set, value int64, bounds []float64) metricdata.HistogramDataPoint[int64] {
	return metricdata.HistogramDataPoint[int64]{
		Attributes:   attrs,
		Count:        1,
		Bounds:       bounds,
		BucketCounts: backendRequestBucketCounts(value, bounds),
		Min:          metricdata.NewExtrema(value),
		Max:          metricdata.NewExtrema(value),
		Sum:          value,
	}
}

func backendRequestBucketCounts(value int64, bounds []float64) []uint64 {
	counts := make([]uint64, len(bounds)+1)
	for i, bound := range bounds {
		if float64(value) <= bound {
			counts[i] = 1
			return counts
		}
	}
	counts[len(counts)-1] = 1
	return counts
}
