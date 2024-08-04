// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector

import (
	"context"
	"crypto/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestConnectorStart(t *testing.T) {
	// Create servicegraph connector
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	procCreationParams := connectortest.NewNopSettings()
	traceConnector, err := factory.CreateTracesToMetrics(context.Background(), procCreationParams, cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Test
	smp := traceConnector.(*serviceGraphConnector)
	err = smp.Start(context.Background(), componenttest.NewNopHost())
	defer require.NoError(t, smp.Shutdown(context.Background()))

	// Verify
	assert.NoError(t, err)
}

func TestConnectorShutdown(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.MetricsSink)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	p, err := newConnector(set, cfg, next)
	require.NoError(t, err)
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestConnectorConsume(t *testing.T) {
	t.Run("test common case", func(t *testing.T) {
		// Prepare
		cfg := &Config{
			Dimensions: []string{"some-attribute", "non-existing-attribute"},
			Store:      StoreConfig{MaxItems: 10},
		}

		set := componenttest.NewNopTelemetrySettings()
		set.Logger = zaptest.NewLogger(t)
		conn, err := newConnector(set, cfg, newMockMetricsExporter())
		require.NoError(t, err)
		assert.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))

		// Test & verify
		td := buildSampleTrace(t, "val")
		// The assertion is part of verifyHappyCaseMetrics func.
		assert.NoError(t, conn.ConsumeTraces(context.Background(), td))

		// Force collection
		conn.store.Expire()
		md, err := conn.buildMetrics()
		assert.NoError(t, err)
		verifyHappyCaseMetrics(t, md)

		// Shutdown the connector
		assert.NoError(t, conn.Shutdown(context.Background()))
	})
	t.Run("test fix failed label not work", func(t *testing.T) {
		cfg := &Config{
			Store: StoreConfig{MaxItems: 10},
		}
		set := componenttest.NewNopTelemetrySettings()
		set.Logger = zaptest.NewLogger(t)
		conn, err := newConnector(set, cfg, newMockMetricsExporter())
		require.NoError(t, err)

		assert.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
		defer require.NoError(t, conn.Shutdown(context.Background()))

		// this trace simulate two services' trace: foo, bar
		// foo called bar three times, two success, one failed
		td, err := golden.ReadTraces("testdata/failed-label-not-work-simple-trace.yaml")
		assert.NoError(t, err)
		assert.NoError(t, conn.ConsumeTraces(context.Background(), td))

		// Force collection
		conn.store.Expire()
		actualMetrics, err := conn.buildMetrics()
		assert.NoError(t, err)

		// Verify
		expectedMetrics, err := golden.ReadMetrics("testdata/failed-label-not-work-expect-metrics.yaml")
		assert.NoError(t, err)

		err = pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreDatapointAttributesOrder(),
		)
		require.NoError(t, err)
	})
}

func verifyHappyCaseMetrics(t *testing.T, md pmetric.Metrics) {
	verifyHappyCaseMetricsWithDuration(1)(t, md)
}

func verifyHappyCaseMetricsWithDuration(durationSum float64) func(t *testing.T, md pmetric.Metrics) {
	return func(t *testing.T, md pmetric.Metrics) {
		assert.Equal(t, 3, md.MetricCount())

		rms := md.ResourceMetrics()
		assert.Equal(t, 1, rms.Len())

		sms := rms.At(0).ScopeMetrics()
		assert.Equal(t, 1, sms.Len())

		ms := sms.At(0).Metrics()
		assert.Equal(t, 3, ms.Len())

		mCount := ms.At(0)
		verifyCount(t, mCount)

		mServerDuration := ms.At(1)
		assert.Equal(t, "traces_service_graph_request_server_seconds", mServerDuration.Name())
		verifyDuration(t, mServerDuration, durationSum)

		mClientDuration := ms.At(2)
		assert.Equal(t, "traces_service_graph_request_client_seconds", mClientDuration.Name())
		verifyDuration(t, mClientDuration, durationSum)
	}
}

func verifyCount(t *testing.T, m pmetric.Metric) {
	assert.Equal(t, "traces_service_graph_request_total", m.Name())

	assert.Equal(t, pmetric.MetricTypeSum, m.Type())
	dps := m.Sum().DataPoints()
	assert.Equal(t, 1, dps.Len())

	dp := dps.At(0)
	assert.Equal(t, pmetric.NumberDataPointValueTypeInt, dp.ValueType())
	assert.Equal(t, int64(1), dp.IntValue())

	attributes := dp.Attributes()
	assert.Equal(t, 5, attributes.Len())
	verifyAttr(t, attributes, "client", "some-service")
	verifyAttr(t, attributes, "server", "some-service")
	verifyAttr(t, attributes, "connection_type", "")
	verifyAttr(t, attributes, "failed", "false")
	verifyAttr(t, attributes, "client_some-attribute", "val")
}

