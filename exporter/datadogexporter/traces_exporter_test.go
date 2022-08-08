// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	otelconfig "go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.opentelemetry.io/collector/service/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

func TestMain(m *testing.M) {
	tracelog.SetLogger(&testlogger{})
	os.Exit(m.Run())
}

type testlogger struct{}

// Trace implements Logger.
func (testlogger) Trace(v ...interface{}) {}

// Tracef implements Logger.
func (testlogger) Tracef(format string, params ...interface{}) {}

// Debug implements Logger.
func (testlogger) Debug(v ...interface{}) { fmt.Println("DEBUG", fmt.Sprint(v...)) }

// Debugf implements Logger.
func (testlogger) Debugf(format string, params ...interface{}) {
	fmt.Println("DEBUG", fmt.Sprintf(format, params...))
}

// Info implements Logger.
func (testlogger) Info(v ...interface{}) { fmt.Println("INFO", fmt.Sprint(v...)) }

// Infof implements Logger.
func (testlogger) Infof(format string, params ...interface{}) {
	fmt.Println("INFO", fmt.Sprintf(format, params...))
}

// Warn implements Logger.
func (testlogger) Warn(v ...interface{}) error {
	fmt.Println("WARN", fmt.Sprint(v...))
	return nil
}

// Warnf implements Logger.
func (testlogger) Warnf(format string, params ...interface{}) error {
	fmt.Println("WARN", fmt.Sprintf(format, params...))
	return nil
}

// Error implements Logger.
func (testlogger) Error(v ...interface{}) error {
	fmt.Println("ERROR", fmt.Sprint(v...))
	return nil
}

// Errorf implements Logger.
func (testlogger) Errorf(format string, params ...interface{}) error {
	fmt.Println("ERROR", fmt.Sprintf(format, params...))
	return nil
}

// Critical implements Logger.
func (testlogger) Critical(v ...interface{}) error {
	fmt.Println("CRITICAL", fmt.Sprint(v...))
	return nil
}

// Criticalf implements Logger.
func (testlogger) Criticalf(format string, params ...interface{}) error {
	fmt.Println("CRITICAL", fmt.Sprintf(format, params...))
	return nil
}

// Flush implements Logger.
func (testlogger) Flush() {}

func TestTracesSource(t *testing.T) {
	reqs := make(chan []byte, 1)
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/series" {
			// we only want to capture series payloads
			return
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			t.Fatalf("Metrics server handler error: %v", err)
		}
		reqs <- buf.Bytes()
		w.Write([]byte("{\"status\": \"ok\"}")) // nolint:errcheck
	}))
	defer metricsServer.Close()
	tracesServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer tracesServer.Close()

	cfg := config.Config{
		ExporterSettings: otelconfig.NewExporterSettings(otelconfig.NewComponentID(typeStr)),
		API: config.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: config.TagsConfig{
			Hostname: "fallbackHostname",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Metrics: config.MetricsConfig{
			TCPAddr: confignet.TCPAddr{Endpoint: metricsServer.URL},
		},
		Traces: config.TracesConfig{
			TCPAddr:         confignet.TCPAddr{Endpoint: tracesServer.URL},
			IgnoreResources: []string{},
		},
	}

	assert := assert.New(t)
	params := componenttest.NewNopExporterCreateSettings()
	reg := featuregate.NewRegistry()
	reg.Apply(map[string]bool{
		metadata.HostnamePreviewFeatureGate: true,
	})
	f := newFactoryWithRegistry(reg)
	exporter, err := f.CreateTracesExporter(context.Background(), params, &cfg)
	assert.NoError(err)

	// Payload specifies a sub-set of a metrics series payload.
	type Payload struct {
		Series []struct {
			Host string   `json:"host,omitempty"`
			Tags []string `json:"tags,omitempty"`
		} `json:"series"`
	}
	// getHostTags extracts the host and tags from the metrics series payload
	// body found in data.
	getHostTags := func(data []byte) (host string, tags []string) {
		var p Payload
		assert.NoError(json.Unmarshal(data, &p))
		assert.Len(p.Series, 1)
		return p.Series[0].Host, p.Series[0].Tags
	}
	for _, tt := range []struct {
		attrs map[string]interface{}
		host  string
		tags  []string
	}{
		{
			attrs: map[string]interface{}{},
			host:  "fallbackHostname",
			tags:  []string{"version:latest", "command:otelcol"},
		},
		{
			attrs: map[string]interface{}{
				attributes.AttributeDatadogHostname: "customName",
			},
			host: "customName",
			tags: []string{"version:latest", "command:otelcol"},
		},
		{
			attrs: map[string]interface{}{
				semconv.AttributeContainerID: "containerID",
			},
			host: "containerID",
			tags: []string{"version:latest", "command:otelcol"},
		},
		{
			attrs: map[string]interface{}{
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
				host, tags := getHostTags(data)
				assert.Equal(host, tt.host)
				assert.EqualValues(tags, tt.tags)
			case <-timeout:
				t.Fatal("timeout")
			}
		})
	}
}

