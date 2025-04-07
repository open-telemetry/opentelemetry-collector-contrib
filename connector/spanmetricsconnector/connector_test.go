// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/lightstep/go-expohisto/structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

const (
	stringAttrName           = "stringAttrName"
	intAttrName              = "intAttrName"
	doubleAttrName           = "doubleAttrName"
	boolAttrName             = "boolAttrName"
	nullAttrName             = "nullAttrName"
	mapAttrName              = "mapAttrName"
	arrayAttrName            = "arrayAttrName"
	notInSpanAttrName0       = "shouldBeInMetric"
	notInSpanAttrName1       = "shouldNotBeInMetric"
	regionResourceAttrName   = "region"
	exceptionTypeAttrName    = "exception.type"
	dimensionsCacheSize      = 2
	resourceMetricsCacheSize = 5

	sampleRegion   = "us-east-1"
	sampleDuration = float64(11)
)

// metricID represents the minimum attributes that uniquely identifies a metric in our tests.
type metricID struct {
	service    string
	name       string
	kind       string
	statusCode string
}

type metricDataPoint interface {
	Attributes() pcommon.Map
}

type serviceSpans struct {
	serviceName string
	spans       []span
}

type span struct {
	name       string
	kind       ptrace.SpanKind
	statusCode ptrace.StatusCode
	traceID    [16]byte
	spanID     [8]byte
}

// verifyDisabledHistogram expects that histograms are disabled.
func verifyDisabledHistogram(tb testing.TB, input pmetric.Metrics) bool {
	for i := 0; i < input.ResourceMetrics().Len(); i++ {
		rm := input.ResourceMetrics().At(i)
		ism := rm.ScopeMetrics()
		// Checking all metrics, naming notice: ismC/mC - C here is for Counter.
		for ismC := 0; ismC < ism.Len(); ismC++ {
			m := ism.At(ismC).Metrics()
			for mC := 0; mC < m.Len(); mC++ {
				metric := m.At(mC)
				assert.NotEqual(tb, pmetric.MetricTypeExponentialHistogram, metric.Type())
				assert.NotEqual(tb, pmetric.MetricTypeHistogram, metric.Type())
			}
		}
	}
	return true
}

func verifyExemplarsExist(tb testing.TB, input pmetric.Metrics) bool {
	for i := 0; i < input.ResourceMetrics().Len(); i++ {
		rm := input.ResourceMetrics().At(i)
		ism := rm.ScopeMetrics()

		// Checking all metrics, naming notice: ismC/mC - C here is for Counter.
		for ismC := 0; ismC < ism.Len(); ismC++ {
			m := ism.At(ismC).Metrics()

			for mC := 0; mC < m.Len(); mC++ {
				metric := m.At(mC)

				if metric.Type() != pmetric.MetricTypeHistogram {
					continue
				}
				dps := metric.Histogram().DataPoints()
				for dp := 0; dp < dps.Len(); dp++ {
					d := dps.At(dp)
					assert.Positive(tb, d.Exemplars().Len())
				}
			}
		}
	}
	return true
}

// verifyConsumeMetricsInputCumulative expects one accumulation of metrics, and marked as cumulative
func verifyConsumeMetricsInputCumulative(tb testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(tb, input, pmetric.AggregationTemporalityCumulative, 1)
}

func verifyBadMetricsOkay(_ testing.TB, _ pmetric.Metrics) bool {
	return true // Validating no exception
}

// verifyConsumeMetricsInputDelta expects one accumulation of metrics, and marked as delta
func verifyConsumeMetricsInputDelta(tb testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(tb, input, pmetric.AggregationTemporalityDelta, 1)
}

// verifyMultipleCumulativeConsumptions expects the amount of accumulations as kept track of by numCumulativeConsumptions.
// numCumulativeConsumptions acts as a multiplier for the values, since the cumulative metrics are additive.
func verifyMultipleCumulativeConsumptions() func(tb testing.TB, input pmetric.Metrics) bool {
	numCumulativeConsumptions := 0
	return func(tb testing.TB, input pmetric.Metrics) bool {
		numCumulativeConsumptions++
		return verifyConsumeMetricsInput(tb, input, pmetric.AggregationTemporalityCumulative, numCumulativeConsumptions)
	}
}

// verifyConsumeMetricsInput verifies the input of the ConsumeMetrics call from this connector.
// This is the best point to verify the computed metrics from spans are as expected.
func verifyConsumeMetricsInput(tb testing.TB, input pmetric.Metrics, expectedTemporality pmetric.AggregationTemporality, numCumulativeConsumptions int) bool {
	require.Equal(tb, 6, input.DataPointCount(),
		"Should be 3 for each of call count and latency split into two resource scopes defined by: "+
			"service-a: service-a (server kind) -> service-a (client kind) and "+
			"service-b: service-b (service kind)",
	)

	require.Equal(tb, 2, input.ResourceMetrics().Len())

	for i := 0; i < input.ResourceMetrics().Len(); i++ {
		rm := input.ResourceMetrics().At(i)

		var numDataPoints int
		val, ok := rm.Resource().Attributes().Get(serviceNameKey)
		require.True(tb, ok)
		serviceName := val.AsString()
		switch serviceName {
		case "service-a":
			numDataPoints = 2
		case "service-b":
			numDataPoints = 1
		}

		ilm := rm.ScopeMetrics()
		require.Equal(tb, 1, ilm.Len())
		assert.Equal(tb, "spanmetricsconnector", ilm.At(0).Scope().Name())

		m := ilm.At(0).Metrics()
		require.Equal(tb, 2, m.Len(), "only sum and histogram metric types generated")

		// validate calls - sum metrics
		metric := m.At(0)
		assert.Equal(tb, metricNameCalls, metric.Name())
		assert.Equal(tb, expectedTemporality, metric.Sum().AggregationTemporality())
		assert.True(tb, metric.Sum().IsMonotonic())

		seenMetricIDs := make(map[metricID]bool)
		callsDps := metric.Sum().DataPoints()
		require.Equal(tb, numDataPoints, callsDps.Len())
		for dpi := 0; dpi < numDataPoints; dpi++ {
			dp := callsDps.At(dpi)
			assert.Equal(tb,
				int64(numCumulativeConsumptions),
				dp.IntValue(),
				"There should only be one metric per Service/name/kind combination",
			)
			assert.NotZero(tb, dp.StartTimestamp(), "StartTimestamp should be set")
			assert.NotZero(tb, dp.Timestamp(), "Timestamp should be set")
			verifyMetricLabels(tb, dp, seenMetricIDs)
		}

		// validate latency - histogram metrics
		metric = m.At(1)
		assert.Equal(tb, metricNameDuration, metric.Name())
		assert.Equal(tb, defaultUnit.String(), metric.Unit())

		if metric.Type() == pmetric.MetricTypeExponentialHistogram {
			hist := metric.ExponentialHistogram()
			assert.Equal(tb, expectedTemporality, hist.AggregationTemporality())
			verifyExponentialHistogramDataPoints(tb, hist.DataPoints(), numDataPoints, numCumulativeConsumptions)
		} else {
			hist := metric.Histogram()
			assert.Equal(tb, expectedTemporality, hist.AggregationTemporality())
			verifyExplicitHistogramDataPoints(tb, hist.DataPoints(), numDataPoints, numCumulativeConsumptions)
		}
	}
	return true
}

