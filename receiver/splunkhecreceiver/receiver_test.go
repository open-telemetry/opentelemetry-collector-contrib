// Copyright 2020, OpenTelemetry Authors
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

package splunkhecreceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func Test_splunkhecreceiver_NewLogsReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	emptyEndpointConfig := createDefaultConfig().(*Config)
	emptyEndpointConfig.Endpoint = ""
	type args struct {
		config       Config
		logsConsumer consumer.LogsConsumer
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
			wantErr: errNilNextConsumer,
		},
		{
			name: "empty_endpoint",
			args: args{
				config:       *emptyEndpointConfig,
				logsConsumer: new(exportertest.SinkLogsExporter),
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "default_endpoint",
			args: args{
				config:       *defaultConfig,
				logsConsumer: exportertest.NewNopLogsExporter(),
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
				logsConsumer: exportertest.NewNopLogsExporter(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLogsReceiver(zap.NewNop(), tt.args.config, tt.args.logsConsumer)
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
		metricsConsumer consumer.MetricsConsumer
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
			wantErr: errNilNextConsumer,
		},
		{
			name: "empty_endpoint",
			args: args{
				config:          *emptyEndpointConfig,
				metricsConsumer: new(exportertest.SinkMetricsExporter),
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "default_endpoint",
			args: args{
				config:          *defaultConfig,
				metricsConsumer: exportertest.NewNopMetricsExporter(),
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
				metricsConsumer: exportertest.NewNopMetricsExporter(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMetricsReceiver(zap.NewNop(), tt.args.config, tt.args.metricsConsumer)
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
	splunkMsg := buildSplunkHecMsg(currentTime, "foo", 3)

	tests := []struct {
		name           string
		req            *http.Request
		assertResponse func(t *testing.T, status int, body string)
	}{
		{
			name: "incorrect_method",
			req:  httptest.NewRequest("PUT", "http://localhost", nil),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseInvalidMethod, body)
			},
		},
		{
			name: "incorrect_content_type",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Header.Set("Content-Type", "application/not-json")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidContentType, body)
			},
		},
		{
			name: "metric_unsupported",
			req: func() *http.Request {
				metricMsg := buildSplunkHecMsg(currentTime, "foo", 3)
				metricMsg.Event = "metric"
				msgBytes, err := json.Marshal(metricMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/json")
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
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Header.Set("Content-Type", "application/json")
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
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader([]byte{1, 2, 3, 4}))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrUnmarshalBody, body)
			},
		},
		{
			name: "empty_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(nil))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "msg_accepted",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusAccepted, status)
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

				req := httptest.NewRequest("POST", "http://localhost", &buf)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusAccepted, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "bad_gzipped_msg",
			req: func() *http.Request {
				msgBytes, err := json.Marshal(splunkMsg)
				require.NoError(t, err)

				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/json")
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
			sink := new(exportertest.SinkLogsExporter)
			rcv, err := NewLogsReceiver(zap.NewNop(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*splunkReceiver)
			w := httptest.NewRecorder()
			r.handleReq(w, tt.req)

			resp := w.Result()
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			tt.assertResponse(t, resp.StatusCode, bodyStr)
		})
	}
}

func Test_consumer_err(t *testing.T) {
	currentTime := float64(time.Now().UnixNano()) / 1e6
	splunkMsg := buildSplunkHecMsg(currentTime, "foo", 3)
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	sink := new(exportertest.SinkLogsExporter)
	sink.SetConsumeLogError(errors.New("bad consumer"))
	rcv, err := NewLogsReceiver(zap.NewNop(), *config, sink)
	assert.NoError(t, err)

	r := rcv.(*splunkReceiver)
	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
	req.Header.Set("Content-Type", "application/json")
	r.handleReq(w, req)

	resp := w.Result()
	respBytes, err := ioutil.ReadAll(resp.Body)
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
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	sink := new(exportertest.SinkMetricsExporter)
	sink.SetConsumeMetricsError(errors.New("bad consumer"))
	rcv, err := NewMetricsReceiver(zap.NewNop(), *config, sink)
	assert.NoError(t, err)

	r := rcv.(*splunkReceiver)
	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
	req.Header.Set("Content-Type", "application/json")
	r.handleReq(w, req)

	resp := w.Result()
	respBytes, err := ioutil.ReadAll(resp.Body)
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
			CertFile: "./testdata/testcert.crt",
			KeyFile:  "./testdata/testkey.key",
		},
	}
	sink := new(exportertest.SinkLogsExporter)
	r, err := NewLogsReceiver(zap.NewNop(), *cfg, sink)
	require.NoError(t, err)
	defer r.Shutdown(context.Background())

	// NewNopHost swallows errors so using NewErrorWaitingHost to catch any potential errors starting the
	// receiver.
	mh := componenttest.NewErrorWaitingHost()
	require.NoError(t, r.Start(context.Background(), mh), "should not have failed to start log reception")

	// If there are errors reported through host.ReportFatalError() this will retrieve it.
	receivedError, receivedErr := mh.WaitForFatalError(500 * time.Millisecond)
	require.NoError(t, receivedErr, "should not have failed to start log reception")
	require.False(t, receivedError)
	t.Log("Event Reception Started")

	now := time.Now()
	msecInt64 := now.UnixNano() / 1e6
	sec := float64(msecInt64) / 1e3
	lr := pdata.NewLogRecord()
	lr.InitEmpty()
	lr.SetTimestamp(pdata.TimestampUnixNano(int64(sec * 1e9)))

	lr.Body().SetStringVal("foo")
	logs := pdata.NewLogs()
	rl := pdata.NewResourceLogs()
	rl.InitEmpty()
	rl.Resource().InitEmpty()
	rl.Resource().Attributes().InsertString("host.hostname", "")
	rl.Resource().Attributes().InsertString("service.name", "")
	rl.Resource().Attributes().InsertString("com.splunk.sourcetype", "")
	ill := pdata.NewInstrumentationLibraryLogs()
	ill.InitEmpty()
	ill.Logs().Append(lr)
	rl.InstrumentationLibraryLogs().Append(ill)
	logs.ResourceLogs().Append(rl)
	want := logs

	t.Log("Sending Splunk HEC data Request")

	body, err := json.Marshal(buildSplunkHecMsg(sec, "foo", 0))
	require.NoError(t, err, fmt.Sprintf("failed to marshal Splunk message: %v", err))

	url := fmt.Sprintf("https://%s%s", addr, hecPath)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoErrorf(t, err, "should have no errors with new request: %v", err)
	req.Header.Set("Content-Type", "application/json")

	caCert, err := ioutil.ReadFile("./testdata/testcert.crt")
	require.NoErrorf(t, err, "failed to load certificate: %v", err)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	resp, err := client.Do(req)
	require.NoErrorf(t, err, "should not have failed when sending to splunk HEC receiver %v", err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	t.Log("Splunk HEC Request Received")

	got := sink.AllLogs()
	require.Equal(t, 1, len(got))
	assert.Equal(t, want, got[0])
}

func Test_splunkhecReceiver_AccessTokenPassthrough(t *testing.T) {
	tests := []struct {
		name        string
		passthrough bool
		token       pdata.AttributeValue
	}{
		{
			name:        "No token provided and passthrough false",
			passthrough: false,
			token:       pdata.NewAttributeValueNull(),
		},
		{
			name:        "No token provided and passthrough true",
			passthrough: true,
			token:       pdata.NewAttributeValueNull(),
		},
		{
			name:        "token provided and passthrough false",
			passthrough: false,
			token:       pdata.NewAttributeValueString("myToken"),
		},
		{
			name:        "token provided and passthrough true",
			passthrough: true,
			token:       pdata.NewAttributeValueString("myToken"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultConfig().(*Config)
			config.Endpoint = "localhost:0"
			config.AccessTokenPassthrough = tt.passthrough

			sink := new(exportertest.SinkLogsExporter)
			rcv, err := NewLogsReceiver(zap.NewNop(), *config, sink)
			assert.NoError(t, err)

			currentTime := float64(time.Now().UnixNano()) / 1e6
			splunkhecMsg := buildSplunkHecMsg(currentTime, "foo", 3)
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
			req.Header.Set("Content-Type", "application/json")
			if tt.token.Type() != pdata.AttributeValueNULL {
				req.Header.Set("Splunk", tt.token.StringVal())
			}

			r := rcv.(*splunkReceiver)
			w := httptest.NewRecorder()
			r.handleReq(w, req)

			resp := w.Result()
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)

			got := sink.AllLogs()

			resource := got[0].ResourceLogs().At(0).Resource()
			if resource.IsNil() {
				resource.InitEmpty()
			}
			tokenLabel, exists := resource.Attributes().Get("com.splunk.hec.access_token")

			if tt.passthrough {
				if tt.token.Type() == pdata.AttributeValueNULL {
					assert.False(t, exists)
				} else {
					assert.Equal(t, tt.token.StringVal(), tokenLabel.StringVal())
				}
			} else {
				assert.Empty(t, tokenLabel)
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

func buildSplunkHecMsg(time float64, value string, dimensions uint) *splunk.Event {
	ev := &splunk.Event{
		Time:   time,
		Event:  value,
		Fields: map[string]interface{}{},
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
