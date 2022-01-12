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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/internal/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/keybuilder"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/mocks"
)

const (
	stringAttrName              = "stringAttrName"
	intAttrName                 = "intAttrName"
	doubleAttrName              = "doubleAttrName"
	boolAttrName                = "boolAttrName"
	nullAttrName                = "nullAttrName"
	mapAttrName                 = "mapAttrName"
	arrayAttrName               = "arrayAttrName"
	notInSpanAttrName0          = "shouldBeInMetric"
	notInSpanAttrName1          = "shouldNotBeInMetric"
	regionResourceAttrName      = "region"
	DimensionsCacheSize         = 2
	ResourceAttributesCacheSize = 2

	resourceAttr1          = "resourceAttr1"
	resourceAttr2          = "resourceAttr2"
	notInSpanResourceAttr0 = "resourceAttrShouldBeInMetric"
	notInSpanResourceAttr1 = "resourceAttrShouldNotBeInMetric"

	defaultNotInSpanAttrVal = "defaultNotInSpanAttrVal"

	sampleRegion          = "us-east-1"
	sampleLatency         = float64(11)
	sampleLatencyDuration = time.Duration(sampleLatency) * time.Millisecond
)

// metricID represents the minimum attributes that uniquely identifies a metric in our tests.
type metricID struct {
	operation  string
	kind       string
	statusCode string
}

type metricDataPoint interface {
	Attributes() pdata.AttributeMap
}

type serviceSpans struct {
	serviceName string
	spans       []span
}

type span struct {
	operation  string
	kind       pdata.SpanKind
	statusCode pdata.StatusCode
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

func TestProcessorConsumeTracesConcurrentSafe(t *testing.T) {
	testcases := []struct {
		name                   string
		aggregationTemporality string
		traces                 []pdata.Traces
	}{
		{
			name:                   "Test single consumption, three spans (Cumulative).",
			aggregationTemporality: cumulative,
			traces:                 []pdata.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta).",
			aggregationTemporality: delta,
			traces:                 []pdata.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative).",
			aggregationTemporality: cumulative,
			traces:                 []pdata.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta).",
			aggregationTemporality: delta,
			traces:                 []pdata.Traces{buildSampleTrace(), buildSampleTrace()},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Prepare
			mexp := &mocks.MetricsExporter{}
			tcon := &mocks.TracesConsumer{}

			mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
			tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

			defaultNullValue := "defaultNullValue"
			p := newProcessorImp(mexp, tcon, &defaultNullValue, tc.aggregationTemporality, t)

			for _, traces := range tc.traces {
				// Test
				traces := traces
				// create an excessive concurrent usage. The processor will not be used in this way practically.
				// Run the test to verify this public function ConsumeTraces() doesn't cause race conditions.
				go func() {
					ctx := metadata.NewIncomingContext(context.Background(), nil)
					err := p.ConsumeTraces(ctx, traces)
					assert.NoError(t, err)
				}()
			}
		})
	}
}

