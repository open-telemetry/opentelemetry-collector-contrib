// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var allKinds = []metricKind{
	kindGauge, kindDeltaSum, kindCumulativeSum, kindDeltaHistogram, kindCumulativeHistogram,
	kindDeltaExponentialHistogram, kindCumulativeExponentialHistogram, kindSummary,
}

// newTestProcessor returns a read-only processor whose telemetry is read
// through the returned reader. mutate adjusts the default test config; next
// defaults to a sink.
func newTestProcessor(t *testing.T, mutate func(*Config), next consumer.Metrics) (*cardinalityProcessor, *sdkmetric.ManualReader) {
	t.Helper()
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        300,
		TopOffendersCount:           10,
		ReadOnly:                    true,
	}
	if mutate != nil {
		mutate(cfg)
	}
	require.NoError(t, cfg.Validate())
	if next == nil {
		next = new(consumertest.MetricsSink)
	}
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.WithoutCancel(t.Context()))) })
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	set.MeterProvider = provider
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)
	return proc.(*cardinalityProcessor), reader
}

func testValues(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s-%d", prefix, i)
	}
	return out
}

// kindBatch returns one metric of the given kind with one data point per value.
// Each point carries label=value and, unless label is "region", region=r1.
func kindBatch(kind metricKind, metricName, label string, values []string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(metricName)
	temporality := pmetric.AggregationTemporalityCumulative
	switch kind {
	case kindDeltaSum, kindDeltaHistogram, kindDeltaExponentialHistogram:
		temporality = pmetric.AggregationTemporalityDelta
	}
	var newPoint func() pcommon.Map
	switch kind {
	case kindGauge:
		g := m.SetEmptyGauge()
		newPoint = func() pcommon.Map {
			dp := g.DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			return dp.Attributes()
		}
	case kindDeltaSum, kindCumulativeSum:
		s := m.SetEmptySum()
		s.SetAggregationTemporality(temporality)
		newPoint = func() pcommon.Map {
			dp := s.DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			return dp.Attributes()
		}
	case kindDeltaHistogram, kindCumulativeHistogram:
		h := m.SetEmptyHistogram()
		h.SetAggregationTemporality(temporality)
		newPoint = func() pcommon.Map {
			dp := h.DataPoints().AppendEmpty()
			dp.SetCount(1)
			dp.ExplicitBounds().FromRaw([]float64{1})
			dp.BucketCounts().FromRaw([]uint64{1, 0})
			return dp.Attributes()
		}
	case kindDeltaExponentialHistogram, kindCumulativeExponentialHistogram:
		e := m.SetEmptyExponentialHistogram()
		e.SetAggregationTemporality(temporality)
		newPoint = func() pcommon.Map {
			dp := e.DataPoints().AppendEmpty()
			dp.SetCount(1)
			return dp.Attributes()
		}
	case kindSummary:
		s := m.SetEmptySummary()
		newPoint = func() pcommon.Map {
			dp := s.DataPoints().AppendEmpty()
			dp.SetCount(1)
			return dp.Attributes()
		}
	}
	for _, v := range values {
		attrs := newPoint()
		attrs.PutStr(label, v)
		if label != "region" {
			attrs.PutStr("region", "r1")
		}
	}
	return md
}

func withService(md pmetric.Metrics, service string) pmetric.Metrics {
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr("service.name", service)
	return md
}

func protoBytes(t *testing.T, md pmetric.Metrics) []byte {
	t.Helper()
	b, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(md)
	require.NoError(t, err)
	return b
}

func consume(t *testing.T, p *cardinalityProcessor, md pmetric.Metrics) {
	t.Helper()
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))
}

type distinctPoint struct {
	metricName, labelKey, metricType, recommendation string
	multiService                                     bool
	value                                            int64
}

// topDistinct returns the processor_cardinality_top.distinct data points. The
// SDK omits the metric when the callback observes nothing.
func topDistinct(t *testing.T, reader *sdkmetric.ManualReader) []distinctPoint {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &rm))
	var gauge metricdata.Gauge[int64]
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "otelcol_processor_cardinality_top.distinct" {
				var ok bool
				gauge, ok = m.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
			}
		}
	}
	str := func(set attribute.Set, key string) string {
		v, found := set.Value(attribute.Key(key))
		require.True(t, found, key)
		return v.AsString()
	}
	out := make([]distinctPoint, 0, len(gauge.DataPoints))
	for _, dp := range gauge.DataPoints {
		multi, found := dp.Attributes.Value("multi_service")
		require.True(t, found)
		out = append(out, distinctPoint{
			metricName:     str(dp.Attributes, "metric_name"),
			labelKey:       str(dp.Attributes, "label_key"),
			metricType:     str(dp.Attributes, "metric_type"),
			recommendation: str(dp.Attributes, "recommendation"),
			multiService:   multi.AsBool(),
			value:          dp.Value,
		})
	}
	return out
}

