// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/mocks"
)

const (
	stringAttrName         = "stringAttrName"
	intAttrName            = "intAttrName"
	doubleAttrName         = "doubleAttrName"
	boolAttrName           = "boolAttrName"
	nullAttrName           = "nullAttrName"
	mapAttrName            = "mapAttrName"
	arrayAttrName          = "arrayAttrName"
	notInSpanAttrName0     = "shouldBeInMetric"
	notInSpanAttrName1     = "shouldNotBeInMetric"
	regionResourceAttrName = "region"
	DimensionsCacheSize    = 2

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
func verifyDisabledHistogram(t testing.TB, input pmetric.Metrics) bool {
	for i := 0; i < input.ResourceMetrics().Len(); i++ {
		rm := input.ResourceMetrics().At(i)
		ism := rm.ScopeMetrics()
		// Checking all metrics, naming notice: ismC/mC - C here is for Counter.
		for ismC := 0; ismC < ism.Len(); ismC++ {
			m := ism.At(ismC).Metrics()
			for mC := 0; mC < m.Len(); mC++ {
				metric := m.At(mC)
				assert.NotEqual(t, pmetric.MetricTypeExponentialHistogram, metric.Type())
				assert.NotEqual(t, pmetric.MetricTypeHistogram, metric.Type())
			}
		}
	}
	return true
}

func verifyExemplarsExist(t testing.TB, input pmetric.Metrics) bool {
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
					assert.True(t, d.Exemplars().Len() > 0)
				}
			}
		}
	}
	return true
}

// verifyConsumeMetricsInputCumulative expects one accumulation of metrics, and marked as cumulative
func verifyConsumeMetricsInputCumulative(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityCumulative, 1)
}

func verifyBadMetricsOkay(_ testing.TB, _ pmetric.Metrics) bool {
	return true // Validating no exception
}

// verifyConsumeMetricsInputDelta expects one accumulation of metrics, and marked as delta
func verifyConsumeMetricsInputDelta(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityDelta, 1)
}

// verifyMultipleCumulativeConsumptions expects the amount of accumulations as kept track of by numCumulativeConsumptions.
// numCumulativeConsumptions acts as a multiplier for the values, since the cumulative metrics are additive.
func verifyMultipleCumulativeConsumptions() func(t testing.TB, input pmetric.Metrics) bool {
	numCumulativeConsumptions := 0
	return func(t testing.TB, input pmetric.Metrics) bool {
		numCumulativeConsumptions++
		return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityCumulative, numCumulativeConsumptions)
	}
}

// verifyConsumeMetricsInput verifies the input of the ConsumeMetrics call from this connector.
// This is the best point to verify the computed metrics from spans are as expected.
func verifyConsumeMetricsInput(t testing.TB, input pmetric.Metrics, expectedTemporality pmetric.AggregationTemporality, numCumulativeConsumptions int) bool {
	require.Equal(t, 6, input.DataPointCount(),
		"Should be 3 for each of call count and latency split into two resource scopes defined by: "+
			"service-a: service-a (server kind) -> service-a (client kind) and "+
			"service-b: service-b (service kind)",
	)

	require.Equal(t, 2, input.ResourceMetrics().Len())

	for i := 0; i < input.ResourceMetrics().Len(); i++ {
		rm := input.ResourceMetrics().At(i)

		var numDataPoints int
		val, ok := rm.Resource().Attributes().Get(serviceNameKey)
		require.True(t, ok)
		serviceName := val.AsString()
		if serviceName == "service-a" {
			numDataPoints = 2
		} else if serviceName == "service-b" {
			numDataPoints = 1
		}

		ilm := rm.ScopeMetrics()
		require.Equal(t, 1, ilm.Len())
		assert.Equal(t, "spanmetricsconnector", ilm.At(0).Scope().Name())

		m := ilm.At(0).Metrics()
		require.Equal(t, 2, m.Len(), "only sum and histogram metric types generated")

		// validate calls - sum metrics
		metric := m.At(0)
		assert.Equal(t, metricNameCalls, metric.Name())
		assert.Equal(t, expectedTemporality, metric.Sum().AggregationTemporality())
		assert.True(t, metric.Sum().IsMonotonic())

		seenMetricIDs := make(map[metricID]bool)
		callsDps := metric.Sum().DataPoints()
		require.Equal(t, numDataPoints, callsDps.Len())
		for dpi := 0; dpi < numDataPoints; dpi++ {
			dp := callsDps.At(dpi)
			assert.Equal(t,
				int64(numCumulativeConsumptions),
				dp.IntValue(),
				"There should only be one metric per Service/name/kind combination",
			)
			assert.NotZero(t, dp.StartTimestamp(), "StartTimestamp should be set")
			assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")
			verifyMetricLabels(dp, t, seenMetricIDs)
		}

		// validate latency - histogram metrics
		metric = m.At(1)
		assert.Equal(t, metricNameDuration, metric.Name())
		assert.Equal(t, defaultUnit.String(), metric.Unit())

		if metric.Type() == pmetric.MetricTypeExponentialHistogram {
			hist := metric.ExponentialHistogram()
			assert.Equal(t, expectedTemporality, hist.AggregationTemporality())
			verifyExponentialHistogramDataPoints(t, hist.DataPoints(), numDataPoints, numCumulativeConsumptions)
		} else {
			hist := metric.Histogram()
			assert.Equal(t, expectedTemporality, hist.AggregationTemporality())
			verifyExplicitHistogramDataPoints(t, hist.DataPoints(), numDataPoints, numCumulativeConsumptions)
		}
	}
	return true
}

