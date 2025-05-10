// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
	pkgdatadog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
)

var _ component.Component = (*traceToMetricConnectorNative)(nil) // testing that the connectorImp properly implements the type Component interface

// create test to create a connector, check that basic code compiles
func TestNewConnectorNative(t *testing.T) {
	factory := NewFactory()

	creationParams := connectortest.NewNopSettings(metadata.Type)
	cfg := factory.CreateDefaultConfig().(*Config)

	tconn, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := tconn.(*traceToMetricConnectorNative)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

func TestTraceToTraceConnectorNative(t *testing.T) {
	factory := NewFactory()

	creationParams := connectortest.NewNopSettings(metadata.Type)
	cfg := factory.CreateDefaultConfig().(*Config)

	tconn, err := factory.CreateTracesToTraces(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := tconn.(*traceToTraceConnector)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

func creteConnectorNative(t *testing.T) (*traceToMetricConnectorNative, *consumertest.MetricsSink) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Traces.ResourceAttributesAsContainerTags = []string{string(semconv.CloudAvailabilityZoneKey), string(semconv.CloudRegionKey), "az"}
	return creteConnectorNativeWithCfg(t, cfg)
}

func creteConnectorNativeWithCfg(t *testing.T, cfg *Config) (*traceToMetricConnectorNative, *consumertest.MetricsSink) {
	factory := NewFactory()

	creationParams := connectortest.NewNopSettings(metadata.Type)
	metricsSink := &consumertest.MetricsSink{}

	cfg.Traces.BucketInterval = 1 * time.Second
	tconn, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, metricsSink)
	assert.NoError(t, err)

	connector, ok := tconn.(*traceToMetricConnectorNative)
	require.True(t, ok)
	oconf := obfuscate.Config{Redis: obfuscate.RedisConfig{Enabled: false}}
	connector.obfuscator = obfuscate.NewObfuscator(oconf)
	return connector, metricsSink
}

func TestContainerTagsNative(t *testing.T) {
	connector, metricsSink := creteConnectorNative(t)
	err := connector.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		_ = connector.Shutdown(context.Background())
	}()

	trace1 := generateTrace()

	err = connector.ConsumeTraces(context.Background(), trace1)
	assert.NoError(t, err)

	// Send two traces to ensure unique container tags are added to the cache
	trace2 := generateTrace()
	err = connector.ConsumeTraces(context.Background(), trace2)
	assert.NoError(t, err)

	for len(metricsSink.AllMetrics()) == 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// check if the container tags are added to the metrics
	metrics := metricsSink.AllMetrics()
	assert.Len(t, metrics, 1)

	ch := make(chan []byte, 100)
	tr := newTranslatorWithStatsChannel(t, zap.NewNop(), ch)
	_, err = tr.MapMetrics(context.Background(), metrics[0], nil, nil)
	require.NoError(t, err)
	msg := <-ch
	sp := &pb.StatsPayload{}

	err = proto.Unmarshal(msg, sp)
	require.NoError(t, err)

	tags := sp.Stats[0].Tags
	assert.Len(t, tags, 3)
	assert.ElementsMatch(t, []string{"region:my-region", "zone:my-zone", "az:my-az"}, tags)
}

var (
	testTraceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	testSpanID1 = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	testSpanID2 = [8]byte{2, 2, 3, 4, 5, 6, 7, 8}
	testSpanID3 = [8]byte{3, 2, 3, 4, 5, 6, 7, 8}
	testSpanID4 = [8]byte{4, 2, 3, 4, 5, 6, 7, 8}
)

func TestMeasuredAndClientKindNative(t *testing.T) {
	t.Run("OperationAndResourceNameV1", func(t *testing.T) {
		testMeasuredAndClientKindNative(t, false)
	})
	t.Run("OperationAndResourceNameV2", func(t *testing.T) {
		testMeasuredAndClientKindNative(t, true)
	})
}

func testMeasuredAndClientKindNative(t *testing.T, enableOperationAndResourceNameV2 bool) {
	if err := featuregate.GlobalRegistry().Set("datadog.EnableOperationAndResourceNameV2", enableOperationAndResourceNameV2); err != nil {
		t.Fatal(err)
	}
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Traces.ComputeTopLevelBySpanKind = true
	connector, metricsSink := creteConnectorNativeWithCfg(t, cfg)
	err := connector.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		require.NoError(t, connector.Shutdown(context.Background()))
	}()

	td := ptrace.NewTraces()
	res := td.ResourceSpans().AppendEmpty().Resource()
	res.Attributes().PutStr("service.name", "svc")
	res.Attributes().PutStr(string(semconv.DeploymentEnvironmentNameKey), "my-env")

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

	err = connector.ConsumeTraces(context.Background(), td)
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
	_, err = tr.MapMetrics(context.Background(), metrics[0], nil, nil)
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
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Traces.BucketInterval = time.Second

	prevVal := pkgdatadog.ReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()
	if err := featuregate.GlobalRegistry().Set("datadog.EnableOperationAndResourceNameV2", true); err != nil {
		t.Fatal(err)
	}

	connector, metricsSink := creteConnectorNativeWithCfg(t, cfg)

	err := connector.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, connector.Shutdown(context.Background()))
	}()

	td := ptrace.NewTraces()
	res := td.ResourceSpans().AppendEmpty().Resource()
	res.Attributes().PutStr(string(semconv.ServiceNameKey), "svc")
	res.Attributes().PutStr(string(semconv.DeploymentEnvironmentNameKey), "my-env")

	ss := td.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans()
	s := ss.AppendEmpty()
	s.SetName("name")
	s.SetKind(ptrace.SpanKindClient)
	s.SetTraceID(testTraceID)
	s.SetSpanID(testSpanID1)
	s.Attributes().PutStr(string(semconv.DBSystemKey), semconv.DBSystemMySQL.Value.AsString())
	s.Attributes().PutStr(string(semconv.DBOperationNameKey), "SELECT")
	s.Attributes().PutStr(string(semconv.DBQueryTextKey), "SELECT username FROM users WHERE id = 123") // id value 123 should be obfuscated

	err = connector.ConsumeTraces(context.Background(), td)
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
	_, err = tr.MapMetrics(context.Background(), metrics[0], nil, nil)
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
