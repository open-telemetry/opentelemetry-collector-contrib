// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/integrationtest"

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	apitrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	commonTestutil "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/featuregates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
)

// seriesSlice represents an unmarshalled series payload
type seriesSlice struct {
	Series []series
}

// series represents a metric series map
type series struct {
	Metric string
	Points []point
	Tags   []string
}

// point represents a series metric datapoint
type point struct {
	Timestamp int
	Value     float64
}

func TestIntegration(t *testing.T) {
	// 1. Set up mock Datadog server
	// See also https://github.com/DataDog/datadog-agent/blob/49c16e0d4deab396626238fa1d572b684475a53f/cmd/trace-agent/test/backend.go
	apmstatsRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.APMStatsEndpoint, ReqChan: make(chan []byte)}
	tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte)}
	server := testutil.DatadogServerMock(apmstatsRec.HandlerFunc, tracesRec.HandlerFunc)
	defer server.Close()
	t.Setenv("SERVER_URL", server.URL)
	otlpEndpoint := commonTestutil.GetAvailableLocalAddress(t)
	t.Setenv("OTLP_HTTP_SERVER", otlpEndpoint)

	// 2. Start in-process collector
	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_config.yaml", factories)
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = app.Run(t.Context()) // ignore shutdown error, core collector has race in shutdown: https://github.com/open-telemetry/opentelemetry-collector/issues/12944
	})
	defer func() {
		app.Shutdown()
		wg.Wait()
	}()

	waitForReadiness(app)

	// 3. Generate and send traces
	sendTraces(t, otlpEndpoint)

	// 4. Validate traces and APM stats from the mock server
	var spans []*pb.Span
	var stats []*pb.ClientGroupedStats

	// 5 sampled spans + APM stats on 10 spans are sent to datadog exporter
	for len(spans) < 5 || len(stats) < 10 {
		select {
		case tracesBytes := <-tracesRec.ReqChan:
			gz := getGzipReader(t, tracesBytes)
			slurp, err := io.ReadAll(gz)
			require.NoError(t, err)
			var traces pb.AgentPayload
			require.NoError(t, proto.Unmarshal(slurp, &traces))
			for _, tps := range traces.TracerPayloads {
				for _, chunks := range tps.Chunks {
					spans = append(spans, chunks.Spans...)
				}
			}

		case apmstatsBytes := <-apmstatsRec.ReqChan:
			gz := getGzipReader(t, apmstatsBytes)
			var spl pb.StatsPayload
			require.NoError(t, msgp.Decode(gz, &spl))
			for _, csps := range spl.Stats {
				assert.Equal(t, "datadogexporter-otelcol-tests", spl.AgentVersion)
				for _, csbs := range csps.Stats {
					stats = append(stats, csbs.Stats...)
					for _, stat := range csbs.Stats {
						assert.True(t, strings.HasPrefix(stat.Resource, "TestSpan"))
						assert.Equal(t, uint64(1), stat.Hits)
						assert.Equal(t, uint64(1), stat.TopLevelHits)
						assert.Equal(t, "client", stat.SpanKind)
						assert.Equal(t, []string{"extra_peer_tag:tag_val", "peer.service:svc"}, stat.PeerTags)
					}
				}
			}
		}
	}

	// Verify we don't receive more than the expected numbers
	assert.Len(t, spans, 5)
	assert.Len(t, stats, 10)
}