func verifyExplicitHistogramDataPoints(tb testing.TB, dps pmetric.HistogramDataPointSlice, numDataPoints, numCumulativeConsumptions int) {
	seenMetricIDs := make(map[metricID]bool)
	require.Equal(tb, numDataPoints, dps.Len())
	for dpi := 0; dpi < numDataPoints; dpi++ {
		dp := dps.At(dpi)
		assert.Equal(
			tb,
			sampleDuration*float64(numCumulativeConsumptions),
			dp.Sum(),
			"Should be a 11ms duration measurement, multiplied by the number of stateful accumulations.")
		assert.NotZero(tb, dp.Timestamp(), "Timestamp should be set")

		// Verify bucket counts.

		// The bucket counts should be 1 greater than the explicit bounds as documented in:
		// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto.
		assert.Equal(tb, dp.ExplicitBounds().Len()+1, dp.BucketCounts().Len())

		// Find the bucket index where the 11ms duration should belong in.
		var foundDurationIndex int
		for foundDurationIndex = 0; foundDurationIndex < dp.ExplicitBounds().Len(); foundDurationIndex++ {
			if dp.ExplicitBounds().At(foundDurationIndex) > sampleDuration {
				break
			}
		}

		// Then verify that all histogram buckets are empty except for the bucket with the 11ms duration.
		var wantBucketCount uint64
		for bi := 0; bi < dp.BucketCounts().Len(); bi++ {
			wantBucketCount = 0
			if bi == foundDurationIndex {
				wantBucketCount = uint64(numCumulativeConsumptions)
			}
			assert.Equal(tb, wantBucketCount, dp.BucketCounts().At(bi))
		}
		verifyMetricLabels(tb, dp, seenMetricIDs)
	}
}

func verifyExponentialHistogramDataPoints(tb testing.TB, dps pmetric.ExponentialHistogramDataPointSlice, numDataPoints, numCumulativeConsumptions int) {
	seenMetricIDs := make(map[metricID]bool)
	require.Equal(tb, numDataPoints, dps.Len())
	for dpi := 0; dpi < numDataPoints; dpi++ {
		dp := dps.At(dpi)
		assert.Equal(
			tb,
			sampleDuration*float64(numCumulativeConsumptions),
			dp.Sum(),
			"Should be a 11ms duration measurement, multiplied by the number of stateful accumulations.")
		assert.Equal(tb, uint64(numCumulativeConsumptions), dp.Count())
		assert.Equal(tb, []uint64{uint64(numCumulativeConsumptions)}, dp.Positive().BucketCounts().AsRaw())
		assert.NotZero(tb, dp.Timestamp(), "Timestamp should be set")

		verifyMetricLabels(tb, dp, seenMetricIDs)
	}
}

func verifyMetricLabels(tb testing.TB, dp metricDataPoint, seenMetricIDs map[metricID]bool) {
	mID := metricID{}
	wantDimensions := map[string]pcommon.Value{
		stringAttrName:         pcommon.NewValueStr("stringAttrValue"),
		intAttrName:            pcommon.NewValueInt(99),
		doubleAttrName:         pcommon.NewValueDouble(99.99),
		boolAttrName:           pcommon.NewValueBool(true),
		nullAttrName:           pcommon.NewValueEmpty(),
		arrayAttrName:          pcommon.NewValueSlice(),
		mapAttrName:            pcommon.NewValueMap(),
		notInSpanAttrName0:     pcommon.NewValueStr("defaultNotInSpanAttrVal"),
		regionResourceAttrName: pcommon.NewValueStr(sampleRegion),
	}
	for k, v := range dp.Attributes().All() {
		switch k {
		case serviceNameKey:
			mID.service = v.Str()
		case spanNameKey:
			mID.name = v.Str()
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

	// Service/name/kind should be a unique metric.
	assert.False(tb, seenMetricIDs[mID])
	seenMetricIDs[mID] = true
}

func buildBadSampleTrace() ptrace.Traces {
	badTrace := buildSampleTrace()
	span := badTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	now := time.Now()
	// Flipping timestamp for a bad duration
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetStartTimestamp(
		pcommon.NewTimestampFromTime(now.Add(time.Duration(sampleDuration) * time.Millisecond)))
	return badTrace
}

// buildSampleTrace builds the following trace:
//
//	service-a/ping (server) ->
//	  service-a/ping (client) ->
//	    service-b/ping (server)
func buildSampleTrace() ptrace.Traces {
	traces := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
					traceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
					spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
				},
				{
					name:       "/ping",
					kind:       ptrace.SpanKindClient,
					statusCode: ptrace.StatusCodeOk,
					traceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
					spanID:     [8]byte{0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x10},
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
					traceID:    [16]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x10},
					spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(serviceSpans{}, traces.ResourceSpans().AppendEmpty())
	return traces
}

func initServiceSpans(serviceSpans serviceSpans, spans ptrace.ResourceSpans) {
	if serviceSpans.serviceName != "" {
		spans.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceSpans.serviceName)
	}

	spans.Resource().Attributes().PutStr(regionResourceAttrName, sampleRegion)

	ils := spans.ScopeSpans().AppendEmpty()
	for _, span := range serviceSpans.spans {
		initSpan(span, ils.Spans().AppendEmpty())
	}
}

func initSpan(span span, s ptrace.Span) {
	s.SetName(span.name)
	s.SetKind(span.kind)
	s.Status().SetCode(span.statusCode)
	now := time.Now()
	s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	s.SetEndTimestamp(
		pcommon.NewTimestampFromTime(now.Add(time.Duration(sampleDuration) * time.Millisecond)))

	s.Attributes().PutStr(stringAttrName, "stringAttrValue")
	s.Attributes().PutInt(intAttrName, 99)
	s.Attributes().PutDouble(doubleAttrName, 99.99)
	s.Attributes().PutBool(boolAttrName, true)
	s.Attributes().PutEmpty(nullAttrName)
	s.Attributes().PutEmptyMap(mapAttrName)
	s.Attributes().PutEmptySlice(arrayAttrName)
	s.SetTraceID(pcommon.TraceID(span.traceID))
	s.SetSpanID(pcommon.SpanID(span.spanID))

	e := s.Events().AppendEmpty()
	e.SetName("exception")
	e.Attributes().PutStr(exceptionTypeAttrName, "NullPointerException")
}

func disabledExemplarsConfig() ExemplarsConfig {
	return ExemplarsConfig{
		Enabled: false,
	}
}

func enabledExemplarsConfig() ExemplarsConfig {
	return ExemplarsConfig{
		Enabled: true,
	}
}

func enabledEventsConfig() EventsConfig {
	return EventsConfig{
		Enabled:    true,
		Dimensions: []Dimension{{Name: "exception.type"}},
	}
}

func disabledEventsConfig() EventsConfig {
	return EventsConfig{
		Enabled: false,
	}
}

