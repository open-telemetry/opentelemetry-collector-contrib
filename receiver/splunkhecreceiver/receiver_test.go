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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
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
			got, err := newLogsReceiver(componenttest.NewNopReceiverCreateSettings(), tt.args.config, tt.args.logsConsumer)
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
			got, err := newMetricsReceiver(componenttest.NewNopReceiverCreateSettings(), tt.args.config, tt.args.metricsConsumer)
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
				req := httptest.NewRequest("POST", "http://localhost/foo", nil)
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
				assert.Equal(t, responseErrUnmarshalBody, body)
			},
		},
		{
			name: "empty_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(nil))
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
				req := httptest.NewRequest("POST", "http://localhost/foo", bytes.NewReader(msgBytes))
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

				req := httptest.NewRequest("POST", "http://localhost/foo", &buf)
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
			rcv, err := newLogsReceiver(componenttest.NewNopReceiverCreateSettings(), *config, sink)
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
	splunkMsg := buildSplunkHecMsg(currentTime, 3)
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint
	rcv, err := newLogsReceiver(componenttest.NewNopReceiverCreateSettings(), *config, consumertest.NewErr(errors.New("bad consumer")))
	assert.NoError(t, err)

	r := rcv.(*splunkReceiver)
	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
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
	config.Endpoint = "localhost:0" // Actually not creating the endpoint\
	rcv, err := newMetricsReceiver(componenttest.NewNopReceiverCreateSettings(), *config, consumertest.NewErr(errors.New("bad consumer")))
	assert.NoError(t, err)

	r := rcv.(*splunkReceiver)
	w := httptest.NewRecorder()
	msgBytes, err := json.Marshal(splunkMsg)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
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
			CertFile: "./testdata/server.crt",
			KeyFile:  "./testdata/server.key",
		},
	}
	sink := new(consumertest.LogsSink)
	r, err := newLogsReceiver(componenttest.NewNopReceiverCreateSettings(), *cfg, sink)
	require.NoError(t, err)
	defer r.Shutdown(context.Background())

	mh := newAssertNoErrorHost(t)
	require.NoError(t, r.Start(context.Background(), mh), "should not have failed to start log reception")

	// If there are errors reported through host.ReportFatalError() this will retrieve it.
	<-time.After(500 * time.Millisecond)
	t.Log("Event Reception Started")

	logs := pdata.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	lr := ill.Logs().AppendEmpty()

	now := time.Now()
	msecInt64 := now.UnixNano() / 1e6
	sec := float64(msecInt64) / 1e3
	lr.SetTimestamp(pdata.Timestamp(int64(sec * 1e9)))
	lr.SetName("custom:sourcetype")

	lr.Body().SetStringVal("foo")
	lr.Attributes().InsertString("com.splunk.sourcetype", "custom:sourcetype")
	lr.Attributes().InsertString("com.splunk.index", "myindex")
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
			token:       pdata.NewAttributeValueEmpty(),
		},
		{
			name:        "No token provided and passthrough true",
			passthrough: true,
			token:       pdata.NewAttributeValueEmpty(),
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

			sink := new(consumertest.LogsSink)
			rcv, err := newLogsReceiver(componenttest.NewNopReceiverCreateSettings(), *config, sink)
			assert.NoError(t, err)

			currentTime := float64(time.Now().UnixNano()) / 1e6
			splunkhecMsg := buildSplunkHecMsg(currentTime, 3)
			msgBytes, _ := json.Marshal(splunkhecMsg)
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
			if tt.token.Type() != pdata.AttributeValueTypeEmpty {
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
			tokenLabel, exists := resource.Attributes().Get("com.splunk.hec.access_token")

			if tt.passthrough {
				if tt.token.Type() == pdata.AttributeValueTypeEmpty {
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
				body, err := ioutil.ReadAll(req.Body)
				assert.NoError(t, err)
				rw.WriteHeader(http.StatusAccepted)
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
			exporter, err := factory.CreateLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), exporterConfig)
			exporter.Start(context.Background(), nil)
			assert.NoError(t, err)
			rcv, err := newLogsReceiver(componenttest.NewNopReceiverCreateSettings(), *cfg, exporter)
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
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)
			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))
			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
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
				body, err := ioutil.ReadAll(req.Body)
				assert.NoError(t, err)
				rw.WriteHeader(http.StatusAccepted)
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

			exporter, err := factory.CreateMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), exporterConfig)
			exporter.Start(context.Background(), nil)
			assert.NoError(t, err)
			rcv, err := newMetricsReceiver(componenttest.NewNopReceiverCreateSettings(), *cfg, exporter)
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
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)
			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))
			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
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
		Time:  &time,
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
		Time:       &time,
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
				req := httptest.NewRequest("POST", "http://localhost/foo", nil)
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
				assert.Equal(t, http.StatusOK, status)
			},
		},

		{
			name: "two_logs",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/foo", strings.NewReader("foo\nbar"))
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusAccepted, status)
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
				assert.Equal(t, http.StatusAccepted, status)
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
				assert.Equal(t, http.StatusAccepted, status)
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
			rcv, err := newLogsReceiver(componenttest.NewNopReceiverCreateSettings(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*splunkReceiver)
			assert.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			defer r.Shutdown(context.Background())
			w := httptest.NewRecorder()
			r.handleRawReq(w, tt.req)

			resp := w.Result()
			respBytes, err := ioutil.ReadAll(resp.Body)
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
