// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

// metricID represents the minimum attributes that uniquely identifies a metric in our tests.
type metricID struct {
	service    string
	spanName   string
	kind       string
	statusCode string
}

type metricDataPoint interface {
	Attributes() pcommon.Map
}

func stringp(str string) *string {
	return &str
}

func TestConnectorConsumeTraces(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name     string
		verifier func(tb testing.TB, input pmetric.Metrics) bool
		traces   []ptrace.Traces
	}{
		{
			name:     "Test single consumption, three spans.",
			verifier: verifyConsumeMetricsInputCumulative,
			traces:   []ptrace.Traces{buildSampleTrace()},
		},
		{
			name:     "Test two consumptions",
			verifier: verifyMultipleCumulativeConsumptions(),
			traces:   []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// Consumptions with improper timestamps
			name:     "Test bad consumptions",
			verifier: verifyBadMetricsOkay,
			traces:   []ptrace.Traces{buildBadSampleTrace()},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			msink := &consumertest.MetricsSink{}

			p := newTestMetricsConnector(msink, stringp("defaultNullValue"), zaptest.NewLogger(t))

			ctx := metadata.NewIncomingContext(context.Background(), nil)
			err := p.Start(ctx, componenttest.NewNopHost())
			defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
			require.NoError(t, err)

			for _, traces := range tc.traces {
				err = p.ConsumeTraces(ctx, traces)
				assert.NoError(t, err)

				metrics := msink.AllMetrics()
				assert.NotEmpty(t, metrics)
				tc.verifier(t, metrics[len(metrics)-1])
			}
		})
	}
	t.Run("Test without exemplars", func(t *testing.T) {
		msink := &consumertest.MetricsSink{}

		p := newTestMetricsConnector(msink, stringp("defaultNullValue"), zaptest.NewLogger(t))
		p.config.Exemplars.Enabled = false

		ctx := metadata.NewIncomingContext(context.Background(), nil)
		err := p.Start(ctx, componenttest.NewNopHost())
		defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
		require.NoError(t, err)

		err = p.ConsumeTraces(ctx, buildBadSampleTrace())
		assert.NoError(t, err)

		metrics := msink.AllMetrics()
		assert.NotEmpty(t, metrics)
		verifyBadMetricsOkay(t, metrics[len(metrics)-1])
	})
}

func BenchmarkConnectorConsumeTraces(b *testing.B) {
	msink := &consumertest.MetricsSink{}

	conn := newTestMetricsConnector(msink, stringp("defaultNullValue"), zaptest.NewLogger(b))
	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	for n := 0; n < b.N; n++ {
		assert.NoError(b, conn.ConsumeTraces(ctx, traces))
	}
}

func newTestMetricsConnector(mcon consumer.Metrics, defaultNullValue *string, logger *zap.Logger) *metricsConnector {
	cfg := &Config{
		Dimensions: []Dimension{
			// Set nil defaults to force a lookup for the attribute in the span.
			{stringAttrName, nil},
			{intAttrName, nil},
			{doubleAttrName, nil},
			{boolAttrName, nil},
			{mapAttrName, nil},
			{arrayAttrName, nil},
			{nullAttrName, defaultNullValue},
			// Add a default value for an attribute that doesn't exist in a span
			{notInSpanAttrName0, stringp("defaultNotInSpanAttrVal")},
			// Leave the default value unset to test that this dimension should not be added to the metric.
			{notInSpanAttrName1, nil},

			// Exception specific dimensions
			{exceptionTypeKey, nil},
			{exceptionMessageKey, nil},
		},
		Exemplars: Exemplars{
			Enabled: true,
		},
	}
	c := newMetricsConnector(logger, cfg)
	c.metricsConsumer = mcon
	return c
}

// verifyConsumeMetricsInputCumulative expects one accumulation of metrics, and marked as cumulative
func verifyConsumeMetricsInputCumulative(tb testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(tb, input, 1)
}

func verifyBadMetricsOkay(_ testing.TB, _ pmetric.Metrics) bool {
	return true // Validating no exception
}

// verifyMultipleCumulativeConsumptions expects the amount of accumulations as kept track of by numCumulativeConsumptions.
// numCumulativeConsumptions acts as a multiplier for the values, since the cumulative metrics are additive.
func verifyMultipleCumulativeConsumptions() func(tb testing.TB, input pmetric.Metrics) bool {
	numCumulativeConsumptions := 0
	return func(tb testing.TB, input pmetric.Metrics) bool {
		numCumulativeConsumptions++
		return verifyConsumeMetricsInput(tb, input, numCumulativeConsumptions)
	}
}