func explicitHistogramsConfig() HistogramConfig {
	return HistogramConfig{
		Unit: defaultUnit,
		Explicit: &ExplicitHistogramConfig{
			Buckets: []time.Duration{4 * time.Second, 6 * time.Second, 8 * time.Second},
		},
	}
}

func exponentialHistogramsConfig() HistogramConfig {
	return HistogramConfig{
		Unit: defaultUnit,
		Exponential: &ExponentialHistogramConfig{
			MaxSize: 10,
		},
	}
}

func disabledHistogramsConfig() HistogramConfig {
	return HistogramConfig{
		Unit:    defaultUnit,
		Disable: true,
	}
}

func newConnectorImp(defaultNullValue *string, histogramConfig func() HistogramConfig, exemplarsConfig func() ExemplarsConfig, eventsConfig func() EventsConfig, temporality string, expiration time.Duration, resourceMetricsKeyAttributes []string, deltaTimestampCacheSize int, clock clockwork.Clock, excludedDimensions ...string) (*connectorImp, error) {
	cfg := &Config{
		AggregationTemporality:       temporality,
		Histogram:                    histogramConfig(),
		Exemplars:                    exemplarsConfig(),
		ExcludeDimensions:            excludedDimensions,
		DimensionsCacheSize:          dimensionsCacheSize,
		ResourceMetricsCacheSize:     resourceMetricsCacheSize,
		ResourceMetricsKeyAttributes: resourceMetricsKeyAttributes,
		Dimensions: []Dimension{
			// Set nil defaults to force a lookup for the attribute in the span.
			{Name: stringAttrName, Default: nil},
			{Name: intAttrName, Default: nil},
			{Name: doubleAttrName, Default: nil},
			{Name: boolAttrName, Default: nil},
			{Name: mapAttrName, Default: nil},
			{Name: arrayAttrName, Default: nil},
			{Name: nullAttrName, Default: defaultNullValue},
			// Add a default value for an attribute that doesn't exist in a span
			{Name: notInSpanAttrName0, Default: stringp("defaultNotInSpanAttrVal")},
			// Leave the default value unset to test that this dimension should not be added to the metric.
			{Name: notInSpanAttrName1, Default: nil},
			// Add a resource attribute to test "process" attributes like IP, host, region, cluster, etc.
			{Name: regionResourceAttrName, Default: nil},
		},
		Events:               eventsConfig(),
		MetricsExpiration:    expiration,
		TimestampCacheSize:   &deltaTimestampCacheSize,
		MetricsFlushInterval: time.Nanosecond,
	}

	c, err := newConnector(zap.NewNop(), cfg, clock)
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = consumertest.NewNop()
	return c, nil
}

func stringp(str string) *string {
	return &str
}

func TestBuildKeySameServiceNameCharSequence(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock())
	require.NoError(t, err)

	span0 := ptrace.NewSpan()
	span0.SetName("c")
	k0 := c.buildKey("ab", span0, nil, pcommon.NewMap())

	span1 := ptrace.NewSpan()
	span1.SetName("bc")
	k1 := c.buildKey("a", span1, nil, pcommon.NewMap())

	assert.NotEqual(t, k0, k1)
	assert.Equal(t, metrics.Key("ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k0)
	assert.Equal(t, metrics.Key("a\u0000bc\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k1)
}

func TestBuildKeyExcludeDimensionsAll(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ExcludeDimensions = []string{"span.kind", "service.name", "span.name", "status.code"}
	c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock())
	require.NoError(t, err)

	span0 := ptrace.NewSpan()
	span0.SetName("spanName")
	k0 := c.buildKey("serviceName", span0, nil, pcommon.NewMap())
	assert.Equal(t, metrics.Key(""), k0)
}

func TestBuildKeyExcludeWrongDimensions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ExcludeDimensions = []string{"span.kind", "service.name.wrong.name", "span.name", "status.code"}
	c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock())
	require.NoError(t, err)

	span0 := ptrace.NewSpan()
	span0.SetName("spanName")
	k0 := c.buildKey("serviceName", span0, nil, pcommon.NewMap())
	assert.Equal(t, metrics.Key("serviceName"), k0)
}

func TestBuildKeyWithDimensions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock())
	require.NoError(t, err)

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
			key := c.buildKey("ab", span0, tc.optionalDims, resAttr)
			assert.Equal(t, metrics.Key(tc.wantKey), key)
		})
	}
}

func TestStart(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	createParams := connectortest.NewNopSettings(factory.Type())
	conn, err := factory.CreateTracesToMetrics(context.Background(), createParams, cfg, consumertest.NewNop())
	require.NoError(t, err)

	smc := conn.(*connectorImp)
	ctx := context.Background()
	err = smc.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := smc.Shutdown(ctx); require.NoError(t, sdErr) }()
	assert.NoError(t, err)
}

func TestConcurrentShutdown(t *testing.T) {
	// Prepare
	ctx := context.Background()
	core, observedLogs := observer.New(zapcore.InfoLevel)

	// Test
	p, err := newConnectorImp(nil, explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, []string{}, 1000, clockwork.NewFakeClock())
	require.NoError(t, err)
	// Override the default no-op consumer and logger for testing.
	p.metricsConsumer = new(consumertest.MetricsSink)
	p.logger = zap.New(core)
	err = p.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Allow goroutines time to start.
	time.Sleep(time.Millisecond)

	// Simulate many goroutines trying to concurrently shutdown.
	var wg sync.WaitGroup
	const concurrency = 1000
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			err := p.Shutdown(ctx)
			assert.NoError(t, err)
			wg.Done()
		}()
	}
	wg.Wait()

	// Allow time for log observer to sync all logs emitted.
	// Even though the WaitGroup has been given the "done" signal, there's still a potential race condition
	// between the WaitGroup being unblocked and when the logs will be flushed.
	var allLogs []observer.LoggedEntry
	assert.Eventually(t, func() bool {
		allLogs = observedLogs.All()
		return len(allLogs) > 0
	}, time.Second, time.Millisecond*10)

	// Starting spanmetricsconnector...
	// Shutting down spanmetricsconnector...
	// Stopping ticker.
	assert.Len(t, allLogs, 3)
}

func TestConnectorCapabilities(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock())
	// Override the default no-op consumer for testing.
	c.metricsConsumer = new(consumertest.MetricsSink)
	assert.NoError(t, err)
	caps := c.Capabilities()

	// Verify
	assert.NotNil(t, c)
	assert.False(t, caps.MutatesData)
}

type errConsumer struct {
	wg      *sync.WaitGroup
	fakeErr error
}

func (e *errConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *errConsumer) ConsumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	e.wg.Done()
	return e.fakeErr
}

