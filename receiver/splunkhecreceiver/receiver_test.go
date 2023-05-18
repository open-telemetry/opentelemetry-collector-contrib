// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_splunkhecreceiver_NewLogsReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	emptyEndpointConfig := createDefaultConfig().(*Config)
	emptyEndpointConfig.Endpoint = ""
	type args struct {
		config       Config
		logsConsumer consumer.Logs
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "nil_nextConsumer",
			args: args{
				config: *defaultConfig,
			},
			wantErr: errNilNextLogsConsumer,
		},
		{
			name: "empty_endpoint",
			args: args{
				config:       *emptyEndpointConfig,
				logsConsumer: new(consumertest.LogsSink),
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "default_endpoint",
			args: args{
				config:       *defaultConfig,
				logsConsumer: consumertest.NewNop(),
			},
		},
		{
			name: "happy_path",
			args: args{
				config: Config{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: "localhost:1234",
					},
				},
				logsConsumer: consumertest.NewNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newLogsReceiver(receivertest.NewNopCreateSettings(), tt.args.config, tt.args.logsConsumer)
			assert.Equal(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func Test_splunkhecreceiver_NewMetricsReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	emptyEndpointConfig := createDefaultConfig().(*Config)
	emptyEndpointConfig.Endpoint = ""
	type args struct {
		config          Config
		metricsConsumer consumer.Metrics
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "nil_nextConsumer",
			args: args{
				config: *defaultConfig,
			},
			wantErr: errNilNextMetricsConsumer,
		},
		{
			name: "empty_endpoint",
			args: args{
				config:          *emptyEndpointConfig,
				metricsConsumer: new(consumertest.MetricsSink),
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "default_endpoint",
			args: args{
				config:          *defaultConfig,
				metricsConsumer: consumertest.NewNop(),
			},
		},
		{
			name: "happy_path",
			args: args{
				config: Config{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: "localhost:1234",
					},
				},
				metricsConsumer: consumertest.NewNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newMetricsReceiver(receivertest.NewNopCreateSettings(), tt.args.config, tt.args.metricsConsumer)
			assert.Equal(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func Test_splunkhecReceiver_handleReq(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint

	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)

	tests := []struct {
		name           string
		req            *http.Request
		assertResponse func(t *testing.T, status int, body string)
	}{
		{
			name: "incorrect_method",
			req:  httptest.NewRequest("PUT", "http://localhost/foo", nil),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseInvalidMethod, body)
			},
		},
		{
			name: "incorrect_content_type",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/not-json")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "metric_unsupported",
			req: func() *http.Request {
				metricMsg := buildSplunkHecMsg(currentTime, 3)
				metricMsg.Event = "metric"
				msgBytes, err := json.Marshal(metricMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrUnsupportedMetricEvent, body)
			},
		},
		{
			name: "incorrect_content_encoding",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", nil)
				req.Header.Set("Content-Encoding", "superzipper")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidEncoding, body)
			},
		},
		{
			name: "bad_data_in_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader([]byte{1, 2, 3, 4}))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseInvalidDataFormat, body)
			},
		},
		{
			name: "empty_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(nil))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseNoData, body)
			},
		},
		{
			name: "invalid_data_format",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(`{"foo":"bar"}`)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseInvalidDataFormat, body)
			},
		},
		{
			name: "event_required_error",
			req: func() *http.Request {
				nilEventMsg := buildSplunkHecMsg(currentTime, 3)
				nilEventMsg.Event = nil
				msgBytes, err := json.Marshal(nilEventMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrEventRequired, body)
			},
		},
		{
			name: "event_cannot_be_blank_error",
			req: func() *http.Request {
				blankEventMsg := buildSplunkHecMsg(currentTime, 3)
				blankEventMsg.Event = ""
				msgBytes, err := json.Marshal(blankEventMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrEventBlank, body)
			},
		},
		{
			name: "msg_accepted",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "msg_accepted_gzipped",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				var buf bytes.Buffer
				gzipWriter := gzip.NewWriter(&buf)
				_, err = gzipWriter.Write(msgBytes)
				require.NoError(t, err)
				require.NoError(t, gzipWriter.Close())

				req := httptest.NewRequest("POST", "http://localhost/foo", &buf)
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "bad_gzipped_msg",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrGzipReader, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*splunkReceiver)
			w := httptest.NewRecorder()
			r.handleReq(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			tt.assertResponse(t, resp.StatusCode, bodyStr)
		})
	}
}

