// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

// Regression test for the sync.Pool use-after-free bug fixed in
// DataDog/datadog-agent@fb75960 (pkg/util/quantile/ddsketch.go).
//
// When the translator converts multiple ExponentialHistogram data points to
// sketches in a single MapMetrics call, each resulting quantile.Sketch must
// own its bin memory independently of the pool.  Without the fix,
// sparseStore.bins shared the pool's backing array: once putBinList returned
// it, the next ConvertDDSketchIntoSketch call reused the same array and silently
// overwrote the earlier sketch's bin keys, producing non-monotonic keys and
// wildly wrong quantile estimates for the first sketch.

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	otlpmetrics "github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics"
	"github.com/DataDog/datadog-agent/pkg/util/quantile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// TestExponentialHistogramSketchPoolSafety reproduces the scenario from
// https://github.com/DataDog/datadog-agent/issues/48508:
// two ExponentialHistogram data points with very different value ranges are
// converted in a single MapMetrics call.  The first sketch's p50 must stay
// within the value range of the first data point, not jump to the value range
// of the second data point due to pool-backed array reuse.
func TestExponentialHistogramSketchPoolSafety(t *testing.T) {
	ctx := t.Context()

	// buildDeltaDP creates a delta ExponentialHistogram data point with
	// scale=0 (base=2) so that bucket semantics are easy to reason about:
	// bucket at offset+i covers the value range [2^(offset+i), 2^(offset+i+1)).
	buildDeltaDP := func(offset int32, bucketCounts []uint64, sum, minVal, maxVal float64) pmetric.ExponentialHistogramDataPoint {
		dp := pmetric.NewExponentialHistogramDataPoint()
		ts := pcommon.Timestamp(1_000_000_000)
		dp.SetStartTimestamp(ts - 10_000_000_000)
		dp.SetTimestamp(ts)
		var total uint64
		for _, c := range bucketCounts {
			total += c
		}
		dp.SetCount(total)
		dp.SetSum(sum)
		dp.SetMin(minVal)
		dp.SetMax(maxVal)
		dp.SetScale(0) // base = 2
		dp.Positive().SetOffset(offset)
		dp.Positive().BucketCounts().FromRaw(bucketCounts)
		return dp
	}

	// Data point A: 500 observations spread across buckets [-12, -8].
	// Each bucket i covers [2^(i), 2^(i+1)), so values are in the range
	// [2^-12, 2^-7) ≈ [0.24 ms, 7.8 ms].  The sketch's bins will have
	// keys in the low range.
	dpA := buildDeltaDP(
		-12,
		[]uint64{100, 100, 100, 100, 100},
		// approximate sum using bucket midpoints: Σ 100 * 2^(i+0.5) for i in -12..-8
		0.001,
		1.0/4096, // 2^-12
		1.0/128,  // 2^-7
	)
	dpA.Attributes().PutStr("dp", "A")

	// Data point B: 1 observation in bucket 10, covering [2^10, 2^11) = [1024, 2048).
	// Without the fix, converting B reuses the pool backing array that holds A's
	// bins and writes the high key (≈1024) into position [0], corrupting A's sketch.
	dpB := buildDeltaDP(
		10,
		[]uint64{1},
		1536.0, // midpoint of [1024, 2048)
		1024.0,
		2048.0,
	)
	dpB.Attributes().PutStr("dp", "B")

	// Build a single Metrics payload with both data points under one metric.
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test.exp.histogram.pool")
	expo := m.SetEmptyExponentialHistogram()
	expo.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dpA.CopyTo(expo.DataPoints().AppendEmpty())
	dpB.CopyTo(expo.DataPoints().AppendEmpty())

	// Set up the translator in Distributions mode to activate the
	// ExponentialHistogram → quantile.Sketch conversion path.
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zap.NewNop()
	attrTranslator, err := attributes.NewTranslator(set)
	require.NoError(t, err)
	tr, err := otlpmetrics.NewDefaultTranslator(set, attrTranslator,
		otlpmetrics.WithHistogramMode(otlpmetrics.HistogramModeDistributions),
		otlpmetrics.WithFallbackSourceProvider(testProvider("fallbackHost")),
	)
	require.NoError(t, err)

	consumer := NewConsumer(nil)
	_, err = tr.MapMetrics(ctx, md, consumer, nil)
	require.NoError(t, err)

	require.Len(t, consumer.sl, 2, "expected one sketch per data point")

	cfg := quantile.Default()
	skA := consumer.sl[0].Points[0].Sketch
	skB := consumer.sl[1].Points[0].Sketch

	require.NotNil(t, skA)
	require.NotNil(t, skB)

	// Sketch A must have count 500.
	assert.EqualValues(t, 500, skA.Basic.Cnt, "sketch A count")
	// Sketch B must have count 1.
	assert.EqualValues(t, 1, skB.Basic.Cnt, "sketch B count")

	// Sketch A's p50 must be in the range of data point A's values (~0.24 ms – 7.8 ms).
	// Without the fix, pool reuse corrupts A's first bin key to match B's high key
	// (~1024 s), causing p50 to be far outside this range.
	p50A := skA.Quantile(cfg, 0.5)
	assert.Greater(t, p50A, 0.0, "sketch A p50 must be positive")
	assert.Less(t, p50A, 0.1,
		"sketch A p50 must be <100 ms; got %v s — pool reuse may have corrupted sketch A's bins", p50A)

	// Sketch B's p50 must be in the range of data point B's values (~1024 s – 2048 s).
	p50B := skB.Quantile(cfg, 0.5)
	assert.Greater(t, p50B, 100.0,
		"sketch B p50 must be >100 s; got %v s", p50B)
}