func verifyExplicitHistogramDataPoints(t testing.TB, dps pmetric.HistogramDataPointSlice, numDataPoints, numCumulativeConsumptions int) {
	seenMetricIDs := make(map[metricID]bool)
	require.Equal(t, numDataPoints, dps.Len())
	for dpi := 0; dpi < numDataPoints; dpi++ {
		dp := dps.At(dpi)
		assert.Equal(
			t,
			sampleDuration*float64(numCumulativeConsumptions),
			dp.Sum(),
			"Should be a 11ms duration measurement, multiplied by the number of stateful accumulations.")
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")

		// Verify bucket counts.

		// The bucket counts should be 1 greater than the explicit bounds as documented in:
		// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto.
		assert.Equal(t, dp.ExplicitBounds().Len()+1, dp.BucketCounts().Len())

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
			assert.Equal(t, wantBucketCount, dp.BucketCounts().At(bi))
		}
		verifyMetricLabels(dp, t, seenMetricIDs)
	}
}

func verifyExponentialHistogramDataPoints(t testing.TB, dps pmetric.ExponentialHistogramDataPointSlice, numDataPoints, numCumulativeConsumptions int) {
	seenMetricIDs := make(map[metricID]bool)
	require.Equal(t, numDataPoints, dps.Len())
	for dpi := 0; dpi < numDataPoints; dpi++ {
		dp := dps.At(dpi)
		assert.Equal(
			t,
			sampleDuration*float64(numCumulativeConsumptions),
			dp.Sum(),
			"Should be a 11ms duration measurement, multiplied by the number of stateful accumulations.")
		assert.Equal(t, uint64(numCumulativeConsumptions), dp.Count())
		assert.Equal(t, []uint64{uint64(numCumulativeConsumptions)}, dp.Positive().BucketCounts().AsRaw())
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")

		verifyMetricLabels(dp, t, seenMetricIDs)
	}
}

func verifyMetricLabels(dp metricDataPoint, t testing.TB, seenMetricIDs map[metricID]bool) {
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
	dp.Attributes().Range(func(k string, v pcommon.Value) bool {
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
			assert.Fail(t, notInSpanAttrName1+" should not be in this metric")
		default:
			assert.Equal(t, wantDimensions[k], v)
			delete(wantDimensions, k)
		}
		return true
	})
	assert.Empty(t, wantDimensions, "Did not see all expected dimensions in metric. Missing: ", wantDimensions)

	// Service/name/kind should be a unique metric.
	assert.False(t, seenMetricIDs[mID])
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
}

func disabledExemplarConfig() ExemplarConfig {
	return ExemplarConfig{
		Enabled: false,
	}
}