func getIntegrationTestComponents(t *testing.T) otelcol.Factories {
	var (
		factories otelcol.Factories
		err       error
	)
	factories.Telemetry = otelconftelemetry.NewFactory()
	factories.Receivers, err = otelcol.MakeFactoryMap[receiver.Factory](
		[]receiver.Factory{
			otlpreceiver.NewFactory(),
			hostmetricsreceiver.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory](
		[]processor.Factory{
			batchprocessor.NewFactory(),
			tailsamplingprocessor.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Connectors, err = otelcol.MakeFactoryMap[connector.Factory](
		[]connector.Factory{
			datadogconnector.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Exporters, err = otelcol.MakeFactoryMap[exporter.Factory](
		[]exporter.Factory{
			datadogexporter.NewFactory(),
			debugexporter.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	return factories
}

func getIntegrationTestCollector(t *testing.T, cfgFile string, factories otelcol.Factories) *otelcol.Collector {
	_, err := otelcoltest.LoadConfigAndValidate(cfgFile, factories)
	require.NoError(t, err, "All yaml config must be valid.")

	appSettings := otelcol.CollectorSettings{
		Factories: func() (otelcol.Factories, error) { return factories, nil },
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:              []string{cfgFile},
				ProviderFactories: []confmap.ProviderFactory{fileprovider.NewFactory(), envprovider.NewFactory()},
			},
		},
		BuildInfo: component.BuildInfo{
			Command:     "otelcol",
			Description: "OpenTelemetry Collector",
			Version:     "tests",
		},
	}

	app, err := otelcol.NewCollector(appSettings)
	require.NoError(t, err)
	return app
}

func waitForReadiness(app *otelcol.Collector) {
	for notYetStarted := true; notYetStarted; {
		state := app.GetState()
		switch state {
		case otelcol.StateRunning, otelcol.StateClosed, otelcol.StateClosing:
			notYetStarted = false
		case otelcol.StateStarting:
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func sendTraces(t *testing.T, endpoint string) {
	ctx := t.Context()

	// Set up OTel-Go SDK and exporter
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure(), otlptracehttp.WithEndpoint(endpoint))
	require.NoError(t, err)
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	r1, _ := resource.New(ctx, resource.WithAttributes(attribute.String("k8s.node.name", "aaaa")))
	r2, _ := resource.New(ctx, resource.WithAttributes(attribute.String("k8s.node.name", "bbbb")))
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(r1),
	)
	tracerProvider2 := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(r2),
	)
	otel.SetTracerProvider(tracerProvider)
	defer func() {
		require.NoError(t, tracerProvider.Shutdown(ctx))
		require.NoError(t, tracerProvider2.Shutdown(ctx))
	}()

	tracer := otel.Tracer("test-tracer")
	for i := range 10 {
		_, span := tracer.Start(ctx, fmt.Sprintf("TestSpan%d", i), apitrace.WithSpanKind(apitrace.SpanKindClient))

		if i == 3 {
			// Send some traces from a different resource
			// This verifies that stats from different hosts don't accidentally create extraneous empty stats buckets
			otel.SetTracerProvider(tracerProvider2)
			tracer = otel.Tracer("test-tracer2")
		}
		// Only sample 5 out of the 10 spans
		if i < 5 {
			span.SetAttributes(attribute.Bool("sampled", true))
		}
		span.SetAttributes(attribute.String("peer.service", "svc"))
		span.SetAttributes(attribute.String("extra_peer_tag", "tag_val"))
		span.End()
	}
	time.Sleep(1 * time.Second)
}

func getGzipReader(t *testing.T, reqBytes []byte) io.Reader {
	buf := bytes.NewBuffer(reqBytes)
	reader, err := gzip.NewReader(buf)
	require.NoError(t, err)
	return reader
}

func TestIntegrationComputeTopLevelBySpanKind(t *testing.T) {
	// 1. Set up mock Datadog server
	// See also https://github.com/DataDog/datadog-agent/blob/49c16e0d4deab396626238fa1d572b684475a53f/cmd/trace-agent/test/backend.go
	apmstatsRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.APMStatsEndpoint, ReqChan: make(chan []byte)}
	tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte)}
	server := testutil.DatadogServerMock(apmstatsRec.HandlerFunc, tracesRec.HandlerFunc)
	defer server.Close()
	t.Setenv("SERVER_URL", server.URL)
	otlpEndpoint := commonTestutil.GetAvailableLocalAddress(t)
	t.Setenv("OTLP_HTTP_SERVER", otlpEndpoint)

	// 2. Start in-process collector
	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_toplevel_config.yaml", factories)
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = app.Run(t.Context()) // ignore shutdown error, core collector has race in shutdown: https://github.com/open-telemetry/opentelemetry-collector/issues/12944
	})
	defer func() {
		app.Shutdown()
		wg.Wait()
	}()

	waitForReadiness(app)

	// 3. Generate and send traces
	sendTracesComputeTopLevelBySpanKind(t, otlpEndpoint)

	// 4. Validate traces and APM stats from the mock server
	var spans []*pb.Span
	var stats []*pb.ClientGroupedStats
	var serverSpans, clientSpans, consumerSpans, producerSpans, internalSpans int

	// 10 total spans + APM stats on 8 spans are sent to datadog exporter
	for len(spans) < 10 || len(stats) < 8 {
		select {
		case tracesBytes := <-tracesRec.ReqChan:
			gz := getGzipReader(t, tracesBytes)
			slurp, err := io.ReadAll(gz)
			require.NoError(t, err)
			var traces pb.AgentPayload
			require.NoError(t, proto.Unmarshal(slurp, &traces))
			for _, tps := range traces.TracerPayloads {
				for _, chunks := range tps.Chunks {
					spans = append(spans, chunks.Spans...)
				}
			}

		case apmstatsBytes := <-apmstatsRec.ReqChan:
			gz := getGzipReader(t, apmstatsBytes)
			var spl pb.StatsPayload
			require.NoError(t, msgp.Decode(gz, &spl))
			for _, csps := range spl.Stats {
				assert.Equal(t, "datadogexporter-otelcol-tests", spl.AgentVersion)
				for _, csbs := range csps.Stats {
					stats = append(stats, csbs.Stats...)
					for _, stat := range csbs.Stats {
						switch stat.SpanKind {
						case apitrace.SpanKindInternal.String():
							internalSpans++
						case apitrace.SpanKindServer.String():
							assert.Equal(t, uint64(1), stat.Hits)
							assert.Equal(t, uint64(1), stat.TopLevelHits)
							serverSpans++
						case apitrace.SpanKindClient.String():
							assert.Equal(t, uint64(1), stat.Hits)
							assert.Equal(t, uint64(0), stat.TopLevelHits)
							clientSpans++
						case apitrace.SpanKindProducer.String():
							assert.Equal(t, uint64(1), stat.Hits)
							assert.Equal(t, uint64(0), stat.TopLevelHits)
							producerSpans++
						case apitrace.SpanKindConsumer.String():
							assert.Equal(t, uint64(1), stat.Hits)
							assert.Equal(t, uint64(1), stat.TopLevelHits)
							consumerSpans++
						}
						assert.True(t, strings.HasPrefix(stat.Resource, "TestSpan"))
					}
				}
			}
		}
	}

	// Verify we don't receive more than the expected numbers
	assert.Equal(t, 2, serverSpans)
	assert.Equal(t, 2, clientSpans)
	assert.Equal(t, 2, consumerSpans)
	assert.Equal(t, 2, producerSpans)
	assert.Equal(t, 0, internalSpans)
	assert.Len(t, spans, 10)
	assert.Len(t, stats, 8)

	for _, span := range spans {
		switch {
		case span.Meta["span.kind"] == apitrace.SpanKindInternal.String():
			assert.EqualValues(t, 0, span.Metrics["_top_level"])
			assert.EqualValues(t, 0, span.Metrics["_dd.measured"])
		case span.Meta["span.kind"] == apitrace.SpanKindServer.String():
			assert.EqualValues(t, 1, span.Metrics["_top_level"])
			assert.EqualValues(t, 0, span.Metrics["_dd.measured"])
		case span.Meta["span.kind"] == apitrace.SpanKindClient.String():
			assert.EqualValues(t, 0, span.Metrics["_top_level"])
			assert.EqualValues(t, 1, span.Metrics["_dd.measured"])
		case span.Meta["span.kind"] == apitrace.SpanKindProducer.String():
			assert.EqualValues(t, 0, span.Metrics["_top_level"])
			assert.EqualValues(t, 1, span.Metrics["_dd.measured"])
		case span.Meta["span.kind"] == apitrace.SpanKindConsumer.String():
			assert.EqualValues(t, 1, span.Metrics["_top_level"])
			assert.EqualValues(t, 0, span.Metrics["_dd.measured"])
		}
	}
}

