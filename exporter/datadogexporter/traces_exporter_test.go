// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions127 "go.opentelemetry.io/collector/semconv/v1.27.0"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	pkgdatadog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func setupTestMain(m *testing.M) {
	tracelog.SetLogger(&testlogger{})
	os.Exit(m.Run())
}

type testlogger struct{}

// Trace implements Logger.
func (testlogger) Trace(_ ...any) {}

// Tracef implements Logger.
func (testlogger) Tracef(_ string, _ ...any) {}

// Debug implements Logger.
func (testlogger) Debug(v ...any) { fmt.Println("DEBUG", fmt.Sprint(v...)) }

// Debugf implements Logger.
func (testlogger) Debugf(format string, params ...any) {
	fmt.Println("DEBUG", fmt.Sprintf(format, params...))
}

// Info implements Logger.
func (testlogger) Info(v ...any) { fmt.Println("INFO", fmt.Sprint(v...)) }

// Infof implements Logger.
func (testlogger) Infof(format string, params ...any) {
	fmt.Println("INFO", fmt.Sprintf(format, params...))
}

// Warn implements Logger.
func (testlogger) Warn(v ...any) error {
	fmt.Println("WARN", fmt.Sprint(v...))
	return nil
}

// Warnf implements Logger.
func (testlogger) Warnf(format string, params ...any) error {
	fmt.Println("WARN", fmt.Sprintf(format, params...))
	return nil
}

// Error implements Logger.
func (testlogger) Error(v ...any) error {
	fmt.Println("ERROR", fmt.Sprint(v...))
	return nil
}

// Errorf implements Logger.
func (testlogger) Errorf(format string, params ...any) error {
	fmt.Println("ERROR", fmt.Sprintf(format, params...))
	return nil
}

// Critical implements Logger.
func (testlogger) Critical(v ...any) error {
	fmt.Println("CRITICAL", fmt.Sprint(v...))
	return nil
}

// Criticalf implements Logger.
func (testlogger) Criticalf(format string, params ...any) error {
	fmt.Println("CRITICAL", fmt.Sprintf(format, params...))
	return nil
}

// Flush implements Logger.
func (testlogger) Flush() {}

func TestTracesSource(t *testing.T) {
	t.Run("ReceiveResourceSpansV1", func(t *testing.T) {
		testTracesSource(t, false)
	})

	t.Run("ReceiveResourceSpansV2", func(t *testing.T) {
		testTracesSource(t, true)
	})
}