func TestProcessorConsumeTraces(t *testing.T) {
	testcases := []struct {
		name                   string
		aggregationTemporality string
		verifier               func(t testing.TB, input pdata.Metrics) bool
		traces                 []pdata.Traces
	}{
		{
			name:                   "Test single consumption, three spans (Cumulative).",
			aggregationTemporality: cumulative,
			verifier:               verifyConsumeMetricsInputCumulative,
			traces:                 []pdata.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta).",
			aggregationTemporality: delta,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []pdata.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative).",
			aggregationTemporality: cumulative,
			verifier:               verifyMultipleCumulativeConsumptions(),
			traces:                 []pdata.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta).",
			aggregationTemporality: delta,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []pdata.Traces{buildSampleTrace(), buildSampleTrace()},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Prepare
			mexp := &mocks.MetricsExporter{}
			tcon := &mocks.TracesConsumer{}

			// Mocked metric exporter will perform validation on metrics, during p.ConsumeTraces()
			mexp.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pdata.Metrics) bool {
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

func TestResourceCopying(t *testing.T) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pdata.Metrics) bool {
		rm := input.ResourceMetrics()
		require.Equal(t, 2, rm.Len())

		var rmA, rmB pdata.ResourceMetrics
		rm0 := rm.At(0)
		rm1 := rm.At(1)
		serviceName, ok := getServiceName(rm0)
		assert.True(t, ok, "should get service name from resourceMetric")

		if serviceName == "service-a" {
			rmA = rm0
			rmB = rm1
		} else {
			rmB = rm0
			rmA = rm1
		}

		require.Equal(t, 4, rmA.Resource().Attributes().Len())
		require.Equal(t, 4, rmA.InstrumentationLibraryMetrics().At(0).Metrics().Len())
		require.Equal(t, 2, rmB.Resource().Attributes().Len())
		require.Equal(t, 2, rmB.InstrumentationLibraryMetrics().At(0).Metrics().Len())

		wantResourceAttrServiceA := map[string]string{
			resourceAttr1:          "1",
			resourceAttr2:          "2",
			notInSpanResourceAttr0: defaultNotInSpanAttrVal,
			serviceNameKey:         "service-a",
		}
		rmA.Resource().Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			value := v.StringVal()
			switch k {
			case notInSpanResourceAttr1:
				assert.Fail(t, notInSpanResourceAttr1+" should not be in this metric")
			default:
				assert.Equal(t, wantResourceAttrServiceA[k], value)
				delete(wantResourceAttrServiceA, k)
			}
			return true
		})
		assert.Empty(t, wantResourceAttrServiceA, "Did not see all expected dimensions in metric. Missing: ", wantResourceAttrServiceA)

		wantResourceAttrServiceB := map[string]string{
			notInSpanResourceAttr0: defaultNotInSpanAttrVal,
			serviceNameKey:         "service-b",
		}
		rmB.Resource().Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			value := v.StringVal()
			switch k {
			case notInSpanResourceAttr1:
				assert.Fail(t, notInSpanResourceAttr1+" should not be in this metric")
			default:
				assert.Equal(t, wantResourceAttrServiceB[k], value)
				delete(wantResourceAttrServiceB, k)
			}
			return true
		})
		assert.Empty(t, wantResourceAttrServiceB, "Must have no attributes listed inside the slice. Missing: ", wantResourceAttrServiceB)

		return true
	})).Return(nil)

	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := "defaultNullValue"
	p := newProcessorImp(mexp, tcon, &defaultNullValue, cumulative, t)

	traces := buildSampleTrace()
	traces.ResourceSpans().At(0).Resource().Attributes().Insert(resourceAttr1, pdata.NewAttributeValueString("1"))
	traces.ResourceSpans().At(0).Resource().Attributes().Insert(resourceAttr2, pdata.NewAttributeValueString("2"))

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err := p.ConsumeTraces(ctx, traces)

	// Verify
	assert.NoError(t, err)
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
	localDefaultNotInSpanAttrVal := defaultNotInSpanAttrVal
	// use size 2 for LRU cache for testing purpose
	metricKeyToDimensions, err := cache.NewCache(DimensionsCacheSize)
	if err != nil {
		panic(err)
	}

	resourceKeyToDimensions, err := cache.NewCache(ResourceAttributesCacheSize)
	if err != nil {
		panic(err)
	}

	return &processorImp{
		logger:          zaptest.NewLogger(tb),
		config:          Config{AggregationTemporality: temporality},
		metricsExporter: mexp,
		nextConsumer:    tcon,

		startTime:            time.Now(),
		callSum:              make(map[resourceKey]map[metricKey]int64),
		latencySum:           make(map[resourceKey]map[metricKey]float64),
		latencyCount:         make(map[resourceKey]map[metricKey]uint64),
		latencyBucketCounts:  make(map[resourceKey]map[metricKey][]uint64),
		latencyBounds:        defaultLatencyHistogramBucketsMs,
		latencyExemplarsData: make(map[resourceKey]map[metricKey][]exemplarData),
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
			{notInSpanAttrName0, &localDefaultNotInSpanAttrVal},
			// Leave the default value unset to test that this dimension should not be added to the metric.
			{notInSpanAttrName1, nil},
			// Add a resource attribute to test "process" attributes like IP, host, region, cluster, etc.
			{regionResourceAttrName, nil},
		},
		resourceAttributes: []Dimension{
			{resourceAttr1, nil},
			{resourceAttr2, nil},
			{notInSpanResourceAttr0, &localDefaultNotInSpanAttrVal},
			{notInSpanResourceAttr1, nil},
		},
		resourceKeyToDimensions: resourceKeyToDimensions,
		metricKeyToDimensions:   metricKeyToDimensions,
	}
}

// verifyConsumeMetricsInputCumulative expects one accumulation of metrics, and marked as cumulative
func verifyConsumeMetricsInputCumulative(t testing.TB, input pdata.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pdata.MetricAggregationTemporalityCumulative, 1)
}