func TestConsumeMetricsErrors(t *testing.T) {
	// Prepare
	fakeErr := fmt.Errorf("consume metrics error")

	core, observedLogs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	var wg sync.WaitGroup
	mockClock := clockwork.NewFakeClock()
	p, err := newConnectorImp(nil, explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, []string{}, 1000, mockClock)
	require.NoError(t, err)
	// Override the default no-op consumer and logger for testing.
	p.metricsConsumer = &errConsumer{
		wg:      &wg,
		fakeErr: fakeErr,
	}
	p.logger = logger

	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err = p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	traces := buildSampleTrace()

	// Test
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// Trigger flush.
	wg.Add(1)
	mockClock.Advance(time.Nanosecond)
	wg.Wait()

	// Allow time for log observer to sync all logs emitted.
	// Even though the WaitGroup has been given the "done" signal, there's still a potential race condition
	// between the WaitGroup being unblocked and when the logs will be flushed.
	var allLogs []observer.LoggedEntry
	assert.Eventually(t, func() bool {
		allLogs = observedLogs.All()
		return len(allLogs) > 0
	}, time.Second, time.Millisecond*10)

	assert.Equal(t, "Failed ConsumeMetrics", allLogs[0].Message)
}

func TestConsumeTraces(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name                   string
		aggregationTemporality string
		histogramConfig        func() HistogramConfig
		exemplarConfig         func() ExemplarsConfig
		verifier               func(tb testing.TB, input pmetric.Metrics) bool
		traces                 []ptrace.Traces
	}{
		// disabling histogram
		{
			name:                   "Test histogram metrics are disabled",
			aggregationTemporality: cumulative,
			histogramConfig:        disabledHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyDisabledHistogram,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		// exponential buckets histogram
		{
			name:                   "Test single consumption, three spans (Cumulative), using exp. histogram",
			aggregationTemporality: cumulative,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyConsumeMetricsInputCumulative,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta), using exp. histogram",
			aggregationTemporality: delta,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative), using exp. histogram",
			aggregationTemporality: cumulative,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyMultipleCumulativeConsumptions(),
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta), using exp. histogram",
			aggregationTemporality: delta,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// Consumptions with improper timestamps
			name:                   "Test bad consumptions (Delta), using exp. histogram",
			aggregationTemporality: cumulative,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyBadMetricsOkay,
			traces:                 []ptrace.Traces{buildBadSampleTrace()},
		},

		// explicit buckets histogram
		{
			name:                   "Test single consumption, three spans (Cumulative).",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyConsumeMetricsInputCumulative,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta).",
			aggregationTemporality: delta,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative).",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyMultipleCumulativeConsumptions(),
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta).",
			aggregationTemporality: delta,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// Consumptions with improper timestamps
			name:                   "Test bad consumptions (Delta).",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarsConfig,
			verifier:               verifyBadMetricsOkay,
			traces:                 []ptrace.Traces{buildBadSampleTrace()},
		},
		// enabling exemplars
		{
			name:                   "Test Exemplars are enabled",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         enabledExemplarsConfig,
			verifier:               verifyExemplarsExist,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare

			mcon := &consumertest.MetricsSink{}
			mockClock := clockwork.NewFakeClock()
			p, err := newConnectorImp(stringp("defaultNullValue"), tc.histogramConfig, tc.exemplarConfig, disabledEventsConfig, tc.aggregationTemporality, 0, []string{}, 1000, mockClock)
			require.NoError(t, err)
			// Override the default no-op consumer with metrics sink for testing.
			p.metricsConsumer = mcon

			ctx := metadata.NewIncomingContext(context.Background(), nil)
			err = p.Start(ctx, componenttest.NewNopHost())
			defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
			require.NoError(t, err)

			for _, traces := range tc.traces {
				// Test
				err = p.ConsumeTraces(ctx, traces)
				assert.NoError(t, err)

				// Trigger flush.
				mockClock.Advance(time.Nanosecond)
				require.Eventually(t, func() bool {
					return len(mcon.AllMetrics()) > 0
				}, 1*time.Second, 10*time.Millisecond)
				tc.verifier(t, mcon.AllMetrics()[len(mcon.AllMetrics())-1])
			}
		})
	}
}

func TestMetricKeyCache(t *testing.T) {
	p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, []string{}, 1000, clockwork.NewFakeClock())
	require.NoError(t, err)
	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// 0 key was cached at beginning
	assert.Zero(t, p.metricKeyToDimensions.Len())

	err = p.ConsumeTraces(ctx, traces)
	// Validate
	require.NoError(t, err)
	// 2 key was cached, 1 key was evicted and cleaned after the processing
	assert.Eventually(t, func() bool {
		return p.metricKeyToDimensions.Len() == dimensionsCacheSize
	}, 10*time.Second, time.Millisecond*100)

	// consume another batch of traces
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// 2 key was cached, other keys were evicted and cleaned after the processing
	assert.Eventually(t, func() bool {
		return p.metricKeyToDimensions.Len() == dimensionsCacheSize
	}, 10*time.Second, time.Millisecond*100)
}

func TestResourceMetricsCache(t *testing.T) {
	p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, []string{}, 1000, clockwork.NewFakeClock())
	require.NoError(t, err)

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// 0 resources in the beginning
	assert.Zero(t, p.resourceMetrics.Len())

	err = p.ConsumeTraces(ctx, buildSampleTrace())
	// Validate
	require.NoError(t, err)
	assert.Equal(t, 2, p.resourceMetrics.Len())

	// consume another batch of traces for the same resources
	err = p.ConsumeTraces(ctx, buildSampleTrace())
	require.NoError(t, err)
	assert.Equal(t, 2, p.resourceMetrics.Len())

	// consume more batches for new resources. Max size is exceeded causing old resource entries to be discarded
	for i := 0; i < resourceMetricsCacheSize; i++ {
		traces := buildSampleTrace()

		// add resource attributes to simulate additional resources providing data
		for j := 0; j < traces.ResourceSpans().Len(); j++ {
			traces.ResourceSpans().At(j).Resource().Attributes().PutStr("dummy", fmt.Sprintf("%d", i))
		}

		err = p.ConsumeTraces(ctx, traces)
		require.NoError(t, err)
	}

	// validate that the cache doesn't grow past its limit
	assert.Equal(t, resourceMetricsCacheSize, p.resourceMetrics.Len())
}

func TestResourceMetricsExpiration(t *testing.T) {
	p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 1*time.Millisecond, []string{}, 1000, clockwork.NewFakeClock())
	require.NoError(t, err)

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// 0 resources in the beginning
	assert.Zero(t, p.resourceMetrics.Len())

	err = p.ConsumeTraces(ctx, buildSampleTrace())
	// Validate
	require.NoError(t, err)
	assert.Equal(t, 2, p.resourceMetrics.Len())

	// consume another batch of traces for the same resources
	err = p.ConsumeTraces(ctx, buildSampleTrace())
	require.NoError(t, err)
	assert.Equal(t, 2, p.resourceMetrics.Len())
}