// pointFor returns the data point for metric "m" and labelKey.
func pointFor(points []distinctPoint, labelKey string) (distinctPoint, bool) {
	for _, dp := range points {
		if dp.metricName == "m" && dp.labelKey == labelKey {
			return dp, true
		}
	}
	return distinctPoint{}, false
}

func TestReadOnly_DoesNotMutate(t *testing.T) {
	for _, kind := range allKinds {
		t.Run(kind.String(), func(t *testing.T) {
			p, reader := newTestProcessor(t, func(c *Config) {
				c.MaxCardinalityDeltaPerEpoch = 1
				c.MaxTrackerCount = 1 // region is rejected
				c.EnforcementMode = EnforcementStripAndReaggregate
			}, nil)
			for epoch := range 3 {
				md := kindBatch(kind, "m", "user_id", testValues(fmt.Sprint(epoch), 50*(epoch+1)))
				want := pmetric.NewMetrics()
				md.CopyTo(want)
				md.MarkReadOnly() // any mutation panics
				consume(t, p, md)
				p.rotate()
				assert.Equal(t, protoBytes(t, want), protoBytes(t, md))
			}
			assert.Positive(t, p.trackersRejected.Load())
			assert.Zero(t, p.labelsStripped.Load())
			dp, ok := pointFor(topDistinct(t, reader), "user_id")
			require.True(t, ok)
			assert.InDelta(t, 150, dp.value, 2)
		})
	}
}

func TestReadOnly_PassesSameBatch(t *testing.T) {
	var got pmetric.Metrics
	next, err := consumer.NewMetrics(func(_ context.Context, md pmetric.Metrics) error {
		got = md
		return nil
	})
	require.NoError(t, err)
	p, _ := newTestProcessor(t, nil, next)
	md := kindBatch(kindGauge, "m", "user_id", testValues("u", 3))
	consume(t, p, md)

	// A write through the received value is visible in md only if it is the same batch.
	got.ResourceMetrics().At(0).Resource().Attributes().PutStr("probe", "x")
	_, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get("probe")
	assert.True(t, ok)
}

func TestReadOnly_ReturnsNextConsumerError(t *testing.T) {
	errBoom := errors.New("boom")
	p, _ := newTestProcessor(t, nil, consumertest.NewErr(errBoom))
	err := p.ConsumeMetrics(t.Context(), kindBatch(kindGauge, "m", "user_id", testValues("u", 3)))
	assert.ErrorIs(t, err, errBoom)
}

func TestCapabilities(t *testing.T) {
	for _, mode := range []EnforcementMode{EnforcementTagOnly, EnforcementOverflowAttribute, EnforcementStripAndReaggregate} {
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/read_only=%t", mode, readOnly), func(t *testing.T) {
				p, _ := newTestProcessor(t, func(c *Config) {
					c.EnforcementMode = mode
					c.ReadOnly = readOnly
				}, nil)
				assert.Equal(t, !readOnly, p.Capabilities().MutatesData)
			})
		}
	}
}

func TestReadOnly_UnusualInput(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	p, _ := newTestProcessor(t, nil, sink)
	consume(t, p, pmetric.NewMetrics())

	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	ms.AppendEmpty().SetName("empty_type")
	dps := ms.AppendEmpty().SetEmptyGauge().DataPoints()
	dps.AppendEmpty() // no attributes
	attrs := dps.AppendEmpty().Attributes()
	attrs.PutInt("i", 1)
	attrs.PutBool("b", true)
	attrs.PutDouble("d", 1.5)
	attrs.PutEmptyBytes("bytes").FromRaw([]byte{1, 2})
	attrs.PutEmptyMap("map").PutStr("k", "v")
	attrs.PutEmptySlice("slice").AppendEmpty().SetStr("v")
	want := pmetric.NewMetrics()
	md.CopyTo(want)
	md.MarkReadOnly()

	consume(t, p, md)
	p.rotate()
	assert.Equal(t, protoBytes(t, want), protoBytes(t, md))
	assert.Len(t, sink.AllMetrics(), 2)
	assert.Equal(t, int64(6), p.trackerCount.Load())
}