func testTracesSource(t *testing.T, enableReceiveResourceSpansV2 bool) {
	prevVal := pkgdatadog.ReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", enableReceiveResourceSpansV2))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()

	reqs := make(chan []byte, 1)
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var expectedMetricEndpoint string
		if isMetricExportV2Enabled() {
			expectedMetricEndpoint = testutil.MetricV2Endpoint
		} else {
			expectedMetricEndpoint = testutil.MetricV1Endpoint
		}
		if r.URL.Path != expectedMetricEndpoint {
			// we only want to capture series payloads
			return
		}
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(r.Body)
		assert.NoError(t, err, "Metrics server handler error: %v", err)
		reqs <- buf.Bytes()
		_, err = w.Write([]byte("{\"status\": \"ok\"}"))
		assert.NoError(t, err)
	}))
	defer metricsServer.Close()
	tracesServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer tracesServer.Close()

	cfg := datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: datadogconfig.TagsConfig{
			Hostname: "fallbackHostname",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: metricsServer.URL},
		},
		Traces: datadogconfig.TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: tracesServer.URL},
			TracesConfig: datadogconfig.TracesConfig{
				IgnoreResources: []string{},
			},
		},
	}

	assert := assert.New(t)
	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()
	exporter, err := f.CreateTraces(context.Background(), params, &cfg)
	assert.NoError(err)

	// Payload specifies a sub-set of a Zorkian metrics series payload.
	type Payload struct {
		Series []struct {
			Host string   `json:"host,omitempty"`
			Tags []string `json:"tags,omitempty"`
		} `json:"series"`
	}
	// getHostTags extracts the host and tags from the Zorkian metrics series payload
	// body found in data.
	getHostTags := func(data []byte) (host string, tags []string) {
		var p Payload
		assert.NoError(json.Unmarshal(data, &p))
		assert.Len(p.Series, 1)
		return p.Series[0].Host, p.Series[0].Tags
	}
	// getHostTagsV2 extracts the host and tags from the native DatadogV2 metrics series payload
	// body found in data.
	getHostTagsV2 := func(data []byte) (host string, tags []string) {
		buf := bytes.NewBuffer(data)
		reader, derr := gzip.NewReader(buf)
		assert.NoError(derr)
		dec := json.NewDecoder(reader)
		var p datadogV2.MetricPayload
		assert.NoError(dec.Decode(&p))
		assert.GreaterOrEqual(len(p.Series), 1)
		assert.Len(p.Series[0].Resources, 1)
		return *p.Series[0].Resources[0].Name, p.Series[0].Tags
	}
	for _, tt := range []struct {
		attrs map[string]any
		host  string
		tags  []string
	}{
		{
			attrs: map[string]any{},
			host:  "fallbackHostname",
			tags:  []string{"version:latest", "command:otelcol"},
		},
		{
			attrs: map[string]any{
				attributes.AttributeDatadogHostname: "customName",
			},
			host: "customName",
			tags: []string{"version:latest", "command:otelcol"},
		},
		{
			attrs: map[string]any{
				semconv.AttributeCloudProvider:      semconv.AttributeCloudProviderAWS,
				semconv.AttributeCloudPlatform:      semconv.AttributeCloudPlatformAWSECS,
				semconv.AttributeAWSECSTaskARN:      "example-task-ARN",
				semconv.AttributeAWSECSTaskFamily:   "example-task-family",
				semconv.AttributeAWSECSTaskRevision: "example-task-revision",
				semconv.AttributeAWSECSLaunchtype:   semconv.AttributeAWSECSLaunchtypeFargate,
			},
			host: "",
			tags: []string{"version:latest", "command:otelcol", "task_arn:example-task-ARN"},
		},
	} {
		t.Run("", func(t *testing.T) {
			ctx := context.Background()
			err = exporter.ConsumeTraces(ctx, simpleTraces(tt.attrs, nil, ptrace.SpanKindInternal))
			assert.NoError(err)
			timeout := time.After(time.Second)
			select {
			case data := <-reqs:
				var host string
				var tags []string
				if isMetricExportV2Enabled() {
					host, tags = getHostTagsV2(data)
				} else {
					host, tags = getHostTags(data)
				}
				assert.Equal(tt.host, host)
				assert.Equal(tt.tags, tags)
			case <-timeout:
				t.Fatal("timeout")
			}
		})
	}
}

func TestTraceExporter(t *testing.T) {
	t.Run("ReceiveResourceSpansV1", func(t *testing.T) {
		testTraceExporter(t, false)
	})

	t.Run("ReceiveResourceSpansV2", func(t *testing.T) {
		testTraceExporter(t, true)
	})
}

func testTraceExporter(t *testing.T, enableReceiveResourceSpansV2 bool) {
	prevVal := pkgdatadog.ReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", enableReceiveResourceSpansV2))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()
	metricsServer := testutil.DatadogServerMock()
	defer metricsServer.Close()

	got := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", req.Header.Get("DD-Api-Key"))
		got <- req.Header.Get("Content-Type")
		rw.WriteHeader(http.StatusAccepted)
	}))

	defer server.Close()
	cfg := datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: datadogconfig.TagsConfig{
			Hostname: "test-host",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: metricsServer.URL,
			},
		},
		Traces: datadogconfig.TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: server.URL,
			},
			TracesConfig: datadogconfig.TracesConfig{
				IgnoreResources: []string{},
			},
			TraceBuffer: 2,
		},
	}
	cfg.Traces.SetFlushInterval(0.1)

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()
	exporter, err := f.CreateTraces(context.Background(), params, &cfg)
	assert.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, simpleTraces(nil, nil, ptrace.SpanKindInternal))
	assert.NoError(t, err)
	timeout := time.After(2 * time.Second)
	select {
	case out := <-got:
		require.Equal(t, "application/x-protobuf", out)
	case <-timeout:
		t.Fatal("Timed out")
	}
	require.NoError(t, exporter.Shutdown(context.Background()))
}

