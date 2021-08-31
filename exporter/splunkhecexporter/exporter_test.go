// Copyright 2019, OpenTelemetry Authors
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

package splunkhecexporter

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

func TestNew(t *testing.T) {
	buildInfo := component.DefaultBuildInfo()
	got, err := createExporter(nil, zap.NewNop(), &buildInfo)
	assert.EqualError(t, err, "nil config")
	assert.Nil(t, got)

	config := &Config{
		Token:           "someToken",
		Endpoint:        "https://example.com:8088",
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
	}
	got, err = createExporter(config, zap.NewNop(), &buildInfo)
	assert.NoError(t, err)
	require.NotNil(t, got)

	config = &Config{
		Token:           "someToken",
		Endpoint:        "https://example.com:8088",
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
		TLSSetting: configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile:   "file-not-found",
				CertFile: "file-not-found",
				KeyFile:  "file-not-found",
			},
			InsecureSkipVerify: false,
		},
	}
	got, err = createExporter(config, zap.NewNop(), &buildInfo)
	assert.Error(t, err)
	require.Nil(t, got)
}

func TestConsumeMetricsData(t *testing.T) {
	smallBatch := &agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			Attributes: map[string]string{
				"com.splunk.source": "test_splunk",
			},
		},
		Resource: &resourcepb.Resource{Type: "test"},
		Metrics: []*metricspb.Metric{
			metricstestutil.Gauge(
				"test_gauge",
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					metricstestutil.Double(time.Now(), 123))),
		},
	}
	tests := []struct {
		name             string
		md               *agentmetricspb.ExportMetricsServiceRequest
		reqTestFunc      func(t *testing.T, r *http.Request)
		httpResponseCode int
		wantErr          bool
	}{
		{
			name: "happy_path",
			md:   smallBatch,
			reqTestFunc: func(t *testing.T, r *http.Request) {
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, "keep-alive", r.Header.Get("Connection"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "OpenTelemetry-Collector Splunk Exporter/v0.0.1", r.Header.Get("User-Agent"))
				assert.Equal(t, "Splunk 1234", r.Header.Get("Authorization"))
				if r.Header.Get("Content-Encoding") == "gzip" {
					t.Fatal("Small batch should not be compressed")
				}
				firstPayload := strings.Split(string(body), "\n")[0]
				var metric splunk.Event
				err = json.Unmarshal([]byte(firstPayload), &metric)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, "test_splunk", metric.Source)
				assert.Equal(t, "test_type", metric.SourceType)
				assert.Equal(t, "test_index", metric.Index)

			},
			httpResponseCode: http.StatusAccepted,
		},
		{
			name:             "response_forbidden",
			md:               smallBatch,
			reqTestFunc:      nil,
			httpResponseCode: http.StatusForbidden,
			wantErr:          true,
		},
		{
			name:             "large_batch",
			md:               generateLargeBatch(),
			reqTestFunc:      nil,
			httpResponseCode: http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.reqTestFunc != nil {
					tt.reqTestFunc(t, r)
				}
				w.WriteHeader(tt.httpResponseCode)
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			options := &exporterOptions{
				url:   serverURL,
				token: "1234",
			}

			config := NewFactory().CreateDefaultConfig().(*Config)
			config.Source = "test"
			config.SourceType = "test_type"
			config.Token = "1234"
			config.Index = "test_index"
			config.SplunkAppName = "OpenTelemetry-Collector Splunk Exporter"
			config.SplunkAppVersion = "v0.0.1"

			sender, err := buildClient(options, config, zap.NewNop())
			assert.NoError(t, err)

			md := internaldata.OCToMetrics(tt.md.Node, tt.md.Resource, tt.md.Metrics)
			err = sender.pushMetricsData(context.Background(), md)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func generateLargeBatch() *agentmetricspb.ExportMetricsServiceRequest {
	md := &agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test_splunkhec"},
		},
		Resource: &resourcepb.Resource{Type: "test"},
	}

	ts := time.Now()
	for i := 0; i < 65000; i++ {
		md.Metrics = append(md.Metrics,
			metricstestutil.Gauge(
				"test_"+strconv.Itoa(i),
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					&metricspb.Point{
						Timestamp: timestamppb.New(ts),
						Value:     &metricspb.Point_Int64Value{Int64Value: int64(i)},
					},
				),
			),
		)
	}

	return md
}

