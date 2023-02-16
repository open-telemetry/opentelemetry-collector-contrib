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
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/internal/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/mocks"
)

func TestConnectorStart(t *testing.T) {
	factory := NewConnectorFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	createParams := connectortest.NewNopCreateSettings()
	conn, err := factory.CreateTracesToMetrics(context.Background(), createParams, cfg, consumertest.NewNop())
	require.NoError(t, err)

	smc := conn.(*processorImp)
	ctx := context.Background()
	err = smc.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := smc.Shutdown(ctx); require.NoError(t, sdErr) }()
	assert.NoError(t, err)
}

func TestConnectorConcurrentShutdown(t *testing.T) {
	// Prepare
	ctx := context.Background()
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	mockClock := clock.NewMock(time.Now())
	ticker := mockClock.NewTicker(time.Nanosecond)

	// Test
	p := newConnectorImp(new(consumertest.MetricsSink), nil, cumulative, logger, ticker)
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

	// Starting spanmetricsconnector...
	// Shutting down spanmetricsconnector...
	// Stopping ticker.
	assert.Len(t, allLogs, 3)
}

func TestConnectorConfigureLatencyBounds(t *testing.T) {
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
	c, err := newProcessor(zaptest.NewLogger(t), cfg, nil)
	c.metricsConsumer = new(consumertest.MetricsSink)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, []float64{0.000003, 0.003, 3, 3000}, c.latencyBounds)
}

func TestConnectorCapabilities(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	c, err := newProcessor(zaptest.NewLogger(t), cfg, nil)
	c.metricsConsumer = new(consumertest.MetricsSink)
	assert.NoError(t, err)
	caps := c.Capabilities()

	// Verify
	assert.NotNil(t, c)
	assert.Equal(t, false, caps.MutatesData)
}

func TestConnectorConsumeMetricsErrors(t *testing.T) {
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
	p := newConnectorImp(mcon, nil, cumulative, logger, ticker)

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

func TestConnectorConsumeTraces(t *testing.T) {
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
		{
			// Consumptions with improper timestamps
			name:                   "Test bad consumptions (Delta).",
			aggregationTemporality: cumulative,
			verifier:               verifyBadMetricsOkay,
			traces:                 []ptrace.Traces{buildBadSampleTrace()},
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

			defaultNullValue := pcommon.NewValueStr("defaultNullValue")

			mockClock := clock.NewMock(time.Now())
			ticker := mockClock.NewTicker(time.Nanosecond)

			p := newConnectorImp(mcon, &defaultNullValue, tc.aggregationTemporality, zaptest.NewLogger(t), ticker)

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

func TestConnectorMetricKeyCache(t *testing.T) {
	mcon := &mocks.MetricsConsumer{}
	mcon.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	p := newConnectorImp(mcon, &defaultNullValue, cumulative, zaptest.NewLogger(t), nil)
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

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	conn := newConnectorImp(mcon, &defaultNullValue, cumulative, zaptest.NewLogger(b), nil)

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	for n := 0; n < b.N; n++ {
		assert.NoError(b, conn.ConsumeTraces(ctx, traces))
	}
}

func newConnectorImp(mcon consumer.Metrics, defaultNullValue *pcommon.Value, temporality string, logger *zap.Logger, ticker *clock.Ticker) *processorImp {
	defaultNotInSpanAttrVal := pcommon.NewValueStr("defaultNotInSpanAttrVal")
	// use size 2 for LRU cache for testing purpose
	metricKeyToDimensions, err := cache.NewCache[metricKey, pcommon.Map](DimensionsCacheSize)
	if err != nil {
		panic(err)
	}
	return &processorImp{
		logger:          logger,
		config:          Config{AggregationTemporality: temporality},
		metricsConsumer: mcon,

		startTimestamp: pcommon.NewTimestampFromTime(time.Now()),
		histograms:     make(map[metricKey]*histogramData),
		latencyBounds:  defaultLatencyHistogramBucketsMs,
		dimensions: []dimension{
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
		keyBuf:                new(bytes.Buffer),
		metricKeyToDimensions: metricKeyToDimensions,
		ticker:                ticker,
		done:                  make(chan struct{}),
	}
}

func TestConnectorDuplicateDimensions(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Duplicate dimension with reserved label after sanitization.
	cfg.Dimensions = []Dimension{
		{Name: "status_code"},
	}

	// Test
	c, err := newProcessor(zaptest.NewLogger(t), cfg, nil)
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestConnectorUpdateExemplars(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	traces := buildSampleTrace()
	traceID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
	spanID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SpanID()
	key := metricKey("metricKey")
	c, err := newProcessor(zaptest.NewLogger(t), cfg, nil)
	c.metricsConsumer = new(consumertest.MetricsSink)
	value := float64(42)

	// ----- call -------------------------------------------------------------
	c.updateHistogram(key, value, traceID, spanID)

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.NotEmpty(t, c.histograms[key].exemplarsData)
	assert.Equal(t, c.histograms[key].exemplarsData[0], exemplarData{traceID: traceID, spanID: spanID, value: value})

	// ----- call -------------------------------------------------------------
	c.resetExemplarData()

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.Empty(t, c.histograms[key].exemplarsData)
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
					operation:  "/ping",
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
					operation:  "/ping",
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
					operation:  "/ping",
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
					operation:  "/ping",
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
					operation:  "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces1.ResourceSpans().AppendEmpty())

	mcon := &mocks.MetricsConsumer{}

	wantDataPointCounts := []int{
		6, // (calls_total + latency) * (service-a + service-b + service-c)
		4, // (calls_total + latency) * (service-b + service-c)
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

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	mockClock := clock.NewMock(time.Now())
	ticker := mockClock.NewTicker(time.Nanosecond)

	// Note: default dimension key cache size is 2.
	p := newConnectorImp(mcon, &defaultNullValue, cumulative, zaptest.NewLogger(t), ticker)

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