func verifyDuration(t *testing.T, m pmetric.Metric, durationSum float64) {
	assert.Equal(t, pmetric.MetricTypeHistogram, m.Type())
	dps := m.Histogram().DataPoints()
	assert.Equal(t, 1, dps.Len())

	dp := dps.At(0)
	assert.Equal(t, durationSum, dp.Sum()) // Duration: 1sec
	assert.Equal(t, uint64(1), dp.Count())
	buckets := pcommon.NewUInt64Slice()
	buckets.FromRaw([]uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	assert.Equal(t, buckets, dp.BucketCounts())

	attributes := dp.Attributes()
	assert.Equal(t, 5, attributes.Len())
	verifyAttr(t, attributes, "client", "some-service")
	verifyAttr(t, attributes, "server", "some-service")
	verifyAttr(t, attributes, "connection_type", "")
	verifyAttr(t, attributes, "client_some-attribute", "val")
}

func verifyAttr(t *testing.T, attrs pcommon.Map, k, expected string) {
	v, ok := attrs.Get(k)
	assert.True(t, ok)
	assert.Equal(t, expected, v.AsString())
}

func buildSampleTrace(t *testing.T, attrValue string) ptrace.Traces {
	tStart := time.Date(2022, 1, 2, 3, 4, 5, 6, time.UTC)
	tEnd := time.Date(2022, 1, 2, 3, 4, 6, 6, time.UTC)

	traces := ptrace.NewTraces()

	resourceSpans := traces.ResourceSpans().AppendEmpty()
	resourceSpans.Resource().Attributes().PutStr(semconv.AttributeServiceName, "some-service")

	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	var traceID pcommon.TraceID
	_, err := rand.Read(traceID[:])
	assert.NoError(t, err)

	var clientSpanID, serverSpanID pcommon.SpanID
	_, err = rand.Read(clientSpanID[:])
	assert.NoError(t, err)
	_, err = rand.Read(serverSpanID[:])
	assert.NoError(t, err)

	clientSpan := scopeSpans.Spans().AppendEmpty()
	clientSpan.SetName("client span")
	clientSpan.SetSpanID(clientSpanID)
	clientSpan.SetTraceID(traceID)
	clientSpan.SetKind(ptrace.SpanKindClient)
	clientSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	clientSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
	clientSpan.Attributes().PutStr("some-attribute", attrValue) // Attribute selected as dimension for metrics
	serverSpan := scopeSpans.Spans().AppendEmpty()
	serverSpan.SetName("server span")
	serverSpan.SetSpanID(serverSpanID)
	serverSpan.SetTraceID(traceID)
	serverSpan.SetParentSpanID(clientSpanID)
	serverSpan.SetKind(ptrace.SpanKindServer)
	serverSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	serverSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))

	return traces
}

var _ exporter.Metrics = (*mockMetricsExporter)(nil)

func newMockMetricsExporter() *mockMetricsExporter {
	return &mockMetricsExporter{}
}

type mockMetricsExporter struct {
	mtx sync.Mutex
	md  []pmetric.Metrics
}

func (m *mockMetricsExporter) Start(context.Context, component.Host) error { return nil }

func (m *mockMetricsExporter) Shutdown(context.Context) error { return nil }

func (m *mockMetricsExporter) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }

func (m *mockMetricsExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.md = append(m.md, md)
	return nil
}

// GetMetrics is the race-condition-safe way to get the metrics that have been consumed by the exporter.
func (m *mockMetricsExporter) GetMetrics() []pmetric.Metrics {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Create a copy of m.md to avoid returning a reference to the original slice
	mdCopy := make([]pmetric.Metrics, len(m.md))
	copy(mdCopy, m.md)

	return mdCopy
}

func TestUpdateDurationMetrics(t *testing.T) {
	p := serviceGraphConnector{
		reqTotal:                             make(map[string]int64),
		reqFailedTotal:                       make(map[string]int64),
		reqServerDurationSecondsSum:          make(map[string]float64),
		reqServerDurationSecondsCount:        make(map[string]uint64),
		reqServerDurationSecondsBucketCounts: make(map[string][]uint64),
		reqClientDurationSecondsSum:          make(map[string]float64),
		reqClientDurationSecondsCount:        make(map[string]uint64),
		reqClientDurationSecondsBucketCounts: make(map[string][]uint64),
		reqDurationBounds:                    defaultLatencyHistogramBuckets,
		keyToMetric:                          make(map[string]metricSeries),
		config: &Config{
			Dimensions: []string{},
		},
	}
	metricKey := p.buildMetricKey("foo", "bar", "", "false", map[string]string{})

	testCases := []struct {
		caseStr  string
		duration float64
	}{

		{
			caseStr:  "index 0 latency",
			duration: 0,
		},
		{
			caseStr:  "out-of-range latency 1",
			duration: 25_000,
		},
		{
			caseStr:  "out-of-range latency 2",
			duration: 125_000,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.caseStr, func(_ *testing.T) {
			p.updateDurationMetrics(metricKey, tc.duration, tc.duration)
		})
	}
}

