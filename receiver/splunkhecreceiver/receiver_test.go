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
	"go.opentelemetry.io/collector/component/componentstatus"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

func assertHecSuccessResponse(t *testing.T, resp *http.Response, body any) {
	status := resp.StatusCode
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "application/json", resp.Header.Get(httpContentTypeHeader))
	assert.Equal(t, map[string]any{"code": float64(0), "text": "Success"}, body)
}

func assertHecSuccessResponseWithAckID(t *testing.T, resp *http.Response, body any, ackID uint64) {
	status := resp.StatusCode
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "application/json", resp.Header.Get(httpContentTypeHeader))
	assert.Equal(t, map[string]any{"code": float64(0), "text": "Success", "ackId": float64(ackID)}, body)
}

func Test_splunkhecreceiver_NewReceiver(t *testing.T) {
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
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:1234",
					},
				},
				logsConsumer: consumertest.NewNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newReceiver(receivertest.NewNopSettings(metadata.Type), tt.args.config)
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
		name              string
		req               *http.Request
		assertResponse    func(t *testing.T, resp *http.Response, body any)
		assertSink        func(t *testing.T, sink *consumertest.LogsSink)
		assertMetricsSink func(t *testing.T, sink *consumertest.MetricsSink)
	}{
		{
			name: "incorrect_method",
			req:  httptest.NewRequest(http.MethodPut, "http://localhost/foo", nil),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, "Only \"POST\" method is supported", body)
			},
		},
		{
			name: "incorrect_content_type",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/not-json")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, map[string]any{
					"text": "Success",
					"code": float64(0),
				}, body)
			},
		},
		{
			name: "incorrect_content_encoding",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", nil)
				req.Header.Set("Content-Encoding", "superzipper")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, `"Content-Encoding" must be "gzip" or empty`, body)
			},
		},
		{
			name: "bad_data_in_body",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader([]byte{1, 2, 3, 4}))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(6), "text": "Invalid data format"}, body)
			},
		},
		{
			name: "empty_body",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(nil))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(5), "text": "No data"}, body)
			},
		},
		{
			name: "invalid_data_format",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(`{"foo":"bar"}`)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(6), "text": "Invalid data format"}, body)
			},
		},
		{
			name: "event_required_error",
			req: func() *http.Request {
				nilEventMsg := buildSplunkHecMsg(currentTime, 3)
				nilEventMsg.Event = nil
				msgBytes, err := json.Marshal(nilEventMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(12), "text": "Event field is required"}, body)
			},
		},
		{
			name: "event_cannot_be_blank_error",
			req: func() *http.Request {
				blankEventMsg := buildSplunkHecMsg(currentTime, 3)
				blankEventMsg.Event = ""
				msgBytes, err := json.Marshal(blankEventMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(13), "text": "Event field cannot be blank"}, body)
			},
		},
		{
			name: "msg_accepted",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponse(t, resp, body)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Len(t, sink.AllLogs(), 1)
			},
			assertMetricsSink: func(t *testing.T, sink *consumertest.MetricsSink) {
				assert.Empty(t, sink.AllMetrics())
			},
		},
		{
			name: "metric_msg_accepted",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(buildSplunkHecMetricsMsg("metric", 3, 4, 3))
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponse(t, resp, body)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Empty(t, sink.AllLogs())
			},
			assertMetricsSink: func(t *testing.T, sink *consumertest.MetricsSink) {
				assert.Len(t, sink.AllMetrics(), 1)
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

				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", &buf)
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponse(t, resp, body)
			},
		},
		{
			name: "bad_gzipped_msg",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, `Error on gzip body`, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			metricsSink := new(consumertest.MetricsSink)
			f := NewFactory()

			_, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), config, sink)
			assert.NoError(t, err)
			rcv, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), config, metricsSink)
			assert.NoError(t, err)

			r := rcv.(*sharedcomponent.SharedComponent).Component.(*splunkReceiver)
			w := httptest.NewRecorder()
			r.handleReq(w, tt.req)

			resp := w.Result()
			defer resp.Body.Close()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var body any
			fmt.Println(string(respBytes))
			assert.NoError(t, json.Unmarshal(respBytes, &body))

			tt.assertResponse(t, resp, body)
			if tt.assertSink != nil {
				tt.assertSink(t, sink)
			}
			if tt.assertMetricsSink != nil {
				tt.assertMetricsSink(t, metricsSink)
			}
		})
	}
}