func TestResourceMetricsKeyAttributes(t *testing.T) {
	resourceMetricsKeyAttributes := []string{
		"service.name",
	}

	p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, resourceMetricsKeyAttributes, 1000, clockwork.NewFakeClock())
	require.NoError(t, err)

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// 0 resources in the beginning
	assert.Zero(t, p.resourceMetrics.Len())

	err = p.ConsumeTraces(ctx, buildSampleTrace())
	// Validate
	require.NoError(t, err)
	assert.Equal(t, 2, p.resourceMetrics.Len())

	// consume another batch of traces for the same resources
	err = p.ConsumeTraces(ctx, buildSampleTrace())
	require.NoError(t, err)
	assert.Equal(t, 2, p.resourceMetrics.Len())

	// consume more batches for new resources. Max size is exceeded causing old resource entries to be discarded
	for i := 0; i < resourceMetricsCacheSize; i++ {
		traces := buildSampleTrace()

		// add resource attributes to simulate additional resources providing data
		for j := 0; j < traces.ResourceSpans().Len(); j++ {
			traces.ResourceSpans().At(j).Resource().Attributes().PutStr("not included in resource key attributes", fmt.Sprintf("%d", i))
		}

		err = p.ConsumeTraces(ctx, traces)
		require.NoError(t, err)
	}

	// validate that the additional resources providing data did not result in additional resource metrics
	assert.Equal(t, 2, p.resourceMetrics.Len())
}

func BenchmarkConnectorConsumeTraces(b *testing.B) {
	// Prepare
	conn, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, []string{}, 1000, clockwork.NewFakeClock())
	require.NoError(b, err)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	for n := 0; n < b.N; n++ {
		assert.NoError(b, conn.ConsumeTraces(ctx, traces))
	}
}

func TestExcludeDimensionsConsumeTraces(t *testing.T) {
	testcases := []struct {
		dsc                string
		featureGateEnabled bool
	}{
		{
			dsc:                fmt.Sprintf("%s enabled", legacyMetricNamesFeatureGateID),
			featureGateEnabled: true,
		},
		{
			dsc:                fmt.Sprintf("%s disabled", legacyMetricNamesFeatureGateID),
			featureGateEnabled: false,
		},
	}

	excludeDimensions := []string{"span.kind", "span.name", "totallyWrongNameDoesNotAffectAnything"}
	for _, tc := range testcases {
		t.Run(tc.dsc, func(t *testing.T) {
			// Set feature gate value
			previousValue := legacyMetricNamesFeatureGate.IsEnabled()
			require.NoError(t, featuregate.GlobalRegistry().Set(legacyMetricNamesFeatureGate.ID(), tc.featureGateEnabled))
			defer func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(legacyMetricNamesFeatureGate.ID(), previousValue))
			}()

			p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, []string{}, 1000, clockwork.NewFakeClock(), excludeDimensions...)
			require.NoError(t, err)
			traces := buildSampleTrace()

			ctx := metadata.NewIncomingContext(context.Background(), nil)

			err = p.ConsumeTraces(ctx, traces)
			require.NoError(t, err)
			metrics := p.buildMetrics()

			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				rm := metrics.ResourceMetrics().At(i)
				ism := rm.ScopeMetrics()
				// Checking all metrics, naming notice: ilmC/mC - C here is for Counter.
				for ilmC := 0; ilmC < ism.Len(); ilmC++ {
					m := ism.At(ilmC).Metrics()
					for mC := 0; mC < m.Len(); mC++ {
						metric := m.At(mC)
						// We check only sum and histogram metrics here, because for now only they are present in this module.

						switch metric.Type() {
						case pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeHistogram:
							{
								dp := metric.Histogram().DataPoints()
								for dpi := 0; dpi < dp.Len(); dpi++ {
									for attributeKey := range dp.At(dpi).Attributes().AsRaw() {
										assert.NotContains(t, excludeDimensions, attributeKey)
									}
								}
							}
						case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeSum, pmetric.MetricTypeSummary:
							{
								dp := metric.Sum().DataPoints()
								for dpi := 0; dpi < dp.Len(); dpi++ {
									for attributeKey := range dp.At(dpi).Attributes().AsRaw() {
										assert.NotContains(t, excludeDimensions, attributeKey)
									}
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestConnectorConsumeTracesEvictedCacheKey(t *testing.T) {
	// Prepare
	traces0 := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			// This should be moved to the evicted list of the LRU cache once service-c is added
			// since the cache size is configured to 2.
			serviceName: "service-a",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
				},
			},
		}, traces0.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces0.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-c",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces0.ResourceSpans().AppendEmpty())

	// This trace does not have service-a, and may not result in an attempt to publish metrics for
	// service-a because service-a may be removed from the metricsKeyCache's evicted list.
	traces1 := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces1.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-c",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces1.ResourceSpans().AppendEmpty())

	mcon := &consumertest.MetricsSink{}

	wantDataPointCounts := []int{
		6, // (calls + duration) * (service-a + service-b + service-c)
		4, // (calls + duration) * (service-b + service-c)
	}

	// Ensure the assertion that wantDataPointCounts is performed only after all ConsumeMetrics
	// invocations are complete.
	var wg sync.WaitGroup
	wg.Add(len(wantDataPointCounts))

	// Note: default dimension key cache size is 2.
	mockClock := clockwork.NewFakeClock()
	p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 0, []string{}, 1000, mockClock)
	require.NoError(t, err)
	// Override the default no-op consumer with metrics sink for testing.
	p.metricsConsumer = mcon

	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err = p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	for _, traces := range []ptrace.Traces{traces0, traces1} {
		// Test
		err = p.ConsumeTraces(ctx, traces)
		require.NoError(t, err)

		// Allow time for metrics aggregation to complete.
		time.Sleep(time.Millisecond)

		// Trigger flush.
		mockClock.Advance(time.Nanosecond)

		// Allow time for metrics flush to complete.
		time.Sleep(time.Millisecond)
	}

	require.Eventually(t, func() bool {
		return len(mcon.AllMetrics()) == len(wantDataPointCounts)
	}, 1*time.Second, 10*time.Millisecond)

	// Verify
	require.NotEmpty(t, wantDataPointCounts)

	for i, wanted := range wantDataPointCounts {
		// GreaterOrEqual is particularly necessary for the second assertion where we
		// expect 4 data points; but could also be 6 due to the non-deterministic nature
		// of the p.histograms map which, through the act of "Get"ting a cached key, will
		// lead to updating its recent-ness.
		require.GreaterOrEqual(t, mcon.AllMetrics()[i].DataPointCount(), wanted)
	}
}

func TestConnectorConsumeTracesExpiredMetrics(t *testing.T) {
	// Prepare
	traces0 := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
				},
			},
		}, traces0.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces0.ResourceSpans().AppendEmpty())

	traces1 := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-x",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
				},
			},
		}, traces1.ResourceSpans().AppendEmpty())

	mcon := &consumertest.MetricsSink{}

	// Creating a connector with a very short metricsTTL to ensure that the metrics are expired.
	mockClock := clockwork.NewFakeClock()
	p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarsConfig, disabledEventsConfig, cumulative, 1*time.Nanosecond, []string{}, 1000, mockClock)
	require.NoError(t, err)
	// Override the default no-op consumer with metrics sink for testing.
	p.metricsConsumer = mcon

	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err = p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	// Test expiration by sending three batches of traces, with the second and third batch being identical.
	// On firsts test run, we should see only data points for traces #0 (= 4 data points)
	// On second test run, we want to see both data points for traces #0 and #1 (= 6 data points)
	// On third test run, we want to see only data points for traces #1, since metrics for traces #0 should be expired (= 2 data points)
	wantedDataPointCounts := []int{4, 6, 2}
	for i, traces := range []ptrace.Traces{traces0, traces1, traces1} {
		// Test
		err = p.ConsumeTraces(ctx, traces)
		require.NoError(t, err)

		// Allow time for metrics aggregation to complete.
		time.Sleep(time.Millisecond)

		// Trigger flush.
		mockClock.Advance(time.Nanosecond)

		// Allow time for metrics flush to complete.
		time.Sleep(time.Millisecond)

		// Verify.
		require.Equal(t, wantedDataPointCounts[i], mcon.AllMetrics()[0].DataPointCount())

		// Reset so we can verify the next set of metrics from scratch.
		mcon.Reset()
	}
}

