// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
)

func TestSpanMetrics_SelfTime(t *testing.T) {
	tests := []struct {
		name                 string
		selfTimeEnabled      bool
		expectSelfTimeMetric bool
		expectMetricCount    int
		expectDurationSumMS  float64
		expectSelfTimeSumMS  float64
		buildTrace           func() ptrace.Traces
	}{
		{
			name:                 "disabled by default leaves metric set unchanged",
			selfTimeEnabled:      false,
			expectSelfTimeMetric: false,
			expectMetricCount:    2, // calls + duration
			expectDurationSumMS:  150,
			buildTrace:           buildOverlapSelfTimeTrace,
		},
		{
			name:                 "enabled emits separate self_time metric from span tree",
			selfTimeEnabled:      true,
			expectSelfTimeMetric: true,
			expectMetricCount:    3, // calls + duration + self_time
			expectDurationSumMS:  150,
			// parent 60ms + two children 20ms + 30ms = 110ms
			expectSelfTimeSumMS: 110,
			buildTrace:          buildOverlapSelfTimeTrace,
		},
		{
			name:                 "partial trace omits self_time observations",
			selfTimeEnabled:      true,
			expectSelfTimeMetric: true,
			expectMetricCount:    3,
			expectDurationSumMS:  140, // root 100ms + orphan 40ms
			expectSelfTimeSumMS:  0,
			buildTrace:           buildPartialSelfTimeTrace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Histogram.Unit = metrics.Milliseconds
			cfg.SelfTime = SelfTimeConfig{Enabled: tt.selfTimeEnabled}
			c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock(), instanceID)
			require.NoError(t, err)

			require.NoError(t, c.ConsumeTraces(t.Context(), tt.buildTrace()))
			got := c.buildMetrics()

			require.Equal(t, 1, got.ResourceMetrics().Len())
			metricsSlice := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			assert.Equal(t, tt.expectMetricCount, metricsSlice.Len())

			var foundCalls, foundDuration, foundSelfTime bool
			var selfTimeSum float64
			for i := 0; i < metricsSlice.Len(); i++ {
				m := metricsSlice.At(i)
				switch m.Name() {
				case buildMetricName(DefaultNamespace, metricNameCalls):
					foundCalls = true
				case buildMetricName(DefaultNamespace, metricNameDuration):
					foundDuration = true
					assert.Equal(t, "ms", m.Unit())
					assert.InDelta(t, tt.expectDurationSumMS, histogramSum(m), 0.001)
				case buildMetricName(DefaultNamespace, metricNameSelfTime):
					foundSelfTime = true
					assert.Equal(t, "ms", m.Unit())
					selfTimeSum = histogramSum(m)
				case buildMetricName(DefaultNamespace, metricNameEvents):
					t.Fatalf("events metric should not appear in this test")
				}
			}

			assert.True(t, foundCalls)
			assert.True(t, foundDuration)
			assert.Equal(t, tt.expectSelfTimeMetric, foundSelfTime)
			if tt.expectSelfTimeMetric {
				assert.InDelta(t, tt.expectSelfTimeSumMS, selfTimeSum, 0.001)
			}
		})
	}
}

func TestComputeSelfTime_SimpleNesting(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{1})

	parent := spans.AppendEmpty()
	parent.SetTraceID(traceID)
	parent.SetSpanID(pcommon.SpanID([8]byte{1}))
	parent.SetStartTimestamp(0)
	parent.SetEndTimestamp(100)

	child := spans.AppendEmpty()
	child.SetTraceID(traceID)
	child.SetSpanID(pcommon.SpanID([8]byte{2}))
	child.SetParentSpanID(parent.SpanID())
	child.SetStartTimestamp(20)
	child.SetEndTimestamp(50)

	got := computeSelfTimeBySpanID(traces)
	assert.Equal(t, int64(70), got[parent.SpanID()])
	assert.Equal(t, int64(30), got[child.SpanID()])
}

func TestComputeSelfTime_OverlapUnionOnce(t *testing.T) {
	got := computeSelfTimeBySpanID(buildOverlapSelfTimeTrace())
	parentID := pcommon.SpanID([8]byte{0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa})
	assert.Equal(t, int64(60*time.Millisecond), got[parentID])
}

func TestComputeSelfTime_PartialOmits(t *testing.T) {
	got := computeSelfTimeBySpanID(buildPartialSelfTimeTrace())
	assert.Empty(t, got)
}