func Test_consumer_err(t *testing.T) {
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 5)
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
	assert.NoError(t, err)
	rcv.logsConsumer = consumertest.NewErr(errors.New("bad consumer"))

	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
	rcv.handleReq(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var bodyStr string
	assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "Internal Server Error", bodyStr)
}

func Test_consumer_err_metrics(t *testing.T) {
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMetricsMsg("metric", currentTime, 13, 2)
	assert.True(t, splunkMsg.IsMetric())
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint\
	rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
	assert.NoError(t, err)
	rcv.metricsConsumer = consumertest.NewErr(errors.New("bad consumer"))

	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
	rcv.handleReq(w, req)

	resp := w.Result()
	defer resp.Body.Close()
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
	cfg.TLSSetting = &configtls.ServerConfig{
		Config: configtls.Config{
			CertFile: "./testdata/server.crt",
			KeyFile:  "./testdata/server.key",
		},
	}
	sink := new(consumertest.LogsSink)
	set := receivertest.NewNopSettings(metadata.Type)
	r, err := newReceiver(set, *cfg)
	require.NoError(t, err)
	r.logsConsumer = sink
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()

	require.NoError(t, r.Start(context.Background(), &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			assert.NoError(t, event.Err())
		},
	}), "should not have failed to start log reception")
	require.NoError(t, r.Start(context.Background(), &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			assert.NoError(t, event.Err())
		},
	}), "should not fail to start log on second Start call")
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()

	// If there are errors reported through ReportStatus this will retrieve it.
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
	require.NoErrorf(t, err, "failed to marshal Splunk message: %v", err)

	url := "https://" + addr

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	require.NoErrorf(t, err, "should have no errors with new request: %v", err)

	tlscs := configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "./testdata/ca.crt",
			CertFile: "./testdata/client.crt",
			KeyFile:  "./testdata/client.key",
		},
		ServerName: "localhost",
	}
	tls, errTLS := tlscs.LoadTLSConfig(context.Background())
	assert.NoError(t, errTLS)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tls,
		},
	}

	resp, err := client.Do(req)
	require.NoErrorf(t, err, "should not have failed when sending to splunk HEC receiver %v", err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Log("Splunk HEC Request Received")

	got := sink.AllLogs()
	require.Len(t, got, 1)
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
				splunkhecMsg = buildSplunkHecMetricsMsg("metric", currentTime, 1.0, 3)
			} else {
				splunkhecMsg = buildSplunkHecMsg(currentTime, 3)
			}
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
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
				exporter, err := factory.CreateMetrics(context.Background(), exportertest.NewNopSettings(metadata.Type), exporterConfig)
				assert.NoError(t, exporter.Start(context.Background(), nil))
				defer func() {
					require.NoError(t, exporter.Shutdown(context.Background()))
				}()
				assert.NoError(t, err)
				rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
				assert.NoError(t, err)
				rcv.metricsConsumer = exporter
				w := httptest.NewRecorder()
				rcv.handleReq(w, req)
				resp := w.Result()
				defer resp.Body.Close()
				_, err = io.ReadAll(resp.Body)
				assert.NoError(t, err)
			} else {
				exporter, err := factory.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), exporterConfig)
				assert.NoError(t, exporter.Start(context.Background(), nil))
				defer func() {
					require.NoError(t, exporter.Shutdown(context.Background()))
				}()
				assert.NoError(t, err)
				rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
				assert.NoError(t, err)
				rcv.logsConsumer = exporter
				w := httptest.NewRecorder()
				rcv.handleReq(w, req)
				resp := w.Result()
				defer resp.Body.Close()
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
			exporter, err := factory.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), exporterConfig)
			assert.NoError(t, exporter.Start(context.Background(), nil))
			assert.NoError(t, err)
			defer func() {
				require.NoError(t, exporter.Shutdown(context.Background()))
			}()
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *cfg)
			assert.NoError(t, err)
			rcv.logsConsumer = exporter

			currentTime := float64(time.Now().UnixNano()) / 1e6
			splunkhecMsg := buildSplunkHecMsg(currentTime, 3)
			splunkhecMsg.Index = tt.index
			splunkhecMsg.SourceType = tt.sourcetype
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))

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

			w := httptest.NewRecorder()
			rcv.handleReq(w, req)
			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			defer resp.Body.Close()
			var body any
			assert.NoError(t, json.Unmarshal(respBytes, &body))
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, map[string]any{
				"text": "Success",
				"code": float64(0),
			}, body)
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
		event      string
	}{
		{
			name:  "No index, no source type",
			event: "metric",
		},
		{
			name:  "Index, no source type",
			index: "myindex",
			event: "metric",
		},
		{
			name:       "Index and source type",
			index:      "myindex",
			sourcetype: "source:type",
			event:      "metric",
		},
		{
			name:  "empty event",
			event: "",
		},
		{
			name:  "any value event",
			event: "some event",
		},
		{
			name: "nil event",
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

			exporter, err := factory.CreateMetrics(context.Background(), exportertest.NewNopSettings(metadata.Type), exporterConfig)
			assert.NoError(t, exporter.Start(context.Background(), nil))
			defer func() {
				require.NoError(t, exporter.Shutdown(context.Background()))
			}()
			assert.NoError(t, err)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *cfg)
			assert.NoError(t, err)
			rcv.metricsConsumer = exporter

			currentTime := float64(time.Now().UnixNano()) / 1e6
			splunkhecMsg := buildSplunkHecMetricsMsg(tt.event, currentTime, 42, 3)
			splunkhecMsg.Index = tt.index
			splunkhecMsg.SourceType = tt.sourcetype
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))

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

			w := httptest.NewRecorder()
			rcv.handleReq(w, req)
			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			assert.NoError(t, err)
			var body any
			assert.NoError(t, json.Unmarshal(respBytes, &body))
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, map[string]any{
				"text": "Success",
				"code": float64(0),
			}, body)
			select {
			case <-done:
				break
			case <-time.After(5 * time.Second):
				assert.Fail(t, "Timeout waiting for logs")
			}
		})
	}
}