// verifyConsumeMetricsInputDelta expects one accumulation of metrics, and marked as delta
func verifyConsumeMetricsInputDelta(t testing.TB, input pdata.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pdata.MetricAggregationTemporalityDelta, 1)
}

// verifyMultipleCumulativeConsumptions expects the amount of accumulations as kept track of by numCumulativeConsumptions.
// numCumulativeConsumptions acts as a multiplier for the values, since the cumulative metrics are additive.
func verifyMultipleCumulativeConsumptions() func(t testing.TB, input pdata.Metrics) bool {
	numCumulativeConsumptions := 0
	return func(t testing.TB, input pdata.Metrics) bool {
		numCumulativeConsumptions++
		return verifyConsumeMetricsInput(t, input, pdata.MetricAggregationTemporalityCumulative, numCumulativeConsumptions)
	}
}

// verifyConsumeMetricsInput verifies the input of the ConsumeMetrics call from this processor.
// This is the best point to verify the computed metrics from spans are as expected.
func verifyConsumeMetricsInput(t testing.TB, input pdata.Metrics, expectedTemporality pdata.MetricAggregationTemporality, numCumulativeConsumptions int) bool {
	require.Equal(t, 6, input.MetricCount(),
		"Should be 3 for each of call count and latency. Each group of 3 metrics is made of: "+
			"service-a (server kind) -> service-a (client kind) -> service-b (service kind)",
	)

	rm := input.ResourceMetrics()
	require.Equal(t, 2, rm.Len())

	var rmA, rmB *pdata.ResourceMetrics
	rm0 := rm.At(0)
	rm1 := rm.At(1)
	name, ok := getServiceName(rm0)
	assert.True(t, ok, "should get service name from resourceMetric")

	if name == "service-a" {
		rmA = &rm0
		rmB = &rm1
	} else {
		rmA = &rm1
		rmB = &rm0
	}

	ilmA := rmA.InstrumentationLibraryMetrics()

	require.Equal(t, 1, ilmA.Len())
	assert.Equal(t, instrumentationLibraryName, ilmA.At(0).InstrumentationLibrary().Name())

	mA := ilmA.At(0).Metrics()
	require.Equal(t, 4, mA.Len())

	ilmB := rmB.InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilmB.Len())
	assert.Equal(t, instrumentationLibraryName, ilmB.At(0).InstrumentationLibrary().Name())

	mB := ilmB.At(0).Metrics()
	require.Equal(t, 2, mB.Len())

	verifyMetrics(mA, expectedTemporality, numCumulativeConsumptions, 2, t)
	verifyMetrics(mB, expectedTemporality, numCumulativeConsumptions, 1, t)

	return true
}

func getServiceName(m pdata.ResourceMetrics) (string, bool) {
	if val, ok := m.Resource().Attributes().Get(serviceNameKey); ok {
		return val.AsString(), true
	}

	return "", false
}

func verifyMetrics(m pdata.MetricSlice, expectedTemporality pdata.MetricAggregationTemporality, numCumulativeConsumptions int, numOfCallCounts int, t testing.TB) {
	seenMetricIDs := make(map[metricID]bool)
	mi := 0
	// The first <numOfCallCounts> metrics are for call counts.
	for ; mi < numOfCallCounts; mi++ {
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

		if expectedTemporality == pdata.MetricAggregationTemporalityDelta {
			assert.Equal(t, sampleLatency, dp.Sum(), "Should be a single 11ms latency measurement")
		} else {
			assert.Equal(t, sampleLatency*float64(numCumulativeConsumptions), dp.Sum(), "Should be cumulative latency measurement")
		}

		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")

		// Verify bucket counts. Firstly, find the bucket index where the 11ms latency should belong in.
		var foundLatencyIndex int
		for foundLatencyIndex = 0; foundLatencyIndex < len(dp.ExplicitBounds()); foundLatencyIndex++ {
			if dp.ExplicitBounds()[foundLatencyIndex] > sampleLatency {
				break
			}
		}

		// Then verify that all histogram buckets are empty except for the bucket with the 11ms latency.
		var wantBucketCount uint64
		for bi := 0; bi < len(dp.BucketCounts()); bi++ {
			wantBucketCount = 0
			if bi == foundLatencyIndex {
				if expectedTemporality == pdata.MetricAggregationTemporalityDelta {
					wantBucketCount = 1
				} else {
					wantBucketCount = uint64(1 * numCumulativeConsumptions)
				}
			}
			assert.Equal(t, wantBucketCount, dp.BucketCounts()[bi])
		}
		verifyMetricLabels(dp, t, seenMetricIDs)
	}
}