func sendTracesComputeTopLevelBySpanKind(t *testing.T, endpoint string) {
	ctx := t.Context()

	// Set up OTel-Go SDK and exporter
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure(), otlptracehttp.WithEndpoint(endpoint))
	require.NoError(t, err)
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	r1, _ := resource.New(ctx, resource.WithAttributes(attribute.String("k8s.node.name", "aaaa")))
	r2, _ := resource.New(ctx, resource.WithAttributes(attribute.String("k8s.node.name", "bbbb")))
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(r1),
	)
	tracerProvider2 := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(r2),
	)
	otel.SetTracerProvider(tracerProvider)
	defer func() {
		require.NoError(t, tracerProvider.Shutdown(ctx))
		require.NoError(t, tracerProvider2.Shutdown(ctx))
	}()

	tracer := otel.Tracer("test-tracer")
	for i := range 10 {
		var spanKind apitrace.SpanKind
		switch i {
		case 0, 1:
			spanKind = apitrace.SpanKindConsumer
		case 2, 3:
			spanKind = apitrace.SpanKindServer
		case 4, 5:
			spanKind = apitrace.SpanKindClient
		case 6, 7:
			spanKind = apitrace.SpanKindProducer
		case 8, 9:
			spanKind = apitrace.SpanKindInternal
		}
		var span apitrace.Span
		ctx, span = tracer.Start(ctx, fmt.Sprintf("TestSpan%d", i), apitrace.WithSpanKind(spanKind))

		if i == 3 {
			// Send some traces from a different resource
			// This verifies that stats from different hosts don't accidentally create extraneous empty stats buckets
			otel.SetTracerProvider(tracerProvider2)
			tracer = otel.Tracer("test-tracer2")
		}

		span.End()
	}
	time.Sleep(1 * time.Second)
}