func buildSplunkHecMetricsMsg(event any, time float64, value int64, dimensions uint) *splunk.Event {
	ev := &splunk.Event{
		Time:  time,
		Event: event,
		Fields: map[string]any{
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
		Fields:     map[string]any{},
		Index:      "myindex",
		SourceType: "custom:sourcetype",
	}
	for dim := uint(0); dim < dimensions; dim++ {
		ev.Fields[fmt.Sprintf("k%d", dim)] = fmt.Sprintf("v%d", dim)
	}

	return ev
}

func buildSplunkHecAckMsg(acks []uint64) *splunk.AckRequest {
	return &splunk.AckRequest{
		Acks: acks,
	}
}

type badReqBody struct{}

var _ io.ReadCloser = (*badReqBody)(nil)

func (b badReqBody) Read(_ []byte) (n int, err error) {
	return 0, errors.New("badReqBody: can't read it")
}

func (b badReqBody) Close() error {
	return nil
}

func Test_splunkhecReceiver_handleRawReq(t *testing.T) {
	t.Parallel()
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	config.RawPath = "/foo"

	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)

	tests := []struct {
		name           string
		req            *http.Request
		assertResponse func(t *testing.T, resp *http.Response, body any)
	}{
		{
			name: "incorrect_method",
			req:  httptest.NewRequest(http.MethodPut, "http://localhost/foo", nil),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, `Only "POST" method is supported`, body)
			},
		},
		{
			name: "incorrect_content_type",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", strings.NewReader("foo\nbar"))
				req.Header.Set("Content-Type", "application/not-json")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, _ any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusOK, status)
			},
		},
		{
			name: "incorrect_content_encoding",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", nil)
				req.Header.Set("Content-Encoding", "superzipper")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, `"Content-Encoding" must be "gzip" or empty`, body)
			},
		},
		{
			name: "empty_body",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(nil))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(5), "text": "No data"}, body)
			},
		},

		{
			name: "two_logs",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", strings.NewReader("foo\nbar"))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, _ any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusOK, status)
			},
		},
		{
			name: "msg_accepted",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponse(t, resp, body)
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

				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", &buf)
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponse(t, resp, body)
			},
		},
		{
			name: "bad_gzipped_msg",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, `Error on gzip body`, body)
			},
		},
		{
			name: "raw_endpoint_bad_time_negative_number",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodPost, "http://localhost/service/collector/raw", bytes.NewReader(msgBytes))

				q := req.URL.Query()
				q.Add(queryTime, "-5")
				req.URL.RawQuery = q.Encode()

				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(6), "text": "Invalid data format"}, body)
			},
		},
		{
			name: "raw_endpoint_bad_time_not_a_number",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodPost, "http://localhost/service/collector/raw", bytes.NewReader(msgBytes))

				q := req.URL.Query()
				q.Add(queryTime, "notANumber")
				req.URL.RawQuery = q.Encode()

				return req
			}(),
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(6), "text": "Invalid data format"}, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			assert.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, rcv.Shutdown(context.Background()))
			}()
			w := httptest.NewRecorder()
			rcv.handleRawReq(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			defer resp.Body.Close()

			var body any
			if len(respBytes) > 0 {
				assert.NoError(t, json.Unmarshal(respBytes, &body))
			}

			tt.assertResponse(t, resp, body)
		})
	}
}

