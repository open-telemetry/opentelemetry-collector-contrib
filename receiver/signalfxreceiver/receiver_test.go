// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"go.opentelemetry.io/collector/component/componentstatus"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver/internal/metadata"
)

func Test_signalfxreceiver_New(t *testing.T) {
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
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:1234",
					},
				},
				nextConsumer: consumertest.NewNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newReceiver(receivertest.NewNopSettings(metadata.Type), tt.args.config)
			require.NoError(t, err)
			if tt.args.nextConsumer != nil {
				got.RegisterMetricsConsumer(tt.args.nextConsumer)
			}
			err = got.Start(context.Background(), componenttest.NewNopHost())
			assert.Equal(t, tt.wantStartErr, err)
			if err == nil {
				assert.NoError(t, got.Shutdown(context.Background()))
			}
		})
	}
}

func Test_signalfxreceiver_EndToEnd(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = addr
	sink := new(consumertest.MetricsSink)
	r, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *cfg)
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
	exp, err := signalfxexporter.NewFactory().CreateMetrics(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
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

	otlpContentHeader := "application/x-protobuf;format=otlp"
	otlpMetrics := buildOtlpMetrics(5)
	marshaler := &pmetric.ProtoMarshaler{}

	tests := []struct {
		name             string
		req              *http.Request
		skipRegistration bool
		assertResponse   func(t *testing.T, status int, body string)
	}{
		{
			name: "incorrect_method",
			req:  httptest.NewRequest(http.MethodPut, "http://localhost", nil),
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
				req.Header.Set("Content-Type", "application/not-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidContentType, body)
			},
		},
		{
			name: "incorrect_content_encoding_sfx",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
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
			name: "incorrect_content_encoding_otlp",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
				req.Header.Set("Content-Type", otlpContentHeader)
				req.Header.Set("Content-Encoding", "superzipper")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseInvalidEncoding, body)
			},
		},
		{
			name: "fail_to_read_body_sfx",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
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
			name: "fail_to_read_body_otlp",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
				req.Body = badReqBody{}
				req.Header.Set("Content-Type", otlpContentHeader)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrReadBody, body)
			},
		},
		{
			name: "bad_data_in_body_sfx",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader([]byte{1, 2, 3, 4}))
				req.Header.Set("Content-Type", "application/x-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrUnmarshalBody, body)
			},
		},
		{
			name: "bad_data_in_body_otlp",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader([]byte{1, 2, 3, 4}))
				req.Header.Set("Content-Type", otlpContentHeader)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrUnmarshalBody, body)
			},
		},
		{
			name: "empty_body_sfx",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(nil))
				req.Header.Set("Content-Type", "application/x-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "empty_body_otlp",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(nil))
				req.Header.Set("Content-Type", otlpContentHeader)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "msg_accepted_sfx",
			req: func() *http.Request {
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "msg_accepted_otlp",
			req: func() *http.Request {
				msgBytes, err := marshaler.MarshalMetrics(*otlpMetrics)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", otlpContentHeader)
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "msg_accepted_gzipped_sfx",
			req: func() *http.Request {
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)
				msgBytes = compressGzip(t, msgBytes)
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
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
			name: "msg_accepted_gzipped_otlp",
			req: func() *http.Request {
				msgBytes, err := marshaler.MarshalMetrics(*otlpMetrics)
				require.NoError(t, err)
				msgBytes = compressGzip(t, msgBytes)
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", otlpContentHeader)
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, responseOK, body)
			},
		},
		{
			name: "bad_gzipped_msg_sfx",
			req: func() *http.Request {
				msgBytes, err := sFxMsg.Marshal()
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", "application/x-protobuf")
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, responseErrGzipReader, body)
			},
		},
		{
			name: "bad_gzipped_msg_otlp",
			req: func() *http.Request {
				msgBytes, err := marshaler.MarshalMetrics(*otlpMetrics)
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
				req.Header.Set("Content-Type", otlpContentHeader)
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
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
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
			req:  httptest.NewRequest(http.MethodPut, "http://localhost", nil),
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
				req.Header.Set("Content-Type", "application/x-protobuf;format=otlp")
				return req
			}(),
			assertResponse: func(t *testing.T, status int, body string) {
				assert.Equal(t, http.StatusUnsupportedMediaType, status)
				assert.Equal(t, responseEventsInvalidContentType, body)
			},
		},
		{
			name: "incorrect_content_encoding",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", nil)
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader([]byte{1, 2, 3, 4}))
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(nil))
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
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
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
				msgBytes = compressGzip(t, msgBytes)
				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
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

				req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
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
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			require.NoError(t, err)
			if !tt.skipRegistration {
				rcv.RegisterLogsConsumer(sink)
			}

			w := httptest.NewRecorder()
			rcv.handleEventReq(w, tt.req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			defer resp.Body.Close()

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
	cfg.TLS = &configtls.ServerConfig{
		Config: configtls.Config{
			CertFile: "./testdata/server.crt",
			KeyFile:  "./testdata/server.key",
		},
	}
	sink := new(consumertest.MetricsSink)
	cs := receivertest.NewNopSettings(metadata.Type)
	r, err := newReceiver(cs, *cfg)
	require.NoError(t, err)
	r.RegisterMetricsConsumer(sink)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()

	mh := &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			require.NoError(t, event.Err())
		},
	}
	require.NoError(t, r.Start(context.Background(), mh), "should not have failed to start metric reception")

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
	require.NoErrorf(t, err, "failed to marshal SFx message: %v", err)

	url := fmt.Sprintf("https://%s/v2/datapoint", addr)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoErrorf(t, err, "should have no errors with new request: %v", err)
	req.Header.Set("Content-Type", "application/x-protobuf")

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
	require.NoErrorf(t, err, "should not have failed when sending to signalFx receiver %v", err)
	defer resp.Body.Close()
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
		otlp        bool
	}{
		{
			name:        "No token provided and passthrough false",
			passthrough: false,
			token:       "",
			otlp:        false,
		},
		{
			name:        "No token provided and passthrough true",
			passthrough: true,
			token:       "",
			otlp:        false,
		},
		{
			name:        "token provided and passthrough false",
			passthrough: false,
			token:       "myToken",
			otlp:        false,
		},
		{
			name:        "token provided and passthrough true",
			passthrough: true,
			token:       "myToken",
			otlp:        false,
		},
		{
			name:        "No token provided and passthrough false for OTLP payload",
			passthrough: false,
			token:       "",
			otlp:        true,
		},
		{
			name:        "No token provided and passthrough true for OTLP payload",
			passthrough: true,
			token:       "",
			otlp:        true,
		},
		{
			name:        "token provided and passthrough false for OTLP payload",
			passthrough: false,
			token:       "myToken",
			otlp:        true,
		},
		{
			name:        "token provided and passthrough true for OTLP payload",
			passthrough: true,
			token:       "myToken",
			otlp:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultConfig().(*Config)
			config.Endpoint = "localhost:0"
			config.AccessTokenPassthrough = tt.passthrough

			sink := new(consumertest.MetricsSink)
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			require.NoError(t, err)
			rcv.RegisterMetricsConsumer(sink)

			currentTime := time.Now().Unix() * 1e3

			var msgBytes []byte
			var contentHeader string
			if tt.otlp {
				marshaler := &pmetric.ProtoMarshaler{}
				msgBytes, err = marshaler.MarshalMetrics(*buildOtlpMetrics(5))
				require.NoError(t, err)
				contentHeader = otlpProtobufContentType
			} else {
				sFxMsg := buildSFxDatapointMsg(currentTime, 13, 3)
				msgBytes, _ = sFxMsg.Marshal()
				contentHeader = "application/x-protobuf"
			}
			req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
			req.Header.Set("Content-Type", contentHeader)
			if tt.token != "" {
				req.Header.Set("x-sf-token", tt.token)
			}

			w := httptest.NewRecorder()
			rcv.handleDatapointReq(w, req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			defer resp.Body.Close()

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
			rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *config)
			require.NoError(t, err)
			rcv.RegisterLogsConsumer(sink)

			currentTime := time.Now().Unix() * 1e3
			sFxMsg := buildSFxEventMsg(currentTime, 3)
			msgBytes, _ := sFxMsg.Marshal()
			req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(msgBytes))
			req.Header.Set("Content-Type", "application/x-protobuf")
			if tt.token != "" {
				req.Header.Set("x-sf-token", tt.token)
			}

			w := httptest.NewRecorder()
			rcv.handleEventReq(w, req)

			resp := w.Result()
			respBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			defer resp.Body.Close()

			var bodyStr string
			assert.NoError(t, json.Unmarshal(respBytes, &bodyStr))

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, responseOK, bodyStr)

			got := sink.AllLogs()
			require.Len(t, got, 1)

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

