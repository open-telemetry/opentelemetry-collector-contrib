// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint:errcheck,gocritic
package spanmetricsprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/internal/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/mocks"
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

	sampleRegion          = "us-east-1"
	sampleLatency         = float64(11)
	sampleLatencyDuration = time.Duration(sampleLatency) * time.Millisecond
)

// metricID represents the minimum attributes that uniquely identifies a metric in our tests.
type metricID struct {
	service    string
	operation  string
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
	operation  string
	kind       ptrace.SpanKind
	statusCode ptrace.StatusCode
}

func TestProcessorStart(t *testing.T) {
	// Create otlp exporters.
	otlpConfig, mexp, texp := newOTLPExporters(t)

	for _, tc := range []struct {
		name            string
		exporter        component.Exporter
		metricsExporter string
		wantErrorMsg    string
	}{
		{"export to active otlp metrics exporter", mexp, "otlp", ""},
		{"unable to find configured exporter in active exporter list", mexp, "prometheus", "failed to find metrics exporter: 'prometheus'; please configure metrics_exporter from one of: [otlp]"},
		{"export to active otlp traces exporter should error", texp, "otlp", "the exporter \"otlp\" isn't a metrics exporter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			exporters := map[config.DataType]map[config.ComponentID]component.Exporter{
				config.MetricsDataType: {
					otlpConfig.ID(): tc.exporter,
				},
			}
			mhost := &mocks.Host{}
			mhost.On("GetExporters").Return(exporters)

			// Create spanmetrics processor
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.MetricsExporter = tc.metricsExporter

			procCreationParams := componenttest.NewNopProcessorCreateSettings()
			traceProcessor, err := factory.CreateTracesProcessor(context.Background(), procCreationParams, cfg, consumertest.NewNop())
			require.NoError(t, err)

			// Test
			smp := traceProcessor.(*processorImp)
			err = smp.Start(context.Background(), mhost)

			// Verify
			if tc.wantErrorMsg != "" {
				assert.EqualError(t, err, tc.wantErrorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessorShutdown(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)
	assert.NoError(t, err)
	err = p.Shutdown(context.Background())

	// Verify
	assert.NoError(t, err)
}

func TestConfigureLatencyBounds(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.LatencyHistogramBuckets = []time.Duration{
		3 * time.Nanosecond,
		3 * time.Microsecond,
		3 * time.Millisecond,
		3 * time.Second,
	}

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, []float64{0.000003, 0.003, 3, 3000, maxDurationMs}, p.latencyBounds)
}

func TestProcessorCapabilities(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)
	assert.NoError(t, err)
	caps := p.Capabilities()

	// Verify
	assert.NotNil(t, p)
	assert.Equal(t, false, caps.MutatesData)
}

func TestProcessorConsumeTracesErrors(t *testing.T) {
	for _, tc := range []struct {
		name              string
		consumeMetricsErr error
		consumeTracesErr  error
		wantErrMsg        string
	}{
		{
			name:              "metricsExporter error",
			consumeMetricsErr: fmt.Errorf("metricsExporter error"),
			wantErrMsg:        "metricsExporter error",
		},
		{
			name:             "nextConsumer error",
			consumeTracesErr: fmt.Errorf("nextConsumer error"),
			wantErrMsg:       "nextConsumer error",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			mexp := &mocks.MetricsExporter{}
			mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(tc.consumeMetricsErr)

			tcon := &mocks.TracesConsumer{}
			tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(tc.consumeTracesErr)

			p := newProcessorImp(mexp, tcon, nil, cumulative, t)

			traces := buildSampleTrace()

			// Test
			ctx := metadata.NewIncomingContext(context.Background(), nil)
			err := p.ConsumeTraces(ctx, traces)

			// Verify
			assert.EqualError(t, err, tc.wantErrMsg)
		})
	}
}