func Test_splunkhecReceiver_Start(t *testing.T) {
	tests := []struct {
		name          string
		getConfig     func() *Config
		errorExpected bool
	}{
		{
			name: "no_ack_extension_configured",
			getConfig: func() *Config {
				return createDefaultConfig().(*Config)
			},
			errorExpected: false,
		},
		{
			name: "ack_extension_does_not_exist",
			getConfig: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Extension = &component.ID{}
				return config
			},
			errorExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *tt.getConfig())
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			if tt.errorExpected {
				assert.Error(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
			} else {
				assert.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
			}
			assert.NoError(t, rcv.Shutdown(context.Background()))
		})
	}
}

func Test_splunkhecReceiver_handleAck(t *testing.T) {
	t.Parallel()
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	config.Path = "/ack"
	id := component.MustNewID("ack_extension")
	config.Extension = &id

	tests := []struct {
		name                  string
		req                   *http.Request
		setupMockAckExtension func() component.Component
		assertResponse        func(t *testing.T, resp *http.Response, body any)
	}{
		{
			name: "incorrect_method",
			req:  httptest.NewRequest(http.MethodPut, "http://localhost/ack", nil),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, "Only \"POST\" method is supported", body)
			},
		},
		{
			name: "no_channel_header",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack", nil)
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(10), "text": "Data channel is missing"}, body)
			},
		},
		{
			name: "invalid_channel_header",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack", nil)
				req.Header.Set("X-Splunk-Request-Channel", "invalid-id")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"text": "Invalid data channel", "code": float64(11)}, body)
			},
		},
		{
			name: "empty_request_body",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack", nil)
				req.Header.Set("X-Splunk-Request-Channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"text": "Invalid data format", "code": float64(6)}, body)
			},
		},
		{
			name: "empty_ack_in_request_body",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(buildSplunkHecAckMsg([]uint64{}))
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"text": "Invalid data format", "code": float64(6)}, body)
			},
		},
		{
			name: "invalid_request_body",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack", bytes.NewReader([]byte(`hi there`)))
				req.Header.Set("X-Splunk-Request-Channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"text": "Invalid data format", "code": float64(6)}, body)
			},
		},
		{
			name: "happy_path",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(buildSplunkHecAckMsg([]uint64{1, 2, 3}))
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					queryAcks: func(_ string, _ []uint64) map[uint64]bool {
						return map[uint64]bool{
							1: true,
							2: false,
							3: true,
						}
					},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, map[string]any{"acks": map[string]any{
					"1": true,
					"2": false,
					"3": true,
				}}, body)
			},
		},
		{
			name: "happy_path_with_case_insensitive_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(buildSplunkHecAckMsg([]uint64{1, 2, 3}))
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack", bytes.NewReader(msgBytes))
				req.Header.Set("x-splunk-request-channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					queryAcks: func(_ string, _ []uint64) map[uint64]bool {
						return map[uint64]bool{
							1: true,
							2: false,
							3: true,
						}
					},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, map[string]any{"acks": map[string]any{
					"1": true,
					"2": false,
					"3": true,
				}}, body)
			},
		},
		{
			name: "happy_path_with_query_param",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(buildSplunkHecAckMsg([]uint64{1, 2, 3}))
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/ack?channel=fbd3036f-0f1c-4e98-b71c-d4cd61213f90", bytes.NewReader(msgBytes))
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					queryAcks: func(_ string, _ []uint64) map[uint64]bool {
						return map[uint64]bool{
							1: true,
							2: false,
							3: true,
						}
					},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, map[string]any{"acks": map[string]any{
					"1": true,
					"2": false,
					"3": true,
				}}, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			mockHost := mockHost{extensions: map[component.ID]component.Component{
				id: tt.setupMockAckExtension(),
			}}

			assert.NoError(t, rcv.Start(context.Background(), mockHost))
			defer func() {
				assert.NoError(t, rcv.Shutdown(context.Background()))
			}()
			w := httptest.NewRecorder()
			rcv.handleAck(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			defer resp.Body.Close()

			var body any
			if len(respBytes) > 0 {
				assert.NoError(t, json.Unmarshal(respBytes, &body))
			}

			tt.assertResponse(t, resp, body)
		})
	}
}

