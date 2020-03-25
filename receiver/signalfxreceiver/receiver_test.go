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
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/proto"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exportertest"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	"github.com/open-telemetry/opentelemetry-collector/testutils"
	"github.com/open-telemetry/opentelemetry-collector/testutils/metricstestutils"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
)

func Test_signalfxeceiver_New(t *testing.T) {
	defaultConfig := (&Factory{}).CreateDefaultConfig().(*Config)
	type args struct {
		config       Config
		nextConsumer consumer.MetricsConsumerOld
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
				config:       *defaultConfig,
				nextConsumer: new(exportertest.SinkMetricsExporter),
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "happy_path",
			args: args{
				config: Config{
					ReceiverSettings: configmodels.ReceiverSettings{
						Endpoint: "localhost:1234",
					},
				},
				nextConsumer: new(exportertest.SinkMetricsExporter),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(zap.NewNop(), tt.args.config, tt.args.nextConsumer)
			assert.Equal(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func Test_signalfxeceiver_EndToEnd(t *testing.T) {
	addr := testutils.GetAvailableLocalAddress(t)
	cfg := (&Factory{}).CreateDefaultConfig().(*Config)
	cfg.Endpoint = addr
	sink := new(exportertest.SinkMetricsExporter)
	r, err := New(zap.NewNop(), *cfg, sink)
	require.NoError(t, err)

	mh := component.NewMockHost()
	err = r.Start(mh)
	require.NoError(t, err)
	runtime.Gosched()
	defer r.Shutdown()
	require.Equal(t, oterr.ErrAlreadyStarted, r.Start(mh))

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	doubleVal := 1234.5678
	doublePt := metricstestutils.Double(tsUnix, doubleVal)
	int64Val := int64(123)
	int64Pt := &metricspb.Point{
		Timestamp: metricstestutils.Timestamp(tsUnix),
		Value:     &metricspb.Point_Int64Value{Int64Value: int64Val},
	}
	want := consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			metricstestutils.Gauge("gauge_double_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, doublePt)),
			metricstestutils.GaugeInt("gauge_int_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, int64Pt)),
			metricstestutils.Cumulative("cumulative_double_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, doublePt)),
			metricstestutils.CumulativeInt("cumulative_int_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, int64Pt)),
		},
	}

	expCfg := &signalfxexporter.Config{
		URL: "http://" + addr + "/v2/datapoint",
	}
	exp, err := signalfxexporter.New(expCfg, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, exp.Start(mh))
	defer exp.Shutdown()
	require.NoError(t, exp.ConsumeMetricsData(context.Background(), want))
	// Description, unit and start time are expected to be dropped during conversions.
	for _, metric := range want.Metrics {
		metric.MetricDescriptor.Description = ""
		metric.MetricDescriptor.Unit = ""
		for _, ts := range metric.Timeseries {
			ts.StartTimestamp = nil
		}
	}

	got := sink.AllMetrics()
	require.Equal(t, 1, len(got))
	assert.Equal(t, want, got[0])

	assert.NoError(t, r.Shutdown())
	assert.Equal(t, oterr.ErrAlreadyStopped, r.Shutdown())
}

func Test_sfxReceiver_handleReq(t *testing.T) {
	config := (&Factory{}).CreateDefaultConfig().(*Config)
	config.Endpoint = "localhost:0" // Actually not creating the endpoint

	buildSFxMsgFn := func() *sfxpb.DataPointUploadMessage {
		return &sfxpb.DataPointUploadMessage{
			Datapoints: []*sfxpb.DataPoint{
				{
					Metric: strPtr("single"),
					Timestamp: func() *int64 {
						l := time.Now().Unix() * 1e3
						return &l
					}(),
					Value: &sfxpb.Datum{
						IntValue: int64Ptr(13),
					},
					MetricType: sfxTypePtr(sfxpb.MetricType_GAUGE),
					Dimensions: buildNDimensions(3),
				},
			},
		}
	}

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
				sfxMsg := buildSFxMsgFn()
				msgBytes, err := proto.Marshal(sfxMsg)
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
				sfxMsg := buildSFxMsgFn()
				msgBytes, err := proto.Marshal(sfxMsg)
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
				sfxMsg := buildSFxMsgFn()
				msgBytes, err := proto.Marshal(sfxMsg)
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
			sink := new(exportertest.SinkMetricsExporter)
			rcv, err := New(zap.NewNop(), *config, sink)
			assert.NoError(t, err)

			r := rcv.(*sfxReceiver)
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

type badReqBody struct{}

var _ io.ReadCloser = (*badReqBody)(nil)

func (b badReqBody) Read(p []byte) (n int, err error) {
	return 0, errors.New("badReqBody: can't read it")
}

func (b badReqBody) Close() error {
	return nil
}
