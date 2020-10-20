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

package signalfxreceiver

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
	"runtime"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/testutil"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
)

func Test_signalfxeceiver_New(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	type args struct {
		config       Config
		nextConsumer consumer.MetricsConsumer
	}
	tests := []struct {
		name         string
		args         args
		wantStartErr error
	}{
		{
			name: "nil_nextConsumer",
			args: args{
				config: *defaultConfig,
			},
			wantStartErr: errNilNextConsumer,
		},
		{
			name: "default_endpoint",
			args: args{
				config:       *defaultConfig,
				nextConsumer: consumertest.NewMetricsNop(),
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
				nextConsumer: consumertest.NewMetricsNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newReceiver(zap.NewNop(), tt.args.config)
			if tt.args.nextConsumer != nil {
				got.RegisterMetricsConsumer(tt.args.nextConsumer)
			}
			err := got.Start(context.Background(), componenttest.NewNopHost())
			assert.Equal(t, tt.wantStartErr, err)
		})
	}
}

func Test_signalfxeceiver_EndToEnd(t *testing.T) {
	port := testutil.GetAvailablePort(t)
	addr := fmt.Sprintf("localhost:%d", port)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = addr
	sink := new(consumertest.MetricsSink)
	r := newReceiver(zap.NewNop(), *cfg)
	r.RegisterMetricsConsumer(sink)

	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	runtime.Gosched()
	defer r.Shutdown(context.Background())
	require.Equal(t, componenterror.ErrAlreadyStarted, r.Start(context.Background(), componenttest.NewNopHost()))

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	doubleVal := 1234.5678
	doublePt := metricstestutil.Double(tsUnix, doubleVal)
	int64Val := int64(123)
	int64Pt := &metricspb.Point{
		Timestamp: metricstestutil.Timestamp(tsUnix),
		Value:     &metricspb.Point_Int64Value{Int64Value: int64Val},
	}
	want := consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			metricstestutil.Gauge("gauge_double_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
			metricstestutil.GaugeInt("gauge_int_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, int64Pt)),
			metricstestutil.Cumulative("cumulative_double_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
			metricstestutil.CumulativeInt("cumulative_int_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, int64Pt)),
		},
	}

	expCfg := &signalfxexporter.Config{
		IngestURL:   "http://" + addr + "/v2/datapoint",
		APIURL:      "http://localhost",
		AccessToken: "access_token",
	}
	exp, err := signalfxexporter.NewFactory().CreateMetricsExporter(
		context.Background(),
		component.ExporterCreateParams{Logger: zap.NewNop()},
		expCfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, testutil.WaitForPort(t, port))
	defer exp.Shutdown(context.Background())
	require.NoError(t, exp.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(want)))
	// Description, unit and start time are expected to be dropped during conversions.
	for _, metric := range want.Metrics {
		if len(metric.MetricDescriptor.LabelKeys) == 0 {
			metric.MetricDescriptor.LabelKeys = nil
		}
		metric.MetricDescriptor.Description = ""
		metric.MetricDescriptor.Unit = ""
		for _, ts := range metric.Timeseries {
			ts.StartTimestamp = nil
			if len(ts.LabelValues) == 0 {
				ts.LabelValues = nil
			}
		}
	}

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := internaldata.MetricsToOC(mds[0])
	require.Len(t, got, 1)
	assert.Equal(t, want, got[0])

	assert.NoError(t, r.Shutdown(context.Background()))
	assert.Equal(t, componenterror.ErrAlreadyStopped, r.Shutdown(context.Background()))
}

func Test_sfxReceiver_handleReq(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint

	currentTime := time.Now().Unix() * 1e3
	sFxMsg := buildSFxDatapointMsg(currentTime, 13, 3)

	tests := []struct {
		name             string
		req              *http.Request
		skipRegistration bool
		assertResponse   func(t *testing.T, status int, body string)
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
			name: "incorrect_pipeline",
			req: func() *http.Request {
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
				return req
			}(),
			skipRegistration: true,
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrMetricsNotConfigured, body)
			},
		},
		{
			name: "incorrect_content_type",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Header.Set("Content-Type", "application/not-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidContentType, body)
			},
		},
		{
			name: "incorrect_content_encoding",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Header.Set("Content-Type", "application/x-protobuf")
				req.Header.Set("Content-Encoding", "superzipper")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidEncoding, body)
			},
		},
		{
			name: "fail_to_read_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Body = badReqBody{}
				req.Header.Set("Content-Type", "application/x-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrReadBody, body)
			},
		},
		{
			name: "bad_data_in_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader([]byte{1, 2, 3, 4}))
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)

				var buf bytes.Buffer
				gzipWriter := gzip.NewWriter(&buf)
				_, err = gzipWriter.Write(msgBytes)
				require.NoError(t, err)
				require.NoError(t, gzipWriter.Close())

				req := httptest.NewRequest("POST", "http://localhost", &buf)
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)

				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
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
			sink := new(consumertest.MetricsSink)
			rcv := newReceiver(zap.NewNop(), *config)
			if !tt.skipRegistration {
				rcv.RegisterMetricsConsumer(sink)
			}

			w := httptest.NewRecorder()
			rcv.handleDatapointReq(w, tt.req)

			resp := w.Result()
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			tt.assertResponse(t, resp.StatusCode, bodyStr)
		})
	}
}