func TestBuildMetricName(t *testing.T) {
	tests := []struct {
		namespace  string
		metricName string
		expected   string
	}{
		{"", "metric", "metric"},
		{"ns", "metric", "ns.metric"},
		{"longer_namespace", "metric", "longer_namespace.metric"},
	}

	for _, test := range tests {
		actual := buildMetricName(test.namespace, test.metricName)
		assert.Equal(t, test.expected, actual)
	}
}

func TestConnector_durationsToUnits(t *testing.T) {
	tests := []struct {
		input []time.Duration
		unit  metrics.Unit
		want  []float64
	}{
		{
			input: []time.Duration{
				3 * time.Nanosecond,
				3 * time.Microsecond,
				3 * time.Millisecond,
				3 * time.Second,
			},
			unit: defaultUnit,
			want: []float64{0.000003, 0.003, 3, 3000},
		},
		{
			input: []time.Duration{
				3 * time.Nanosecond,
				3 * time.Microsecond,
				3 * time.Millisecond,
				3 * time.Second,
			},
			unit: metrics.Seconds,
			want: []float64{3e-09, 3e-06, 0.003, 3},
		},
		{
			input: []time.Duration{},
			unit:  defaultUnit,
			want:  []float64{},
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := durationsToUnits(tt.input, unitDivider(tt.unit))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConnector_initHistogramMetrics(t *testing.T) {
	defaultHistogramBucketsSeconds := make([]float64, len(defaultHistogramBucketsMs))
	for i, v := range defaultHistogramBucketsMs {
		defaultHistogramBucketsSeconds[i] = v / 1000
	}

	tests := []struct {
		name   string
		config Config
		want   metrics.HistogramMetrics
	}{
		{
			name:   "initialize histogram with no config provided",
			config: Config{},
			want:   metrics.NewExplicitHistogramMetrics(defaultHistogramBucketsMs, nil),
		},
		{
			name: "Disable histogram",
			config: Config{
				Histogram: HistogramConfig{
					Disable: true,
				},
			},
			want: nil,
		},
		{
			name: "initialize explicit histogram with default bounds (ms)",
			config: Config{
				Histogram: HistogramConfig{
					Unit: metrics.Milliseconds,
				},
			},
			want: metrics.NewExplicitHistogramMetrics(defaultHistogramBucketsMs, nil),
		},
		{
			name: "initialize explicit histogram with default bounds (seconds)",
			config: Config{
				Histogram: HistogramConfig{
					Unit: metrics.Seconds,
				},
			},
			want: metrics.NewExplicitHistogramMetrics(defaultHistogramBucketsSeconds, nil),
		},
		{
			name: "initialize explicit histogram with bounds (seconds)",
			config: Config{
				Histogram: HistogramConfig{
					Unit: metrics.Seconds,
					Explicit: &ExplicitHistogramConfig{
						Buckets: []time.Duration{
							100 * time.Millisecond,
							1000 * time.Millisecond,
						},
					},
				},
			},
			want: metrics.NewExplicitHistogramMetrics([]float64{0.1, 1}, nil),
		},
		{
			name: "initialize explicit histogram with bounds (ms)",
			config: Config{
				Histogram: HistogramConfig{
					Unit: metrics.Milliseconds,
					Explicit: &ExplicitHistogramConfig{
						Buckets: []time.Duration{
							100 * time.Millisecond,
							1000 * time.Millisecond,
						},
					},
				},
			},
			want: metrics.NewExplicitHistogramMetrics([]float64{100, 1000}, nil),
		},
		{
			name: "initialize exponential histogram",
			config: Config{
				Histogram: HistogramConfig{
					Unit: metrics.Milliseconds,
					Exponential: &ExponentialHistogramConfig{
						MaxSize: 10,
					},
				},
			},
			want: metrics.NewExponentialHistogramMetrics(10, nil),
		},
		{
			name: "initialize exponential histogram with default max buckets count",
			config: Config{
				Histogram: HistogramConfig{
					Unit:        metrics.Milliseconds,
					Exponential: &ExponentialHistogramConfig{},
				},
			},
			want: metrics.NewExponentialHistogramMetrics(structure.DefaultMaxSize, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initHistogramMetrics(tt.config)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSpanMetrics_Events(t *testing.T) {
	tests := []struct {
		name                    string
		eventsConfig            EventsConfig
		shouldEventsMetricExist bool
	}{
		{
			name:                    "events disabled",
			eventsConfig:            EventsConfig{Enabled: false, Dimensions: []Dimension{{Name: "exception.type", Default: stringp("NullPointerException")}}},
			shouldEventsMetricExist: false,
		},
		{
			name:                    "events enabled",
			eventsConfig:            EventsConfig{Enabled: true, Dimensions: []Dimension{{Name: "exception.type", Default: stringp("NullPointerException")}}},
			shouldEventsMetricExist: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Events = tt.eventsConfig
			c, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewFakeClock())
			require.NoError(t, err)
			err = c.ConsumeTraces(context.Background(), buildSampleTrace())
			require.NoError(t, err)
			metrics := c.buildMetrics()
			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				rm := metrics.ResourceMetrics().At(i)
				ism := rm.ScopeMetrics()
				for ilmC := 0; ilmC < ism.Len(); ilmC++ {
					m := ism.At(ilmC).Metrics()
					if !tt.shouldEventsMetricExist {
						assert.Equal(t, 2, m.Len())
						continue
					}
					assert.Equal(t, 3, m.Len())
					for mC := 0; mC < m.Len(); mC++ {
						metric := m.At(mC)
						if metric.Name() != "events" {
							continue
						}
						assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
						for idp := 0; idp < metric.Sum().DataPoints().Len(); idp++ {
							attrs := metric.Sum().DataPoints().At(idp).Attributes()
							assert.Contains(t, attrs.AsRaw(), exceptionTypeAttrName)
						}
					}
				}
			}
		})
	}
}

func TestExemplarsAreDiscardedAfterFlushing(t *testing.T) {
	tests := []struct {
		name            string
		temporality     string
		histogramConfig func() HistogramConfig
	}{
		{
			name:            "cumulative explicit histogram",
			temporality:     cumulative,
			histogramConfig: explicitHistogramsConfig,
		},
		{
			name:            "cumulative exponential histogram",
			temporality:     cumulative,
			histogramConfig: exponentialHistogramsConfig,
		},
		{
			name:            "delta explicit histogram",
			temporality:     delta,
			histogramConfig: explicitHistogramsConfig,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := newConnectorImp(stringp("defaultNullValue"), tt.histogramConfig, enabledExemplarsConfig, enabledEventsConfig, tt.temporality, 0, []string{}, 1000, clockwork.NewFakeClock())
			p.metricsConsumer = &consumertest.MetricsSink{}
			require.NoError(t, err)

			traces := ptrace.NewTraces()
			trace1ID := [16]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x10}
			initServiceSpans(
				serviceSpans{
					serviceName: "service-b",
					spans: []span{
						{
							name:       "/ping",
							kind:       ptrace.SpanKindServer,
							statusCode: ptrace.StatusCodeError,
							traceID:    trace1ID,
							spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
						},
					},
				}, traces.ResourceSpans().AppendEmpty())

			// Test
			ctx := metadata.NewIncomingContext(context.Background(), nil)

			// Verify exactly 1 exemplar is added to all data points when flushing
			err = p.ConsumeTraces(ctx, traces)
			require.NoError(t, err)

			p.exportMetrics(ctx)
			m := p.metricsConsumer.(*consumertest.MetricsSink).AllMetrics()[0]
			assertDataPointsHaveExactlyOneExemplarForTrace(t, m, trace1ID)

			// Verify exemplars from previous batch's trace are replaced with exemplars for the new batch's trace
			traces = ptrace.NewTraces()
			trace2ID := [16]byte{0x00, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x10}
			initServiceSpans(
				serviceSpans{
					serviceName: "service-b",
					spans: []span{
						{
							name:       "/ping",
							kind:       ptrace.SpanKindServer,
							statusCode: ptrace.StatusCodeError,
							traceID:    trace2ID,
							spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
						},
					},
				}, traces.ResourceSpans().AppendEmpty())

			err = p.ConsumeTraces(ctx, traces)
			require.NoError(t, err)

			p.exportMetrics(ctx)
			m = p.metricsConsumer.(*consumertest.MetricsSink).AllMetrics()[1]
			assertDataPointsHaveExactlyOneExemplarForTrace(t, m, trace2ID)
		})
	}
}

func assertDataPointsHaveExactlyOneExemplarForTrace(t *testing.T, metrics pmetric.Metrics, traceID pcommon.TraceID) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		ism := rm.ScopeMetrics()
		// Checking all metrics, naming notice: ilmC/mC - C here is for Counter.
		for ilmC := 0; ilmC < ism.Len(); ilmC++ {
			m := ism.At(ilmC).Metrics()
			for mC := 0; mC < m.Len(); mC++ {
				metric := m.At(mC)
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					assert.Positive(t, dps.Len())
					for dpi := 0; dpi < dps.Len(); dpi++ {
						dp := dps.At(dpi)
						assert.Equal(t, 1, dp.Exemplars().Len())
						assert.Equal(t, dp.Exemplars().At(0).TraceID(), traceID)
					}
				case pmetric.MetricTypeHistogram:
					dps := metric.Histogram().DataPoints()
					assert.Positive(t, dps.Len())
					for dpi := 0; dpi < dps.Len(); dpi++ {
						dp := dps.At(dpi)
						assert.Equal(t, 1, dp.Exemplars().Len())
						assert.Equal(t, dp.Exemplars().At(0).TraceID(), traceID)
					}
				case pmetric.MetricTypeExponentialHistogram:
					dps := metric.ExponentialHistogram().DataPoints()
					assert.Positive(t, dps.Len())
					for dpi := 0; dpi < dps.Len(); dpi++ {
						dp := dps.At(dpi)
						assert.Equal(t, 1, dp.Exemplars().Len())
						assert.Equal(t, dp.Exemplars().At(0).TraceID(), traceID)
					}
				default:
					t.Fatalf("Unexpected metric type %s", metric.Type())
				}
			}
		}
	}
}