func Test_consumer_err(t *testing.T) {
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, consumertest.NewErr(errors.New("bad consumer")))
	assert.NoError(t, err)

	r := rcv.(*splunkReceiver)
	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
	r.handleReq(w, req)

	resp := w.Result()
	respBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var bodyStr string
	assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "Internal Server Error", bodyStr)
}

func Test_consumer_err_metrics(t *testing.T) {
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMetricsMsg(currentTime, 13, 3)
	assert.True(t, splunkMsg.IsMetric())
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint\
	rcv, err := newMetricsReceiver(receivertest.NewNopCreateSettings(), *config, consumertest.NewErr(errors.New("bad consumer")))
	assert.NoError(t, err)

	r := rcv.(*splunkReceiver)
	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
	r.handleReq(w, req)

	resp := w.Result()
	respBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var bodyStr string
	assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "Internal Server Error", bodyStr)
}

func Test_splunkhecReceiver_TLS(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = addr
	cfg.TLSSetting = &configtls.TLSServerSetting{
		TLSSetting: configtls.TLSSetting{
			CertFile: "./testdata/server.crt",
			KeyFile:  "./testdata/server.key",
		},
	}
	sink := new(consumertest.LogsSink)
	r, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *cfg, sink)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()

	mh := newAssertNoErrorHost(t)
	require.NoError(t, r.Start(context.Background(), mh), "should not have failed to start log reception")
	require.NoError(t, r.Start(context.Background(), mh), "should not fail to start log on second Start call")

	// If there are errors reported through host.ReportFatalError() this will retrieve it.
	<-time.After(500 * time.Millisecond)
	t.Log("Event Reception Started")

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	now := time.Now()
	msecInt64 := now.UnixNano() / 1e6
	sec := float64(msecInt64) / 1e3
	lr.SetTimestamp(pcommon.Timestamp(int64(sec * 1e9)))

	lr.Body().SetStr("foo")
	rl.Resource().Attributes().PutStr("com.splunk.sourcetype", "custom:sourcetype")
	rl.Resource().Attributes().PutStr("com.splunk.index", "myindex")
	want := logs

	t.Log("Sending Splunk HEC data Request")

	body, err := json.Marshal(buildSplunkHecMsg(sec, 0))
	require.NoError(t, err, fmt.Sprintf("failed to marshal Splunk message: %v", err))

	url := fmt.Sprintf("https://%s", addr)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoErrorf(t, err, "should have no errors with new request: %v", err)

	tlscs := configtls.TLSClientSetting{
		TLSSetting: configtls.TLSSetting{
			CAFile:   "./testdata/ca.crt",
			CertFile: "./testdata/client.crt",
			KeyFile:  "./testdata/client.key",
		},
		ServerName: "localhost",
	}
	tls, errTLS := tlscs.LoadTLSConfig()
	assert.NoError(t, errTLS)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tls,
		},
	}

	resp, err := client.Do(req)
	require.NoErrorf(t, err, "should not have failed when sending to splunk HEC receiver %v", err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Log("Splunk HEC Request Received")

	got := sink.AllLogs()
	require.Equal(t, 1, len(got))
	assert.Equal(t, want, got[0])
}

