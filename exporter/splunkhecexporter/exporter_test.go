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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func TestNew(t *testing.T) {
	buildInfo := component.NewDefaultBuildInfo()
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

func TestNewWithHealthCheckSuccess(t *testing.T) {

	rr := make(chan receivedRequest)
	capture := CapturingData{receivedRequest: rr, statusCode: 200}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := &http.Server{
		Handler: &capture,
	}
	defer s.Close()
	go func() {
		if e := s.Serve(listener); e != http.ErrServerClosed {
			require.NoError(t, e)
		}
	}()

	endpoint := "http://" + listener.Addr().String() + "/services/collector"

	config := &Config{
		Token:                 "someToken",
		Endpoint:              endpoint,
		TimeoutSettings:       exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
		HecHealthCheckEnabled: true,
	}
	buildInfo := component.NewDefaultBuildInfo()
	got, err := createExporter(config, zap.NewNop(), &buildInfo)
	assert.NoError(t, err)
	require.NotNil(t, got)

}

func TestNewWithHealthCheckFail(t *testing.T) {

	rr := make(chan receivedRequest)
	capture := CapturingData{receivedRequest: rr, statusCode: 500}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := &http.Server{
		Handler: &capture,
	}
	defer s.Close()
	go func() {
		if e := s.Serve(listener); e != http.ErrServerClosed {
			require.NoError(t, e)
		}
	}()

	endpoint := "http://" + listener.Addr().String() + "/services/collector"

	config := &Config{
		Token:                 "someToken",
		Endpoint:              endpoint,
		TimeoutSettings:       exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
		HecHealthCheckEnabled: true,
	}
	buildInfo := component.NewDefaultBuildInfo()
	got, err := createExporter(config, zap.NewNop(), &buildInfo)
	assert.Error(t, err)
	require.Nil(t, got)

}

func TestConsumeMetricsData(t *testing.T) {
	smallBatch := pmetric.NewMetrics()
	smallBatch.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("com.splunk.source", "test_splunk")
	m := smallBatch.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test_gauge")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(123)
	tests := []struct {
		name             string
		md               pmetric.Metrics
		reqTestFunc      func(t *testing.T, r *http.Request)
		httpResponseCode int
		maxContentLength int
		wantErr          bool
	}{
		{
			name: "happy_path",
			md:   smallBatch,
			reqTestFunc: func(t *testing.T, r *http.Request) {
				body, err := io.ReadAll(r.Body)
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
			name: "large_batch",
			md:   generateLargeBatch(),
			reqTestFunc: func(t *testing.T, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, "keep-alive", r.Header.Get("Connection"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "OpenTelemetry-Collector Splunk Exporter/v0.0.1", r.Header.Get("User-Agent"))
				assert.Equal(t, "Splunk 1234", r.Header.Get("Authorization"))
				assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
				zipReader, err := gzip.NewReader(bytes.NewReader(body))
				assert.NoError(t, err)
				bodyBytes, _ := io.ReadAll(zipReader)
				firstPayload := strings.Split(string(bodyBytes), "}{")[0]
				var metric splunk.Event
				err = json.Unmarshal([]byte(firstPayload+"}"), &metric)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, "test_splunk", metric.Source)
				assert.Equal(t, "test_type", metric.SourceType)
				assert.Equal(t, "test_index", metric.Index)

			},
			maxContentLength: 1800,
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
			config.MaxContentLengthMetrics = 1800

			sender, err := buildClient(options, config, zap.NewNop())
			assert.NoError(t, err)

			err = sender.pushMetricsData(context.Background(), tt.md)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func generateLargeBatch() pmetric.Metrics {
	ts := time.Now()
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "test_splunkhec")
	rm.Resource().Attributes().PutStr(splunk.DefaultSourceLabel, "test_splunk")
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()

	for i := 0; i < 6500; i++ {
		m := ms.AppendEmpty()
		m.SetName("test_" + strconv.Itoa(i))
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("k0", "v0")
		dp.Attributes().PutStr("k1", "v1")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetIntValue(int64(i))
	}

	return metrics
}

func generateLargeLogsBatch() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().EnsureCapacity(65000)
	ts := pcommon.Timestamp(123)
	for i := 0; i < 65000; i++ {
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("mylog")
		logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
		logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
		logRecord.Attributes().PutStr(splunk.DefaultIndexLabel, "myindex")
		logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
		logRecord.Attributes().PutStr("custom", "custom")
		logRecord.SetTimestamp(ts)
	}

	return logs
}

func TestConsumeLogsData(t *testing.T) {
	smallBatch := plog.NewLogs()
	logRecord := smallBatch.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.Body().SetStr("mylog")
	logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
	logRecord.Attributes().PutStr("custom", "custom")
	logRecord.SetTimestamp(123)
	tests := []struct {
		name             string
		ld               plog.Logs
		reqTestFunc      func(t *testing.T, r *http.Request)
		httpResponseCode int
		wantErr          bool
	}{
		{
			name: "happy_path",
			ld:   smallBatch,
			reqTestFunc: func(t *testing.T, r *http.Request) {
				body, err := io.ReadAll(r.Body)
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
	buildInfo := component.NewDefaultBuildInfo()
	config := &Config{
		Endpoint: "https://example.com:8088",
		Token:    "abc",
	}
	e, err := createExporter(config, zap.NewNop(), &buildInfo)
	assert.NoError(t, err)
	assert.NoError(t, e.start(context.Background(), componenttest.NewNopHost()))
}
