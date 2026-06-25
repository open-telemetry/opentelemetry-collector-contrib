// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apmstats // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apmstats"

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	otlpmetrics "github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	otelstats "github.com/DataDog/datadog-agent/pkg/trace/otel/stats"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/internal/metadata"
)

var _ component.Component = (*traceToMetricConnector)(nil) // testing that the connectorImp properly implements the type Component interface

var datadogComponentType = component.MustNewType("datadog")

// create test to create a connector, check that basic code compiles
func TestNewConnector(t *testing.T) {
	factory := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, nil, nil, nil)

	creationParams := connectortest.NewNopSettings(datadogComponentType)
	cfg := factory.CreateDefaultConfig().(*datadogconfig.ConnectorComponentConfig)

	tconn, err := factory.CreateTracesToMetrics(t.Context(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := tconn.(*traceToMetricConnector)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

func TestTraceToTraceConnector(t *testing.T) {
	factory := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, nil, nil, nil)

	creationParams := connectortest.NewNopSettings(datadogComponentType)
	cfg := factory.CreateDefaultConfig().(*datadogconfig.ConnectorComponentConfig)

	tconn, err := factory.CreateTracesToTraces(t.Context(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := tconn.(*traceToTraceConnector)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

func createConnector(t *testing.T) (*traceToMetricConnector, *consumertest.MetricsSink) {
	cfg := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, nil, nil, nil).CreateDefaultConfig().(*datadogconfig.ConnectorComponentConfig)
	cfg.Traces.ResourceAttributesAsContainerTags = []string{"cloud.availability_zone", "cloud.region", "az"}
	return createConnectorCfg(t, cfg)
}

const (
	fallBackHostname = "test-host"
)

func createConnectorCfg(t *testing.T, cfg *datadogconfig.ConnectorComponentConfig) (*traceToMetricConnector, *consumertest.MetricsSink) {
	factory := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, testutil.NewTestTaggerClient(), func(_ context.Context) (string, error) {
		return fallBackHostname, nil
	}, nil)

	creationParams := connectortest.NewNopSettings(datadogComponentType)
	metricsSink := &consumertest.MetricsSink{}

	cfg.Traces.BucketInterval = 1 * time.Second
	tconn, err := factory.CreateTracesToMetrics(t.Context(), creationParams, cfg, metricsSink)
	assert.NoError(t, err)

	connector, ok := tconn.(*traceToMetricConnector)
	require.True(t, ok)
	oconf := obfuscate.Config{Redis: obfuscate.RedisConfig{Enabled: false}}
	connector.obfuscator = obfuscate.NewObfuscator(oconf)
	return connector, metricsSink
}

var (
	spanStartTimestamp = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC))
	spanEventTimestamp = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 13, 123, time.UTC))
	spanEndTimestamp   = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC))
)