func Test_splunkhecReceiver_AccessTokenPassthrough(t *testing.T) {
	tests := []struct {
		name          string
		passthrough   bool
		tokenProvided string
		tokenExpected string
		metric        bool
	}{
		{
			name:          "Log, No token provided and passthrough false",
			tokenExpected: "ignored",
			passthrough:   false,
			metric:        false,
		},
		{
			name:          "Log, No token provided and passthrough true",
			tokenExpected: "ignored",
			passthrough:   true,
			metric:        false,
		},
		{
			name:          "Log, token provided and passthrough false",
			tokenProvided: "passthroughToken",
			passthrough:   false,
			tokenExpected: "ignored",
			metric:        false,
		},
		{
			name:          "Log, token provided and passthrough true",
			passthrough:   true,
			tokenProvided: "passthroughToken",
			tokenExpected: "passthroughToken",
			metric:        false,
		},
		{
			name:          "Metric, No token provided and passthrough false",
			tokenExpected: "ignored",
			passthrough:   false,
			metric:        true,
		},
		{
			name:          "Metric, No token provided and passthrough true",
			tokenExpected: "ignored",
			passthrough:   true,
			metric:        true,
		},
		{
			name:          "Metric, token provided and passthrough false",
			tokenProvided: "passthroughToken",
			passthrough:   false,
			tokenExpected: "ignored",
			metric:        true,
		},
		{
			name:          "Metric, token provided and passthrough true",
			passthrough:   true,
			tokenProvided: "passthroughToken",
			tokenExpected: "passthroughToken",
			metric:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultConfig().(*Config)
			config.Endpoint = "localhost:0"
			config.AccessTokenPassthrough = tt.passthrough
			accessTokensChan := make(chan string)

			endServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				_, err := io.Copy(io.Discard, req.Body)
				assert.NoError(t, err)
				rw.WriteHeader(http.StatusOK)
				accessTokensChan <- req.Header.Get("Authorization")
			}))
			defer endServer.Close()
			factory := splunkhecexporter.NewFactory()
			exporterConfig := factory.CreateDefaultConfig().(*splunkhecexporter.Config)
			exporterConfig.Token = "ignored"
			exporterConfig.SourceType = "defaultsourcetype"
			exporterConfig.Index = "defaultindex"
			exporterConfig.DisableCompression = true
			exporterConfig.Endpoint = endServer.URL

			currentTime := float64(time.Now().UnixNano()) / 1e6
			var splunkhecMsg *splunk.Event
			if tt.metric {
				splunkhecMsg = buildSplunkHecMetricsMsg(currentTime, 1.0, 3)
			} else {
				splunkhecMsg = buildSplunkHecMsg(currentTime, 3)
			}
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
			if tt.passthrough {
				if tt.tokenProvided != "" {
					req.Header.Set("Authorization", "Splunk "+tt.tokenProvided)
				}
			}

			done := make(chan bool)
			go func() {
				tokenReceived := <-accessTokensChan
				assert.Equal(t, "Splunk "+tt.tokenExpected, tokenReceived)
				done <- true
			}()

			if tt.metric {
				exporter, err := factory.CreateMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), exporterConfig)
				assert.NoError(t, exporter.Start(context.Background(), nil))
				assert.NoError(t, err)
				rcv, err := newMetricsReceiver(receivertest.NewNopCreateSettings(), *config, exporter)
				assert.NoError(t, err)
				r := rcv.(*splunkReceiver)
				w := httptest.NewRecorder()
				r.handleReq(w, req)
				resp := w.Result()
				_, err = io.ReadAll(resp.Body)
				assert.NoError(t, err)
			} else {
				exporter, err := factory.CreateLogsExporter(context.Background(), exportertest.NewNopCreateSettings(), exporterConfig)
				assert.NoError(t, exporter.Start(context.Background(), nil))
				assert.NoError(t, err)
				rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, exporter)
				assert.NoError(t, err)
				r := rcv.(*splunkReceiver)
				w := httptest.NewRecorder()
				r.handleReq(w, req)
				resp := w.Result()
				_, err = io.ReadAll(resp.Body)
				assert.NoError(t, err)
			}

			select {
			case <-done:
				break
			case <-time.After(5 * time.Second):
				assert.Fail(t, "Timeout")
			}

		})
	}
}