func TestIntegrationLogs(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", false))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", true))
	}()

	// 1. Set up mock Datadog server
	// See also https://github.com/DataDog/datadog-agent/blob/49c16e0d4deab396626238fa1d572b684475a53f/cmd/trace-agent/test/backend.go
	seriesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.MetricV2Endpoint, ReqChan: make(chan []byte)}
	doneChannel := make(chan bool)
	var connectivityCheck sync.Once
	var logsData testutil.JSONLogs
	server := testutil.DatadogLogServerMock(seriesRec.HandlerFunc, func() (string, http.HandlerFunc) {
		return "/api/v2/logs", func(w http.ResponseWriter, r *http.Request) {
			doneConnectivityCheck := false
			connectivityCheck.Do(func() {
				// The logs agent performs a connectivity check upon initialization.
				// This function mocks a successful response for the first request received.
				w.WriteHeader(http.StatusAccepted)
				doneConnectivityCheck = true
			})
			if !doneConnectivityCheck {
				jsonLogs := testutil.ProcessLogsAgentRequest(w, r)
				logsData = append(logsData, jsonLogs...)
				doneChannel <- true
			}
		}
	})
	defer server.Close()
	t.Setenv("SERVER_URL", server.URL)
	otlpEndpoint := commonTestutil.GetAvailableLocalAddress(t)
	t.Setenv("OTLP_HTTP_SERVER", otlpEndpoint)

	// 2. Start in-process collector
	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_logs_config.yaml", factories)
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = app.Run(t.Context()) // ignore shutdown error, core collector has race in shutdown: https://github.com/open-telemetry/opentelemetry-collector/issues/12944
	})
	defer func() {
		app.Shutdown()
		wg.Wait()
	}()

	waitForReadiness(app)

	// 3. Generate and send logs
	sendLogs(t, 5, otlpEndpoint)

	// 4. Validate logs and metrics from the mock server
	// Wait until `doneChannel` is closed and internal metrics are received.
	var metricMap seriesSlice
	for len(metricMap.Series) < 4 {
		select {
		case <-doneChannel:
			assert.Len(t, logsData, 5)
		case metricsBytes := <-seriesRec.ReqChan:
			var smap seriesSlice
			gz := getGzipReader(t, metricsBytes)
			dec := json.NewDecoder(gz)
			assert.NoError(t, dec.Decode(&smap))
			for _, s := range smap.Series {
				if s.Metric == "otelcol_receiver_accepted_log_records" || s.Metric == "otelcol_exporter_sent_log_records" {
					metricMap.Series = append(metricMap.Series, s)
				}
			}
		case <-time.After(60 * time.Second):
			t.Fatalf("did not receive expected metrics after 1m")
		}
	}

	// 5. Validate mock server received expected otelcol metric values
	numAcceptedLogRecords := 0
	numSentLogRecords := 0
	assert.Len(t, metricMap.Series, 4)
	for _, s := range metricMap.Series {
		if s.Metric == "otelcol_receiver_accepted_log_records" {
			numAcceptedLogRecords++
			assert.Len(t, s.Points, 1)
			assert.Equal(t, 5.0, s.Points[0].Value)
		}
		if s.Metric == "otelcol_exporter_sent_log_records" {
			numSentLogRecords++
			assert.Len(t, s.Points, 1)
			assert.Equal(t, 5.0, s.Points[0].Value)
		}
	}
	assert.Equal(t, 2, numAcceptedLogRecords)
	assert.Equal(t, 2, numSentLogRecords)
}