func Test_sfxReceiver_handleEventReq(t *testing.T) {
	config := (NewFactory()).CreateDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint

	currentTime := time.Now().Unix() * 1e3
	sFxMsg := buildSFxEventMsg(currentTime, 13, 3)

	tests := []struct {
		name             string
		req              *http.Request
		skipRegistration bool
		assertResponse   func(t *testing.T, status int, body string)
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
			name: "incorrect_pipeline",
			req: func() *http.Request {
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
				return req
			}(),
			skipRegistration: true,
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrLogsNotConfigured, body)
			},
		},
		{
			name: "incorrect_content_type",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Header.Set("Content-Type", "application/not-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidContentType, body)
			},
		},
		{
			name: "incorrect_content_encoding",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Header.Set("Content-Type", "application/x-protobuf")
				req.Header.Set("Content-Encoding", "superzipper")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidEncoding, body)
			},
		},
		{
			name: "fail_to_read_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", nil)
				req.Body = badReqBody{}
				req.Header.Set("Content-Type", "application/x-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrReadBody, body)
			},
		},
		{
			name: "bad_data_in_body",
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader([]byte{1, 2, 3, 4}))
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)
				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)

				var buf bytes.Buffer
				gzipWriter := gzip.NewWriter(&buf)
				_, err = gzipWriter.Write(msgBytes)
				require.NoError(t, err)
				require.NoError(t, gzipWriter.Close())

				req := httptest.NewRequest("POST", "http://localhost", &buf)
				req.Header.Set("Content-Type", "application/x-protobuf")
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
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)

				req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
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
			rcv := newReceiver(zap.NewNop(), *config)
			if !tt.skipRegistration {
				rcv.RegisterLogsConsumer(sink)
			}

			w := httptest.NewRecorder()
			rcv.handleEventReq(w, tt.req)

			resp := w.Result()
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			tt.assertResponse(t, resp.StatusCode, bodyStr)
		})
	}
}

func Test_sfxReceiver_TLS(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = addr
	cfg.HTTPServerSettings.TLSSetting = &configtls.TLSServerSetting{
		TLSSetting: configtls.TLSSetting{
			CertFile: "./testdata/testcert.crt",
			KeyFile:  "./testdata/testkey.key",
		},
	}
	sink := new(consumertest.MetricsSink)
	r := newReceiver(zap.NewNop(), *cfg)
	r.RegisterMetricsConsumer(sink)
	defer r.Shutdown(context.Background())

	// NewNopHost swallows errors so using NewErrorWaitingHost to catch any potential errors starting the
	// receiver.
	mh := componenttest.NewErrorWaitingHost()
	require.NoError(t, r.Start(context.Background(), mh), "should not have failed to start metric reception")

	// If there are errors reported through host.ReportFatalError() this will retrieve it.
	receivedError, receivedErr := mh.WaitForFatalError(500 * time.Millisecond)
	require.NoError(t, receivedErr, "should not have failed to start metric reception")
	require.False(t, receivedError)
	t.Log("Metric Reception Started")

	msec := time.Now().Unix() * 1e3
	want := consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "single",
					Type: metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "k0"}, {Key: "k1"}, {Key: "k2"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{
								Value:    "v0",
								HasValue: true,
							},
							{
								Value:    "v1",
								HasValue: true,
							},
							{
								Value:    "v2",
								HasValue: true,
							},
						},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: msec / 1e3,
								Nanos:   int32(msec%1e3) * 1e3,
							},
							Value: &metricspb.Point_Int64Value{Int64Value: 13},
						}},
					},
				},
			},
		},
	}

	t.Log("Sending SignalFx metric data Request")

	sfxMsg := buildSFxDatapointMsg(msec, 13, 3)
	body, err := sfxMsg.Marshal()
	require.NoError(t, err, fmt.Sprintf("failed to marshal SFx message: %v", err))

	url := fmt.Sprintf("https://%s/v2/datapoint", addr)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoErrorf(t, err, "should have no errors with new request: %v", err)
	req.Header.Set("Content-Type", "application/x-protobuf")

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
	require.NoErrorf(t, err, "should not have failed when sending to signalFx receiver %v", err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	t.Log("SignalFx Request Received")

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := internaldata.MetricsToOC(mds[0])
	require.Len(t, got, 1)
	assert.Equal(t, want, got[0])
}