// verifyConsumeMetricsInput verifies the input of the ConsumeMetrics call from this connector.
// This is the best point to verify the computed metrics from spans are as expected.
func verifyConsumeMetricsInput(tb testing.TB, input pmetric.Metrics, numCumulativeConsumptions int) bool {
	require.Equal(tb, 3, input.DataPointCount(), "Should be 1 for each generated span")

	rm := input.ResourceMetrics()
	require.Equal(tb, 1, rm.Len())

	ilm := rm.At(0).ScopeMetrics()
	require.Equal(tb, 1, ilm.Len())
	assert.Equal(tb, "exceptionsconnector", ilm.At(0).Scope().Name())

	m := ilm.At(0).Metrics()
	require.Equal(tb, 1, m.Len())

	seenMetricIDs := make(map[metricID]bool)
	// The first 3 data points are for call counts.
	assert.Equal(tb, "exceptions", m.At(0).Name())
	assert.True(tb, m.At(0).Sum().IsMonotonic())
	callsDps := m.At(0).Sum().DataPoints()
	require.Equal(tb, 3, callsDps.Len())
	for dpi := 0; dpi < 3; dpi++ {
		dp := callsDps.At(dpi)
		assert.Equal(tb, int64(numCumulativeConsumptions), dp.IntValue(), "There should only be one metric per Service/kind combination")
		assert.NotZero(tb, dp.StartTimestamp(), "StartTimestamp should be set")
		assert.NotZero(tb, dp.Timestamp(), "Timestamp should be set")
		verifyMetricLabels(tb, dp, seenMetricIDs)

		assert.Equal(tb, 1, dp.Exemplars().Len())
		exemplar := dp.Exemplars().At(0)
		assert.NotZero(tb, exemplar.Timestamp())
		assert.NotZero(tb, exemplar.TraceID())
		assert.NotZero(tb, exemplar.SpanID())
	}
	return true
}

func verifyMetricLabels(tb testing.TB, dp metricDataPoint, seenMetricIDs map[metricID]bool) {
	mID := metricID{}
	wantDimensions := map[string]pcommon.Value{
		stringAttrName:      pcommon.NewValueStr("stringAttrValue"),
		intAttrName:         pcommon.NewValueInt(99),
		doubleAttrName:      pcommon.NewValueDouble(99.99),
		boolAttrName:        pcommon.NewValueBool(true),
		nullAttrName:        pcommon.NewValueEmpty(),
		arrayAttrName:       pcommon.NewValueSlice(),
		mapAttrName:         pcommon.NewValueMap(),
		notInSpanAttrName0:  pcommon.NewValueStr("defaultNotInSpanAttrVal"),
		exceptionTypeKey:    pcommon.NewValueStr("Exception"),
		exceptionMessageKey: pcommon.NewValueStr("Exception message"),
	}
	for k, v := range dp.Attributes().All() {
		switch k {
		case serviceNameKey:
			mID.service = v.Str()
		case spanNameKey:
			mID.spanName = v.Str()
		case spanKindKey:
			mID.kind = v.Str()
		case statusCodeKey:
			mID.statusCode = v.Str()
		case notInSpanAttrName1:
			assert.Fail(tb, notInSpanAttrName1+" should not be in this metric")
		default:
			assert.Equal(tb, wantDimensions[k], v)
			delete(wantDimensions, k)
		}
	}
	assert.Empty(tb, wantDimensions, "Did not see all expected dimensions in metric. Missing: ", wantDimensions)

	// Service/kind should be a unique metric.
	assert.False(tb, seenMetricIDs[mID])
	seenMetricIDs[mID] = true
}

func buildBadSampleTrace() ptrace.Traces {
	badTrace := buildSampleTrace()
	span := badTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	now := time.Now()
	// Flipping timestamp for a bad duration
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(sampleLatencyDuration)))
	return badTrace
}

func TestBuildKeySameServiceOperationCharSequence(t *testing.T) {
	span0 := ptrace.NewSpan()
	span0.SetName("c")
	buf := &bytes.Buffer{}
	buildKey(buf, "ab", span0, nil, pcommon.NewMap(), pcommon.NewMap())
	k0 := buf.String()
	buf.Reset()
	span1 := ptrace.NewSpan()
	span1.SetName("bc")
	buildKey(buf, "a", span1, nil, pcommon.NewMap(), pcommon.NewMap())
	k1 := buf.String()
	assert.NotEqual(t, k0, k1)
	assert.Equal(t, "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET", k0)
	assert.Equal(t, "a\u0000bc\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET", k1)
}

func TestBuildKeyWithDimensions(t *testing.T) {
	defaultFoo := pcommon.NewValueStr("bar")
	for _, tc := range []struct {
		name            string
		optionalDims    []pdatautil.Dimension
		resourceAttrMap map[string]any
		spanAttrMap     map[string]any
		wantKey         string
	}{
		{
			name:    "nil optionalDims",
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "neither span nor resource contains key, dim provides default",
			optionalDims: []pdatautil.Dimension{
				{Name: "foo", Value: &defaultFoo},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000bar",
		},
		{
			name: "neither span nor resource contains key, dim provides no default",
			optionalDims: []pdatautil.Dimension{
				{Name: "foo"},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "span attribute contains dimension",
			optionalDims: []pdatautil.Dimension{
				{Name: "foo"},
			},
			spanAttrMap: map[string]any{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "resource attribute contains dimension",
			optionalDims: []pdatautil.Dimension{
				{Name: "foo"},
			},
			resourceAttrMap: map[string]any{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "both span and resource attribute contains dimension, should prefer span attribute",
			optionalDims: []pdatautil.Dimension{
				{Name: "foo"},
			},
			spanAttrMap: map[string]any{
				"foo": 100,
			},
			resourceAttrMap: map[string]any{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000100",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resAttr := pcommon.NewMap()
			assert.NoError(t, resAttr.FromRaw(tc.resourceAttrMap))
			span0 := ptrace.NewSpan()
			assert.NoError(t, span0.Attributes().FromRaw(tc.spanAttrMap))
			span0.SetName("c")
			buf := &bytes.Buffer{}
			buildKey(buf, "ab", span0, tc.optionalDims, pcommon.NewMap(), resAttr)
			assert.Equal(t, tc.wantKey, buf.String())
		})
	}
}
