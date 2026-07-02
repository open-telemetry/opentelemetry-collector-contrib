// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/metadatatest"
)

func TestProcessorTelemetry(t *testing.T) {
	now := time.Now()
	sumAttrs := attribute.NewSet(attribute.String("metric_type", "sum"))
	histAttrs := attribute.NewSet(attribute.String("metric_type", "histogram"))
	dropAttrs := func(metricType, reason string) attribute.Set {
		return attribute.NewSet(
			attribute.String("metric_type", metricType),
			attribute.String("reason", reason),
		)
	}

	tests := []struct {
		name     string
		inputs   []pmetric.Metrics
		wantConv []metricdata.DataPoint[int64]
		wantStrm []metricdata.DataPoint[int64]
		wantDrop []metricdata.DataPoint[int64]
	}{
		{
			// Four cumulative sum points: the first establishes the baseline
			// (initial drop), the next two are valid conversions, and the
			// fourth is a reset (drop).
			name: "sum: two conversions, one reset",
			inputs: []pmetric.Metrics{
				newSumMetrics("metric_1", now, 0, 100),
				newSumMetrics("metric_1", now, 1, 150),
				newSumMetrics("metric_1", now, 2, 200),
				newSumMetrics("metric_1", now, 3, 50),
			},
			wantConv: []metricdata.DataPoint[int64]{
				{Attributes: sumAttrs, Value: 2},
			},
			wantStrm: []metricdata.DataPoint[int64]{{Value: 1}},
			wantDrop: []metricdata.DataPoint[int64]{
				{Attributes: dropAttrs("sum", "initial"), Value: 1},
				{Attributes: dropAttrs("sum", "reset"), Value: 1},
			},
		},
		{
			// Three histogram points: first is the baseline (initial drop),
			// second is a valid conversion, third's count drops below the
			// previous one which counts as a reset. Note that for histograms
			// the reset point is forwarded with the full cumulative value as
			// if it were a delta (the conversion counter is incremented and
			// no drop is recorded). This is incorrect behavior — see #48278
			name: "histogram: reset does not drop",
			inputs: []pmetric.Metrics{
				newHistogramMetrics("metric_1", now, 0, 10, 100, []uint64{10}),
				newHistogramMetrics("metric_1", now, 1, 20, 200, []uint64{20}),
				newHistogramMetrics("metric_1", now, 2, 5, 50, []uint64{5}),
			},
			wantConv: []metricdata.DataPoint[int64]{
				{Attributes: histAttrs, Value: 2},
			},
			wantStrm: []metricdata.DataPoint[int64]{{Value: 1}},
			wantDrop: []metricdata.DataPoint[int64]{
				{Attributes: dropAttrs("histogram", "initial"), Value: 1},
			},
		},
		{
			// Three distinct streams (two sums with different names + one
			// histogram) — exercises the streams_tracked gauge.
			name: "three distinct streams",
			inputs: []pmetric.Metrics{
				newSumMetrics("metric_a", now, 0, 100),
				newSumMetrics("metric_b", now, 0, 100),
				newHistogramMetrics("hist_a", now, 0, 10, 100, []uint64{10}),
			},
			wantStrm: []metricdata.DataPoint[int64]{{Value: 3}},
			wantDrop: []metricdata.DataPoint[int64]{
				{Attributes: dropAttrs("histogram", "initial"), Value: 1},
				{Attributes: dropAttrs("sum", "initial"), Value: 2},
			},
		},
		{
			// Histogram with bucket count changing between observations — the
			// second point cannot be diffed against the first and is dropped
			// with reason=bucket_mismatch.
			name: "histogram: bucket_mismatch drop",
			inputs: []pmetric.Metrics{
				newHistogramMetrics("metric_1", now, 0, 10, 100, []uint64{10}),
				newHistogramMetrics("metric_1", now, 1, 20, 200, []uint64{15, 5}),
			},
			wantStrm: []metricdata.DataPoint[int64]{{Value: 1}},
			wantDrop: []metricdata.DataPoint[int64]{
				{Attributes: dropAttrs("histogram", "bucket_mismatch"), Value: 1},
				{Attributes: dropAttrs("histogram", "initial"), Value: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tel := componenttest.NewTelemetry()
			defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

			next := new(consumertest.MetricsSink)
			factory := NewFactory()
			set := metadatatest.NewSettings(tel)
			cfg := factory.CreateDefaultConfig()
			mp, err := factory.CreateMetrics(t.Context(), set, cfg, next)
			require.NoError(t, err)
			require.NoError(t, mp.Start(t.Context(), componenttest.NewNopHost()))
			defer func() { require.NoError(t, mp.Shutdown(t.Context())) }()

			for _, md := range tt.inputs {
				require.NoError(t, mp.ConsumeMetrics(t.Context(), md))
			}

			if tt.wantConv != nil {
				metadatatest.AssertEqualCumulativetodeltaDatapoints(t, tel, tt.wantConv, metricdatatest.IgnoreTimestamp())
			}
			metadatatest.AssertEqualCumulativetodeltaStreamsTracked(t, tel, tt.wantStrm, metricdatatest.IgnoreTimestamp())
			if tt.wantDrop != nil {
				metadatatest.AssertEqualCumulativetodeltaDatapointsDropped(t, tel, tt.wantDrop, metricdatatest.IgnoreTimestamp())
			}
		})
	}
}

func newSumMetrics(name string, base time.Time, offsetSec int, value float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(name)
	sum := m.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(base.Add(time.Duration(offsetSec) * time.Second)))
	dp.SetDoubleValue(value)
	return md
}

func newHistogramMetrics(name string, base time.Time, offsetSec int, count uint64, sum float64, buckets []uint64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(name)
	hist := m.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := hist.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(base.Add(time.Duration(offsetSec) * time.Second)))
	dp.SetCount(count)
	dp.SetSum(sum)
	dp.BucketCounts().FromRaw(buckets)
	return md
}
