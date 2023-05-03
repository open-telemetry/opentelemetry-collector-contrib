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

package signalfxreceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_signalfxeceiver_New(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	type args struct {
		config       Config
		nextConsumer consumer.Metrics
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
			wantStartErr: component.ErrNilNextConsumer,
		},
		{
			name: "default_endpoint",
			args: args{
				config:       *defaultConfig,
				nextConsumer: consumertest.NewNop(),
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
				nextConsumer: consumertest.NewNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newReceiver(receivertest.NewNopCreateSettings(), tt.args.config)
			require.NoError(t, err)
			if tt.args.nextConsumer != nil {
				got.RegisterMetricsConsumer(tt.args.nextConsumer)
			}
			err = got.Start(context.Background(), componenttest.NewNopHost())
			assert.Equal(t, tt.wantStartErr, err)
		})
	}
}

func Test_signalfxeceiver_EndToEnd(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = addr
	sink := new(consumertest.MetricsSink)
	r, err := newReceiver(receivertest.NewNopCreateSettings(), *cfg)
	require.NoError(t, err)
	r.RegisterMetricsConsumer(sink)

	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	runtime.Gosched()
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	ts := pcommon.NewTimestampFromTime(time.Unix(unixSecs, unixNSecs))

	const doubleVal = 1234.5678
	const int64Val = int64(123)

	want := pmetric.NewMetrics()
	ilm := want.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

	{
		m := ilm.Metrics().AppendEmpty()
		m.SetName("gauge_double_with_dims")
		doublePt := m.SetEmptyGauge().DataPoints().AppendEmpty()
		doublePt.SetTimestamp(ts)
		doublePt.SetDoubleValue(doubleVal)
	}
	{
		m := ilm.Metrics().AppendEmpty()
		m.SetName("gauge_int_with_dims")
		int64Pt := m.SetEmptyGauge().DataPoints().AppendEmpty()
		int64Pt.SetTimestamp(ts)
		int64Pt.SetIntValue(int64Val)
	}
	{
		m := ilm.Metrics().AppendEmpty()
		m.SetName("cumulative_double_with_dims")
		m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		m.Sum().SetIsMonotonic(true)
		doublePt := m.Sum().DataPoints().AppendEmpty()
		doublePt.SetTimestamp(ts)
		doublePt.SetDoubleValue(doubleVal)
	}
	{
		m := ilm.Metrics().AppendEmpty()
		m.SetName("cumulative_int_with_dims")
		m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		m.Sum().SetIsMonotonic(true)
		int64Pt := m.Sum().DataPoints().AppendEmpty()
		int64Pt.SetTimestamp(ts)
		int64Pt.SetIntValue(int64Val)
	}

	expCfg := &signalfxexporter.Config{
		IngestURL:   "http://" + addr,
		APIURL:      "http://localhost",
		AccessToken: "access_token",
	}
	exp, err := signalfxexporter.NewFactory().CreateMetricsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		expCfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))
	assert.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", addr)
		if err == nil && conn != nil {
			conn.Close()
			return true
		}
		return false
	}, 10*time.Second, 5*time.Millisecond, "failed to wait for the port to be open")
	defer func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	}()
	require.NoError(t, exp.ConsumeMetrics(context.Background(), want))

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := mds[0]
	require.Equal(t, 1, got.ResourceMetrics().Len())
	assert.Equal(t, want, got)
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
				assert.Equal(t, http.StatusOK, status)
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
				assert.Equal(t, http.StatusOK, status)
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
			rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *config)
			require.NoError(t, err)
			if !tt.skipRegistration {
				rcv.RegisterMetricsConsumer(sink)
			}

			w := httptest.NewRecorder()
			rcv.handleDatapointReq(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
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
	sFxMsg := buildSFxEventMsg(currentTime, 3)

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
				assert.Equal(t, http.StatusOK, status)
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
				assert.Equal(t, http.StatusOK, status)
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
			rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *config)
			require.NoError(t, err)
			if !tt.skipRegistration {
				rcv.RegisterLogsConsumer(sink)
			}

			w := httptest.NewRecorder()
			rcv.handleEventReq(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
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
			CertFile: "./testdata/server.crt",
			KeyFile:  "./testdata/server.key",
		},
	}
	sink := new(consumertest.MetricsSink)
	r, err := newReceiver(receivertest.NewNopCreateSettings(), *cfg)
	require.NoError(t, err)
	r.RegisterMetricsConsumer(sink)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()

	mh := newAssertNoErrorHost(t)
	require.NoError(t, r.Start(context.Background(), mh), "should not have failed to start metric reception")

	// If there are errors reported through host.ReportFatalError() this will retrieve it.
	<-time.After(500 * time.Millisecond)
	t.Log("Metric Reception Started")

	msec := time.Now().Unix() * 1e3

	want := pmetric.NewMetrics()
	m := want.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()

	m.SetName("single")
	dps := m.SetEmptyGauge().DataPoints()
	dp := dps.AppendEmpty()
	dp.SetTimestamp(pcommon.Timestamp(msec * 1e6))
	dp.SetIntValue(13)

	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.Attributes().PutStr("k2", "v2")

	t.Log("Sending SignalFx metric data Request")

	sfxMsg := buildSFxDatapointMsg(msec, 13, 3)
	body, err := sfxMsg.Marshal()
	require.NoError(t, err, fmt.Sprintf("failed to marshal SFx message: %v", err))

	url := fmt.Sprintf("https://%s/v2/datapoint", addr)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoErrorf(t, err, "should have no errors with new request: %v", err)
	req.Header.Set("Content-Type", "application/x-protobuf")

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
	require.NoErrorf(t, err, "should not have failed when sending to signalFx receiver %v", err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Log("SignalFx Request Received")

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := mds[0]
	require.Equal(t, 1, got.ResourceMetrics().Len())
	require.NoError(t, pmetrictest.CompareMetrics(want, got))
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
			rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *config)
			require.NoError(t, err)
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
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			resource := mds[0].ResourceMetrics().At(0).Resource()
			tokenLabel := ""
			if label, ok := resource.Attributes().Get("com.splunk.signalfx.access_token"); ok {
				tokenLabel = label.Str()
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
			rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *config)
			require.NoError(t, err)
			rcv.RegisterLogsConsumer(sink)

			currentTime := time.Now().Unix() * 1e3
			sFxMsg := buildSFxEventMsg(currentTime, 3)
			msgBytes, _ := sFxMsg.Marshal()
			req := httptest.NewRequest("POST", "http://localhost", bytes.NewReader(msgBytes))
			req.Header.Set("Content-Type", "application/x-protobuf")
			if tt.token != "" {
				req.Header.Set("x-sf-token", tt.token)
			}

			w := httptest.NewRecorder()
			rcv.handleEventReq(w, req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)

			got := sink.AllLogs()
			require.Equal(t, 1, len(got))

			tokenLabel := ""
			if accessTokenAttr, ok := got[0].ResourceLogs().At(0).Resource().Attributes().Get("com.splunk.signalfx.access_token"); ok {
				tokenLabel = accessTokenAttr.Str()
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

func buildSFxEventMsg(time int64, dimensions uint) *sfxpb.EventUploadMessage {
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

func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func sfxTypePtr(t sfxpb.MetricType) *sfxpb.MetricType {
	return &t
}

func sfxCategoryPtr(t sfxpb.EventCategory) *sfxpb.EventCategory {
	return &t
}

func buildNDimensions(n uint) []*sfxpb.Dimension {
	d := make([]*sfxpb.Dimension, 0, n)
	for i := uint(0); i < n; i++ {
		idx := int(i)
		suffix := strconv.Itoa(idx)
		d = append(d, &sfxpb.Dimension{
			Key:   "k" + suffix,
			Value: "v" + suffix,
		})
	}
	return d
}