func TestReadOnly_SkipsProtectedLabels(t *testing.T) {
	p, reader := newTestProcessor(t, func(c *Config) { c.NeverDropLabels = []string{"host"} }, nil)
	consume(t, p, kindBatch(kindDeltaSum, "m", "host", testValues("h", 20)))
	p.rotate()
	_, ok := pointFor(topDistinct(t, reader), "host")
	assert.False(t, ok)
	_, ok = pointFor(topDistinct(t, reader), "region")
	assert.True(t, ok)
}

func TestReadOnly_ConcurrentConsumeAndRotate(t *testing.T) {
	p, _ := newTestProcessor(t, nil, nil)
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Go(func() {
			for i := range 200 {
				md := kindBatch(allKinds[(g+i)%len(allKinds)], fmt.Sprintf("m%d", g), "user_id", testValues(fmt.Sprintf("%d-%d", g, i), 20))
				assert.NoError(t, p.ConsumeMetrics(t.Context(), md))
			}
		})
	}
	for range 50 {
		p.rotate()
	}
	wg.Wait()
}

func TestKindOf(t *testing.T) {
	for _, kind := range allKinds {
		m := kindBatch(kind, "m", "l", nil).ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, kind, kindOf(m), kind.String())
	}
	unspecified := pmetric.NewMetric()
	unspecified.SetEmptySum()
	assert.Equal(t, kindCumulativeSum, kindOf(unspecified), "unspecified temporality counts as cumulative")
	assert.Equal(t, kindUnknown, kindOf(pmetric.NewMetric()))
}

func TestRecommendation(t *testing.T) {
	for kind, want := range map[metricKind]string{
		kindGauge:                          "remove_and_reaggregate",
		kindDeltaSum:                       "remove_and_reaggregate",
		kindDeltaHistogram:                 "remove_and_reaggregate",
		kindDeltaExponentialHistogram:      "remove_and_reaggregate",
		kindCumulativeSum:                  "reduce_at_source",
		kindCumulativeHistogram:            "reduce_at_source",
		kindCumulativeExponentialHistogram: "reduce_at_source",
		kindSummary:                        "reduce_at_source",
		kindMixed:                          "reduce_at_source",
		kindUnknown:                        "reduce_at_source",
	} {
		assert.Equal(t, want, recommendation(kind, false), kind.String())
		assert.Equal(t, "check_per_service", recommendation(kind, true), kind.String())
	}
}

// The hot path refreshes its cached estimate at insert 1024 of 1087; the
// reported value must still count all 1087.
func TestTopDistinct_ReportsEveryValueOfTheEpoch(t *testing.T) {
	p, reader := newTestProcessor(t, nil, nil)
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("u", 1087)))
	p.rotate()
	dp, ok := pointFor(topDistinct(t, reader), "user_id")
	require.True(t, ok)
	assert.InDelta(t, 1087, dp.value, 5)
	assert.Equal(t, distinctPoint{
		metricName: "m", labelKey: "user_id", metricType: "delta_sum",
		recommendation: "remove_and_reaggregate", value: dp.value,
	}, dp)
}

// A stable label keeps its full count every epoch; growth-based detection
// stops reporting it after the first one.
func TestTopDistinct_StableLabelReportedEveryEpoch(t *testing.T) {
	p, reader := newTestProcessor(t, nil, nil)
	for range 3 {
		consume(t, p, kindBatch(kindGauge, "m", "user_id", testValues("u", 500)))
		p.rotate()
		dp, ok := pointFor(topDistinct(t, reader), "user_id")
		require.True(t, ok)
		assert.InDelta(t, 500, dp.value, 2)
	}
}

func TestTopDistinct_RanksAndTruncates(t *testing.T) {
	p, reader := newTestProcessor(t, func(c *Config) { c.TopOffendersCount = 2 }, nil)
	for _, m := range []struct {
		name string
		n    int
	}{{"m3", 30}, {"m2b", 40}, {"m1", 50}, {"m2", 40}} {
		consume(t, p, kindBatch(kindGauge, m.name, "user_id", testValues("u", m.n)))
	}
	p.rotate()
	got := topDistinct(t, reader)
	require.Len(t, got, 2)
	names := []string{got[0].metricName, got[1].metricName}
	assert.ElementsMatch(t, []string{"m1", "m2"}, names, "tie with m2b broken by name")
}