func Test_splunkhecReceiver_handleRawReq_WithAck(t *testing.T) {
	t.Parallel()
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	config.RawPath = "/foo"
	id := component.MustNewID("ack_extension")
	config.Extension = &id
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)
	currAckID := uint64(0)
	tests := []struct {
		name                  string
		req                   *http.Request
		setupMockAckExtension func() component.Component
		assertResponse        func(t *testing.T, resp *http.Response, body any)
	}{
		{
			name: "no_channel_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponse(t, resp, body)
			},
		},
		{
			name: "empty_channel_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(10), "text": "Data channel is missing"}, body)
			},
		},
		{
			name: "invalid_channel_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "invalid-id")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"text": "Invalid data channel", "code": float64(11)}, body)
			},
		},
		{
			name: "happy_path",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					processEvent: func(_ string) (ackID uint64) {
						currAckID++
						return currAckID
					},
					ack: func(_ string, _ uint64) {},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponseWithAckID(t, resp, body, currAckID)
			},
		},
		{
			name: "happy_path_with_case_insensitive_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("x-splunk-request-channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					processEvent: func(_ string) (ackID uint64) {
						currAckID++
						return currAckID
					},
					ack: func(_ string, _ uint64) {},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponseWithAckID(t, resp, body, currAckID)
			},
		},
		{
			name: "happy_path_with_query_param",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo?Channel=fbd3036f-0f1c-4e98-b71c-d4cd61213f90", bytes.NewReader(msgBytes))
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					processEvent: func(_ string) (ackID uint64) {
						currAckID++
						return currAckID
					},
					ack: func(_ string, _ uint64) {},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponseWithAckID(t, resp, body, currAckID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			mh := mockHost{extensions: map[component.ID]component.Component{
				id: tt.setupMockAckExtension(),
			}}

			assert.NoError(t, rcv.Start(context.Background(), mh))
			defer func() {
				assert.NoError(t, rcv.Shutdown(context.Background()))
			}()
			w := httptest.NewRecorder()
			rcv.handleRawReq(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			defer resp.Body.Close()

			var body any
			if len(respBytes) > 0 {
				assert.NoError(t, json.Unmarshal(respBytes, &body))
			}

			tt.assertResponse(t, resp, body)
		})
	}
}