func Test_sfxReceiver_DatapointAccessTokenPassthrough(t *testing.T) {
	tests := []struct {
		name        string
		passthrough bool
		token       string
	}{
		{
			name:        "No token provided and passthrough false",
			passthrough: false,
			token:       "",
		},
		{
			name:        "No token provided and passthrough true",
			passthrough: true,
			token:       "",
		},
		{
			name:        "token provided and passthrough false",
			passthrough: false,
			token:       "myToken",
		},
		{
			name:        "token provided and passthrough true",
			passthrough: true,
			token:       "myToken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultConfig().(*Config)
			config.Endpoint = "localhost:0"
			config.AccessTokenPassthrough = tt.passthrough

			sink := new(consumertest.MetricsSink)
			rcv := newReceiver(zap.NewNop(), *config)
			rcv.RegisterMetricsConsumer(sink)

			currentTime := time.Now().Unix() * 1e3
			sFxMsg := buildSFxDatapointMsg(currentTime, 13, 3)
			msgBytes, _ := sFxMsg.Marshal()
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
			req.Header.Set("Content-Type", "application/x-protobuf")
			if tt.token != "" {
				req.Header.Set("x-sf-token", tt.token)
			}

			w := httptest.NewRecorder()
			rcv.handleDatapointReq(w, req)

			resp := w.Result()
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			got := internaldata.MetricsToOC(mds[0])
			require.Len(t, got, 1)
			tokenLabel := ""
			if got[0].Resource != nil && got[0].Resource.Labels != nil {
				tokenLabel = got[0].Resource.Labels["com.splunk.signalfx.access_token"]
			}

			if tt.passthrough {
				assert.Equal(t, tt.token, tokenLabel)
			} else {
				assert.Empty(t, tokenLabel)
			}
		})
	}
}

func Test_sfxReceiver_EventAccessTokenPassthrough(t *testing.T) {
	tests := []struct {
		name        string
		passthrough bool
		token       string
	}{
		{
			name:        "No token provided and passthrough false",
			passthrough: false,
			token:       "",
		},
		{
			name:        "No token provided and passthrough true",
			passthrough: true,
			token:       "",
		},
		{
			name:        "token provided and passthrough false",
			passthrough: false,
			token:       "myToken",
		},
		{
			name:        "token provided and passthrough true",
			passthrough: true,
			token:       "myToken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := (NewFactory()).CreateDefaultConfig().(*Config)
			config.Endpoint = "localhost:0"
			config.AccessTokenPassthrough = tt.passthrough

			sink := new(consumertest.LogsSink)
			rcv := newReceiver(zap.NewNop(), *config)
			rcv.RegisterLogsConsumer(sink)

			currentTime := time.Now().Unix() * 1e3
			sFxMsg := buildSFxEventMsg(currentTime, 13, 3)
			msgBytes, _ := sFxMsg.Marshal()
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
			req.Header.Set("Content-Type", "application/x-protobuf")
			if tt.token != "" {
				req.Header.Set("x-sf-token", tt.token)
			}

			w := httptest.NewRecorder()
			rcv.handleEventReq(w, req)

			resp := w.Result()
			respBytes, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)

			got := sink.AllLogs()
			require.Equal(t, 1, len(got))

			tokenLabel := ""
			if !got[0].ResourceLogs().At(0).Resource().IsNil() {
				accessTokenAttr, ok := got[0].ResourceLogs().At(0).Resource().Attributes().Get("com.splunk.signalfx.access_token")
				if ok {
					tokenLabel = accessTokenAttr.StringVal()
				}
			}

			if tt.passthrough {
				assert.Equal(t, tt.token, tokenLabel)
			} else {
				assert.Empty(t, tokenLabel)
			}
		})
	}
}

func buildSFxDatapointMsg(time int64, value int64, dimensions uint) *sfxpb.DataPointUploadMessage {
	return &sfxpb.DataPointUploadMessage{
		Datapoints: []*sfxpb.DataPoint{
			{
				Metric:    "single",
				Timestamp: time,
				Value: sfxpb.Datum{
					IntValue: int64Ptr(value),
				},
				MetricType: sfxTypePtr(sfxpb.MetricType_GAUGE),
				Dimensions: buildNDimensions(dimensions),
			},
		},
	}
}

func buildSFxEventMsg(time int64, value int64, dimensions uint) *sfxpb.EventUploadMessage {
	return &sfxpb.EventUploadMessage{
		Events: []*sfxpb.Event{
			{
				EventType: "single",
				Timestamp: time,
				Properties: []*sfxpb.Property{
					{
						Key: "a",
						Value: &sfxpb.PropertyValue{
							StrValue: strPtr("b"),
						},
					},
				},
				Category:   sfxCategoryPtr(sfxpb.EventCategory_USER_DEFINED),
				Dimensions: buildNDimensions(dimensions),
			},
		},
	}
}

type badReqBody struct{}

var _ io.ReadCloser = (*badReqBody)(nil)

func (b badReqBody) Read(p []byte) (n int, err error) {
	return 0, errors.New("badReqBody: can't read it")
}

func (b badReqBody) Close() error {
	return nil
}