func TestComputeSelfTime_ClampsChildToParentInterval(t *testing.T) {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{3})

	parent := spans.AppendEmpty()
	parent.SetTraceID(traceID)
	parent.SetSpanID(pcommon.SpanID([8]byte{1}))
	parent.SetStartTimestamp(100)
	parent.SetEndTimestamp(200)

	// Child starts before and ends after the parent: only [100,200] counts.
	child := spans.AppendEmpty()
	child.SetTraceID(traceID)
	child.SetSpanID(pcommon.SpanID([8]byte{2}))
	child.SetParentSpanID(parent.SpanID())
	child.SetStartTimestamp(50)
	child.SetEndTimestamp(400)

	got := computeSelfTimeBySpanID(traces)
	assert.Equal(t, int64(0), got[parent.SpanID()])
	assert.Equal(t, int64(350), got[child.SpanID()])
}

func TestComputeSelfTime_DuplicateSpanIDsOmitTrace(t *testing.T) {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{4})
	dupID := pcommon.SpanID([8]byte{7})

	root := spans.AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(dupID)
	root.SetStartTimestamp(0)
	root.SetEndTimestamp(100)

	dup := spans.AppendEmpty()
	dup.SetTraceID(traceID)
	dup.SetSpanID(dupID)
	dup.SetStartTimestamp(10)
	dup.SetEndTimestamp(20)

	assert.Empty(t, computeSelfTimeBySpanID(traces))
}

func TestComputeSelfTime_OrphanFallbackStillComputes(t *testing.T) {
	// No span has an empty parent, so there is no nominal root to contradict
	// the unresolved parent. Parent-child edges are still exact.
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{5})

	orphan := spans.AppendEmpty()
	orphan.SetTraceID(traceID)
	orphan.SetSpanID(pcommon.SpanID([8]byte{1}))
	orphan.SetParentSpanID(pcommon.SpanID([8]byte{9})) // not in the batch
	orphan.SetStartTimestamp(0)
	orphan.SetEndTimestamp(100)

	child := spans.AppendEmpty()
	child.SetTraceID(traceID)
	child.SetSpanID(pcommon.SpanID([8]byte{2}))
	child.SetParentSpanID(orphan.SpanID())
	child.SetStartTimestamp(10)
	child.SetEndTimestamp(40)

	got := computeSelfTimeBySpanID(traces)
	assert.Equal(t, int64(70), got[orphan.SpanID()])
	assert.Equal(t, int64(30), got[child.SpanID()])
}

func TestComputeSelfTime_MultipleRootsInBatch(t *testing.T) {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{6})

	rootA := spans.AppendEmpty()
	rootA.SetTraceID(traceID)
	rootA.SetSpanID(pcommon.SpanID([8]byte{1}))
	rootA.SetStartTimestamp(0)
	rootA.SetEndTimestamp(100)

	rootB := spans.AppendEmpty()
	rootB.SetTraceID(traceID)
	rootB.SetSpanID(pcommon.SpanID([8]byte{2}))
	rootB.SetStartTimestamp(200)
	rootB.SetEndTimestamp(260)

	got := computeSelfTimeBySpanID(traces)
	assert.Equal(t, int64(100), got[rootA.SpanID()])
	assert.Equal(t, int64(60), got[rootB.SpanID()])
}

func TestSpanMetrics_SelfTimeUsesDurationDimensions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Histogram.Unit = metrics.Milliseconds
	cfg.SelfTime = SelfTimeConfig{Enabled: true}
	c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock(), instanceID)
	require.NoError(t, err)

	require.NoError(t, c.ConsumeTraces(t.Context(), buildOverlapSelfTimeTrace()))
	got := c.buildMetrics()

	metricsSlice := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	durationAttrs := histogramDataPointAttributes(t, metricsSlice, buildMetricName(DefaultNamespace, metricNameDuration))
	selfTimeAttrs := histogramDataPointAttributes(t, metricsSlice, buildMetricName(DefaultNamespace, metricNameSelfTime))

	// Data point order is not deterministic, so compare the attribute sets.
	assert.ElementsMatch(t, durationAttrs, selfTimeAttrs)
}

func TestSpanMetrics_SelfTimeWithDurationHistogramDisabled(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Histogram.Unit = metrics.Milliseconds
	cfg.Histogram.Disable = true
	cfg.SelfTime = SelfTimeConfig{Enabled: true}
	c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock(), instanceID)
	require.NoError(t, err)

	require.NoError(t, c.ConsumeTraces(t.Context(), buildOverlapSelfTimeTrace()))
	got := c.buildMetrics()

	metricsSlice := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	require.Equal(t, 2, metricsSlice.Len()) // calls + self_time, no duration

	var foundSelfTime bool
	for i := 0; i < metricsSlice.Len(); i++ {
		m := metricsSlice.At(i)
		require.NotEqual(t, buildMetricName(DefaultNamespace, metricNameDuration), m.Name())
		if m.Name() == buildMetricName(DefaultNamespace, metricNameSelfTime) {
			foundSelfTime = true
			assert.InDelta(t, 110.0, histogramSum(m), 0.001)
		}
	}
	assert.True(t, foundSelfTime)
}