func Test_splunkhecReceiver_handleReq_WithAck(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	id := component.MustNewID("ack_extension")
	config.Extension = &id
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 3)

	tests := []struct {
		name                  string
		req                   *http.Request
		assertResponse        func(t *testing.T, resp *http.Response, body any)
		assertSink            func(t *testing.T, sink *consumertest.LogsSink)
		setupMockAckExtension func() component.Component
	}{
		{
			name: "no_channel_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponse(t, resp, body)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Len(t, sink.AllLogs(), 1)
			},
		},
		{
			name: "empty_channel_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"code": float64(10), "text": "Data channel is missing"}, body)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Empty(t, sink.AllLogs())
			},
		},
		{
			name: "invalid_channel_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "invalid-id")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				status := resp.StatusCode
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"text": "Invalid data channel", "code": float64(11)}, body)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Empty(t, sink.AllLogs())
			},
		},
		{
			name: "msg_accepted",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("X-Splunk-Request-Channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					processEvent: func(_ string) (ackID uint64) {
						return uint64(1)
					},
					ack: func(_ string, _ uint64) {
					},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponseWithAckID(t, resp, body, 1)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Len(t, sink.AllLogs(), 1)
			},
		},
		{
			name: "msg_accepted_with_case_insensitive_header",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(msgBytes))
				req.Header.Set("x-splunk-request-channel", "fbd3036f-0f1c-4e98-b71c-d4cd61213f90")
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					processEvent: func(_ string) (ackID uint64) {
						return uint64(1)
					},
					ack: func(_ string, _ uint64) {
					},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponseWithAckID(t, resp, body, 1)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Len(t, sink.AllLogs(), 1)
			},
		},
		{
			name: "msg_accepted_with_query_param",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/foo?channel=fbd3036f-0f1c-4e98-b71c-d4cd61213f90&isCheesy=true", bytes.NewReader(msgBytes))
				return req
			}(),
			setupMockAckExtension: func() component.Component {
				return &mockAckExtension{
					processEvent: func(_ string) (ackID uint64) {
						return uint64(1)
					},
					ack: func(_ string, _ uint64) {
					},
				}
			},
			assertResponse: func(t *testing.T, resp *http.Response, body any) {
				assertHecSuccessResponseWithAckID(t, resp, body, 1)
			},
			assertSink: func(t *testing.T, sink *consumertest.LogsSink) {
				assert.Len(t, sink.AllLogs(), 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			w := httptest.NewRecorder()

			mh := mockHost{extensions: map[component.ID]component.Component{
				id: tt.setupMockAckExtension(),
			}}

			assert.NoError(t, rcv.Start(context.Background(), mh))
			defer func() {
				assert.NoError(t, rcv.Shutdown(context.Background()))
			}()
			rcv.handleReq(w, tt.req)

			resp := w.Result()
			defer resp.Body.Close()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var body any
			fmt.Println(string(respBytes))
			assert.NoError(t, json.Unmarshal(respBytes, &body))

			tt.assertResponse(t, resp, body)
			if tt.assertSink != nil {
				tt.assertSink(t, sink)
			}
		})
	}
}

func Test_splunkhecreceiver_handleHealthPath(t *testing.T) {
	config := createDefaultConfig().(*Config)
	sink := new(consumertest.LogsSink)
	rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
	assert.NoError(t, err)
	rcv.logsConsumer = sink

	assert.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcv.Shutdown(context.Background()))
	}()
	w := httptest.NewRecorder()
	rcv.handleHealthReq(w, httptest.NewRequest(http.MethodGet, "http://localhost/services/collector/health", nil))

	resp := w.Result()
	respBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.JSONEq(t, responseHecHealthy, string(respBytes))
	assert.Equal(t, 200, resp.StatusCode)
}

func Test_splunkhecreceiver_handle_nested_fields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		field   any
		success bool
	}{
		{
			name:    "map",
			field:   map[string]any{},
			success: false,
		},
		{
			name:    "flat_array",
			field:   []any{1, 2, 3},
			success: true,
		},
		{
			name:    "nested_array",
			field:   []any{1, []any{1, 2}},
			success: false,
		},
		{
			name: "array_of_map",
			field: []any{
				map[string]any{
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
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			assert.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, rcv.Shutdown(context.Background()))
			}()
			currentTime := float64(time.Now().UnixNano()) / 1e6
			event := buildSplunkHecMsg(currentTime, 3)
			event.Fields["nested_map"] = tt.field
			msgBytes, err := jsoniter.Marshal(event)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "http://localhost/services/collector", bytes.NewReader(msgBytes))

			w := httptest.NewRecorder()
			rcv.handleReq(w, req)

			if tt.success {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, 1, sink.LogRecordCount())
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
				assert.JSONEq(t, fmt.Sprintf(responseErrHandlingIndexedFields, 0), w.Body.String())
			}
		})
	}
}