func generateTrace(extraAttributes map[string]string) ptrace.Traces {
	td := ptrace.NewTraces()
	res := td.ResourceSpans().AppendEmpty().Resource()
	res.Attributes().EnsureCapacity(3)
	res.Attributes().PutStr("resource-attr1", "resource-attr-val1")
	res.Attributes().PutStr("container.id", "my-container-id")
	res.Attributes().PutStr("cloud.availability_zone", "my-zone")
	res.Attributes().PutStr("cloud.region", "my-region")
	// add a custom Resource attribute
	res.Attributes().PutStr("az", "my-az")
	for k, v := range extraAttributes {
		res.Attributes().PutStr(k, v)
	}

	ss := td.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans()
	ss.EnsureCapacity(1)
	fillSpanOne(ss.AppendEmpty())
	return td
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetStartTimestamp(spanStartTimestamp)
	span.SetEndTimestamp(spanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	span.SetTraceID([16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10})
	span.SetSpanID([8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18})
	evs := span.Events()
	ev0 := evs.AppendEmpty()
	ev0.SetTimestamp(spanEventTimestamp)
	ev0.SetName("event-with-attr")
	ev0.Attributes().PutStr("span-event-attr", "span-event-attr-val")
	ev0.SetDroppedAttributesCount(2)
	ev1 := evs.AppendEmpty()
	ev1.SetTimestamp(spanEventTimestamp)
	ev1.SetName("event")
	ev1.SetDroppedAttributesCount(2)
	span.SetDroppedEventsCount(1)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
}

//nolint:staticcheck // SA1019: Using deprecated Translator type for StatsToMetrics functionality
func newTranslatorWithStatsChannel(t *testing.T, logger *zap.Logger, ch chan []byte) *otlpmetrics.Translator {
	options := []otlpmetrics.TranslatorOption{
		otlpmetrics.WithHistogramMode(otlpmetrics.HistogramModeDistributions),

		otlpmetrics.WithNumberMode(otlpmetrics.NumberModeCumulativeToDelta),
		otlpmetrics.WithHistogramAggregations(),
		otlpmetrics.WithStatsOut(ch),
	}

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = logger

	attributesTranslator, err := attributes.NewTranslator(set)
	require.NoError(t, err)
	// We use the deprecated NewTranslator because the new NewDefaultTranslator
	// doesn't provide the StatsToMetrics method which is required for APM stats conversion.
	//nolint:staticcheck // SA1019: Using deprecated NewTranslator for StatsToMetrics functionality
	tr, err := otlpmetrics.NewTranslator(
		set,
		attributesTranslator,
		options...,
	)

	require.NoError(t, err)
	return tr
}

func TestContainerTagsAndHostname(t *testing.T) {
	connector, metricsSink := createConnector(t)
	err := connector.Start(t.Context(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		_ = connector.Shutdown(t.Context())
	}()

	trace1 := generateTrace(nil)

	err = connector.ConsumeTraces(t.Context(), trace1)
	assert.NoError(t, err)

	// Send two traces to ensure unique container tags are added to the cache
	trace2 := generateTrace(nil)
	err = connector.ConsumeTraces(t.Context(), trace2)
	assert.NoError(t, err)

	for len(metricsSink.AllMetrics()) == 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// check if the container tags are added to the metrics
	metrics := metricsSink.AllMetrics()
	assert.Len(t, metrics, 1)

	ch := make(chan []byte, 100)
	tr := newTranslatorWithStatsChannel(t, zap.NewNop(), ch)
	_, err = tr.MapMetrics(t.Context(), metrics[0], nil, nil)
	require.NoError(t, err)
	msg := <-ch
	sp := &pb.StatsPayload{}

	err = proto.Unmarshal(msg, sp)
	require.NoError(t, err)

	tags := sp.Stats[0].Tags
	assert.Len(t, tags, 3)
	assert.ElementsMatch(t, []string{"region:my-region", "zone:my-zone", "az:my-az"}, tags)

	hostname := sp.Stats[0].Hostname
	assert.Equal(t, fallBackHostname, hostname)
}

func TestHostnameFromAttributesPreferred(t *testing.T) {
	connector, metricsSink := createConnector(t)
	err := connector.Start(t.Context(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		_ = connector.Shutdown(t.Context())
	}()

	trace1 := generateTrace(map[string]string{"host": "preferred-host"})

	err = connector.ConsumeTraces(t.Context(), trace1)
	assert.NoError(t, err)

	// Send two traces to ensure unique container tags are added to the cache
	trace2 := generateTrace(map[string]string{"host": "preferred-host"})
	err = connector.ConsumeTraces(t.Context(), trace2)
	assert.NoError(t, err)

	for len(metricsSink.AllMetrics()) == 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// check if the container tags are added to the metrics
	metrics := metricsSink.AllMetrics()
	assert.Len(t, metrics, 1)

	ch := make(chan []byte, 100)
	tr := newTranslatorWithStatsChannel(t, zap.NewNop(), ch)
	_, err = tr.MapMetrics(t.Context(), metrics[0], nil, nil)
	require.NoError(t, err)
	msg := <-ch
	sp := &pb.StatsPayload{}

	err = proto.Unmarshal(msg, sp)
	require.NoError(t, err)

	tags := sp.Stats[0].Tags
	assert.Len(t, tags, 3)
	assert.ElementsMatch(t, []string{"region:my-region", "zone:my-zone", "az:my-az"}, tags)

	hostname := sp.Stats[0].Hostname
	assert.Equal(t, "preferred-host", hostname)
}

var (
	testTraceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	testSpanID1 = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	testSpanID2 = [8]byte{2, 2, 3, 4, 5, 6, 7, 8}
	testSpanID3 = [8]byte{3, 2, 3, 4, 5, 6, 7, 8}
	testSpanID4 = [8]byte{4, 2, 3, 4, 5, 6, 7, 8}
)

func TestMeasuredAndClientKind(t *testing.T) {
	t.Run("OperationAndResourceNameV1", func(t *testing.T) {
		testMeasuredAndClientKind(t, false)
	})
	t.Run("OperationAndResourceNameV2", func(t *testing.T) {
		testMeasuredAndClientKind(t, true)
	})
}

func testMeasuredAndClientKind(t *testing.T, enableOperationAndResourceNameV2 bool) {
	if err := featuregate.GlobalRegistry().Set("datadog.EnableOperationAndResourceNameV2", enableOperationAndResourceNameV2); err != nil {
		t.Fatal(err)
	}
	cfg := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, nil, nil, nil).CreateDefaultConfig().(*datadogconfig.ConnectorComponentConfig)
	cfg.Traces.ComputeTopLevelBySpanKind = true
	connector, metricsSink := createConnectorCfg(t, cfg)
	err := connector.Start(t.Context(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		require.NoError(t, connector.Shutdown(t.Context()))
	}()

	td := ptrace.NewTraces()
	res := td.ResourceSpans().AppendEmpty().Resource()
	res.Attributes().PutStr("service.name", "svc")
	res.Attributes().PutStr("deployment.environment.name", "my-env")

	ss := td.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans()
	// Root span
	s1 := ss.AppendEmpty()
	s1.SetName("parent")
	s1.SetKind(ptrace.SpanKindServer)
	s1.SetTraceID(testTraceID)
	s1.SetSpanID(testSpanID1)
	// Child span with internal kind does not get stats
	s2 := ss.AppendEmpty()
	s2.SetName("child1")
	s2.SetKind(ptrace.SpanKindInternal)
	s2.SetTraceID(testTraceID)
	s2.SetSpanID(testSpanID2)
	s2.SetParentSpanID(testSpanID1)
	// Child span with internal kind and the _dd.measured key gets stats
	s3 := ss.AppendEmpty()
	s3.SetName("child2")
	s3.SetKind(ptrace.SpanKindInternal)
	s3.SetTraceID(testTraceID)
	s3.SetSpanID(testSpanID3)
	s3.SetParentSpanID(testSpanID1)
	s3.Attributes().PutInt("_dd.measured", 1)
	// Child span with client kind gets stats
	s4 := ss.AppendEmpty()
	s4.SetName("child3")
	s4.SetKind(ptrace.SpanKindClient)
	s4.SetTraceID(testTraceID)
	s4.SetSpanID(testSpanID4)
	s4.SetParentSpanID(testSpanID1)

	err = connector.ConsumeTraces(t.Context(), td)
	assert.NoError(t, err)

	timeout := time.Now().Add(1 * time.Minute)
	for time.Now().Before(timeout) {
		if len(metricsSink.AllMetrics()) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	metrics := metricsSink.AllMetrics()
	require.Len(t, metrics, 1)

	ch := make(chan []byte, 100)
	tr := newTranslatorWithStatsChannel(t, zap.NewNop(), ch)
	_, err = tr.MapMetrics(t.Context(), metrics[0], nil, nil)
	require.NoError(t, err)
	msg := <-ch
	sp := &pb.StatsPayload{}

	err = proto.Unmarshal(msg, sp)
	require.NoError(t, err)
	assert.Len(t, sp.Stats, 1)
	assert.Len(t, sp.Stats[0].Stats, 1)
	assert.Equal(t, "my-env", sp.Stats[0].Env)
	assert.Len(t, sp.Stats[0].Stats[0].Stats, 3)
	cgss := sp.Stats[0].Stats[0].Stats
	sort.Slice(cgss, func(i, j int) bool {
		return cgss[i].Resource < cgss[j].Resource
	})
	expected := []*pb.ClientGroupedStats{
		{
			Service:      "svc",
			Name:         "opentelemetry.internal",
			Resource:     "child2",
			Type:         "custom",
			Hits:         1,
			TopLevelHits: 0,
			SpanKind:     "internal",
			IsTraceRoot:  pb.Trilean_FALSE,
		},
		{
			Service:      "svc",
			Name:         "opentelemetry.client",
			Resource:     "child3",
			Type:         "http",
			Hits:         1,
			TopLevelHits: 0,
			SpanKind:     "client",
			IsTraceRoot:  pb.Trilean_FALSE,
		},
		{
			Service:      "svc",
			Name:         "opentelemetry.server",
			Resource:     "parent",
			Type:         "web",
			Hits:         1,
			TopLevelHits: 1,
			SpanKind:     "server",
			IsTraceRoot:  pb.Trilean_TRUE,
		},
	}

	if enableOperationAndResourceNameV2 {
		expected[0].Name = "Internal"
		expected[1].Name = "client.request"
		expected[2].Name = "server.request"
	}

	if diff := cmp.Diff(
		cgss,
		expected,
		protocmp.Transform(),
		protocmp.IgnoreFields(&pb.ClientGroupedStats{}, "duration", "okSummary", "errorSummary")); diff != "" {
		t.Errorf("Diff between APM stats -want +got:\n%v", diff)
	}
}

func TestObfuscate(t *testing.T) {
	cfg := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, nil, nil, nil).CreateDefaultConfig().(*datadogconfig.ConnectorComponentConfig)
	cfg.Traces.BucketInterval = time.Second

	prevVal := metadata.DatadogEnableReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()
	if err := featuregate.GlobalRegistry().Set("datadog.EnableOperationAndResourceNameV2", true); err != nil {
		t.Fatal(err)
	}

	connector, metricsSink := createConnectorCfg(t, cfg)

	err := connector.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, connector.Shutdown(t.Context()))
	}()

	td := ptrace.NewTraces()
	res := td.ResourceSpans().AppendEmpty().Resource()
	res.Attributes().PutStr("service.name", "svc")
	res.Attributes().PutStr("deployment.environment.name", "my-env")

	ss := td.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans()
	s := ss.AppendEmpty()
	s.SetName("name")
	s.SetKind(ptrace.SpanKindClient)
	s.SetTraceID(testTraceID)
	s.SetSpanID(testSpanID1)
	s.Attributes().PutStr("db.system", "mysql")
	s.Attributes().PutStr("db.operation.name", "SELECT")
	s.Attributes().PutStr("db.query.text", "SELECT username FROM users WHERE id = 123") // id value 123 should be obfuscated

	err = connector.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	timeout := time.Now().Add(1 * time.Minute)
	for time.Now().Before(timeout) {
		if len(metricsSink.AllMetrics()) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	metrics := metricsSink.AllMetrics()
	require.Len(t, metrics, 1)

	ch := make(chan []byte, 100)
	tr := newTranslatorWithStatsChannel(t, zap.NewNop(), ch)
	_, err = tr.MapMetrics(t.Context(), metrics[0], nil, nil)
	require.NoError(t, err)
	msg := <-ch
	sp := &pb.StatsPayload{}

	err = proto.Unmarshal(msg, sp)
	require.NoError(t, err)
	assert.Len(t, sp.Stats, 1)
	assert.Len(t, sp.Stats[0].Stats, 1)
	assert.Equal(t, "my-env", sp.Stats[0].Env)
	assert.Len(t, sp.Stats[0].Stats[0].Stats, 1)
	cgss := sp.Stats[0].Stats[0].Stats
	expected := []*pb.ClientGroupedStats{
		{
			Service:      "svc",
			Name:         "mysql.query",
			Resource:     "SELECT username FROM users WHERE id = ?",
			Type:         "sql",
			Hits:         1,
			TopLevelHits: 1,
			SpanKind:     "client",
			IsTraceRoot:  pb.Trilean_TRUE,
			PeerTags:     []string{"db.system:mysql"},
		},
	}
	if diff := cmp.Diff(
		cgss,
		expected,
		protocmp.Transform(),
		protocmp.IgnoreFields(&pb.ClientGroupedStats{}, "duration", "okSummary", "errorSummary")); diff != "" {
		t.Errorf("Diff between APM stats -want +got:\n%v", diff)
	}
}

type errorSink struct {
	consumertest.MetricsSink
	mu         sync.Mutex
	err        error
	errorCount int
}

func (es *errorSink) setError(err error) {
	es.mu.Lock()
	es.err = err
	es.mu.Unlock()
}

func (es *errorSink) getErrorCount() int {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.errorCount
}

func (es *errorSink) Reset() {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.errorCount = 0
	es.MetricsSink.Reset()
}

func (es *errorSink) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	es.mu.Lock()
	defer es.mu.Unlock()
	if err := es.err; err != nil {
		es.errorCount++
		return es.err
	}
	return es.MetricsSink.ConsumeMetrics(ctx, md)
}

// TestSamplingWeightFromTracestate verifies that W3C tracestate th values drive
// Concentrator weighting. The Concentrator reads _sample_rate from the root span
// of each trace chunk and applies that weight to every span in the chunk, so the
// th value on the root span controls the Hits count for the whole trace.
func TestSamplingWeightFromTracestate(t *testing.T) {
	if err := featuregate.GlobalRegistry().Set("datadog.EnableOperationAndResourceNameV2", true); err != nil {
		t.Fatal(err)
	}
	cfg := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, nil, nil, nil).CreateDefaultConfig().(*datadogconfig.ConnectorComponentConfig)
	cfg.Traces.ComputeTopLevelBySpanKind = true
	connector, metricsSink := createConnectorCfg(t, cfg)
	err := connector.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, connector.Shutdown(t.Context()))
	}()

	// Trace A: single root Server span with ot=th:8 (50% sampling).
	// weight = 1/0.5 = 2, so Hits should be 2.
	traceA := ptrace.NewTraces()
	resA := traceA.ResourceSpans().AppendEmpty().Resource()
	resA.Attributes().PutStr("service.name", "weight-test-svc")
	resA.Attributes().PutStr("deployment.environment.name", "test-env")
	ssA := traceA.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans()
	spanA := ssA.AppendEmpty()
	spanA.SetName("sampled-op")
	spanA.SetKind(ptrace.SpanKindServer)
	spanA.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	spanA.SetSpanID(pcommon.SpanID([8]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22}))
	spanA.SetStartTimestamp(spanStartTimestamp)
	spanA.SetEndTimestamp(spanEndTimestamp)
	spanA.TraceState().FromRaw("ot=th:8") // 50% sampling → weight=2

	// Trace B: single root Server span with no tracestate.
	// weight = 1, so Hits should be 1.
	traceB := ptrace.NewTraces()
	resB := traceB.ResourceSpans().AppendEmpty().Resource()
	resB.Attributes().PutStr("service.name", "weight-test-svc")
	resB.Attributes().PutStr("deployment.environment.name", "test-env")
	ssB := traceB.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans()
	spanB := ssB.AppendEmpty()
	spanB.SetName("unsampled-op")
	spanB.SetKind(ptrace.SpanKindServer)
	spanB.SetTraceID(pcommon.TraceID([16]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	spanB.SetSpanID(pcommon.SpanID([8]byte{0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA}))
	spanB.SetStartTimestamp(spanStartTimestamp)
	spanB.SetEndTimestamp(spanEndTimestamp)
	// no tracestate

	err = connector.ConsumeTraces(t.Context(), traceA)
	require.NoError(t, err)
	err = connector.ConsumeTraces(t.Context(), traceB)
	require.NoError(t, err)

	timeout := time.Now().Add(1 * time.Minute)
	for time.Now().Before(timeout) {
		if len(metricsSink.AllMetrics()) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	metrics := metricsSink.AllMetrics()
	require.NotEmpty(t, metrics)

	ch := make(chan []byte, 100)
	tr := newTranslatorWithStatsChannel(t, zap.NewNop(), ch)
	_, err = tr.MapMetrics(t.Context(), metrics[0], nil, nil)
	require.NoError(t, err)
	msg := <-ch
	sp := &pb.StatsPayload{}
	require.NoError(t, proto.Unmarshal(msg, sp))

	require.NotEmpty(t, sp.Stats)
	require.NotEmpty(t, sp.Stats[0].Stats)

	hitsByResource := make(map[string]uint64)
	for _, bucket := range sp.Stats[0].Stats {
		for _, cgs := range bucket.Stats {
			hitsByResource[cgs.Resource] += cgs.Hits
		}
	}

	// Trace A root has th:8 → _sample_rate=0.5 → weight=2 → Hits=2.
	assert.Equal(t, uint64(2), hitsByResource["sampled-op"], "span with th:8 tracestate should have Hits=2 (weight=2)")
	// Trace B root has no tracestate → weight=1 → Hits=1.
	assert.Equal(t, uint64(1), hitsByResource["unsampled-op"], "span with no tracestate should have Hits=1 (weight=1)")
}

// TestSampleRateInjectedOnRoot checks that injectSampleRates (the same helper
// ConsumeTraces uses) injects _sample_rate onto the root DD span for both the
// th (threshold) and p (power-of-two) tracestate encodings so the Concentrator
// computes the correct weight. This exercises the full path: OTel tracestate →
// raw tracestate map → DD span Metrics → Root.Metrics, without requiring the
// Concentrator to flush.
func TestSampleRateInjectedOnRoot(t *testing.T) {
	tests := []struct {
		name       string
		traceState string
		wantRate   float64
	}{
		{"th encoding 50%", "ot=th:8", 0.5},
		{"p encoding 50%", "ot=p:1;r:1", 0.5},
		{"p encoding 6.25%", "ot=p:4;r:4", 0.0625},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
			traceID := pcommon.TraceID([16]byte{1})

			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("service.name", "svc")
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetTraceID(traceID)
			span.SetSpanID(rootID)
			span.SetKind(ptrace.SpanKindServer)
			span.SetStartTimestamp(spanStartTimestamp)
			span.SetEndTimestamp(spanEndTimestamp)
			span.TraceState().FromRaw(tt.traceState)

			connector, _ := createConnector(t)

			rawTracestates := rawTracestatesBySpanID(td)
			require.NotEmpty(t, rawTracestates, "rawTracestates must contain an entry for tracestate %q", tt.traceState)

			inputs := otelstats.OTLPTracesToConcentratorInputsWithObfuscation(
				td, connector.tcfg, connector.ctagKeys, connector.peerTagKeys, connector.obfuscator,
			)
			require.Len(t, inputs, 1)
			require.Len(t, inputs[0].Traces, 1)

			injected := injectSampleRates(inputs, rawTracestates, nil)
			assert.Equal(t, 1, injected)

			root := inputs[0].Traces[0].Root
			require.NotNil(t, root)
			rate, ok := root.Metrics[keySamplingRateGlobal]
			require.True(t, ok, "root span must have _sample_rate set after injection")
			assert.InDelta(t, tt.wantRate, rate, 1e-9)
		})
	}
}