func TestTimestampsForUninterruptedStream(t *testing.T) {
	tests := []struct {
		temporality      string
		verifyTimestamps func(startTime1 pcommon.Timestamp, timestamp1 pcommon.Timestamp, startTime2 pcommon.Timestamp, timestamp2 pcommon.Timestamp)
	}{
		{
			temporality: cumulative,
			verifyTimestamps: func(startTime1 pcommon.Timestamp, timestamp1 pcommon.Timestamp, startTime2 pcommon.Timestamp, timestamp2 pcommon.Timestamp) {
				// (T1, T2), (T1, T3) ...
				assert.Greater(t, timestamp1, startTime1)
				assert.Equal(t, startTime1, startTime2)
				assert.Greater(t, timestamp2, startTime2)
			},
		},
		{
			temporality: delta,
			verifyTimestamps: func(startTime1 pcommon.Timestamp, timestamp1 pcommon.Timestamp, startTime2 pcommon.Timestamp, timestamp2 pcommon.Timestamp) {
				// (T1, T2), (T2, T3) ...
				assert.Greater(t, timestamp1, startTime1)
				assert.Equal(t, timestamp1, startTime2)
				assert.Greater(t, timestamp2, startTime2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.temporality, func(t *testing.T) {
			mockClock := newAlwaysIncreasingClock()
			p, err := newConnectorImp(stringp("defaultNullValue"), explicitHistogramsConfig, enabledExemplarsConfig, enabledEventsConfig, tt.temporality, 0, []string{}, 1000, mockClock)
			require.NoError(t, err)
			p.metricsConsumer = &consumertest.MetricsSink{}

			// Test
			ctx := metadata.NewIncomingContext(context.Background(), nil)

			// Send first batch of spans
			err = p.ConsumeTraces(ctx, buildSampleTrace())
			require.NoError(t, err)
			p.exportMetrics(ctx)
			metrics1 := p.metricsConsumer.(*consumertest.MetricsSink).AllMetrics()[0]
			startTimestamp1, timestamp1 := verifyAndCollectCommonTimestamps(t, metrics1)

			// Send an unrelated batch of spans for a different resource
			unrelatedTraces := ptrace.NewTraces()
			initServiceSpans(
				serviceSpans{
					serviceName: "unrelated-service",
					spans: []span{
						{
							name:       "/ping",
							kind:       ptrace.SpanKindServer,
							statusCode: ptrace.StatusCodeOk,
							traceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
							spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
						},
					},
				}, unrelatedTraces.ResourceSpans().AppendEmpty())
			err = p.ConsumeTraces(ctx, unrelatedTraces)
			require.NoError(t, err)
			p.exportMetrics(ctx)

			// Send another set of spans that are the same as the first batch
			err = p.ConsumeTraces(ctx, buildSampleTrace())
			require.NoError(t, err)
			p.exportMetrics(ctx)
			metrics2 := p.metricsConsumer.(*consumertest.MetricsSink).AllMetrics()[2]
			startTimestamp2, timestamp2 := verifyAndCollectCommonTimestamps(t, metrics2)

			tt.verifyTimestamps(startTimestamp1, timestamp1, startTimestamp2, timestamp2)
		})
	}
}

func verifyAndCollectCommonTimestamps(t *testing.T, m pmetric.Metrics) (start pcommon.Timestamp, timestamp pcommon.Timestamp) {
	// Go through all data points and collect the start timestamp and timestamp. They should be the same value for each data point
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		rm := m.ResourceMetrics().At(i)

		serviceName, _ := rm.Resource().Attributes().Get("service.name")
		if serviceName.Str() == "unrelated-service" {
			continue
		}

		ism := rm.ScopeMetrics()
		for ilmC := 0; ilmC < ism.Len(); ilmC++ {
			m := ism.At(ilmC).Metrics()
			for mC := 0; mC < m.Len(); mC++ {
				metric := m.At(mC)

				switch metric.Type() {
				case pmetric.MetricTypeSum:
					{
						dps := metric.Sum().DataPoints()
						for dpi := 0; dpi < dps.Len(); dpi++ {
							if int64(start) == 0 {
								start = dps.At(dpi).StartTimestamp()
								timestamp = dps.At(dpi).Timestamp()
							}
							assert.Equal(t, dps.At(dpi).StartTimestamp(), start)
							assert.Equal(t, dps.At(dpi).Timestamp(), timestamp)
						}
					}
				case pmetric.MetricTypeHistogram:
					{
						dps := metric.Histogram().DataPoints()
						for dpi := 0; dpi < dps.Len(); dpi++ {
							if int64(start) == 0 {
								start = dps.At(dpi).StartTimestamp()
								timestamp = dps.At(dpi).Timestamp()
							}
							assert.Equal(t, dps.At(dpi).StartTimestamp(), start)
							assert.Equal(t, dps.At(dpi).Timestamp(), timestamp)
						}
					}
				default:
					t.Fail()
				}
			}
		}
	}

	return start, timestamp
}

func TestDeltaTimestampCacheExpiry(t *testing.T) {
	timestampCacheSize := 1
	mockClock := newAlwaysIncreasingClock()
	p, err := newConnectorImp(stringp("defaultNullValue"), exponentialHistogramsConfig, enabledExemplarsConfig, enabledEventsConfig, delta, 0, []string{}, timestampCacheSize, mockClock)
	require.NoError(t, err)
	p.metricsConsumer = &consumertest.MetricsSink{}

	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// Send a span from service A which should fill the cache
	serviceATrace1 := ptrace.NewTraces()
	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
					traceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
					spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
				},
			},
		}, serviceATrace1.ResourceSpans().AppendEmpty())
	err = p.ConsumeTraces(ctx, serviceATrace1)
	require.NoError(t, err)
	p.exportMetrics(ctx)

	// Send a span from service B which should evict service A's entries
	serviceBTrace1 := ptrace.NewTraces()
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
					traceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
					spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
				},
			},
		}, serviceBTrace1.ResourceSpans().AppendEmpty())
	err = p.ConsumeTraces(ctx, serviceBTrace1)
	require.NoError(t, err)
	p.exportMetrics(ctx)

	// Send another span from Service A, then verify no error + cache was actually evicted
	serviceATrace2 := ptrace.NewTraces()
	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					name:       "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
					traceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
					spanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
				},
			},
		}, serviceATrace2.ResourceSpans().AppendEmpty())
	err = p.ConsumeTraces(ctx, serviceATrace2)
	require.NoError(t, err)
	p.exportMetrics(ctx)

	serviceATimestamp1 := p.metricsConsumer.(*consumertest.MetricsSink).AllMetrics()[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Timestamp()
	serviceAStartTimestamp2 := p.metricsConsumer.(*consumertest.MetricsSink).AllMetrics()[2].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).StartTimestamp()
	assert.Greater(t, serviceAStartTimestamp2, serviceATimestamp1) // These would be the same if nothing was evicted from the cache
}

