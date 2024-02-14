// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata/payload"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
)

func TestMain(m *testing.M) {
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
		if _, err := buf.ReadFrom(r.Body); err != nil {
			t.Fatalf("Metrics server handler error: %v", err)
		}
		reqs <- buf.Bytes()
		_, err := w.Write([]byte("{\"status\": \"ok\"}"))
		assert.NoError(t, err)
	}))
	defer metricsServer.Close()
	tracesServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer tracesServer.Close()

	cfg := Config{
		API: APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: TagsConfig{
			Hostname: "fallbackHostname",
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{Endpoint: metricsServer.URL},
		},
		Traces: TracesConfig{
			TCPAddr:         confignet.TCPAddr{Endpoint: tracesServer.URL},
			IgnoreResources: []string{},
		},
	}

	assert := assert.New(t)
	params := exportertest.NewNopCreateSettings()
	f := NewFactory()
	exporter, err := f.CreateTracesExporter(context.Background(), params, &cfg)
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
		assert.Len(p.Series, 1)
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
			err = exporter.ConsumeTraces(ctx, simpleTracesWithAttributes(tt.attrs))
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
				assert.EqualValues(tt.tags, tags)
			case <-timeout:
				t.Fatal("timeout")
			}
		})
	}
}

func TestTraceExporter(t *testing.T) {
	metricsServer := testutil.DatadogServerMock()
	defer metricsServer.Close()

	got := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", req.Header.Get("DD-Api-Key"))
		got <- req.Header.Get("Content-Type")
		rw.WriteHeader(http.StatusAccepted)
	}))

	defer server.Close()
	cfg := Config{
		API: APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: TagsConfig{
			Hostname: "test-host",
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: metricsServer.URL,
			},
		},
		Traces: TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
			IgnoreResources: []string{},
			flushInterval:   0.1,
			TraceBuffer:     2,
		},
	}

	params := exportertest.NewNopCreateSettings()
	f := NewFactory()
	exporter, err := f.CreateTracesExporter(context.Background(), params, &cfg)
	assert.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, simpleTraces())
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

	cfg := &Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	cfg.Metrics.TCPAddr.Endpoint = metricsServer.URL
	params := exportertest.NewNopCreateSettings()

	// The client should have been created correctly
	f := NewFactory()
	exp, err := f.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestPushTraceData(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()
	cfg := &Config{
		API: APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: TagsConfig{
			Hostname: "test-host",
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{Endpoint: server.URL},
		},
		Traces: TracesConfig{
			TCPAddr: confignet.TCPAddr{Endpoint: server.URL},
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceFirstResource,
		},
	}

	params := exportertest.NewNopCreateSettings()
	f := NewFactory()
	exp, err := f.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)

	testTraces := ptrace.NewTraces()
	testutil.TestTraces.CopyTo(testTraces)
	err = exp.ConsumeTraces(context.Background(), testTraces)
	assert.NoError(t, err)

	body := <-server.MetadataChan
	var recvMetadata payload.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
}

func simpleTraces() ptrace.Traces {
	return genTraces([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, nil)
}

func simpleTracesWithAttributes(attrs map[string]any) ptrace.Traces {
	return genTraces([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, attrs)
}

func genTraces(traceID pcommon.TraceID, attrs map[string]any) ptrace.Traces {
	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	span := rspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{0, 0, 0, 0, 1, 2, 3, 4})
	if attrs == nil {
		return traces
	}
	//nolint:errcheck
	rspans.Resource().Attributes().FromRaw(attrs)
	return traces
}
