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

package sapmreceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger/model"
	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/sapmprotocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/testutil"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func expectedTraceData(t1, t2, t3 time.Time) pdata.Traces {
	traceID := pdata.NewTraceID(
		[]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
	parentSpanID := pdata.NewSpanID([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18})
	childSpanID := pdata.NewSpanID([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	rs.Resource().InitEmpty()
	rs.Resource().Attributes().InsertString(conventions.AttributeServiceName, "issaTest")
	rs.Resource().Attributes().InsertBool("bool", true)
	rs.Resource().Attributes().InsertString("string", "yes")
	rs.Resource().Attributes().InsertInt("int64", 10000000)
	rs.InstrumentationLibrarySpans().Resize(1)
	rs.InstrumentationLibrarySpans().At(0).Spans().Resize(2)

	span0 := rs.InstrumentationLibrarySpans().At(0).Spans().At(0)
	span0.SetSpanID(childSpanID)
	span0.SetParentSpanID(parentSpanID)
	span0.SetTraceID(traceID)
	span0.SetName("DBSearch")
	span0.SetStartTime(pdata.TimestampUnixNano(uint64(t1.UnixNano())))
	span0.SetEndTime(pdata.TimestampUnixNano(uint64(t2.UnixNano())))
	span0.Status().InitEmpty()
	span0.Status().SetCode(pdata.StatusCodeNotFound)
	span0.Status().SetMessage("Stale indices")

	span1 := rs.InstrumentationLibrarySpans().At(0).Spans().At(1)
	span1.SetSpanID(parentSpanID)
	span1.SetTraceID(traceID)
	span1.SetName("ProxyFetch")
	span1.SetStartTime(pdata.TimestampUnixNano(uint64(t2.UnixNano())))
	span1.SetEndTime(pdata.TimestampUnixNano(uint64(t3.UnixNano())))
	span1.Status().InitEmpty()
	span1.Status().SetCode(pdata.StatusCodeInternalError)
	span1.Status().SetMessage("Frontend crash")

	return traces
}

func grpcFixture(t1 time.Time, d1, d2 time.Duration) *model.Batch {
	traceID := model.TraceID{}
	traceID.Unmarshal([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
	parentSpanID := model.NewSpanID(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18}))
	childSpanID := model.NewSpanID(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}))

	return &model.Batch{
		Process: &model.Process{
			ServiceName: "issaTest",
			Tags: []model.KeyValue{
				model.Bool("bool", true),
				model.String("string", "yes"),
				model.Int64("int64", 1e7),
			},
		},
		Spans: []*model.Span{
			{
				TraceID:       traceID,
				SpanID:        childSpanID,
				OperationName: "DBSearch",
				StartTime:     t1,
				Duration:      d1,
				Tags: []model.KeyValue{
					model.String(tracetranslator.TagStatusMsg, "Stale indices"),
					model.Int64(tracetranslator.TagStatusCode, trace.StatusCodeNotFound),
					model.Bool("error", true),
				},
				References: []model.SpanRef{
					{
						TraceID: traceID,
						SpanID:  parentSpanID,
						RefType: model.SpanRefType_CHILD_OF,
					},
				},
			},
			{
				TraceID:       traceID,
				SpanID:        parentSpanID,
				OperationName: "ProxyFetch",
				StartTime:     t1.Add(d1),
				Duration:      d2,
				Tags: []model.KeyValue{
					model.String(tracetranslator.TagStatusMsg, "Frontend crash"),
					model.Int64(tracetranslator.TagStatusCode, trace.StatusCodeInternal),
					model.Bool("error", true),
				},
			},
		},
	}
}

// sendSapm acts as a client for sending sapm to the receiver.  This could be replaced with a sapm exporter in the future.
func sendSapm(endpoint string, sapm *splunksapm.PostSpansRequest, zipped bool, tlsEnabled bool, token string) (*http.Response, error) {
	// marshal the sapm
	reqBytes, err := sapm.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sapm %v", err.Error())
	}

	if zipped {
		// create a gzip writer
		var buff bytes.Buffer
		writer := gzip.NewWriter(&buff)

		// run the request bytes through the gzip writer
		_, err = writer.Write(reqBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to write gzip sapm %v", err.Error())
		}

		// close the writer
		err = writer.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to close the gzip writer %v", err.Error())
		}

		// save the gzipped bytes as the request bytes
		reqBytes = buff.Bytes()
	}

	// build the request
	url := fmt.Sprintf("http://%s%s", endpoint, sapmprotocol.TraceEndpointV2)
	if tlsEnabled {
		url = fmt.Sprintf("https://%s%s", endpoint, sapmprotocol.TraceEndpointV2)
	}
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBytes))
	req.Header.Set(sapmprotocol.ContentTypeHeaderName, sapmprotocol.ContentTypeHeaderValue)

	// set headers for gzip
	if zipped {
		req.Header.Set(sapmprotocol.ContentEncodingHeaderName, sapmprotocol.GZipEncodingHeaderValue)
		req.Header.Set(sapmprotocol.AcceptEncodingHeaderName, sapmprotocol.GZipEncodingHeaderValue)
	}

	if token != "" {
		req.Header.Set("x-sf-token", token)
	}

	// send the request
	client := &http.Client{}

	if tlsEnabled {
		caCert, errCert := ioutil.ReadFile("./testdata/testcert.crt")
		if errCert != nil {
			return nil, fmt.Errorf("failed to load certificate: %s", errCert.Error())
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("failed to send request to receiver %v", err)
	}

	return resp, nil
}