func TestProcessorConsumeTraces(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name                   string
		aggregationTemporality string
		verifier               func(t testing.TB, input pmetric.Metrics) bool
		traces                 []ptrace.Traces
	}{
		{
			name:                   "Test single consumption, three spans (Cumulative).",
			aggregationTemporality: cumulative,
			verifier:               verifyConsumeMetricsInputCumulative,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta).",
			aggregationTemporality: delta,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative).",
			aggregationTemporality: cumulative,
			verifier:               verifyMultipleCumulativeConsumptions(),
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta).",
			aggregationTemporality: delta,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			mexp := &mocks.MetricsExporter{}
			tcon := &mocks.TracesConsumer{}

			// Mocked metric exporter will perform validation on metrics, during p.ConsumeTraces()
			mexp.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pmetric.Metrics) bool {
				return tc.verifier(t, input)
			})).Return(nil)
			tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

			defaultNullValue := "defaultNullValue"
			p := newProcessorImp(mexp, tcon, &defaultNullValue, tc.aggregationTemporality, t)

			for _, traces := range tc.traces {
				// Test
				ctx := metadata.NewIncomingContext(context.Background(), nil)
				err := p.ConsumeTraces(ctx, traces)

				// Verify
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricKeyCache(t *testing.T) {
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := "defaultNullValue"
	p := newProcessorImp(mexp, tcon, &defaultNullValue, cumulative, t)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// 0 key was cached at beginning
	assert.Empty(t, p.metricKeyToDimensions.Keys())

	err := p.ConsumeTraces(ctx, traces)
	// Validate
	require.NoError(t, err)
	// 2 key was cached, 1 key was evicted and cleaned after the processing
	assert.Len(t, p.metricKeyToDimensions.Keys(), DimensionsCacheSize)

	// consume another batch of traces
	err = p.ConsumeTraces(ctx, traces)
	// 2 key was cached, other keys were evicted and cleaned after the processing
	assert.Len(t, p.metricKeyToDimensions.Keys(), DimensionsCacheSize)

	require.NoError(t, err)
}

func BenchmarkProcessorConsumeTraces(b *testing.B) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := "defaultNullValue"
	p := newProcessorImp(mexp, tcon, &defaultNullValue, cumulative, b)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	for n := 0; n < b.N; n++ {
		p.ConsumeTraces(ctx, traces)
	}
}

func newProcessorImp(mexp *mocks.MetricsExporter, tcon *mocks.TracesConsumer, defaultNullValue *string, temporality string, tb testing.TB) *processorImp {
	defaultNotInSpanAttrVal := "defaultNotInSpanAttrVal"
	// use size 2 for LRU cache for testing purpose
	metricKeyToDimensions, err := cache.NewCache(DimensionsCacheSize)
	if err != nil {
		panic(err)
	}
	return &processorImp{
		logger:          zaptest.NewLogger(tb),
		config:          Config{AggregationTemporality: temporality},
		metricsExporter: mexp,
		nextConsumer:    tcon,

		startTime:            time.Now(),
		callSum:              make(map[metricKey]int64),
		latencySum:           make(map[metricKey]float64),
		latencyCount:         make(map[metricKey]uint64),
		latencyBucketCounts:  make(map[metricKey][]uint64),
		latencyBounds:        defaultLatencyHistogramBucketsMs,
		latencyExemplarsData: make(map[metricKey][]exemplarData),
		dimensions: []Dimension{
			// Set nil defaults to force a lookup for the attribute in the span.
			{stringAttrName, nil},
			{intAttrName, nil},
			{doubleAttrName, nil},
			{boolAttrName, nil},
			{mapAttrName, nil},
			{arrayAttrName, nil},
			{nullAttrName, defaultNullValue},
			// Add a default value for an attribute that doesn't exist in a span
			{notInSpanAttrName0, &defaultNotInSpanAttrVal},
			// Leave the default value unset to test that this dimension should not be added to the metric.
			{notInSpanAttrName1, nil},
			// Add a resource attribute to test "process" attributes like IP, host, region, cluster, etc.
			{regionResourceAttrName, nil},
		},
		metricKeyToDimensions: metricKeyToDimensions,
	}
}

// verifyConsumeMetricsInputCumulative expects one accumulation of metrics, and marked as cumulative
func verifyConsumeMetricsInputCumulative(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.MetricAggregationTemporalityCumulative, 1)
}

// verifyConsumeMetricsInputDelta expects one accumulation of metrics, and marked as delta
func verifyConsumeMetricsInputDelta(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.MetricAggregationTemporalityDelta, 1)
}

// verifyMultipleCumulativeConsumptions expects the amount of accumulations as kept track of by numCumulativeConsumptions.
// numCumulativeConsumptions acts as a multiplier for the values, since the cumulative metrics are additive.
func verifyMultipleCumulativeConsumptions() func(t testing.TB, input pmetric.Metrics) bool {
	numCumulativeConsumptions := 0
	return func(t testing.TB, input pmetric.Metrics) bool {
		numCumulativeConsumptions++
		return verifyConsumeMetricsInput(t, input, pmetric.MetricAggregationTemporalityCumulative, numCumulativeConsumptions)
	}
}