func enabledExemplarConfig() ExemplarConfig {
	return ExemplarConfig{
		Enabled: true,
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

func TestBuildKeySameServiceNameCharSequence(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	c, err := newConnector(zaptest.NewLogger(t), cfg, nil)
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
	c, err := newConnector(zaptest.NewLogger(t), cfg, nil)
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
	c, err := newConnector(zaptest.NewLogger(t), cfg, nil)
	require.NoError(t, err)

	span0 := ptrace.NewSpan()
	span0.SetName("spanName")
	k0 := c.buildKey("serviceName", span0, nil, pcommon.NewMap())
	assert.Equal(t, metrics.Key("serviceName"), k0)
}

func TestBuildKeyWithDimensions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	c, err := newConnector(zaptest.NewLogger(t), cfg, nil)
	require.NoError(t, err)

	defaultFoo := pcommon.NewValueStr("bar")
	for _, tc := range []struct {
		name            string
		optionalDims    []dimension
		resourceAttrMap map[string]interface{}
		spanAttrMap     map[string]interface{}
		wantKey         string
	}{
		{
			name:    "nil optionalDims",
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "neither span nor resource contains key, dim provides default",
			optionalDims: []dimension{
				{name: "foo", value: &defaultFoo},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000bar",
		},
		{
			name: "neither span nor resource contains key, dim provides no default",
			optionalDims: []dimension{
				{name: "foo"},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "span attribute contains dimension",
			optionalDims: []dimension{
				{name: "foo"},
			},
			spanAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "resource attribute contains dimension",
			optionalDims: []dimension{
				{name: "foo"},
			},
			resourceAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "both span and resource attribute contains dimension, should prefer span attribute",
			optionalDims: []dimension{
				{name: "foo"},
			},
			spanAttrMap: map[string]interface{}{
				"foo": 100,
			},
			resourceAttrMap: map[string]interface{}{
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

	createParams := connectortest.NewNopCreateSettings()
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
	logger := zap.New(core)

	mockClock := clock.NewMock(time.Now())
	ticker := mockClock.NewTicker(time.Nanosecond)

	// Test
	p := newConnectorImp(t, new(consumertest.MetricsSink), nil, explicitHistogramsConfig, disabledExemplarConfig, cumulative, logger, ticker)
	err := p.Start(ctx, componenttest.NewNopHost())
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
			require.NoError(t, err)
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

	// Building spanmetrics connector...
	// Starting spanmetricsconnector...
	// Shutting down spanmetricsconnector...
	// Stopping ticker.
	assert.Len(t, allLogs, 4)
}

func TestConnectorCapabilities(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	c, err := newConnector(zaptest.NewLogger(t), cfg, nil)
	c.metricsConsumer = new(consumertest.MetricsSink)
	assert.NoError(t, err)
	caps := c.Capabilities()

	// Verify
	assert.NotNil(t, c)
	assert.Equal(t, false, caps.MutatesData)
}

func TestConsumeMetricsErrors(t *testing.T) {
	// Prepare
	fakeErr := fmt.Errorf("consume metrics error")

	core, observedLogs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	var wg sync.WaitGroup
	mcon := &mocks.MetricsConsumer{}
	mcon.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pmetric.Metrics) bool {
		wg.Done()
		return true
	})).Return(fakeErr)

	mockClock := clock.NewMock(time.Now())
	ticker := mockClock.NewTicker(time.Nanosecond)
	p := newConnectorImp(t, mcon, nil, explicitHistogramsConfig, disabledExemplarConfig, cumulative, logger, ticker)

	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err := p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	traces := buildSampleTrace()

	// Test
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// Trigger flush.
	wg.Add(1)
	mockClock.Add(time.Nanosecond)
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
		exemplarConfig         func() ExemplarConfig
		verifier               func(t testing.TB, input pmetric.Metrics) bool
		traces                 []ptrace.Traces
	}{
		// disabling histogram
		{
			name:                   "Test histogram metrics are disabled",
			aggregationTemporality: cumulative,
			histogramConfig:        disabledHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyDisabledHistogram,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		// exponential buckets histogram
		{
			name:                   "Test single consumption, three spans (Cumulative), using exp. histogram",
			aggregationTemporality: cumulative,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyConsumeMetricsInputCumulative,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta), using exp. histogram",
			aggregationTemporality: delta,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative), using exp. histogram",
			aggregationTemporality: cumulative,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyMultipleCumulativeConsumptions(),
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta), using exp. histogram",
			aggregationTemporality: delta,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// Consumptions with improper timestamps
			name:                   "Test bad consumptions (Delta), using exp. histogram",
			aggregationTemporality: cumulative,
			histogramConfig:        exponentialHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyBadMetricsOkay,
			traces:                 []ptrace.Traces{buildBadSampleTrace()},
		},

		// explicit buckets histogram
		{
			name:                   "Test single consumption, three spans (Cumulative).",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyConsumeMetricsInputCumulative,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta).",
			aggregationTemporality: delta,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative).",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyMultipleCumulativeConsumptions(),
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta).",
			aggregationTemporality: delta,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// Consumptions with improper timestamps
			name:                   "Test bad consumptions (Delta).",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         disabledExemplarConfig,
			verifier:               verifyBadMetricsOkay,
			traces:                 []ptrace.Traces{buildBadSampleTrace()},
		},
		// enabling exemplars
		{
			name:                   "Test Exemplars are enabled",
			aggregationTemporality: cumulative,
			histogramConfig:        explicitHistogramsConfig,
			exemplarConfig:         enabledExemplarConfig,
			verifier:               verifyExemplarsExist,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
	}

	for _, tc := range testcases {
		// Since parallelism is enabled in these tests, to avoid flaky behavior,
		// instantiate a copy of the test case for t.Run's closure to use.
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Prepare

			mcon := &mocks.MetricsConsumer{}

			var wg sync.WaitGroup
			// Mocked metric exporter will perform validation on metrics, during p.ConsumeMetrics()
			mcon.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pmetric.Metrics) bool {
				wg.Done()

				return tc.verifier(t, input)
			})).Return(nil)

			mockClock := clock.NewMock(time.Now())
			ticker := mockClock.NewTicker(time.Nanosecond)

			p := newConnectorImp(t, mcon, stringp("defaultNullValue"), tc.histogramConfig, tc.exemplarConfig, tc.aggregationTemporality, zaptest.NewLogger(t), ticker)

			ctx := metadata.NewIncomingContext(context.Background(), nil)
			err := p.Start(ctx, componenttest.NewNopHost())
			defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
			require.NoError(t, err)

			for _, traces := range tc.traces {
				// Test
				err = p.ConsumeTraces(ctx, traces)
				assert.NoError(t, err)

				// Trigger flush.
				wg.Add(1)
				mockClock.Add(time.Nanosecond)
				wg.Wait()
			}
		})
	}
}