// TestSampleRatePreservesUpstreamValue verifies that injectSampleRates does not
// overwrite an explicitly set upstream _sample_rate on the root span.
func TestSampleRatePreservesUpstreamValue(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	traceID := pcommon.TraceID([16]byte{1})

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(rootID)
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(spanStartTimestamp)
	span.SetEndTimestamp(spanEndTimestamp)
	span.TraceState().FromRaw("ot=th:8") // 50% → 0.5

	connector, _ := createConnector(t)

	rawTracestates := rawTracestatesBySpanID(td)
	require.NotEmpty(t, rawTracestates)

	inputs := otelstats.OTLPTracesToConcentratorInputsWithObfuscation(
		td, connector.tcfg, connector.ctagKeys, connector.peerTagKeys, connector.obfuscator,
	)
	require.Len(t, inputs, 1)
	require.Len(t, inputs[0].Traces, 1)

	root := inputs[0].Traces[0].Root
	require.NotNil(t, root)
	if root.Metrics == nil {
		root.Metrics = make(map[string]float64)
	}
	root.Metrics[keySamplingRateGlobal] = 0.25 // pre-existing upstream value

	injected := injectSampleRates(inputs, rawTracestates, nil)
	assert.Equal(t, 0, injected)

	rate, ok := root.Metrics[keySamplingRateGlobal]
	require.True(t, ok)
	assert.InDelta(t, 0.25, rate, 1e-9, "upstream _sample_rate must be preserved")
}