func verifyMetricLabels(dp metricDataPoint, t testing.TB, seenMetricIDs map[metricID]bool) {
	mID := metricID{}
	wantDimensions := map[string]pdata.AttributeValue{
		stringAttrName:         pdata.NewAttributeValueString("stringAttrValue"),
		intAttrName:            pdata.NewAttributeValueInt(99),
		doubleAttrName:         pdata.NewAttributeValueDouble(99.99),
		boolAttrName:           pdata.NewAttributeValueBool(true),
		nullAttrName:           pdata.NewAttributeValueEmpty(),
		arrayAttrName:          pdata.NewAttributeValueArray(),
		mapAttrName:            pdata.NewAttributeValueMap(),
		notInSpanAttrName0:     pdata.NewAttributeValueString(defaultNotInSpanAttrVal),
		regionResourceAttrName: pdata.NewAttributeValueString(sampleRegion),
	}
	dp.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch k {
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
func buildSampleTrace() pdata.Traces {
	traces := pdata.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					operation:  "/ping",
					kind:       pdata.SpanKindServer,
					statusCode: pdata.StatusCodeOk,
				},
				{
					operation:  "/ping",
					kind:       pdata.SpanKindClient,
					statusCode: pdata.StatusCodeOk,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					operation:  "/ping",
					kind:       pdata.SpanKindServer,
					statusCode: pdata.StatusCodeError,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(serviceSpans{}, traces.ResourceSpans().AppendEmpty())
	return traces
}

func initServiceSpans(serviceSpans serviceSpans, spans pdata.ResourceSpans) {
	if serviceSpans.serviceName != "" {
		spans.Resource().Attributes().
			InsertString(conventions.AttributeServiceName, serviceSpans.serviceName)
	}

	spans.Resource().Attributes().InsertString(regionResourceAttrName, sampleRegion)

	ils := spans.InstrumentationLibrarySpans().AppendEmpty()
	for _, span := range serviceSpans.spans {
		initSpan(span, ils.Spans().AppendEmpty())
	}
}

func initSpan(span span, s pdata.Span) {
	s.SetName(span.operation)
	s.SetKind(span.kind)
	s.Status().SetCode(span.statusCode)
	now := time.Now()
	s.SetStartTimestamp(pdata.NewTimestampFromTime(now))
	s.SetEndTimestamp(pdata.NewTimestampFromTime(now.Add(sampleLatencyDuration)))
	s.Attributes().InsertString(stringAttrName, "stringAttrValue")
	s.Attributes().InsertInt(intAttrName, 99)
	s.Attributes().InsertDouble(doubleAttrName, 99.99)
	s.Attributes().InsertBool(boolAttrName, true)
	s.Attributes().InsertNull(nullAttrName)
	s.Attributes().Insert(mapAttrName, pdata.NewAttributeValueMap())
	s.Attributes().Insert(arrayAttrName, pdata.NewAttributeValueArray())
	s.SetTraceID(pdata.NewTraceID([16]byte{byte(42)}))
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
	//setup
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zap.NewNop(), cfg, next)
	assert.NoError(t, err)

	trace0 := pdata.NewTraces()
	rSpan0 := trace0.ResourceSpans().AppendEmpty()
	ilSpan0 := rSpan0.InstrumentationLibrarySpans().AppendEmpty()
	span0 := ilSpan0.Spans().AppendEmpty()
	span0.SetName("c")
	k0 := p.buildMetricKey(span0, pdata.NewAttributeMap())
	rk0 := p.buildResourceAttrKey("ab", rSpan0.Resource().Attributes())

	trace1 := pdata.NewTraces()
	rSpan1 := trace1.ResourceSpans().AppendEmpty()
	ilSpan1 := rSpan1.InstrumentationLibrarySpans().AppendEmpty()
	span1 := ilSpan1.Spans().AppendEmpty()
	span1.SetName("bc")
	k1 := p.buildMetricKey(span1, pdata.NewAttributeMap())
	rk1 := p.buildResourceAttrKey("a", rSpan1.Resource().Attributes())

	assert.NotEqual(t, k0, k1)
	assert.Equal(t, metricKey("c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k0)
	assert.Equal(t, metricKey("bc\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k1)
	assert.Equal(t, resourceKey("ab"), rk0)
	assert.Equal(t, resourceKey("a"), rk1)
}

func TestBuildKeyWithDimensions(t *testing.T) {
	defaultFoo := "bar"
	for _, tc := range []struct {
		name            string
		optionalDims    []Dimension
		resourceAttrMap map[string]pdata.AttributeValue
		spanAttrMap     map[string]pdata.AttributeValue
		wantKey         string
	}{
		{
			name:    "nil optionalDims",
			wantKey: "c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "neither span nor resource contains key, dim provides default",
			optionalDims: []Dimension{
				{Name: "foo", Default: &defaultFoo},
			},
			wantKey: "c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000bar",
		},
		{
			name: "neither span nor resource contains key, dim provides no default",
			optionalDims: []Dimension{
				{Name: "foo"},
			},
			wantKey: "c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "span attribute contains dimension",
			optionalDims: []Dimension{
				{Name: "foo"},
			},
			spanAttrMap: map[string]pdata.AttributeValue{
				"foo": pdata.NewAttributeValueInt(99),
			},
			wantKey: "c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "resource attribute contains dimension",
			optionalDims: []Dimension{
				{Name: "foo"},
			},
			resourceAttrMap: map[string]pdata.AttributeValue{
				"foo": pdata.NewAttributeValueInt(99),
			},
			wantKey: "c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "both span and resource attribute contains dimension, should prefer span attribute",
			optionalDims: []Dimension{
				{Name: "foo"},
			},
			spanAttrMap: map[string]pdata.AttributeValue{
				"foo": pdata.NewAttributeValueInt(100),
			},
			resourceAttrMap: map[string]pdata.AttributeValue{
				"foo": pdata.NewAttributeValueInt(99),
			},
			wantKey: "c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000100",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Prepare
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			// Duplicate dimension with reserved label after sanitization.
			cfg.Dimensions = tc.optionalDims

			// Test
			next := new(consumertest.TracesSink)
			p, err := newProcessor(zap.NewNop(), cfg, next)
			assert.NoError(t, err)

			resAttr := pdata.NewAttributeMapFromMap(tc.resourceAttrMap)
			span0 := pdata.NewSpan()
			pdata.NewAttributeMapFromMap(tc.spanAttrMap).CopyTo(span0.Attributes())
			span0.SetName("c")
			k := p.buildMetricKey(span0, resAttr)

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

func TestProcessorDuplicateResourceAttributes(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Duplicate dimension with reserved label after sanitization.
	cfg.ResourceAttributes = []Dimension{
		{Name: "service.name"},
	}

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zap.NewNop(), cfg, next)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestValidateDimensions(t *testing.T) {
	for _, tc := range []struct {
		name        string
		dimensions  []Dimension
		expectedErr string
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateDimensions(tc.dimensions, []string{serviceNameKey, spanKindKey, statusCodeKey})
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTraceWithoutServiceNameDoesNotGenerateMetrics(t *testing.T) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pdata.Metrics) bool {
		require.Equal(t, 0, input.MetricCount(),
			"Should be 0 as the trace does not have a service name and hence is skipped when building metrics",
		)
		return true
	})).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := "defaultNullValue"
	p := newProcessorImp(mexp, tcon, &defaultNullValue, cumulative, t)

	trace := pdata.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "",
			spans: []span{
				{
					operation:  "/ping",
					kind:       pdata.SpanKindServer,
					statusCode: pdata.StatusCodeOk,
				},
				{
					operation:  "/ping",
					kind:       pdata.SpanKindClient,
					statusCode: pdata.StatusCodeOk,
				},
			},
		}, trace.ResourceSpans().AppendEmpty())

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err := p.ConsumeTraces(ctx, trace)

	// Verify
	assert.NoError(t, err)
}

func TestSanitize(t *testing.T) {
	require.Equal(t, "", sanitize(""), "")
	require.Equal(t, "key_test", sanitize("_test"))
	require.Equal(t, "key_0test", sanitize("0test"))
	require.Equal(t, "test", sanitize("test"))
	require.Equal(t, "test__", sanitize("test_/"))
}

func TestSetLatencyExemplars(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	traces := buildSampleTrace()
	traceID := traces.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).TraceID()
	exemplarSlice := pdata.NewExemplarSlice()
	timestamp := pdata.NewTimestampFromTime(time.Now())
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
	traceID := traces.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).TraceID()
	mKey := metricKey("metricKey")
	rKey := resourceKey("resourceKey")
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)
	value := float64(42)

	// ----- call -------------------------------------------------------------
	p.updateLatencyExemplars(rKey, mKey, value, traceID)

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.NotEmpty(t, p.latencyExemplarsData[rKey][mKey])
	assert.Equal(t, p.latencyExemplarsData[rKey][mKey][0], exemplarData{traceID: traceID, value: value})
}

func TestProcessorResetExemplarData(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	mKey := metricKey("metricKey")
	rKey := resourceKey("resourceKey")
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), cfg, next)

	// ----- call -------------------------------------------------------------
	p.resetExemplarData()

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.Empty(t, p.latencyExemplarsData[rKey][mKey])
}