func TestMetricKeyCache(t *testing.T) {
	mcon := &mocks.MetricsConsumer{}
	mcon.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)

	p := newConnectorImp(t, mcon, stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarConfig, cumulative, zaptest.NewLogger(t), nil)
	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// 0 key was cached at beginning
	assert.Zero(t, p.metricKeyToDimensions.Len())

	err := p.ConsumeTraces(ctx, traces)
	// Validate
	require.NoError(t, err)
	// 2 key was cached, 1 key was evicted and cleaned after the processing
	assert.Eventually(t, func() bool {
		return p.metricKeyToDimensions.Len() == DimensionsCacheSize
	}, 10*time.Second, time.Millisecond*100)

	// consume another batch of traces
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// 2 key was cached, other keys were evicted and cleaned after the processing
	assert.Eventually(t, func() bool {
		return p.metricKeyToDimensions.Len() == DimensionsCacheSize
	}, 10*time.Second, time.Millisecond*100)
}

func BenchmarkConnectorConsumeTraces(b *testing.B) {
	// Prepare
	mcon := &mocks.MetricsConsumer{}
	mcon.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)

	conn := newConnectorImp(nil, mcon, stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarConfig, cumulative, zaptest.NewLogger(b), nil)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	for n := 0; n < b.N; n++ {
		assert.NoError(b, conn.ConsumeTraces(ctx, traces))
	}
}

func TestExcludeDimensionsConsumeTraces(t *testing.T) {
	mcon := &mocks.MetricsConsumer{}
	mcon.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	excludeDimensions := []string{"span.kind", "span.name", "totallyWrongNameDoesNotAffectAnything"}
	p := newConnectorImp(t, mcon, stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarConfig, cumulative, zaptest.NewLogger(t), nil, excludeDimensions...)
	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	err := p.ConsumeTraces(ctx, traces)
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

}