func (b badReqBody) Read(_ []byte) (n int, err error) {
	return 0, errors.New("badReqBody: can't read it")
}

func (b badReqBody) Close() error {
	return nil
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

func buildGauge(im pmetric.Metric) {
	metricTime := pcommon.NewTimestampFromTime(time.Now())
	im.SetName("gauge_test")
	im.SetDescription("")
	im.SetUnit("1")
	im.SetEmptyGauge()
	idps := im.Gauge().DataPoints()
	idp0 := idps.AppendEmpty()
	addAttributes(2, idp0.Attributes())
	idp0.SetStartTimestamp(metricTime)
	idp0.SetTimestamp(metricTime)
	idp0.SetIntValue(123)
	idp1 := idps.AppendEmpty()
	addAttributes(1, idp1.Attributes())
	idp1.SetStartTimestamp(metricTime)
	idp1.SetTimestamp(metricTime)
	idp1.SetIntValue(456)
}

func buildHistogram(im pmetric.Metric) {
	now := time.Now()
	startTime := pcommon.NewTimestampFromTime(now.Add(-10 * time.Second))
	endTime := pcommon.NewTimestampFromTime(now)
	im.SetName("histogram_test")
	im.SetDescription("")
	im.SetUnit("1")
	im.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	idps := im.Histogram().DataPoints()

	idp0 := idps.AppendEmpty()
	addAttributes(1, idp0.Attributes())
	idp0.SetStartTimestamp(startTime)
	idp0.SetTimestamp(endTime)
	idp0.SetMin(1.0)
	idp0.SetMax(2)
	idp0.SetCount(5)
	idp0.SetSum(7.0)
	idp0.BucketCounts().FromRaw([]uint64{3, 2})
	idp0.ExplicitBounds().FromRaw([]float64{1, 2})

	idp1 := idps.AppendEmpty()
	addAttributes(1, idp1.Attributes())
	idp1.SetStartTimestamp(startTime)
	idp1.SetTimestamp(endTime)
	idp1.SetMin(0.3)
	idp1.SetMax(3)
	idp1.SetCount(10)
	idp1.SetSum(17.5)
}

func addAttributes(count int, dst pcommon.Map) {
	for i := 0; i < count; i++ {
		suffix := strconv.Itoa(i)
		dst.PutStr("k"+suffix, "v"+suffix)
	}
}

func buildOtlpMetrics(metricsCount int) *pmetric.Metrics {
	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("resource-attr", "resource-attr-val-1")
	md.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
	ilm := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	ilm.EnsureCapacity(metricsCount)

	for i := 0; i < metricsCount; i++ {
		switch i % 2 {
		case 0:
			buildGauge(ilm.AppendEmpty())
		case 1:
			buildHistogram(ilm.AppendEmpty())
		}
	}
	return &md
}

func compressGzip(t *testing.T, msgBytes []byte) []byte {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err := gzipWriter.Write(msgBytes)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())
	return buf.Bytes()
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