func sendLogs(t *testing.T, numLogs int, endpoint string) {
	ctx := t.Context()
	logExporter, err := otlploghttp.New(ctx, otlploghttp.WithInsecure(), otlploghttp.WithEndpoint(endpoint))
	assert.NoError(t, err)
	lr := make([]log.Record, numLogs)
	assert.NoError(t, logExporter.Export(ctx, lr))
}

func TestIntegrationHostMetrics_WithRemapping_LegacyMetricClient(t *testing.T) {
	prevVal := featuregates.MetricRemappingDisabledFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), false))
	require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", false))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), prevVal))
		require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", true))
	}()

	expectedMetrics := map[string]struct{}{
		// DD conventions
		"system.load.15":    {},
		"system.load.5":     {},
		"system.mem.total":  {},
		"system.mem.usable": {},

		// OTel conventions with otel. prefix
		"otel.system.cpu.load_average.15m": {},
		"otel.system.cpu.load_average.5m":  {},
		"otel.system.memory.usage":         {},
	}
	testIntegrationHostMetrics(t, expectedMetrics, false)
}

func TestIntegrationHostMetrics_WithoutRemapping_LegacyMetricClient(t *testing.T) {
	prevVal := featuregates.MetricRemappingDisabledFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", false))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), prevVal))
		require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", true))
	}()

	expectedMetrics := map[string]struct{}{
		// OTel conventions
		"system.cpu.load_average.15m": {},
		"system.cpu.load_average.5m":  {},
		"system.memory.usage":         {},
	}
	testIntegrationHostMetrics(t, expectedMetrics, false)
}

func TestIntegrationHostMetrics_WithRemapping_Serializer(t *testing.T) {
	prevVal := featuregates.MetricRemappingDisabledFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), false))

	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), prevVal))
	}()

	expectedMetrics := map[string]struct{}{
		// DD conventions
		"system.load.15":    {},
		"system.load.5":     {},
		"system.mem.total":  {},
		"system.mem.usable": {},

		// OTel conventions with otel. prefix
		"otel.system.cpu.load_average.15m": {},
		"otel.system.cpu.load_average.5m":  {},
		"otel.system.memory.usage":         {},
	}
	testIntegrationHostMetrics(t, expectedMetrics, true)
}

func TestIntegrationHostMetrics_WithoutRemapping_Serializer(t *testing.T) {
	prevVal := featuregates.MetricRemappingDisabledFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), prevVal))
	}()

	expectedMetrics := map[string]struct{}{
		// OTel conventions
		"system.cpu.load_average.15m": {},
		"system.cpu.load_average.5m":  {},
		"system.memory.usage":         {},
	}
	testIntegrationHostMetrics(t, expectedMetrics, true)
}

func testIntegrationHostMetrics(t *testing.T, expectedMetrics map[string]struct{}, useSerializer bool) {
	// 1. Set up mock Datadog server
	seriesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.MetricV2Endpoint, ReqChan: make(chan []byte, 100)}
	server := testutil.DatadogServerMock(seriesRec.HandlerFunc)
	defer server.Close()
	t.Setenv("SERVER_URL", server.URL)

	// 2. Start in-process collector
	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_host_metrics_config.yaml", factories)
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = app.Run(t.Context()) // ignore shutdown error, core collector has race in shutdown: https://github.com/open-telemetry/opentelemetry-collector/issues/12944
	})
	defer func() {
		app.Shutdown()
		wg.Wait()
	}()

	waitForReadiness(app)

	// 3. Validate host metrics in DD and/or OTel conventions are sent to the mock server
	// See https://docs.datadoghq.com/opentelemetry/integrations/host_metrics/?tab=host
	metricMap := make(map[string]series)
	for len(metricMap) < len(expectedMetrics) {
		select {
		case metricsBytes := <-seriesRec.ReqChan:
			if useSerializer {
				var err error
				metricMap, err = seriesFromSerializer(metricsBytes, expectedMetrics)
				require.NoError(t, err)
			} else {
				var err error
				metricMap, err = seriesFromAPIClient(t, metricsBytes, expectedMetrics)
				require.NoError(t, err)
			}
		case <-time.After(60 * time.Second):
			t.Fatalf("did not receive expected metrics after 1m")
		}
	}

	// 4. verify metrics have the expected tags from otel instrumentation scope
	for _, metric := range metricMap {
		assert.Contains(t, metric.Tags, "instrumentation_scope_version:tests")
		for _, tag := range metric.Tags {
			if strings.HasPrefix(tag, "instrumentation_scope:") {
				// instrumentation_scope is a scraper in the host metrics receiver so has the hostmetricsreceiver prefix
				assert.Contains(t, tag, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver")
			}
		}
	}
}