func Test_splunkhecReceiver_rawReqHasmetadataInResource(t *testing.T) {
	t.Parallel()
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
		assert.JSONEq(t, responseOK, body)
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
				req := httptest.NewRequest(http.MethodPost,
					"http://localhost/foo?index=bar&source=bar&sourcetype=bar&host=bar",
					bytes.NewReader(msgBytes))
				return req
			}(),
			assertResource: func(t *testing.T, got []plog.Logs) {
				require.Len(t, got, 1)
				resources := got[0].ResourceLogs()
				assert.Equal(t, 1, resources.Len())
				resource := resources.At(0).Resource().Attributes()
				assert.Equal(t, 4, resource.Len())
				for _, k := range []string{config.HecToOtelAttrs.Index, config.HecToOtelAttrs.SourceType, config.HecToOtelAttrs.Source, config.HecToOtelAttrs.Host} {
					v, ok := resource.Get(k)
					if !ok {
						assert.Fail(t, "does not contain query param: "+k)
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
				req := httptest.NewRequest(http.MethodPost,
					"http://localhost/foo?index=bar&source=bar",
					bytes.NewReader(msgBytes))
				return req
			}(),
			assertResource: func(t *testing.T, got []plog.Logs) {
				require.Len(t, got, 1)
				resources := got[0].ResourceLogs()
				assert.Equal(t, 1, resources.Len())
				resource := resources.At(0).Resource().Attributes()
				assert.Equal(t, 2, resource.Len())
				for _, k := range [2]string{config.HecToOtelAttrs.Index, config.HecToOtelAttrs.Source} {
					v, ok := resource.Get(k)
					if !ok {
						assert.Fail(t, "does not contain query param: "+k)
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
				req := httptest.NewRequest(http.MethodPost,
					"http://localhost/foo?foo=bar",
					bytes.NewReader(msgBytes))
				return req
			}(),
			assertResource: func(t *testing.T, got []plog.Logs) {
				require.Len(t, got, 1)
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
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			assert.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, rcv.Shutdown(context.Background()))
			}()
			w := httptest.NewRecorder()
			rcv.handleRawReq(w, tt.req)

			resp := w.Result()
			assert.NoError(t, err)
			defer resp.Body.Close()

			assertResponse(t, resp.StatusCode, responseOK)
			tt.assertResource(t, sink.AllLogs())
		})
	}
}

func BenchmarkHandleReq(b *testing.B) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0"
	sink := new(consumertest.LogsSink)
	rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
	assert.NoError(b, err)
	rcv.logsConsumer = sink

	w := httptest.NewRecorder()
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, 2)
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
		req := httptest.NewRequest(http.MethodPost, "http://localhost/foo", bytes.NewReader(totalMessage))
		rcv.handleReq(w, req)

		resp := w.Result()
		_, err = io.ReadAll(resp.Body)
		defer resp.Body.Close()
		assert.NoError(b, err)
	}
}

func Test_splunkhecReceiver_healthCheck_success(t *testing.T) {
	t.Parallel()
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
				req := httptest.NewRequest(http.MethodGet, "http://localhost:0/services/collector/health", nil)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.JSONEq(t, responseHecHealthy, body)
			},
		},
		{
			name: "correct_healthcheck_v1",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://localhost:0/services/collector/health/1.0", nil)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.JSONEq(t, responseHecHealthy, body)
			},
		},
		{
			name: "incorrect_healthcheck_methods_v1",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost:0/services/collector/health/1.0", nil)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.JSONEq(t, responseNoData, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			assert.NoError(t, err)
			rcv.logsConsumer = sink

			assert.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, rcv.Shutdown(context.Background()))
			}()

			w := httptest.NewRecorder()
			rcv.server.Handler.ServeHTTP(w, tt.req)
			resp := w.Result()
			defer resp.Body.Close()
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

type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

type mockAckExtension struct {
	queryAcks    func(partitionID string, ackIDs []uint64) map[uint64]bool
	ack          func(partitionID string, ackID uint64)
	processEvent func(partitionID string) (ackID uint64)
}

func (ae *mockAckExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (ae *mockAckExtension) Shutdown(_ context.Context) error {
	return nil
}

func (ae *mockAckExtension) QueryAcks(partitionID string, ackIDs []uint64) map[uint64]bool {
	return ae.queryAcks(partitionID, ackIDs)
}

func (ae *mockAckExtension) Ack(partitionID string, ackID uint64) {
	ae.ack(partitionID, ackID)
}

func (ae *mockAckExtension) ProcessEvent(partitionID string) (ackID uint64) {
	return ae.processEvent(partitionID)
}

var _ componentstatus.Reporter = (*nopHost)(nil)

type nopHost struct {
	reportFunc func(event *componentstatus.Event)
}

func (nh *nopHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *nopHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