func Test_Logs_splunkhecReceiver_IndexSourceTypePassthrough(t *testing.T) {
	tests := []struct {
		name       string
		index      string
		sourcetype string
	}{
		{
			name: "No index, no source type",
		},
		{
			name:  "Index, no source type",
			index: "myindex",
		},
		{
			name:       "Index and source type",
			index:      "myindex",
			sourcetype: "source:type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = "localhost:0"

			receivedSplunkLogs := make(chan []byte)
			endServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				assert.NoError(t, err)
				rw.WriteHeader(http.StatusOK)
				receivedSplunkLogs <- body
			}))
			defer endServer.Close()

			factory := splunkhecexporter.NewFactory()
			exporterConfig := factory.CreateDefaultConfig().(*splunkhecexporter.Config)
			exporterConfig.Token = "ignored"
			exporterConfig.SourceType = "defaultsourcetype"
			exporterConfig.Index = "defaultindex"
			exporterConfig.DisableCompression = true
			exporterConfig.Endpoint = endServer.URL
			exporter, err := factory.CreateLogsExporter(context.Background(), exportertest.NewNopCreateSettings(), exporterConfig)
			assert.NoError(t, exporter.Start(context.Background(), nil))
			assert.NoError(t, err)
			rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *cfg, exporter)
			assert.NoError(t, err)

			currentTime := float64(time.Now().UnixNano()) / 1e6
			splunkhecMsg := buildSplunkHecMsg(currentTime, 3)
			splunkhecMsg.Index = tt.index
			splunkhecMsg.SourceType = tt.sourcetype
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))

			done := make(chan bool)
			go func() {
				got := <-receivedSplunkLogs
				var event splunk.Event
				e := json.Unmarshal(got, &event)
				assert.NoError(t, e)
				if tt.index == "" {
					assert.Equal(t, "defaultindex", event.Index)
				} else {
					assert.Equal(t, tt.index, event.Index)
				}
				if tt.sourcetype == "" {
					assert.Equal(t, "defaultsourcetype", event.SourceType)
				} else {
					assert.Equal(t, tt.sourcetype, event.SourceType)
				}
				done <- true
			}()

			r := rcv.(*splunkReceiver)
			w := httptest.NewRecorder()
			r.handleReq(w, req)
			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)
			select {
			case <-done:
				break
			case <-time.After(5 * time.Second):
				assert.Fail(t, "Timeout waiting for logs")
			}
		})
	}
}