// verifyConsumeMetricsInput verifies the input of the ConsumeMetrics call from this processor.
// This is the best point to verify the computed metrics from spans are as expected.
func verifyConsumeMetricsInput(t testing.TB, input pmetric.Metrics, expectedTemporality pmetric.MetricAggregationTemporality, numCumulativeConsumptions int) bool {
	require.Equal(t, 6, input.MetricCount(),
		"Should be 3 for each of call count and latency. Each group of 3 metrics is made of: "+
			"service-a (server kind) -> service-a (client kind) -> service-b (service kind)",
	)

	rm := input.ResourceMetrics()
	require.Equal(t, 1, rm.Len())

	ilm := rm.At(0).ScopeMetrics()
	require.Equal(t, 1, ilm.Len())
	assert.Equal(t, "spanmetricsprocessor", ilm.At(0).Scope().Name())

	m := ilm.At(0).Metrics()
	require.Equal(t, 6, m.Len())

	seenMetricIDs := make(map[metricID]bool)
	mi := 0
	// The first 3 metrics are for call counts.
	for ; mi < 3; mi++ {
		assert.Equal(t, "calls_total", m.At(mi).Name())

		data := m.At(mi).Sum()
		assert.Equal(t, expectedTemporality, data.AggregationTemporality())
		assert.True(t, data.IsMonotonic())

		dps := data.DataPoints()
		require.Equal(t, 1, dps.Len())

		dp := dps.At(0)
		assert.Equal(t, int64(numCumulativeConsumptions), dp.IntVal(), "There should only be one metric per Service/operation/kind combination")
		assert.NotZero(t, dp.StartTimestamp(), "StartTimestamp should be set")
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")

		verifyMetricLabels(dp, t, seenMetricIDs)
	}

	seenMetricIDs = make(map[metricID]bool)
	// The remaining metrics are for latency.
	for ; mi < m.Len(); mi++ {
		assert.Equal(t, "latency", m.At(mi).Name())

		data := m.At(mi).Histogram()
		assert.Equal(t, expectedTemporality, data.AggregationTemporality())

		dps := data.DataPoints()
		require.Equal(t, 1, dps.Len())

		dp := dps.At(0)
		assert.Equal(t, sampleLatency*float64(numCumulativeConsumptions), dp.Sum(), "Should be a 11ms latency measurement, multiplied by the number of stateful accumulations.")
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")

		// Verify bucket counts. Firstly, find the bucket index where the 11ms latency should belong in.
		var foundLatencyIndex int
		for foundLatencyIndex = 0; foundLatencyIndex < dp.ExplicitBounds().Len(); foundLatencyIndex++ {
			if dp.ExplicitBounds().At(foundLatencyIndex) > sampleLatency {
				break
			}
		}

		// Then verify that all histogram buckets are empty except for the bucket with the 11ms latency.
		var wantBucketCount uint64
		for bi := 0; bi < dp.BucketCounts().Len(); bi++ {
			wantBucketCount = 0
			if bi == foundLatencyIndex {
				wantBucketCount = uint64(numCumulativeConsumptions)
			}
			assert.Equal(t, wantBucketCount, dp.BucketCounts().At(bi))
		}
		verifyMetricLabels(dp, t, seenMetricIDs)
	}
	return true
}

func verifyMetricLabels(dp metricDataPoint, t testing.TB, seenMetricIDs map[metricID]bool) {
	mID := metricID{}
	wantDimensions := map[string]pcommon.Value{
		stringAttrName:         pcommon.NewValueString("stringAttrValue"),
		intAttrName:            pcommon.NewValueInt(99),
		doubleAttrName:         pcommon.NewValueDouble(99.99),
		boolAttrName:           pcommon.NewValueBool(true),
		nullAttrName:           pcommon.NewValueEmpty(),
		arrayAttrName:          pcommon.NewValueSlice(),
		mapAttrName:            pcommon.NewValueMap(),
		notInSpanAttrName0:     pcommon.NewValueString("defaultNotInSpanAttrVal"),
		regionResourceAttrName: pcommon.NewValueString(sampleRegion),
	}
	dp.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case serviceNameKey:
			mID.service = v.StringVal()
		case operationKey:
			mID.operation = v.StringVal()
		case spanKindKey:
			mID.kind = v.StringVal()
		case statusCodeKey:
			mID.statusCode = v.StringVal()
		case notInSpanAttrName1:
			assert.Fail(t, notInSpanAttrName1+" should not be in this metric")
		default:
			assert.Equal(t, wantDimensions[k], v)
			delete(wantDimensions, k)
		}
		return true
	})
	assert.Empty(t, wantDimensions, "Did not see all expected dimensions in metric. Missing: ", wantDimensions)

	// Service/operation/kind should be a unique metric.
	assert.False(t, seenMetricIDs[mID])
	seenMetricIDs[mID] = true
}