func TestBuildResourceAttrKey(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ResourceAttributes = []Dimension{
		{
			Name:    "baz",
			Default: nil,
		},
		{
			Name:    "foo",
			Default: nil,
		},
	}
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zap.NewNop(), cfg, next)
	assert.NoError(t, err)

	type args struct {
		serviceName     string
		resourceAttrMap map[string]pdata.AttributeValue
	}
	tests := []struct {
		name string
		args args
		want resourceKey
	}{
		{
			name: "should build resource attribute key in order",
			args: args{
				serviceName: "serviceA",
				resourceAttrMap: map[string]pdata.AttributeValue{
					"foo": pdata.NewAttributeValueString("foo_val"),
					"bar": pdata.NewAttributeValueString("bar_val"),
					"baz": pdata.NewAttributeValueString("baz_val"),
				},
			},
			want: resourceKey(fmt.Sprintf("serviceA%sbaz_val%sfoo_val", keybuilder.Separator, keybuilder.Separator)),
		},
		{
			name: "should build resource attribute key in order",
			args: args{
				serviceName: "serviceA",
				resourceAttrMap: map[string]pdata.AttributeValue{
					"bar": pdata.NewAttributeValueString("bar_val"),
					"baz": pdata.NewAttributeValueString("baz_val"),
					"foo": pdata.NewAttributeValueString("foo_val"),
				},
			},
			want: resourceKey(fmt.Sprintf("serviceA%sbaz_val%sfoo_val", keybuilder.Separator, keybuilder.Separator)),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := p.buildResourceAttrKey(
				tt.args.serviceName,
				pdata.NewAttributeMapFromMap(tt.args.resourceAttrMap),
			); got != tt.want {
				t.Errorf("buildResourceAttrKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildBuildMetricKey(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Dimensions = []Dimension{
		{
			Name:    "baz",
			Default: nil,
		},
		{
			Name:    "foo",
			Default: nil,
		},
	}
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zap.NewNop(), cfg, next)
	assert.NoError(t, err)

	type args struct {
		span    func() pdata.Span
		attrMap map[string]pdata.AttributeValue
	}
	tests := []struct {
		name string
		args args
		want metricKey
	}{
		{
			name: "should build metric key in order",
			args: args{
				span: func() pdata.Span {
					span := pdata.NewSpan()
					span.SetName("spanA")
					span.SetKind(pdata.SpanKindServer)
					return span
				},
				attrMap: map[string]pdata.AttributeValue{
					"foo": pdata.NewAttributeValueString("foo_val"),
					"bar": pdata.NewAttributeValueString("bar_val"),
					"baz": pdata.NewAttributeValueString("baz_val"),
				},
			},
			want: metricKey(fmt.Sprintf(
				"spanA%sSPAN_KIND_SERVER%sSTATUS_CODE_UNSET%sbaz_val%sfoo_val",
				keybuilder.Separator,
				keybuilder.Separator,
				keybuilder.Separator,
				keybuilder.Separator,
			)),
		},
		{
			name: "should build metric key in order",
			args: args{
				span: func() pdata.Span {
					span := pdata.NewSpan()
					span.SetName("spanA")
					span.SetKind(pdata.SpanKindClient)
					return span
				},
				attrMap: map[string]pdata.AttributeValue{
					"baz": pdata.NewAttributeValueString("baz_val"),
					"bar": pdata.NewAttributeValueString("bar_val"),
					"foo": pdata.NewAttributeValueString("foo_val"),
				},
			},
			want: metricKey(fmt.Sprintf(
				"spanA%sSPAN_KIND_CLIENT%sSTATUS_CODE_UNSET%sbaz_val%sfoo_val",
				keybuilder.Separator,
				keybuilder.Separator,
				keybuilder.Separator,
				keybuilder.Separator,
			)),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := p.buildMetricKey(
				tt.args.span(),
				pdata.NewAttributeMapFromMap(tt.args.attrMap),
			); got != tt.want {
				fmt.Println(got)
				fmt.Println(tt.want)
				t.Errorf("buildResourceAttrKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