func Test_Metrics_splunkhecReceiver_IndexSourceTypePassthrough(t *testing.T) {
	tests := []struct {
		name       string
		index      string
		sourcetype string
	}{
		{
			name: "No index, no source type",
		},
		{
			name:  "Index, no source type",
			index: "myindex",
		},
		{
			name:       "Index and source type",
			index:      "myindex",
			sourcetype: "source:type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = "localhost:0"

			receivedSplunkMetrics := make(chan []byte)
			endServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				assert.NoError(t, err)
				rw.WriteHeader(http.StatusOK)
				receivedSplunkMetrics <- body
			}))
			defer endServer.Close()

			factory := splunkhecexporter.NewFactory()
			exporterConfig := factory.CreateDefaultConfig().(*splunkhecexporter.Config)
			exporterConfig.Token = "ignored"
			exporterConfig.SourceType = "defaultsourcetype"
			exporterConfig.Index = "defaultindex"
			exporterConfig.DisableCompression = true
			exporterConfig.Endpoint = endServer.URL

			exporter, err := factory.CreateMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), exporterConfig)
			assert.NoError(t, exporter.Start(context.Background(), nil))
			assert.NoError(t, err)
			rcv, err := newMetricsReceiver(receivertest.NewNopCreateSettings(), *cfg, exporter)
			assert.NoError(t, err)

			currentTime := float64(time.Now().UnixNano()) / 1e6
			splunkhecMsg := buildSplunkHecMetricsMsg(currentTime, 42, 3)
			splunkhecMsg.Index = tt.index
			splunkhecMsg.SourceType = tt.sourcetype
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))

			done := make(chan bool)
			go func() {
				got := <-receivedSplunkMetrics
				var event splunk.Event
				e := json.Unmarshal(got, &event)
				assert.NoError(t, e)
				if tt.index == "" {
					assert.Equal(t, "defaultindex", event.Index)
				} else {
					assert.Equal(t, tt.index, event.Index)
				}
				if tt.sourcetype == "" {
					assert.Equal(t, "defaultsourcetype", event.SourceType)
				} else {
					assert.Equal(t, tt.sourcetype, event.SourceType)
				}
				done <- true
			}()

			r := rcv.(*splunkReceiver)
			w := httptest.NewRecorder()
			r.handleReq(w, req)
			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)
			select {
			case <-done:
				break
			case <-time.After(5 * time.Second):
				assert.Fail(t, "Timeout waiting for logs")
			}
		})
	}
}

func buildSplunkHecMetricsMsg(time float64, value int64, dimensions uint) *splunk.Event {
	ev := &splunk.Event{
		Time:  time,
		Event: "metric",
		Fields: map[string]interface{}{
			"metric_name:foo": value,
		},
	}
	for dim := uint(0); dim < dimensions; dim++ {
		ev.Fields[fmt.Sprintf("k%d", dim)] = fmt.Sprintf("v%d", dim)
	}

	return ev
}

func buildSplunkHecMsg(time float64, dimensions uint) *splunk.Event {
	ev := &splunk.Event{
		Time:       time,
		Event:      "foo",
		Fields:     map[string]interface{}{},
		Index:      "myindex",
		SourceType: "custom:sourcetype",
	}
	for dim := uint(0); dim < dimensions; dim++ {
		ev.Fields[fmt.Sprintf("k%d", dim)] = fmt.Sprintf("v%d", dim)
	}

	return ev
}

type badReqBody struct{}

var _ io.ReadCloser = (*badReqBody)(nil)

func (b badReqBody) Read(p []byte) (n int, err error) {
	return 0, errors.New("badReqBody: can't read it")
}

func (b badReqBody) Close() error {
	return nil
}

// assertNoErrorHost implements a component.Host that asserts that there were no errors.
type assertNoErrorHost struct {
	component.Host
	*testing.T
}

// newAssertNoErrorHost returns a new instance of assertNoErrorHost.
func newAssertNoErrorHost(t *testing.T) component.Host {
	return &assertNoErrorHost{
		Host: componenttest.NewNopHost(),
		T:    t,
	}
}

func (aneh *assertNoErrorHost) ReportFatalError(err error) {
	assert.NoError(aneh, err)
}

func Test_splunkhecReceiver_handleRawReq(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	config.RawPath = "/foo"

	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)

	tests := []struct {
		name           string
		req            *http.Request
		assertResponse func(t *testing.T, status int, body string)
	}{
		{
			name: "incorrect_method",
			req:  httptest.NewRequest("PUT", "http://localhost/foo", nil),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseInvalidMethod, body)
			},
		},
		{
			name: "incorrect_content_type",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", strings.NewReader("foo\nbar"))
				req.Header.Set("Content-Type", "application/not-json")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
			},
		},
		{
			name: "incorrect_content_encoding",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", nil)
				req.Header.Set("Content-Encoding", "superzipper")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidEncoding, body)
			},
		},
		{
			name: "empty_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(nil))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseNoData, body)
			},
		},

		{
			name: "two_logs",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", strings.NewReader("foo\nbar"))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
			},
		},
		{
			name: "msg_accepted",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
			},
		},
		{
			name: "msg_accepted_gzipped",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				var buf bytes.Buffer
				gzipWriter := gzip.NewWriter(&buf)
				_, err = gzipWriter.Write(msgBytes)
				require.NoError(t, err)
				require.NoError(t, gzipWriter.Close())

				req := httptest.NewRequest("POST", "http://localhost/foo", &buf)
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
			},
		},
		{
			name: "bad_gzipped_msg",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrGzipReader, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*splunkReceiver)
			assert.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, r.Shutdown(context.Background()))
			}()
			w := httptest.NewRecorder()
			r.handleRawReq(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			if len(respBytes) > 0 {
				assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))
			} else {
				bodyStr = ""
			}

			tt.assertResponse(t, resp.StatusCode, bodyStr)
		})
	}
}