func histogramDataPointAttributes(t *testing.T, metricsSlice pmetric.MetricSlice, name string) []map[string]string {
	t.Helper()
	var out []map[string]string
	for i := 0; i < metricsSlice.Len(); i++ {
		m := metricsSlice.At(i)
		if m.Name() != name || m.Type() != pmetric.MetricTypeHistogram {
			continue
		}
		dps := m.Histogram().DataPoints()
		for j := 0; j < dps.Len(); j++ {
			attrs := make(map[string]string)
			dps.At(j).Attributes().Range(func(k string, v pcommon.Value) bool {
				attrs[k] = v.AsString()
				return true
			})
			out = append(out, attrs)
		}
	}
	require.NotEmpty(t, out, "no histogram data points found for %s", name)
	return out
}

func buildOverlapSelfTimeTrace() ptrace.Traces {
	// parent [0,100ms], children [10,30]ms U [20,50]ms => parent self = 60ms
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "orders")
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{0x11})
	parentID := pcommon.SpanID([8]byte{0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa})
	childAID := pcommon.SpanID([8]byte{0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb})
	childBID := pcommon.SpanID([8]byte{0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc})
	base := time.Unix(0, 0).UTC()

	parent := spans.AppendEmpty()
	parent.SetName("GET /orders")
	parent.SetKind(ptrace.SpanKindServer)
	parent.Status().SetCode(ptrace.StatusCodeOk)
	parent.SetTraceID(traceID)
	parent.SetSpanID(parentID)
	parent.SetStartTimestamp(pcommon.NewTimestampFromTime(base))
	parent.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(100 * time.Millisecond)))

	childA := spans.AppendEmpty()
	childA.SetName("db.query")
	childA.SetKind(ptrace.SpanKindClient)
	childA.Status().SetCode(ptrace.StatusCodeOk)
	childA.SetTraceID(traceID)
	childA.SetSpanID(childAID)
	childA.SetParentSpanID(parentID)
	childA.SetStartTimestamp(pcommon.NewTimestampFromTime(base.Add(10 * time.Millisecond)))
	childA.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(30 * time.Millisecond)))

	childB := spans.AppendEmpty()
	childB.SetName("cache.get")
	childB.SetKind(ptrace.SpanKindClient)
	childB.Status().SetCode(ptrace.StatusCodeOk)
	childB.SetTraceID(traceID)
	childB.SetSpanID(childBID)
	childB.SetParentSpanID(parentID)
	childB.SetStartTimestamp(pcommon.NewTimestampFromTime(base.Add(20 * time.Millisecond)))
	childB.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(50 * time.Millisecond)))

	return traces
}

func buildPartialSelfTimeTrace() ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "orders")
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{0x22})

	root := spans.AppendEmpty()
	root.SetName("root")
	root.SetKind(ptrace.SpanKindServer)
	root.Status().SetCode(ptrace.StatusCodeOk)
	root.SetTraceID(traceID)
	root.SetSpanID(pcommon.SpanID([8]byte{1}))
	root.SetStartTimestamp(0)
	root.SetEndTimestamp(pcommon.Timestamp(100 * time.Millisecond))

	orphan := spans.AppendEmpty()
	orphan.SetName("orphan")
	orphan.SetKind(ptrace.SpanKindInternal)
	orphan.Status().SetCode(ptrace.StatusCodeOk)
	orphan.SetTraceID(traceID)
	orphan.SetSpanID(pcommon.SpanID([8]byte{2}))
	orphan.SetParentSpanID(pcommon.SpanID([8]byte{9})) // missing parent
	orphan.SetStartTimestamp(pcommon.Timestamp(10 * time.Millisecond))
	orphan.SetEndTimestamp(pcommon.Timestamp(50 * time.Millisecond))

	return traces
}

func histogramSum(m pmetric.Metric) float64 {
	switch m.Type() {
	case pmetric.MetricTypeHistogram:
		var sum float64
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			sum += dps.At(i).Sum()
		}
		return sum
	case pmetric.MetricTypeExponentialHistogram:
		var sum float64
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			sum += dps.At(i).Sum()
		}
		return sum
	default:
		return 0
	}
}