func TestStaleSeriesCleanup(t *testing.T) {
	// Prepare
	cfg := &Config{
		Dimensions: []string{"some-attribute", "non-existing-attribute"},
		Store: StoreConfig{
			MaxItems: 10,
			TTL:      time.Second,
		},
	}

	mockMetricsExporter := newMockMetricsExporter()

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	p, err := newConnector(set, cfg, mockMetricsExporter)
	require.NoError(t, err)
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))

	// ConsumeTraces
	td := buildSampleTrace(t, "first")
	assert.NoError(t, p.ConsumeTraces(context.Background(), td))

	// Make series stale and force a cache cleanup
	for key, metric := range p.keyToMetric {
		metric.lastUpdated = 0
		p.keyToMetric[key] = metric
	}
	p.cleanCache()
	assert.Equal(t, 0, len(p.keyToMetric))

	// ConsumeTraces with a trace with different attribute value
	td = buildSampleTrace(t, "second")
	assert.NoError(t, p.ConsumeTraces(context.Background(), td))

	// Shutdown the connector
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestMapsAreConsistentDuringCleanup(t *testing.T) {
	// Prepare
	cfg := &Config{
		Dimensions: []string{"some-attribute", "non-existing-attribute"},
		Store: StoreConfig{
			MaxItems: 10,
			TTL:      time.Second,
		},
	}

	mockMetricsExporter := newMockMetricsExporter()

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	p, err := newConnector(set, cfg, mockMetricsExporter)
	require.NoError(t, err)
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))

	// ConsumeTraces
	td := buildSampleTrace(t, "first")
	assert.NoError(t, p.ConsumeTraces(context.Background(), td))

	// Make series stale and force a cache cleanup
	for key, metric := range p.keyToMetric {
		metric.lastUpdated = 0
		p.keyToMetric[key] = metric
	}

	// Start cleanup, but use locks to pretend that we are:
	// - currently collecting metrics (so seriesMutex is locked)
	// - currently getting dimensions for that series (so metricMutex is locked)
	p.seriesMutex.Lock()
	p.metricMutex.RLock()
	go p.cleanCache()

	// Since everything is locked, nothing has happened, so both should still have length 1
	assert.Equal(t, 1, len(p.reqTotal))
	assert.Equal(t, 1, len(p.keyToMetric))

	// Now we pretend that we have stopped collecting metrics, by unlocking seriesMutex
	p.seriesMutex.Unlock()

	// Make sure cleanupCache has continued to the next mutex
	time.Sleep(time.Millisecond)
	p.seriesMutex.Lock()

	// The expired series should have been removed. The metrics collector now won't look
	// for dimensions from that series. It's important that it happens this way around,
	// instead of deleting it from `keyToMetric`, otherwise the metrics collector will try
	// and fail to find dimensions for a series that is about to be removed.
	assert.Equal(t, 0, len(p.reqTotal))
	assert.Equal(t, 1, len(p.keyToMetric))

	p.metricMutex.RUnlock()
	p.seriesMutex.Unlock()

	// Shutdown the connector
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestValidateOwnTelemetry(t *testing.T) {
	cfg := &Config{
		Dimensions: []string{"some-attribute", "non-existing-attribute"},
		Store: StoreConfig{
			MaxItems: 10,
			TTL:      time.Second,
		},
	}

	mockMetricsExporter := newMockMetricsExporter()
	set := setupTestTelemetry()
	p, err := newConnector(set.NewSettings().TelemetrySettings, cfg, mockMetricsExporter)
	require.NoError(t, err)
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))

	// ConsumeTraces
	td := buildSampleTrace(t, "first")
	assert.NoError(t, p.ConsumeTraces(context.Background(), td))

	// Make series stale and force a cache cleanup
	for key, metric := range p.keyToMetric {
		metric.lastUpdated = 0
		p.keyToMetric[key] = metric
	}
	p.cleanCache()
	assert.Equal(t, 0, len(p.keyToMetric))

	// ConsumeTraces with a trace with different attribute value
	td = buildSampleTrace(t, "second")
	assert.NoError(t, p.ConsumeTraces(context.Background(), td))

	// Shutdown the connector
	assert.NoError(t, p.Shutdown(context.Background()))
	set.assertMetrics(t, []metricdata.Metrics{
		{
			Name:        "otelcol_connector_servicegraph_total_edges",
			Description: "Total number of unique edges",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 2},
				},
			},
		},
	})
}