// buildSampleTrace builds the following trace:
//   service-a/ping (server) ->
//     service-a/ping (client) ->
//       service-b/ping (server)
func buildSampleTrace() ptrace.Traces {
	traces := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					operation:  "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
				},
				{
					operation:  "/ping",
					kind:       ptrace.SpanKindClient,
					statusCode: ptrace.StatusCodeOk,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					operation:  "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(serviceSpans{}, traces.ResourceSpans().AppendEmpty())
	return traces
}

func initServiceSpans(serviceSpans serviceSpans, spans ptrace.ResourceSpans) {
	if serviceSpans.serviceName != "" {
		spans.Resource().Attributes().
			InsertString(conventions.AttributeServiceName, serviceSpans.serviceName)
	}

	spans.Resource().Attributes().InsertString(regionResourceAttrName, sampleRegion)

	ils := spans.ScopeSpans().AppendEmpty()
	for _, span := range serviceSpans.spans {
		initSpan(span, ils.Spans().AppendEmpty())
	}
}

func initSpan(span span, s ptrace.Span) {
	s.SetName(span.operation)
	s.SetKind(span.kind)
	s.Status().SetCode(span.statusCode)
	now := time.Now()
	s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(sampleLatencyDuration)))
	s.Attributes().InsertString(stringAttrName, "stringAttrValue")
	s.Attributes().InsertInt(intAttrName, 99)
	s.Attributes().InsertDouble(doubleAttrName, 99.99)
	s.Attributes().InsertBool(boolAttrName, true)
	s.Attributes().InsertNull(nullAttrName)
	s.Attributes().Insert(mapAttrName, pcommon.NewValueMap())
	s.Attributes().Insert(arrayAttrName, pcommon.NewValueSlice())
	s.SetTraceID(pcommon.NewTraceID([16]byte{byte(42)}))
}

func newOTLPExporters(t *testing.T) (*otlpexporter.Config, component.MetricsExporter, component.TracesExporter) {
	otlpExpFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("otlp")),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}
	expCreationParams := componenttest.NewNopExporterCreateSettings()
	mexp, err := otlpExpFactory.CreateMetricsExporter(context.Background(), expCreationParams, otlpConfig)
	require.NoError(t, err)
	texp, err := otlpExpFactory.CreateTracesExporter(context.Background(), expCreationParams, otlpConfig)
	require.NoError(t, err)
	return otlpConfig, mexp, texp
}