func newConnectorImp(t *testing.T, mcon consumer.Metrics, defaultNullValue *string, histogramConfig func() HistogramConfig, exemplarConfig func() ExemplarConfig, temporality string, logger *zap.Logger, ticker *clock.Ticker, excludedDimensions ...string) *connectorImp {

	cfg := &Config{
		AggregationTemporality: temporality,
		Histogram:              histogramConfig(),
		ExemplarConfig:         exemplarConfig(),
		ExcludeDimensions:      excludedDimensions,
		DimensionsCacheSize:    DimensionsCacheSize,
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
			// Add a resource attribute to test "process" attributes like IP, host, region, cluster, etc.
			{regionResourceAttrName, nil},
		},
	}
	c, err := newConnector(logger, cfg, ticker)
	require.NoError(t, err)
	c.metricsConsumer = mcon
	return c
}

func stringp(str string) *string {
	return &str
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

	mcon := &mocks.MetricsConsumer{}

	wantDataPointCounts := []int{
		6, // (calls + duration) * (service-a + service-b + service-c)
		4, // (calls + duration) * (service-b + service-c)
	}

	// Ensure the assertion that wantDataPointCounts is performed only after all ConsumeMetrics
	// invocations are complete.
	var wg sync.WaitGroup
	wg.Add(len(wantDataPointCounts))

	// Mocked metric exporter will perform validation on metrics, during p.ConsumeMetrics()
	mcon.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pmetric.Metrics) bool {
		defer wg.Done()

		// Verify
		require.NotEmpty(t, wantDataPointCounts)

		// GreaterOrEqual is particularly necessary for the second assertion where we
		// expect 4 data points; but could also be 6 due to the non-deterministic nature
		// of the p.histograms map which, through the act of "Get"ting a cached key, will
		// lead to updating its recent-ness.
		require.GreaterOrEqual(t, input.DataPointCount(), wantDataPointCounts[0])
		wantDataPointCounts = wantDataPointCounts[1:] // Dequeue

		return true
	})).Return(nil)

	mockClock := clock.NewMock(time.Now())
	ticker := mockClock.NewTicker(time.Nanosecond)

	// Note: default dimension key cache size is 2.
	p := newConnectorImp(t, mcon, stringp("defaultNullValue"), explicitHistogramsConfig, disabledExemplarConfig, cumulative, zaptest.NewLogger(t), ticker)

	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err := p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	for _, traces := range []ptrace.Traces{traces0, traces1} {
		// Test
		err = p.ConsumeTraces(ctx, traces)
		require.NoError(t, err)

		// Allow time for metrics aggregation to complete.
		time.Sleep(time.Millisecond)

		// Trigger flush.
		mockClock.Add(time.Nanosecond)

		// Allow time for metrics flush to complete.
		time.Sleep(time.Millisecond)
	}

	wg.Wait()
	assert.Empty(t, wantDataPointCounts)
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
			want:   metrics.NewExplicitHistogramMetrics(defaultHistogramBucketsMs),
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
			want: metrics.NewExplicitHistogramMetrics(defaultHistogramBucketsMs),
		},
		{
			name: "initialize explicit histogram with default bounds (seconds)",
			config: Config{
				Histogram: HistogramConfig{
					Unit: metrics.Seconds,
				},
			},
			want: metrics.NewExplicitHistogramMetrics(defaultHistogramBucketsSeconds),
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
			want: metrics.NewExplicitHistogramMetrics([]float64{0.1, 1}),
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
			want: metrics.NewExplicitHistogramMetrics([]float64{100, 1000}),
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
			want: metrics.NewExponentialHistogramMetrics(10),
		},
		{
			name: "initialize exponential histogram with default max buckets count",
			config: Config{
				Histogram: HistogramConfig{
					Unit:        metrics.Milliseconds,
					Exponential: &ExponentialHistogramConfig{},
				},
			},
			want: metrics.NewExponentialHistogramMetrics(structure.DefaultMaxSize),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initHistogramMetrics(tt.config)
			assert.Equal(t, tt.want, got)
		})
	}
}