func TestNewTracesExporter(t *testing.T) {
	metricsServer := testutil.DatadogServerMock()
	defer metricsServer.Close()

	cfg := &datadogconfig.Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	cfg.Metrics.Endpoint = metricsServer.URL
	params := exportertest.NewNopSettings(metadata.Type)

	// The client should have been created correctly
	f := NewFactory()
	exp, err := f.CreateTraces(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestPushTraceData(t *testing.T) {
	t.Run("ReceiveResourceSpansV1", func(t *testing.T) {
		testPushTraceData(t, false)
	})

	t.Run("ReceiveResourceSpansV2", func(t *testing.T) {
		testPushTraceData(t, true)
	})
}

func testPushTraceData(t *testing.T, enableReceiveResourceSpansV2 bool) {
	prevVal := pkgdatadog.ReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", enableReceiveResourceSpansV2))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()
	server := testutil.DatadogServerMock()
	defer server.Close()
	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: datadogconfig.TagsConfig{
			Hostname: "test-host",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
		Traces: datadogconfig.TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled:        true,
			ReporterPeriod: 30 * time.Minute,
		},
	}

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()
	exp, err := f.CreateTraces(context.Background(), params, cfg)
	assert.NoError(t, err)

	testTraces := ptrace.NewTraces()
	testutil.TestTraces.CopyTo(testTraces)
	err = exp.ConsumeTraces(context.Background(), testTraces)
	assert.NoError(t, err)

	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func TestPushTraceDataNewEnvConvention(t *testing.T) {
	t.Run("ReceiveResourceSpansV1", func(t *testing.T) {
		testPushTraceDataNewEnvConvention(t, false)
	})

	t.Run("ReceiveResourceSpansV2", func(t *testing.T) {
		testPushTraceDataNewEnvConvention(t, true)
	})
}

func testPushTraceDataNewEnvConvention(t *testing.T, enableReceiveResourceSpansV2 bool) {
	prevVal := pkgdatadog.ReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", enableReceiveResourceSpansV2))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()

	tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte)}
	server := testutil.DatadogServerMock(tracesRec.HandlerFunc)
	defer server.Close()
	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: datadogconfig.TagsConfig{
			Hostname: "test-host",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
		Traces: datadogconfig.TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
	}
	cfg.Traces.SetFlushInterval(0.1)

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()
	exp, err := f.CreateTraces(context.Background(), params, cfg)
	assert.NoError(t, err)

	err = exp.ConsumeTraces(context.Background(), simpleTraces(map[string]any{conventions127.AttributeDeploymentEnvironmentName: "new_env"}, nil, ptrace.SpanKindInternal))
	assert.NoError(t, err)

	reqBytes := <-tracesRec.ReqChan
	buf := bytes.NewBuffer(reqBytes)
	reader, err := gzip.NewReader(buf)
	require.NoError(t, err)
	slurp, err := io.ReadAll(reader)
	require.NoError(t, err)
	var traces pb.AgentPayload
	require.NoError(t, proto.Unmarshal(slurp, &traces))
	assert.Len(t, traces.TracerPayloads, 1)
	assert.Equal(t, "new_env", traces.TracerPayloads[0].GetEnv())
}