func setupReceiver(t *testing.T, config *Config, sink *consumertest.TracesSink) component.TraceReceiver {
	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	sr, err := New(context.Background(), params, config, sink)
	assert.NoError(t, err, "should not have failed to create the SAPM receiver")
	t.Log("Starting")

	// NewNopHost swallows errors so using NewErrorWaitingHost to catch any potential errors starting the
	// receiver.
	mh := componenttest.NewErrorWaitingHost()
	require.NoError(t, sr.Start(context.Background(), mh), "should not have failed to start trace reception")

	// If there are errors reported through host.ReportFatalError() this will retrieve it.
	receivedError, receivedErr := mh.WaitForFatalError(500 * time.Millisecond)
	require.NoError(t, receivedErr, "should not have failed to start trace reception")
	require.False(t, receivedError)
	t.Log("Trace Reception Started")
	return sr
}

func TestReception(t *testing.T) {
	now := time.Unix(1542158650, 536343000).UTC()
	nowPlus10min := now.Add(10 * time.Minute)
	nowPlus10min2sec := now.Add(10 * time.Minute).Add(2 * time.Second)
	tlsAddress := testutil.GetAvailableLocalAddress(t)

	type args struct {
		config *Config
		sapm   *splunksapm.PostSpansRequest
		zipped bool
		useTLS bool
	}
	tests := []struct {
		name string
		args args
		want pdata.Traces
	}{
		{
			name: "receive uncompressed sapm",
			args: args{
				// 1. Create the SAPM receiver aka "server"
				config: &Config{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: defaultEndpoint,
					},
				},
				sapm:   &splunksapm.PostSpansRequest{Batches: []*model.Batch{grpcFixture(now, time.Minute*10, time.Second*2)}},
				zipped: false,
				useTLS: false,
			},
			want: expectedTraceData(now, nowPlus10min, nowPlus10min2sec),
		},
		{
			name: "receive compressed sapm",
			args: args{
				config: &Config{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: defaultEndpoint,
					},
				},
				sapm:   &splunksapm.PostSpansRequest{Batches: []*model.Batch{grpcFixture(now, time.Minute*10, time.Second*2)}},
				zipped: true,
				useTLS: false,
			},
			want: expectedTraceData(now, nowPlus10min, nowPlus10min2sec),
		},
		{
			name: "connect via TLS compressed sapm",
			args: args{
				config: &Config{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: tlsAddress,
						TLSSetting: &configtls.TLSServerSetting{
							TLSSetting: configtls.TLSSetting{
								CertFile: "./testdata/testcert.crt",
								KeyFile:  "./testdata/testkey.key",
							},
						},
					},
				},
				sapm:   &splunksapm.PostSpansRequest{Batches: []*model.Batch{grpcFixture(now, time.Minute*10, time.Second*2)}},
				zipped: false,
				useTLS: true,
			},
			want: expectedTraceData(now, nowPlus10min, nowPlus10min2sec),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			sink := new(consumertest.TracesSink)
			sr := setupReceiver(t, tt.args.config, sink)
			defer sr.Shutdown(context.Background())

			t.Log("Sending Sapm Request")
			var resp *http.Response
			resp, err := sendSapm(tt.args.config.Endpoint, tt.args.sapm, tt.args.zipped, tt.args.useTLS, "")
			require.NoErrorf(t, err, "should not have failed when sending sapm %v", err)
			assert.Equal(t, 200, resp.StatusCode)
			t.Log("SAPM Request Received")

			// retrieve received traces
			got := sink.AllTraces()
			assert.Equal(t, 1, len(got))

			// compare what we got to what we wanted
			t.Log("Comparing expected data to trace data")
			assert.EqualValues(t, tt.want, got[0])
		})
	}
}

func TestAccessTokenPassthrough(t *testing.T) {
	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		token                  string
	}{
		{
			name:                   "no passthrough and no token",
			accessTokenPassthrough: false,
			token:                  "",
		},
		{
			name:                   "no passthrough and token",
			accessTokenPassthrough: false,
			token:                  "MyAccessToken",
		},
		{
			name:                   "passthrough and no token",
			accessTokenPassthrough: true,
			token:                  "",
		},
		{
			name:                   "passthrough and token",
			accessTokenPassthrough: true,
			token:                  "MyAccessToken",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: defaultEndpoint,
				},
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: tt.accessTokenPassthrough,
				},
			}

			sapm := &splunksapm.PostSpansRequest{
				Batches: []*model.Batch{grpcFixture(time.Now().UTC(), time.Minute*10, time.Second*2)},
			}

			sink := new(consumertest.TracesSink)
			sr := setupReceiver(t, config, sink)
			defer sr.Shutdown(context.Background())

			var resp *http.Response
			resp, err := sendSapm(config.Endpoint, sapm, true, false, tt.token)
			require.NoErrorf(t, err, "should not have failed when sending sapm %v", err)
			assert.Equal(t, 200, resp.StatusCode)

			got := sink.AllTraces()
			assert.Equal(t, 1, len(got))

			received := got[0].ResourceSpans()
			for i := 0; i < received.Len(); i++ {
				rspan := received.At(i)
				attrs := rspan.Resource().Attributes()
				amap, contains := attrs.Get("com.splunk.signalfx.access_token")
				if tt.accessTokenPassthrough && tt.token != "" {
					assert.Equal(t, tt.token, amap.StringVal())
				} else {
					assert.False(t, contains)
				}
			}
		})
	}
}