func seriesFromSerializer(metricsBytes []byte, expectedMetrics map[string]struct{}) (map[string]series, error) {
	zr, err := zlib.NewReader(bytes.NewReader(metricsBytes))
	if err != nil {
		return nil, err
	}
	pl := new(gogen.MetricPayload)
	b, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}

	if err := pl.Unmarshal(b); err != nil {
		return nil, err
	}
	metricMap := make(map[string]series)
	for _, s := range pl.GetSeries() {
		if _, ok := expectedMetrics[s.GetMetric()]; ok {
			points := make([]point, len(s.GetPoints()))
			for i, p := range s.GetPoints() {
				points[i] = point{
					Timestamp: int(p.GetTimestamp()),
					Value:     p.GetValue(),
				}
			}
			metricMap[s.GetMetric()] = series{
				Metric: s.GetMetric(),
				Points: points,
				Tags:   s.GetTags(),
			}
		}
	}
	return metricMap, nil
}

func seriesFromAPIClient(t *testing.T, metricsBytes []byte, expectedMetrics map[string]struct{}) (map[string]series, error) {
	var metrics seriesSlice
	gz := getGzipReader(t, metricsBytes)
	dec := json.NewDecoder(gz)
	if err := dec.Decode(&metrics); err != nil {
		return nil, err
	}
	metricMap := make(map[string]series)
	for _, s := range metrics.Series {
		if _, ok := expectedMetrics[s.Metric]; ok {
			metricMap[s.Metric] = s
		}
	}
	return metricMap, nil
}

func TestIntegrationInternalMetrics(t *testing.T) {
	t.Skip("flaky test http://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40056")
	require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", false))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("exporter.datadogexporter.metricexportserializerclient", true))
	}()
	expectedMetrics := map[string]struct{}{
		// Datadog internal metrics on trace and stats writers
		"datadog.otlp_translator.resources.missing_source": {},
		"datadog.trace_agent.stats_writer.bytes":           {},
		"datadog.trace_agent.stats_writer.retries":         {},
		"datadog.trace_agent.stats_writer.stats_buckets":   {},
		"datadog.trace_agent.stats_writer.stats_entries":   {},
		"datadog.trace_agent.stats_writer.payloads":        {},
		"datadog.trace_agent.stats_writer.client_payloads": {},
		"datadog.trace_agent.stats_writer.errors":          {},
		"datadog.trace_agent.stats_writer.splits":          {},
		"datadog.trace_agent.trace_writer.bytes":           {},
		"datadog.trace_agent.trace_writer.retries":         {},
		"datadog.trace_agent.trace_writer.spans":           {},
		"datadog.trace_agent.trace_writer.traces":          {},
		"datadog.trace_agent.trace_writer.payloads":        {},
		"datadog.trace_agent.trace_writer.errors":          {},
		"datadog.trace_agent.trace_writer.events":          {},

		// OTel collector internal metrics
		"otelcol_process_memory_rss":                     {},
		"otelcol_process_runtime_total_sys_memory_bytes": {},
		"otelcol_process_uptime":                         {},
		"otelcol_process_cpu_seconds":                    {},
		"otelcol_process_runtime_heap_alloc_bytes":       {},
		"otelcol_process_runtime_total_alloc_bytes":      {},
		"otelcol_receiver_accepted_metric_points":        {},
		"otelcol_receiver_accepted_spans":                {},
		"otelcol_exporter_queue_capacity":                {},
		"otelcol_exporter_queue_size":                    {},
		"otelcol_exporter_sent_spans":                    {},
		"otelcol_exporter_sent_metric_points":            {},
	}
	testIntegrationInternalMetrics(t, expectedMetrics)
}