func TestTraceExporter(t *testing.T) {
	metricsServer := testutils.DatadogServerMock()
	defer metricsServer.Close()

	var got string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", req.Header.Get("DD-Api-Key"))
		got = req.Header.Get("Content-Type")
		rw.WriteHeader(http.StatusAccepted)
	}))

	defer server.Close()
	cfg := config.Config{
		ExporterSettings: otelconfig.NewExporterSettings(otelconfig.NewComponentID(typeStr)),
		API: config.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: config.TagsConfig{
			Hostname: "test-host",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Metrics: config.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: metricsServer.URL,
			},
		},
		Traces: config.TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
			IgnoreResources: []string{},
		},
	}

	params := componenttest.NewNopExporterCreateSettings()
	f := NewFactory()
	exporter, err := f.CreateTracesExporter(context.Background(), params, &cfg)
	assert.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, simpleTraces())
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // we need a bit of time for channels to receive things
	require.NoError(t, exporter.Shutdown(context.Background()))
	require.Equal(t, "application/x-protobuf", got)
}

func TestNewTracesExporter(t *testing.T) {
	metricsServer := testutils.DatadogServerMock()
	defer metricsServer.Close()

	cfg := &config.Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	cfg.Metrics.TCPAddr.Endpoint = metricsServer.URL
	params := componenttest.NewNopExporterCreateSettings()

	// The client should have been created correctly
	f := NewFactory()
	exp, err := f.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestPushTraceData(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()
	cfg := &config.Config{
		API: config.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: config.TagsConfig{
			Hostname: "test-host",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Metrics: config.MetricsConfig{
			TCPAddr: confignet.TCPAddr{Endpoint: server.URL},
		},
		Traces: config.TracesConfig{
			TCPAddr: confignet.TCPAddr{Endpoint: server.URL},
		},

		HostMetadata: config.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: config.HostnameSourceFirstResource,
		},
	}

	params := componenttest.NewNopExporterCreateSettings()
	f := NewFactory()
	exp, err := f.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)

	err = exp.ConsumeTraces(context.Background(), testutils.TestTraces.Clone())
	assert.NoError(t, err)

	body := <-server.MetadataChan
	var recvMetadata metadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
}

func simpleTraces() ptrace.Traces {
	return genTraces(pcommon.NewTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}), nil)
}

func simpleTracesWithAttributes(attrs map[string]interface{}) ptrace.Traces {
	return genTraces(pcommon.NewTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}), attrs)
}

func genTraces(traceID pcommon.TraceID, attrs map[string]interface{}) ptrace.Traces {
	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	span := rspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 1, 2, 3, 4}))
	if attrs == nil {
		return traces
	}
	pcommon.NewMapFromRaw(attrs).Range(func(k string, v pcommon.Value) bool {
		rspans.Resource().Attributes().Insert(k, v)
		return true
	})
	return traces
}