func TestPushTraceData_OperationAndResourceNameV2(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("datadog.EnableOperationAndResourceNameV2", true)
	if err != nil {
		t.Fatal(err)
	}
	tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte)}
	server := testutil.DatadogServerMock(tracesRec.HandlerFunc)
	defer server.Close()
	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: datadogconfig.TagsConfig{
			Hostname: "test-host",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
		Traces: datadogconfig.TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
	}
	cfg.Traces.SetFlushInterval(0.1)

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()
	exp, err := f.CreateTraces(context.Background(), params, cfg)
	assert.NoError(t, err)

	err = exp.ConsumeTraces(context.Background(), simpleTraces(map[string]any{conventions127.AttributeDeploymentEnvironmentName: "new_env"}, nil, ptrace.SpanKindServer))
	assert.NoError(t, err)

	reqBytes := <-tracesRec.ReqChan
	buf := bytes.NewBuffer(reqBytes)
	reader, err := gzip.NewReader(buf)
	require.NoError(t, err)
	slurp, err := io.ReadAll(reader)
	require.NoError(t, err)
	var traces pb.AgentPayload
	require.NoError(t, proto.Unmarshal(slurp, &traces))
	assert.Len(t, traces.TracerPayloads, 1)
	assert.Equal(t, "new_env", traces.TracerPayloads[0].GetEnv())
	assert.Equal(t, "server.request", traces.TracerPayloads[0].Chunks[0].Spans[0].Name)
}

func TestResRelatedAttributesInSpanAttributes_ReceiveResourceSpansV2Enabled(t *testing.T) {
	prevVal := pkgdatadog.ReceiveResourceSpansV2FeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("datadog.EnableReceiveResourceSpansV2", prevVal))
	}()

	tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte)}
	server := testutil.DatadogServerMock(tracesRec.HandlerFunc)
	defer server.Close()
	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: datadogconfig.TagsConfig{
			Hostname: "test-host",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
		Traces: datadogconfig.TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL},
		},
	}
	cfg.Traces.SetFlushInterval(0.1)

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()
	exp, err := f.CreateTraces(context.Background(), params, cfg)
	assert.NoError(t, err)

	sattr := map[string]any{
		"datadog.host.name":           "do-not-use",
		"container.id":                "do-not-use",
		"k8s.pod.id":                  "do-not-use",
		"deployment.environment.name": "do-not-use",
		"service.name":                "do-not-use",
		"service.version":             "do-not-use",
	}
	err = exp.ConsumeTraces(context.Background(), simpleTraces(nil, sattr, ptrace.SpanKindInternal))
	assert.NoError(t, err)

	reqBytes := <-tracesRec.ReqChan
	buf := bytes.NewBuffer(reqBytes)
	reader, err := gzip.NewReader(buf)
	require.NoError(t, err)
	slurp, err := io.ReadAll(reader)
	require.NoError(t, err)
	var traces pb.AgentPayload
	require.NoError(t, proto.Unmarshal(slurp, &traces))
	assert.Len(t, traces.TracerPayloads, 1)
	tracerPayload := traces.TracerPayloads[0]
	span := tracerPayload.Chunks[0].Spans[0]
	assert.Equal(t, "test-host", tracerPayload.Hostname)
	assert.Empty(t, tracerPayload.ContainerID)
	assert.Empty(t, tracerPayload.Env)
	assert.Equal(t, "otlpresourcenoservicename", span.Service)
	assert.Empty(t, span.Meta["version"])
}

func simpleTraces(rattrs map[string]any, sattrs map[string]any, kind ptrace.SpanKind) ptrace.Traces {
	return genTraces([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, rattrs, sattrs, kind)
}

func genTraces(traceID pcommon.TraceID, rattrs map[string]any, sattrs map[string]any, kind ptrace.SpanKind) ptrace.Traces {
	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	span := rspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{0, 0, 0, 0, 1, 2, 3, 4})
	span.SetKind(kind)
	if rattrs == nil {
		return traces
	}
	//nolint:errcheck
	rspans.Resource().Attributes().FromRaw(rattrs)
	if sattrs != nil {
		err := span.Attributes().FromRaw(sattrs)
		if err != nil {
			return traces
		}
	}
	return traces
}