// Clock where Now() always returns a greater value than the previous return value
type alwaysIncreasingClock struct {
	clockwork.Clock
}

func newAlwaysIncreasingClock() alwaysIncreasingClock {
	return alwaysIncreasingClock{
		Clock: clockwork.NewFakeClock(),
	}
}

func (c alwaysIncreasingClock) Now() time.Time {
	c.Clock.(*clockwork.FakeClock).Advance(time.Millisecond)
	return c.Clock.Now()
}

func TestBuildAttributes_InstrumentationScope(t *testing.T) {
	tests := []struct {
		name                 string
		instrumentationScope pcommon.InstrumentationScope
		config               Config
		want                 map[string]string
	}{
		{
			name: "with instrumentation scope name and version",
			instrumentationScope: func() pcommon.InstrumentationScope {
				scope := pcommon.NewInstrumentationScope()
				scope.SetName("express")
				scope.SetVersion("1.0.0")
				return scope
			}(),
			config: Config{
				IncludeInstrumentationScope: []string{"express"},
			},
			want: map[string]string{
				serviceNameKey:                 "test_service",
				spanNameKey:                    "test_span",
				spanKindKey:                    "SPAN_KIND_INTERNAL",
				statusCodeKey:                  "STATUS_CODE_UNSET",
				instrumentationScopeNameKey:    "express",
				instrumentationScopeVersionKey: "1.0.0",
			},
		},
		{
			name: "with instrumentation scope but not included",
			instrumentationScope: func() pcommon.InstrumentationScope {
				scope := pcommon.NewInstrumentationScope()
				scope.SetName("express")
				scope.SetVersion("1.0.0")
				return scope
			}(),
			config: Config{},
			want: map[string]string{
				serviceNameKey: "test_service",
				spanNameKey:    "test_span",
				spanKindKey:    "SPAN_KIND_INTERNAL",
				statusCodeKey:  "STATUS_CODE_UNSET",
			},
		},
		{
			name: "without instrumentation scope but version and included in config",
			instrumentationScope: func() pcommon.InstrumentationScope {
				scope := pcommon.NewInstrumentationScope()
				scope.SetVersion("1.0.0")
				return scope
			}(),
			config: Config{
				IncludeInstrumentationScope: []string{"express"},
			},
			want: map[string]string{
				serviceNameKey: "test_service",
				spanNameKey:    "test_span",
				spanKindKey:    "SPAN_KIND_INTERNAL",
				statusCodeKey:  "STATUS_CODE_UNSET",
			},
		},

		{
			name: "with instrumentation scope and instrumentation scope name but no version and included in config",
			instrumentationScope: func() pcommon.InstrumentationScope {
				scope := pcommon.NewInstrumentationScope()
				scope.SetName("express")
				return scope
			}(),
			config: Config{
				IncludeInstrumentationScope: []string{"express"},
			},
			want: map[string]string{
				serviceNameKey:              "test_service",
				spanNameKey:                 "test_span",
				spanKindKey:                 "SPAN_KIND_INTERNAL",
				statusCodeKey:               "STATUS_CODE_UNSET",
				instrumentationScopeNameKey: "express",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create connector
			p := &connectorImp{
				config: tt.config,
			}

			// Create basic span
			span := ptrace.NewSpan()
			span.SetName("test_span")
			span.SetKind(ptrace.SpanKindInternal)

			// Build attributes
			attrs := p.buildAttributes("test_service", span, pcommon.NewMap(), nil, tt.instrumentationScope)

			// Verify results
			assert.Equal(t, len(tt.want), attrs.Len())
			for k, v := range tt.want {
				val, ok := attrs.Get(k)
				assert.True(t, ok)
				assert.Equal(t, v, val.Str())
			}
		})
	}
}