func Test_splunkhecreceiver_handleHealthPath(t *testing.T) {
	config := createDefaultConfig().(*Config)
	sink := new(consumertest.LogsSink)
	rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, sink)
	assert.NoError(t, err)

	r := rcv.(*splunkReceiver)
	assert.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, r.Shutdown(context.Background()))
	}()
	w := httptest.NewRecorder()
	r.handleHealthReq(w, httptest.NewRequest("GET", "http://localhost/services/collector/health", nil))

	resp := w.Result()
	respBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(respBytes), responseHecHealthy)
	assert.Equal(t, 200, resp.StatusCode)
}

func Test_splunkhecreceiver_handle_nested_fields(t *testing.T) {
	tests := []struct {
		name    string
		field   interface{}
		success bool
	}{
		{
			name:    "map",
			field:   map[string]interface{}{},
			success: false,
		},
		{
			name:    "flat_array",
			field:   []interface{}{1, 2, 3},
			success: true,
		},
		{
			name:    "nested_array",
			field:   []interface{}{1, []interface{}{1, 2}},
			success: false,
		},
		{
			name: "array_of_map",
			field: []interface{}{
				map[string]interface{}{
					"key": "value",
				},
			},
			success: false,
		},
		{
			name:    "int",
			field:   int(0),
			success: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultConfig().(*Config)
			sink := new(consumertest.LogsSink)
			rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*splunkReceiver)
			assert.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, r.Shutdown(context.Background()))
			}()
			currentTime := float64(time.Now().UnixNano()) / 1e6
			event := buildSplunkHecMsg(currentTime, 3)
			event.Fields["nested_map"] = tt.field
			msgBytes, err := jsoniter.Marshal(event)
			require.NoError(t, err)
			req := httptest.NewRequest("POST", "http://localhost/services/collector", bytes.NewReader(msgBytes))

			w := httptest.NewRecorder()
			r.handleReq(w, req)

			if tt.success {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, 1, sink.LogRecordCount())
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
				assert.Equal(t, fmt.Sprintf(responseErrHandlingIndexedFields, 0), w.Body.String())
			}

		})
	}
}