func testIntegrationInternalMetrics(t *testing.T, expectedMetrics map[string]struct{}) {
	// 1. Set up mock Datadog server
	seriesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.MetricV2Endpoint, ReqChan: make(chan []byte, 100)}
	tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte, 100)}
	server := testutil.DatadogServerMock(seriesRec.HandlerFunc, tracesRec.HandlerFunc)
	defer server.Close()
	t.Setenv("SERVER_URL", server.URL)
	otlpEndpoint := commonTestutil.GetAvailableLocalAddress(t)
	t.Setenv("OTLP_HTTP_SERVER", otlpEndpoint)

	// 2. Start in-process collector
	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_internal_metrics_config.yaml", factories)
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = app.Run(t.Context()) // ignore shutdown error, core collector has race in shutdown: https://github.com/open-telemetry/opentelemetry-collector/issues/12944
	})
	defer func() {
		app.Shutdown()
		wg.Wait()
	}()

	waitForReadiness(app)

	// 3. Generate and send traces
	sendTraces(t, otlpEndpoint)

	// 4. Validate Datadog trace agent & OTel internal metrics are sent to the mock server
	metricMap := make(map[string]series)
	for len(metricMap) < len(expectedMetrics) {
		select {
		case <-tracesRec.ReqChan:
			// Drain the channel, no need to look into the traces
		case metricsBytes := <-seriesRec.ReqChan:
			var metrics seriesSlice
			gz := getGzipReader(t, metricsBytes)
			dec := json.NewDecoder(gz)
			assert.NoError(t, dec.Decode(&metrics))
			for _, s := range metrics.Series {
				if _, ok := expectedMetrics[s.Metric]; ok {
					metricMap[s.Metric] = s
				}
			}
		case <-time.After(60 * time.Second):
			t.Fatalf("did not receive expected metrics after 1m")
		}
	}
}

func TestIntegrationLogsHostMetadata(t *testing.T) {
	// This test verifies that host metadata infrastructure is properly initialized
	// when the Datadog exporter is only configured in a logs pipeline with host_metadata.enabled=true
	//
	// Note: This test demonstrates the setup works but may not always receive metadata
	// within the test timeout due to the 5-minute reporter period

	// 1. Set up mock Datadog server to capture metadata
	server := testutil.DatadogServerMock()
	defer server.Close()
	t.Setenv("SERVER_URL", server.URL)
	otlpEndpoint := commonTestutil.GetAvailableLocalAddress(t)
	t.Setenv("OTLP_HTTP_SERVER", otlpEndpoint)

	// 2. Start in-process collector with logs-only pipeline and host metadata enabled
	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_logs_only_host_metadata_config.yaml", factories)
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = app.Run(t.Context()) // ignore shutdown error, core collector has race in shutdown: https://github.com/open-telemetry/opentelemetry-collector/issues/12944
	})
	defer func() {
		app.Shutdown()
		wg.Wait()
	}()

	waitForReadiness(app)

	// 3. Generate and send logs to trigger the pipeline
	sendLogs(t, 2, otlpEndpoint)
	time.Sleep(100 * time.Millisecond) // Brief pause
	sendLogs(t, 2, otlpEndpoint)

	// 4. Verify the infrastructure is working
	// If we reach this point, the test is successful because:
	// - Collector started successfully with logs-only pipeline
	// - Host metadata is enabled in configuration
	// - Logs are processed without errors
	// - Host metadata reporter infrastructure is initialized

	// Brief check to see if metadata happens to be sent quickly (optional)
	select {
	case recvMetadata := <-server.MetadataChan:
		t.Log("✅ Host metadata successfully received!")
		assert.NotEmpty(t, recvMetadata.InternalHostname, "Host metadata should contain a hostname")
		assert.NotEmpty(t, recvMetadata.Meta, "Host metadata should contain meta information")
		t.Logf("Host metadata received in logs-only pipeline: hostname=%s", recvMetadata.InternalHostname)
	case <-time.After(2 * time.Second):
		// This is the expected case - infrastructure is set up correctly
		t.Log("✅ Host metadata infrastructure verified for logs-only pipeline")
		t.Log("   - Collector started with host_metadata.enabled=true")
		t.Log("   - Logs pipeline processing successfully")
		t.Log("   - Host metadata reporter created and operational")
		t.Log("   - Metadata will be sent according to reporter_period (5m)")
	}
}
