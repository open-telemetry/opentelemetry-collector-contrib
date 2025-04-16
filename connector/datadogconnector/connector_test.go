// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	otlpmetrics "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
	pkgdatadog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
)

var _ component.Component = (*traceToMetricConnector)(nil) // testing that the connectorImp properly implements the type Component interface

// create test to create a connector, check that basic code compiles
func TestNewConnector(t *testing.T) {
	err := featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), false)
	assert.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), true)
	}()

	factory := NewFactory()

	creationParams := connectortest.NewNopSettings(metadata.Type)
	cfg := factory.CreateDefaultConfig().(*Config)

	traceToMetricsConnector, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := traceToMetricsConnector.(*traceToMetricConnector)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

func TestTraceToTraceConnector(t *testing.T) {
	err := featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), false)
	assert.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), true)
	}()

	factory := NewFactory()

	creationParams := connectortest.NewNopSettings(metadata.Type)
	cfg := factory.CreateDefaultConfig().(*Config)

	traceToTracesConnector, err := factory.CreateTracesToTraces(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := traceToTracesConnector.(*traceToTraceConnector)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

var (
	spanStartTimestamp = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC))
	spanEventTimestamp = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 13, 123, time.UTC))
	spanEndTimestamp   = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC))
)

func generateTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	res := td.ResourceSpans().AppendEmpty().Resource()
	res.Attributes().EnsureCapacity(3)
	res.Attributes().PutStr("resource-attr1", "resource-attr-val1")
	res.Attributes().PutStr("container.id", "my-container-id")
	res.Attributes().PutStr("cloud.availability_zone", "my-zone")
	res.Attributes().PutStr("cloud.region", "my-region")
	// add a custom Resource attribute
	res.Attributes().PutStr("az", "my-az")

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

func creteConnector(t *testing.T) (*traceToMetricConnector, *consumertest.MetricsSink) {
	err := featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), false)
	assert.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), true)
	}()

	factory := NewFactory()

	creationParams := connectortest.NewNopSettings(metadata.Type)
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Traces.ResourceAttributesAsContainerTags = []string{semconv.AttributeCloudAvailabilityZone, semconv.AttributeCloudRegion, "az"}
	cfg.Traces.BucketInterval = 1 * time.Second

	metricsSink := &consumertest.MetricsSink{}

	traceToMetricsConnector, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, metricsSink)
	assert.NoError(t, err)

	connector, ok := traceToMetricsConnector.(*traceToMetricConnector)
	require.True(t, ok)
	return connector, metricsSink
}

func TestContainerTags(t *testing.T) {
	connector, metricsSink := creteConnector(t)
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
	// check if the container tags are added to the cache
	assert.Len(t, connector.containerTagCache.Items(), 1)
	count := 0
	connector.containerTagCache.Items()["my-container-id"].Object.(*sync.Map).Range(func(_, _ any) bool {
		count++
		return true
	})
	assert.Equal(t, 3, count)

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

func TestReceiveResourceSpansV2(t *testing.T) {
	t.Run("ReceiveResourceSpansV1", func(t *testing.T) {
		testReceiveResourceSpansV2(t, false)
	})
	t.Run("ReceiveResourceSpansV2", func(t *testing.T) {
		testReceiveResourceSpansV2(t, true)
	})
}

func testReceiveResourceSpansV2(t *testing.T, enableReceiveResourceSpansV2 bool) {
	prevVal := pkgdatadog.ReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", enableReceiveResourceSpansV2))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()
	connector, metricsSink := creteConnector(t)
	err := connector.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		_ = connector.Shutdown(context.Background())
	}()

	trace := generateTrace()
	sattr := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()

	sattr.PutStr("deployment.environment.name", "do-not-use")

	err = connector.ConsumeTraces(context.Background(), trace)
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

	if enableReceiveResourceSpansV2 {
		assert.Equal(t, "none", sp.Stats[0].Env)
	} else {
		assert.Equal(t, "do-not-use", sp.Stats[0].Env)
	}
}

func TestOperationAndResourceNameV2(t *testing.T) {
	t.Run("OperationAndResourceNameV1", func(t *testing.T) {
		testOperationAndResourceNameV2(t, false)
	})
	t.Run("OperationAndResourceNameV2", func(t *testing.T) {
		testOperationAndResourceNameV2(t, true)
	})
}

func testOperationAndResourceNameV2(t *testing.T, enableOperationAndResourceNameV2 bool) {
	if err := featuregate.GlobalRegistry().Set("datadog.EnableOperationAndResourceNameV2", enableOperationAndResourceNameV2); err != nil {
		t.Fatal(err)
	}
	connector, metricsSink := creteConnector(t)
	err := connector.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		_ = connector.Shutdown(context.Background())
	}()

	trace := generateTrace()
	rspan := trace.ResourceSpans().At(0)
	rspan.Resource().Attributes().PutStr("deployment.environment.name", "new_env")
	rspan.ScopeSpans().At(0).Spans().At(0).SetKind(ptrace.SpanKindServer)

	err = connector.ConsumeTraces(context.Background(), trace)
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

	gotName := sp.Stats[0].Stats[0].Stats[0].Name
	if enableOperationAndResourceNameV2 {
		assert.Equal(t, "server.request", gotName)
	} else {
		assert.Equal(t, "opentelemetry.server", gotName)
	}
}

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
	tr, err := otlpmetrics.NewTranslator(
		set,
		attributesTranslator,
		options...,
	)

	require.NoError(t, err)
	return tr
}

func TestDataRace(t *testing.T) {
	connector, _ := creteConnector(t)
	trace1 := generateTrace()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				connector.populateContainerTagsCache(trace1)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				sp := &pb.StatsPayload{
					Stats: []*pb.ClientStatsPayload{
						{
							ContainerID: "my-container-id",
						},
					},
				}
				connector.enrichStatsPayload(sp)
			}
		}
	}()
	wg.Wait()
}