// TestSampleRateIgnoresNonRootTracestate verifies that a tracestate on a child
// span does not cause any _sample_rate injection. Only the root span's weight
// matters to the Concentrator.
func TestSampleRateIgnoresNonRootTracestate(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{1})
	rootID := pcommon.SpanID(testSpanID1)
	childID := pcommon.SpanID(testSpanID2)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	ss := rs.ScopeSpans().AppendEmpty()

	// Root span: no tracestate.
	root := ss.Spans().AppendEmpty()
	root.SetName("parent")
	root.SetTraceID(traceID)
	root.SetSpanID(rootID)
	root.SetKind(ptrace.SpanKindServer)
	root.SetStartTimestamp(spanStartTimestamp)
	root.SetEndTimestamp(spanEndTimestamp)

	// Child span: has a tracestate that should be ignored.
	child := ss.Spans().AppendEmpty()
	child.SetName("child")
	child.SetTraceID(traceID)
	child.SetSpanID(childID)
	child.SetParentSpanID(rootID)
	child.SetKind(ptrace.SpanKindClient)
	child.SetStartTimestamp(spanStartTimestamp)
	child.SetEndTimestamp(spanEndTimestamp)
	child.TraceState().FromRaw("ot=th:8")

	connector, _ := createConnector(t)

	rawTracestates := rawTracestatesBySpanID(td)
	require.NotEmpty(t, rawTracestates, "child tracestate should be collected into the map")

	inputs := otelstats.OTLPTracesToConcentratorInputsWithObfuscation(
		td, connector.tcfg, connector.ctagKeys, connector.peerTagKeys, connector.obfuscator,
	)
	require.Len(t, inputs, 1)
	require.Len(t, inputs[0].Traces, 1)

	injected := injectSampleRates(inputs, rawTracestates, nil)
	assert.Equal(t, 0, injected)

	ddRoot := inputs[0].Traces[0].Root
	require.NotNil(t, ddRoot)
	_, ok := ddRoot.Metrics[keySamplingRateGlobal]
	assert.False(t, ok, "root must not have _sample_rate set from a child tracestate")
}