func generateLargeLogsBatch() pdata.Logs {
	logs := pdata.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	ill.Logs().EnsureCapacity(65000)
	ts := pdata.Timestamp(123)
	for i := 0; i < 65000; i++ {
		logRecord := ill.Logs().AppendEmpty()
		logRecord.Body().SetStringVal("mylog")
		logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
		logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
		logRecord.Attributes().InsertString(splunk.DefaultIndexLabel, "myindex")
		logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
		logRecord.Attributes().InsertString("custom", "custom")
		logRecord.SetTimestamp(ts)
	}

	return logs
}

func TestConsumeLogsData(t *testing.T) {
	smallBatch := pdata.NewLogs()
	logRecord := smallBatch.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
	logRecord.Body().SetStringVal("mylog")
	logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
	logRecord.Attributes().InsertString("custom", "custom")
	logRecord.SetTimestamp(123)
	tests := []struct {
		name             string
		ld               pdata.Logs
		reqTestFunc      func(t *testing.T, r *http.Request)
		httpResponseCode int
		wantErr          bool
	}{
		{
			name: "happy_path",
			ld:   smallBatch,
			reqTestFunc: func(t *testing.T, r *http.Request) {
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, "keep-alive", r.Header.Get("Connection"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "OpenTelemetry-Collector Splunk Exporter/v0.0.1", r.Header.Get("User-Agent"))
				assert.Equal(t, "Splunk 1234", r.Header.Get("Authorization"))
				if r.Header.Get("Content-Encoding") == "gzip" {
					t.Fatal("Small batch should not be compressed")
				}
				firstPayload := strings.Split(string(body), "\n")[0]
				var event splunk.Event
				err = json.Unmarshal([]byte(firstPayload), &event)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, "test", event.Source)
				assert.Equal(t, "test_type", event.SourceType)
				assert.Equal(t, "test_index", event.Index)

			},
			httpResponseCode: http.StatusAccepted,
		},
		{
			name:             "response_forbidden",
			ld:               smallBatch,
			reqTestFunc:      nil,
			httpResponseCode: http.StatusForbidden,
			wantErr:          true,
		},
		{
			name:             "large_batch",
			ld:               generateLargeLogsBatch(),
			reqTestFunc:      nil,
			httpResponseCode: http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.reqTestFunc != nil {
					tt.reqTestFunc(t, r)
				}
				w.WriteHeader(tt.httpResponseCode)
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			options := &exporterOptions{
				url:   serverURL,
				token: "1234",
			}

			config := NewFactory().CreateDefaultConfig().(*Config)
			config.Source = "test"
			config.SourceType = "test_type"
			config.Token = "1234"
			config.Index = "test_index"
			config.SplunkAppName = "OpenTelemetry-Collector Splunk Exporter"
			config.SplunkAppVersion = "v0.0.1"

			sender, err := buildClient(options, config, zap.NewNop())
			assert.NoError(t, err)

			err = sender.pushLogData(context.Background(), tt.ld)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestExporterStartAlwaysReturnsNil(t *testing.T) {
	buildInfo := component.DefaultBuildInfo()
	config := &Config{
		Endpoint: "https://example.com:8088",
		Token:    "abc",
	}
	e, err := createExporter(config, zap.NewNop(), &buildInfo)
	assert.NoError(t, err)
	assert.NoError(t, e.start(context.Background(), componenttest.NewNopHost()))
}
