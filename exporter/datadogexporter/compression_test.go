// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package datadogexporter

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	traceconfig "github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// The Datadog exporter MUST send every signal gzip-compressed. This is a single
// source of truth that guards against one signal silently diverging from the
// others (which is exactly how metric sketches once shipped uncompressed).
//
// Compression contract enforced here:
//
//	Signal              Path                          Compression  Set by
//	------------------  ----------------------------  -----------  -----------------------------------------------
//	Metrics — series    native datadog-api client     gzip         clientutil.GZipSubmitMetricsOptionalParameters
//	Metrics — sketches  manual POST in pushSketches    gzip         clientutil.ProtobufHeaders + gzip writer
//	Traces              embedded trace agent          gzip         gzip.NewComponent() in newTraceAgent
//	Logs                embedded logs pipeline        gzip         compression_kind=gzip in agentcomponents
//
// APM stats use the same trace-agent gzip component as traces but only flow when
// the traces and metrics exporters are wired together through the internal stats
// channel (only the full collector does this), so they are guarded in
// integrationtest/integration_test.go rather than here.

// assertGzip fails unless the request advertised gzip and its body really is a
// non-empty gzip stream.
func assertGzip(t *testing.T, signal string, hdr http.Header, body []byte) {
	t.Helper()
	assert.Equalf(t, "gzip", hdr.Get("Content-Encoding"), "%s: Content-Encoding must be gzip", signal)
	zr, err := gzip.NewReader(bytes.NewReader(body))
	require.NoErrorf(t, err, "%s: body is not a valid gzip stream", signal)
	decoded, err := io.ReadAll(zr)
	require.NoErrorf(t, err, "%s: failed to gunzip body", signal)
	assert.NotEmptyf(t, decoded, "%s: decompressed body should not be empty", signal)
}

type capturedRequest struct {
	header http.Header
	body   []byte
}

// headerBodyRecorder records the headers and raw body of requests to a path.
// Unlike testutil.HTTPRequestRecorderWithChan (body only) it also captures
// headers, which the compression contract needs, and it synchronizes through a
// channel so it is safe for the asynchronous trace/log send paths.
type headerBodyRecorder struct {
	pattern string
	reqs    chan capturedRequest
}

func (rec *headerBodyRecorder) HandlerFunc() (string, http.HandlerFunc) {
	return rec.pattern, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case rec.reqs <- capturedRequest{header: r.Header.Clone(), body: body}:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// TestSignalCompressionMetrics asserts that both metric payloads — series and
// sketches — are gzip-compressed by the native metrics client.
func TestSignalCompressionMetrics(t *testing.T) {
	seriesRec := &testutil.HTTPRequestRecorder{Pattern: testutil.MetricV2Endpoint}
	sketchRec := &testutil.HTTPRequestRecorder{Pattern: testutil.SketchesMetricEndpoint}
	server := testutil.DatadogServerMock(seriesRec.HandlerFunc, sketchRec.HandlerFunc)
	defer server.Close()

	var once sync.Once
	reporter, err := inframetadata.NewReporter(zap.NewNop(), newTestPusher(t), time.Second)
	require.NoError(t, err)
	attrsTranslator, err := attributes.NewTranslator(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	exp, err := newMetricsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		// Distributions mode turns the histogram in createTestMetrics into a
		// sketch, so this single push exercises both the series and the
		// sketches paths.
		newTestConfig(t, server.URL, nil, datadogconfig.HistogramModeDistributions),
		traceconfig.New(),
		&once,
		attrsTranslator,
		&testutil.MockSourceProvider{Src: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}},
		reporter,
		nil,
		attributes.NewGatewayUsage(),
	)
	require.NoError(t, err)
	exp.getPushTime = func() uint64 { return 0 }

	require.NoError(t, exp.PushMetricsData(t.Context(), createTestMetrics(nil)))

	require.NotNil(t, seriesRec.ByteBody, "expected a series payload to be sent")
	assertGzip(t, "metrics series", seriesRec.Header, seriesRec.ByteBody)

	require.NotNil(t, sketchRec.ByteBody, "expected a sketches payload to be sent")
	assertGzip(t, "metrics sketches", sketchRec.Header, sketchRec.ByteBody)
}

// TestSignalCompressionTraces asserts that the trace payload is gzip-compressed
// by the embedded trace agent.
func TestSignalCompressionTraces(t *testing.T) {
	tracesRec := &headerBodyRecorder{pattern: testutil.TraceEndpoint, reqs: make(chan capturedRequest, 4)}
	server := testutil.DatadogServerMock(tracesRec.HandlerFunc)
	defer server.Close()

	cfg := &datadogconfig.Config{
		API:        datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		TagsConfig: datadogconfig.TagsConfig{Hostname: "test-host"},
		Metrics:    datadogconfig.MetricsConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
		Traces:     datadogconfig.TracesExporterConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
	}
	cfg.Traces.SetFlushInterval(0.1)

	f := NewFactory()
	exp, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exp.ConsumeTraces(t.Context(), simpleTraces(nil, nil, ptrace.SpanKindInternal)))

	select {
	case req := <-tracesRec.reqs:
		assertGzip(t, "traces", req.header, req.body)
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for a trace request")
	}
}

// TestSignalCompressionLogs asserts that the logs payload is gzip-compressed by
// the embedded logs pipeline.
func TestSignalCompressionLogs(t *testing.T) {
	// Capture every request to the logs intake. The logs agent may send a
	// startup connectivity probe (empty/uncompressed) before the real batch, so
	// we record all requests and pick out the actual log payload below rather
	// than guessing which request is which.
	logsReqs := make(chan capturedRequest, 8)
	server := testutil.DatadogLogServerMock(func() (string, http.HandlerFunc) {
		return "/api/v2/logs", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			select {
			case logsReqs <- capturedRequest{header: r.Header.Clone(), body: body}:
			default:
			}
			w.WriteHeader(http.StatusAccepted)
		}
	})
	defer server.Close()

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Logs: datadogconfig.LogsConfig{
			TCPAddrConfig:    confignet.TCPAddrConfig{Endpoint: server.URL},
			UseCompression:   true,
			CompressionLevel: 6,
			BatchWait:        1,
		},
	}

	f := NewFactory()
	exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	// The logs pipeline only sends once the exporter is started.
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { assert.NoError(t, exp.Shutdown(t.Context())) }()

	require.NoError(t, exp.ConsumeLogs(t.Context(), testutil.GenerateLogsOneLogRecord()))

	// Drain requests until we see the real log batch: a non-empty, gzip-decodable
	// body. Empty or non-gzip startup probes are skipped.
	deadline := time.After(60 * time.Second)
	sawRequest := false
	for {
		select {
		case req := <-logsReqs:
			sawRequest = true
			zr, zerr := gzip.NewReader(bytes.NewReader(req.body))
			if zerr != nil {
				continue
			}
			decoded, rerr := io.ReadAll(zr)
			if rerr != nil || len(decoded) == 0 {
				continue
			}
			assert.Equal(t, "gzip", req.header.Get("Content-Encoding"), "logs payload must advertise gzip")
			return
		case <-deadline:
			if sawRequest {
				t.Fatal("received logs request(s) but none were a non-empty gzip payload — logs may not be compressed")
			}
			t.Fatal("timed out waiting for a logs request")
		}
	}
}