func TestTopDistinct_IdleTrackerDropsOut(t *testing.T) {
	p, reader := newTestProcessor(t, nil, nil)
	consume(t, p, kindBatch(kindGauge, "m", "user_id", testValues("u", 20)))
	p.rotate()
	_, ok := pointFor(topDistinct(t, reader), "user_id")
	require.True(t, ok)
	p.rotate() // no data this epoch
	_, ok = pointFor(topDistinct(t, reader), "user_id")
	assert.False(t, ok)
}

func TestTopDistinct_MultiService(t *testing.T) {
	p, reader := newTestProcessor(t, nil, nil)
	md := withService(kindBatch(kindDeltaSum, "m", "user_id", testValues("a", 30)), "checkout")
	withService(kindBatch(kindDeltaSum, "m", "user_id", testValues("b", 30)), "payments").
		ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	consume(t, p, md)
	p.rotate()
	dp, ok := pointFor(topDistinct(t, reader), "user_id")
	require.True(t, ok)
	assert.True(t, dp.multiService)
	assert.Equal(t, "check_per_service", dp.recommendation)
}

func TestTopDistinct_IgnoresMissingServiceName(t *testing.T) {
	p, reader := newTestProcessor(t, nil, nil)
	md := withService(kindBatch(kindDeltaSum, "m", "user_id", testValues("a", 30)), "checkout")
	kindBatch(kindDeltaSum, "m", "user_id", testValues("b", 30)).
		ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	consume(t, p, md)
	p.rotate()
	dp, ok := pointFor(topDistinct(t, reader), "user_id")
	require.True(t, ok)
	assert.False(t, dp.multiService)
	assert.Equal(t, "remove_and_reaggregate", dp.recommendation)
}

func TestTopDistinct_MixedTypes(t *testing.T) {
	p, reader := newTestProcessor(t, nil, nil)
	consume(t, p, kindBatch(kindGauge, "m", "user_id", testValues("u", 30)))
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("u", 30)))
	p.rotate()
	dp, ok := pointFor(topDistinct(t, reader), "user_id")
	require.True(t, ok)
	assert.Equal(t, "mixed", dp.metricType)
	assert.Equal(t, "reduce_at_source", dp.recommendation)
}

// The report works alongside enforcement, which still acts on growth, on every
// path through processMetric: delta sums are re-aggregated in strip and
// overflow modes, cumulative histograms fall back to tag_only.
func TestTopDistinct_ReportedWhileEnforcing(t *testing.T) {
	for _, mode := range []EnforcementMode{EnforcementTagOnly, EnforcementOverflowAttribute, EnforcementStripAndReaggregate} {
		for kind, want := range map[metricKind]string{
			kindDeltaSum:            "remove_and_reaggregate",
			kindCumulativeHistogram: "reduce_at_source",
		} {
			t.Run(fmt.Sprintf("%s/%s", mode, kind), func(t *testing.T) {
				sink := new(consumertest.MetricsSink)
				p, reader := newTestProcessor(t, func(c *Config) {
					c.ReadOnly = false
					c.EnforcementMode = mode
				}, sink)
				md := withService(kindBatch(kind, "m", "user_id", testValues("u", 300)), "checkout")
				in := protoBytes(t, md)
				consume(t, p, md)
				assert.NotEqual(t, in, protoBytes(t, sink.AllMetrics()[0]), "enforcement still acts on growth")
				p.rotate()
				dp, ok := pointFor(topDistinct(t, reader), "user_id")
				require.True(t, ok)
				assert.InDelta(t, 300, dp.value, 2)
				assert.Equal(t, kind.String(), dp.metricType)
				assert.False(t, dp.multiService)
				assert.Equal(t, want, dp.recommendation)

				consume(t, p, withService(kindBatch(kind, "m", "user_id", testValues("v", 300)), "payments"))
				p.rotate()
				dp, ok = pointFor(topDistinct(t, reader), "user_id")
				require.True(t, ok)
				assert.True(t, dp.multiService)
				assert.Equal(t, "check_per_service", dp.recommendation)
			})
		}
	}
}

func TestTopDistinct_Disabled(t *testing.T) {
	p, reader := newTestProcessor(t, func(c *Config) { c.TopOffendersCount = 0 }, nil)
	consume(t, p, kindBatch(kindGauge, "m", "user_id", testValues("u", 30)))
	p.rotate()
	assert.Empty(t, topDistinct(t, reader))
}