func TestError(t *testing.T) {
	factory := NewConnectorFactory(datadogComponentType, component.StabilityLevelBeta, component.StabilityLevelBeta, nil, nil, nil)
	cfg := factory.CreateDefaultConfig().(*datadogconfig.ConnectorComponentConfig)
	cfg.Traces.BucketInterval = time.Millisecond * 100
	metricsSink := &errorSink{}
	conn, err := factory.CreateTracesToMetrics(t.Context(), connectortest.NewNopSettings(datadogComponentType), cfg, metricsSink)
	require.NoError(t, err)

	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))

	// First payload will trigger a downstream error
	metricsSink.setError(errors.New("error"))
	err = conn.ConsumeTraces(t.Context(), generateTrace(nil))
	assert.NoError(t, err)

	// Check that we registered an error and no panic occurred
	require.Eventually(t, func() bool {
		return metricsSink.getErrorCount() > 0
	}, 500*time.Millisecond, 100*time.Millisecond)
	assert.Zero(t, metricsSink.DataPointCount())
	metricsSink.Reset()

	// Second payload will be successfully accepted
	metricsSink.setError(nil)
	err = conn.ConsumeTraces(t.Context(), generateTrace(nil))
	assert.NoError(t, err)

	// Check that metrics were received, and no error was registered
	require.Eventually(t, func() bool {
		return metricsSink.DataPointCount() > 0
	}, 500*time.Millisecond, 100*time.Millisecond)
	assert.Zero(t, metricsSink.getErrorCount())

	err = conn.Shutdown(t.Context())
	require.NoError(t, err)
}