func TestBuildKeySameServiceOperationCharSequence(t *testing.T) {
	span0 := ptrace.NewSpan()
	span0.SetName("c")
	k0 := buildKey("ab", span0, nil, pcommon.NewMap())

	span1 := ptrace.NewSpan()
	span1.SetName("bc")
	k1 := buildKey("a", span1, nil, pcommon.NewMap())

	assert.NotEqual(t, k0, k1)
	assert.Equal(t, metricKey("ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k0)
	assert.Equal(t, metricKey("a\u0000bc\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k1)
}

func TestBuildKeyWithDimensions(t *testing.T) {
	defaultFoo := "bar"
	for _, tc := range []struct {
		name            string
		optionalDims    []Dimension
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
			optionalDims: []Dimension{
				{Name: "foo", Default: &defaultFoo},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000bar",
		},
		{
			name: "neither span nor resource contains key, dim provides no default",
			optionalDims: []Dimension{
				{Name: "foo"},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "span attribute contains dimension",
			optionalDims: []Dimension{
				{Name: "foo"},
			},
			spanAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "resource attribute contains dimension",
			optionalDims: []Dimension{
				{Name: "foo"},
			},
			resourceAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "both span and resource attribute contains dimension, should prefer span attribute",
			optionalDims: []Dimension{
				{Name: "foo"},
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
			resAttr := pcommon.NewMapFromRaw(tc.resourceAttrMap)
			span0 := ptrace.NewSpan()
			pcommon.NewMapFromRaw(tc.spanAttrMap).CopyTo(span0.Attributes())
			span0.SetName("c")
			k := buildKey("ab", span0, tc.optionalDims, resAttr)

			assert.Equal(t, metricKey(tc.wantKey), k)
		})
	}
}

func TestProcessorDuplicateDimensions(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Duplicate dimension with reserved label after sanitization.
	cfg.Dimensions = []Dimension{
		{Name: "status_code"},
	}

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestValidateDimensions(t *testing.T) {
	for _, tc := range []struct {
		name              string
		dimensions        []Dimension
		expectedErr       string
		skipSanitizeLabel bool
	}{
		{
			name:       "no additional dimensions",
			dimensions: []Dimension{},
		},
		{
			name: "no duplicate dimensions",
			dimensions: []Dimension{
				{Name: "http.service_name"},
				{Name: "http.status_code"},
			},
		},
		{
			name: "duplicate dimension with reserved labels",
			dimensions: []Dimension{
				{Name: "service.name"},
			},
			expectedErr: "duplicate dimension name service.name",
		},
		{
			name: "duplicate dimension with reserved labels after sanitization",
			dimensions: []Dimension{
				{Name: "service_name"},
			},
			expectedErr: "duplicate dimension name service_name",
		},
		{
			name: "duplicate additional dimensions",
			dimensions: []Dimension{
				{Name: "service_name"},
				{Name: "service_name"},
			},
			expectedErr: "duplicate dimension name service_name",
		},
		{
			name: "duplicate additional dimensions after sanitization",
			dimensions: []Dimension{
				{Name: "http.status_code"},
				{Name: "http!status_code"},
			},
			expectedErr: "duplicate dimension name http_status_code after sanitization",
		},
		{
			name: "we skip the case if the dimension name is the same after sanitization",
			dimensions: []Dimension{
				{Name: "http_status_code"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.skipSanitizeLabel = false
			err := validateDimensions(tc.dimensions, tc.skipSanitizeLabel)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.Equal(t, "", sanitize("", cfg.skipSanitizeLabel), "")
	require.Equal(t, "key_test", sanitize("_test", cfg.skipSanitizeLabel))
	require.Equal(t, "key__test", sanitize("__test", cfg.skipSanitizeLabel))
	require.Equal(t, "key_0test", sanitize("0test", cfg.skipSanitizeLabel))
	require.Equal(t, "test", sanitize("test", cfg.skipSanitizeLabel))
	require.Equal(t, "test__", sanitize("test_/", cfg.skipSanitizeLabel))
	//testcases with skipSanitizeLabel flag turned on
	cfg.skipSanitizeLabel = true
	require.Equal(t, "", sanitize("", cfg.skipSanitizeLabel), "")
	require.Equal(t, "_test", sanitize("_test", cfg.skipSanitizeLabel))
	require.Equal(t, "key__test", sanitize("__test", cfg.skipSanitizeLabel))
	require.Equal(t, "key_0test", sanitize("0test", cfg.skipSanitizeLabel))
	require.Equal(t, "test", sanitize("test", cfg.skipSanitizeLabel))
	require.Equal(t, "test__", sanitize("test_/", cfg.skipSanitizeLabel))
}

func TestSetLatencyExemplars(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	traces := buildSampleTrace()
	traceID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
	exemplarSlice := pmetric.NewExemplarSlice()
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	value := float64(42)

	ed := []exemplarData{{traceID: traceID, value: value}}

	// ----- call -------------------------------------------------------------
	setLatencyExemplars(ed, timestamp, exemplarSlice)

	// ----- verify -----------------------------------------------------------
	traceIDValue, exist := exemplarSlice.At(0).FilteredAttributes().Get(traceIDKey)

	assert.NotEmpty(t, exemplarSlice)
	assert.True(t, exist)
	assert.Equal(t, traceIDValue.AsString(), traceID.HexString())
	assert.Equal(t, exemplarSlice.At(0).Timestamp(), timestamp)
	assert.Equal(t, exemplarSlice.At(0).DoubleVal(), value)
}

func TestProcessorUpdateLatencyExemplars(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	traces := buildSampleTrace()
	traceID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
	key := metricKey("metricKey")
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)
	value := float64(42)

	// ----- call -------------------------------------------------------------
	p.updateLatencyExemplars(key, value, traceID)

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.NotEmpty(t, p.latencyExemplarsData[key])
	assert.Equal(t, p.latencyExemplarsData[key][0], exemplarData{traceID: traceID, value: value})
}

func TestProcessorResetExemplarData(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	key := metricKey("metricKey")
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)

	// ----- call -------------------------------------------------------------
	p.resetExemplarData()

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.Empty(t, p.latencyExemplarsData[key])
}
