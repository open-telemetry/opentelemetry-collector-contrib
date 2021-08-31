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
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/mocks"
)

const (
	stringAttrName     = "stringAttrName"
	intAttrName        = "intAttrName"
	doubleAttrName     = "doubleAttrName"
	boolAttrName       = "boolAttrName"
	nullAttrName       = "nullAttrName"
	mapAttrName        = "mapAttrName"
	arrayAttrName      = "arrayAttrName"
	notInSpanAttrName0 = "shouldBeInMetric"
	notInSpanAttrName1 = "shouldNotBeInMetric"

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
	p, err := newProcessor(zap.NewNop(), cfg, next)
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
	p, err := newProcessor(zap.NewNop(), cfg, next)

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
	p, err := newProcessor(zap.NewNop(), cfg, next)
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

			p := newProcessorImp(mexp, tcon, nil)

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
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pdata.Metrics) bool {
		return verifyConsumeMetricsInput(input, t)
	})).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := "defaultNullValue"
	p := newProcessorImp(mexp, tcon, &defaultNullValue)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err := p.ConsumeTraces(ctx, traces)

	// Verify
	assert.NoError(t, err)
}

func TestMetricKeyCache(t *testing.T) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := "defaultNullValue"
	p := newProcessorImp(mexp, tcon, &defaultNullValue)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err := p.ConsumeTraces(ctx, traces)

	// Validate
	require.NoError(t, err)

	origKeyCache := make(map[metricKey]pdata.AttributeMap)
	for k, v := range p.metricKeyToDimensions {
		origKeyCache[k] = v
	}
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	assert.Equal(t, origKeyCache, p.metricKeyToDimensions)
}

func BenchmarkProcessorConsumeTraces(b *testing.B) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := "defaultNullValue"
	p := newProcessorImp(mexp, tcon, &defaultNullValue)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	for n := 0; n < b.N; n++ {
		p.ConsumeTraces(ctx, traces)
	}
}

func newProcessorImp(mexp *mocks.MetricsExporter, tcon *mocks.TracesConsumer, defaultNullValue *string) *processorImp {
	defaultNotInSpanAttrVal := "defaultNotInSpanAttrVal"
	return &processorImp{
		logger:          zap.NewNop(),
		metricsExporter: mexp,
		nextConsumer:    tcon,

		startTime:           time.Now(),
		callSum:             make(map[metricKey]int64),
		latencySum:          make(map[metricKey]float64),
		latencyCount:        make(map[metricKey]uint64),
		latencyBucketCounts: make(map[metricKey][]uint64),
		latencyBounds:       defaultLatencyHistogramBucketsMs,
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
		},
		metricKeyToDimensions: make(map[metricKey]pdata.AttributeMap),
	}
}

// verifyConsumeMetricsInput verifies the input of the ConsumeMetrics call from this processor.
// This is the best point to verify the computed metrics from spans are as expected.
func verifyConsumeMetricsInput(input pdata.Metrics, t *testing.T) bool {
	require.Equal(t, 6, input.MetricCount(),
		"Should be 3 for each of call count and latency. Each group of 3 metrics is made of: "+
			"service-a (server kind) -> service-a (client kind) -> service-b (service kind)",
	)

	rm := input.ResourceMetrics()
	require.Equal(t, 1, rm.Len())

	ilm := rm.At(0).InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilm.Len())
	assert.Equal(t, "spanmetricsprocessor", ilm.At(0).InstrumentationLibrary().Name())

	m := ilm.At(0).Metrics()
	require.Equal(t, 6, m.Len())

	seenMetricIDs := make(map[metricID]bool)
	mi := 0
	// The first 3 metrics are for call counts.
	for ; mi < 3; mi++ {
		assert.Equal(t, "calls_total", m.At(mi).Name())

		data := m.At(mi).Sum()
		assert.Equal(t, pdata.AggregationTemporalityCumulative, data.AggregationTemporality())
		assert.True(t, data.IsMonotonic())

		dps := data.DataPoints()
		require.Equal(t, 1, dps.Len())

		dp := dps.At(0)
		assert.Equal(t, int64(1), dp.IntVal(), "There should only be one metric per Service/operation/kind combination")
		assert.NotZero(t, dp.StartTimestamp(), "StartTimestamp should be set")
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")

		verifyMetricLabels(dp, t, seenMetricIDs)
	}

	seenMetricIDs = make(map[metricID]bool)
	// The remaining metrics are for latency.
	for ; mi < m.Len(); mi++ {
		assert.Equal(t, "latency", m.At(mi).Name())

		data := m.At(mi).Histogram()
		assert.Equal(t, pdata.AggregationTemporalityCumulative, data.AggregationTemporality())

		dps := data.DataPoints()
		require.Equal(t, 1, dps.Len())

		dp := dps.At(0)
		assert.Equal(t, sampleLatency, dp.Sum(), "Should be a single 11ms latency measurement")
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
				wantBucketCount = 1
			}
			assert.Equal(t, wantBucketCount, dp.BucketCounts()[bi])
		}
		verifyMetricLabels(dp, t, seenMetricIDs)
	}
	return true
}

func verifyMetricLabels(dp metricDataPoint, t *testing.T, seenMetricIDs map[metricID]bool) {
	mID := metricID{}
	wantDimensions := map[string]pdata.AttributeValue{
		stringAttrName:     pdata.NewAttributeValueString("stringAttrValue"),
		intAttrName:        pdata.NewAttributeValueInt(99),
		doubleAttrName:     pdata.NewAttributeValueDouble(99.99),
		boolAttrName:       pdata.NewAttributeValueBool(true),
		nullAttrName:       pdata.NewAttributeValueNull(),
		arrayAttrName:      pdata.NewAttributeValueArray(),
		mapAttrName:        pdata.NewAttributeValueMap(),
		notInSpanAttrName0: pdata.NewAttributeValueString("defaultNotInSpanAttrVal"),
	}
	dp.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
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
}

func newOTLPExporters(t *testing.T) (*otlpexporter.Config, component.MetricsExporter, component.TracesExporter) {
	otlpExpFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		ExporterSettings: config.NewExporterSettings(config.NewID("otlp")),
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

func TestBuildKey(t *testing.T) {
	span0 := pdata.NewSpan()
	span0.SetName("c")
	k0 := buildKey("ab", span0, nil)

	span1 := pdata.NewSpan()
	span1.SetName("bc")
	k1 := buildKey("a", span1, nil)

	assert.NotEqual(t, k0, k1)
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
		t.Run(tc.name, func(t *testing.T) {
			err := validateDimensions(tc.dimensions)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	require.Equal(t, "", sanitize(""), "")
	require.Equal(t, "key_test", sanitize("_test"))
	require.Equal(t, "key_0test", sanitize("0test"))
	require.Equal(t, "test", sanitize("test"))
	require.Equal(t, "test__", sanitize("test_/"))
}