func TestExtraDimensionsLabels(t *testing.T) {
	extraDimensions := []string{"db.system", "messaging.system"}
	cfg := &Config{
		Dimensions:              extraDimensions,
		LatencyHistogramBuckets: []time.Duration{time.Duration(0.1 * float64(time.Second)), time.Duration(1 * float64(time.Second)), time.Duration(10 * float64(time.Second))},
		Store:                   StoreConfig{MaxItems: 10},
	}

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	conn, err := newConnector(set, cfg, newMockMetricsExporter())
	assert.NoError(t, err)

	assert.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer require.NoError(t, conn.Shutdown(context.Background()))

	td, err := golden.ReadTraces("testdata/extra-dimensions-queue-db-trace.yaml")
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeTraces(context.Background(), td))

	conn.store.Expire()

	metrics := conn.metricsConsumer.(*mockMetricsExporter).GetMetrics()
	require.Len(t, metrics, 1)

	expectedMetrics, err := golden.ReadMetrics("testdata/extra-dimensions-queue-db-expected-metrics.yaml")
	assert.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedMetrics, metrics[0],
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
	)
	require.NoError(t, err)
}

func TestVirtualNodeServerLabels(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33836")
	}

	virtualNodeDimensions := []string{"peer.service", "db.system", "messaging.system"}
	cfg := &Config{
		Dimensions:                virtualNodeDimensions,
		LatencyHistogramBuckets:   []time.Duration{time.Duration(0.1 * float64(time.Second)), time.Duration(1 * float64(time.Second)), time.Duration(10 * float64(time.Second))},
		Store:                     StoreConfig{MaxItems: 10},
		VirtualNodePeerAttributes: virtualNodeDimensions,
		VirtualNodeExtraLabel:     true,
		MetricsFlushInterval:      time.Millisecond,
	}

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)

	trace := "testdata/virtual-node-label-server-trace.yaml"
	expected := "testdata/virtual-node-label-server-expected-metrics.yaml"

	conn, err := newConnector(set, cfg, newMockMetricsExporter())
	assert.NoError(t, err)
	assert.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))

	td, err := golden.ReadTraces(trace)
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeTraces(context.Background(), td))

	conn.store.Expire()
	assert.Eventually(t, func() bool {
		return conn.store.Len() == 0
	}, 100*time.Millisecond, 2*time.Millisecond)
	require.NoError(t, conn.Shutdown(context.Background()))

	metrics := conn.metricsConsumer.(*mockMetricsExporter).GetMetrics()
	require.GreaterOrEqual(t, len(metrics), 1) // Unreliable sleep-based check

	expectedMetrics, err := golden.ReadMetrics(expected)
	assert.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedMetrics, metrics[0],
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
	)
	require.NoError(t, err)
}

func TestVirtualNodeClientLabels(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33836")
	}

	virtualNodeDimensions := []string{"peer.service", "db.system", "messaging.system"}
	cfg := &Config{
		Dimensions:                virtualNodeDimensions,
		LatencyHistogramBuckets:   []time.Duration{time.Duration(0.1 * float64(time.Second)), time.Duration(1 * float64(time.Second)), time.Duration(10 * float64(time.Second))},
		Store:                     StoreConfig{MaxItems: 10},
		VirtualNodePeerAttributes: virtualNodeDimensions,
		VirtualNodeExtraLabel:     true,
		MetricsFlushInterval:      time.Millisecond,
	}

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)

	trace := "testdata/virtual-node-label-client-trace.yaml"
	expected := "testdata/virtual-node-label-client-expected-metrics.yaml"

	conn, err := newConnector(set, cfg, newMockMetricsExporter())
	assert.NoError(t, err)
	assert.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))

	td, err := golden.ReadTraces(trace)
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeTraces(context.Background(), td))

	conn.store.Expire()
	assert.Eventually(t, func() bool {
		return conn.store.Len() == 0
	}, 100*time.Millisecond, 2*time.Millisecond)
	require.NoError(t, conn.Shutdown(context.Background()))

	metrics := conn.metricsConsumer.(*mockMetricsExporter).GetMetrics()
	require.GreaterOrEqual(t, len(metrics), 1) // Unreliable sleep-based check

	expectedMetrics, err := golden.ReadMetrics(expected)
	assert.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedMetrics, metrics[0],
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
	)
	require.NoError(t, err)
}