func Test_splunkhecReceiver_rawReqHasmetadataInResource(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	config.RawPath = "/foo"
	config.HecToOtelAttrs = splunk.HecToOtelAttrs{
		Source:     "com.source.foo",
		SourceType: "com.sourcetype.foo",
		Index:      "com.index.foo",
		Host:       "com.host.foo",
	}

	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)

	assertResponse := func(t *testing.T, status int, body string) {
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, responseOK, body)
	}

	tests := []struct {
		name           string
		req            *http.Request
		assertResource func(t *testing.T, got []plog.Logs)
	}{
		{
			name: "all_metadata",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST",
					"http://localhost/foo?index=bar&source=bar&sourcetype=bar&host=bar",
					bytes.NewReader(msgBytes))
				return req
			}(),
			assertResource: func(t *testing.T, got []plog.Logs) {
				require.Equal(t, 1, len(got))
				resources := got[0].ResourceLogs()
				assert.Equal(t, 1, resources.Len())
				resource := resources.At(0).Resource().Attributes()
				assert.Equal(t, 4, resource.Len())
				for _, k := range []string{config.HecToOtelAttrs.Index, config.HecToOtelAttrs.SourceType, config.HecToOtelAttrs.Source, config.HecToOtelAttrs.Host} {
					v, ok := resource.Get(k)
					if !ok {
						assert.Fail(t, fmt.Sprintf("does not contain query param: %s", k))
					}
					assert.Equal(t, "bar", v.AsString())
				}
			},
		},
		{
			name: "some_metadata",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST",
					"http://localhost/foo?index=bar&source=bar",
					bytes.NewReader(msgBytes))
				return req
			}(),
			assertResource: func(t *testing.T, got []plog.Logs) {
				require.Equal(t, 1, len(got))
				resources := got[0].ResourceLogs()
				assert.Equal(t, 1, resources.Len())
				resource := resources.At(0).Resource().Attributes()
				assert.Equal(t, 2, resource.Len())
				for _, k := range [2]string{config.HecToOtelAttrs.Index, config.HecToOtelAttrs.Source} {
					v, ok := resource.Get(k)
					if !ok {
						assert.Fail(t, fmt.Sprintf("does not contain query param: %s", k))
					}
					assert.Equal(t, "bar", v.AsString())
				}
			},
		},
		{
			name: "no_matching_metadata",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST",
					"http://localhost/foo?foo=bar",
					bytes.NewReader(msgBytes))
				return req
			}(),
			assertResource: func(t *testing.T, got []plog.Logs) {
				require.Equal(t, 1, len(got))
				resources := got[0].ResourceLogs()
				assert.Equal(t, 1, resources.Len())
				resource := resources.At(0).Resource().Attributes()
				assert.Equal(t, 0, resource.Len())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*splunkReceiver)
			assert.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, r.Shutdown(context.Background()))
			}()
			w := httptest.NewRecorder()
			r.handleRawReq(w, tt.req)

			resp := w.Result()
			assert.NoError(t, err)

			assertResponse(t, resp.StatusCode, "OK")
			tt.assertResource(t, sink.AllLogs())
		})
	}
}

func BenchmarkHandleReq(b *testing.B) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0"
	sink := new(consumertest.LogsSink)
	rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, sink)
	assert.NoError(b, err)

	r := rcv.(*splunkReceiver)
	w := httptest.NewRecorder()
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(b, err)
	totalMessage := make([]byte, 100*len(msgBytes))
	for i := 0; i < 100; i++ {
		offset := len(msgBytes) * i
		for bi, b := range msgBytes {
			totalMessage[offset+bi] = b
		}
	}

	for n := 0; n < b.N; n++ {
		req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(totalMessage))
		r.handleReq(w, req)

		resp := w.Result()
		_, err = io.ReadAll(resp.Body)
		assert.NoError(b, err)
	}
}

func Test_splunkhecReceiver_healthCheck_success(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint

	tests := []struct {
		name           string
		req            *http.Request
		assertResponse func(t *testing.T, status int, body string)
	}{
		{
			name: "correct_healthcheck",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "http://localhost:0/services/collector/health", nil)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseHecHealthy, body)
			},
		},
		{
			name: "correct_healthcheck_v1",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "http://localhost:0/services/collector/health/1.0", nil)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseHecHealthy, body)
			},
		},
		{
			name: "incorrect_healthcheck_methods_v1",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost:0/services/collector/health/1.0", nil)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseNoData, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*splunkReceiver)
			assert.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, r.Shutdown(context.Background()))
			}()

			w := httptest.NewRecorder()
			r.server.Handler.ServeHTTP(w, tt.req)
			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			var bodyStr string
			if err := json.Unmarshal(respBytes, &bodyStr); err != nil {
				bodyStr = string(respBytes)
			}

			tt.assertResponse(t, resp.StatusCode, bodyStr)
		})
	}
}
