// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	zipkin2 "github.com/jaegertracing/jaeger/model/converter/thrift/zipkin"
	"github.com/jaegertracing/jaeger/thrift-gen/zipkincore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

const (
	zipkinV2Single      = "../../pkg/translator/zipkin/zipkinv2/testdata/zipkin_v2_single.json"
	zipkinV2NoTimestamp = "../../pkg/translator/zipkin/zipkinv2/testdata/zipkin_v2_notimestamp.json"
	zipkinV1SingleBatch = "../../pkg/translator/zipkin/zipkinv1/testdata/zipkin_v1_single_batch.json"
)

func TestNew(t *testing.T) {
	type args struct {
		address      string
		nextConsumer consumer.Traces
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name:    "nil next Consumer",
			args:    args{},
			wantErr: component.ErrNilNextConsumer,
		},
		{
			name: "happy path",
			args: args{
				nextConsumer: consumertest.NewNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: tt.args.address,
				},
			}
			got, err := newReceiver(cfg, tt.args.nextConsumer, receivertest.NewNopCreateSettings())
			require.Equal(t, tt.wantErr, err)
			if tt.wantErr == nil {
				require.NotNil(t, got)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestZipkinReceiverPortAlreadyInUse(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "failed to open a port: %v", err)
	defer l.Close()
	_, portStr, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err, "failed to split listener address: %v", err)
	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:" + portStr,
		},
	}
	traceReceiver, err := newReceiver(cfg, consumertest.NewNop(), receivertest.NewNopCreateSettings())
	require.NoError(t, err, "Failed to create receiver: %v", err)
	err = traceReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestConvertSpansToTraceSpans_json(t *testing.T) {
	// Using Adrian Cole's sample at https://gist.github.com/adriancole/e8823c19dfed64e2eb71
	blob, err := os.ReadFile("./testdata/sample1.json")
	require.NoError(t, err, "Failed to read sample JSON file: %v", err)
	zi := newTestZipkinReceiver()
	reqs, err := zi.v2ToTraceSpans(blob, nil)
	require.NoError(t, err, "Failed to parse convert Zipkin spans in JSON to Trace spans: %v", err)

	require.Equal(t, 1, reqs.ResourceSpans().Len(), "Expecting only one request since all spans share same node/localEndpoint: %v", reqs.ResourceSpans().Len())

	req := reqs.ResourceSpans().At(0)
	sn, _ := req.Resource().Attributes().Get(conventions.AttributeServiceName)
	assert.Equal(t, "frontend", sn.Str())

	// Expecting 9 non-nil spans
	require.Equal(t, 9, reqs.SpanCount(), "Incorrect non-nil spans count")
}

func TestStartTraceReception(t *testing.T) {
	tests := []struct {
		name    string
		host    component.Host
		wantErr bool
	}{
		{
			name:    "nil_host",
			wantErr: true,
		},
		{
			name: "valid_host",
			host: componenttest.NewNopHost(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.TracesSink)
			cfg := &Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: "localhost:0",
				},
			}
			zr, err := newReceiver(cfg, sink, receivertest.NewNopCreateSettings())
			require.Nil(t, err)
			require.NotNil(t, zr)

			err = zr.Start(context.Background(), tt.host)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				require.Nil(t, zr.Shutdown(context.Background()))
			}
		})
	}
}

func TestReceiverContentTypes(t *testing.T) {
	tests := []struct {
		endpoint string
		content  string
		encoding string
		bodyFn   func() ([]byte, error)
	}{
		{
			endpoint: "/api/v1/spans",
			content:  "application/json",
			encoding: "gzip",
			bodyFn: func() ([]byte, error) {
				return os.ReadFile(zipkinV1SingleBatch)
			},
		},

		{
			endpoint: "/api/v1/spans",
			content:  "application/x-thrift",
			encoding: "gzip",
			bodyFn: func() ([]byte, error) {
				return thriftExample(), nil
			},
		},

		{
			endpoint: "/api/v2/spans",
			content:  "application/json",
			encoding: "gzip",
			bodyFn: func() ([]byte, error) {
				return os.ReadFile(zipkinV2Single)
			},
		},

		{
			endpoint: "/api/v2/spans",
			content:  "application/json",
			encoding: "zlib",
			bodyFn: func() ([]byte, error) {
				return os.ReadFile(zipkinV2Single)
			},
		},

		{
			endpoint: "/api/v2/spans",
			content:  "application/json",
			encoding: "",
			bodyFn: func() ([]byte, error) {
				return os.ReadFile(zipkinV2Single)
			},
		},
	}

	for _, test := range tests {
		name := fmt.Sprintf("%v %v %v", test.endpoint, test.content, test.encoding)
		t.Run(name, func(t *testing.T) {
			body, err := test.bodyFn()
			require.NoError(t, err, "Failed to generate test body: %v", err)

			var requestBody *bytes.Buffer
			switch test.encoding {
			case "":
				requestBody = bytes.NewBuffer(body)
			case "zlib":
				requestBody, err = compressZlib(body)
			case "gzip":
				requestBody, err = compressGzip(body)
			}
			require.NoError(t, err)

			r := httptest.NewRequest("POST", test.endpoint, requestBody)
			r.Header.Add("content-type", test.content)
			r.Header.Add("content-encoding", test.encoding)

			next := new(consumertest.TracesSink)
			cfg := &Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: "",
				},
			}
			zr, err := newReceiver(cfg, next, receivertest.NewNopCreateSettings())
			require.NoError(t, err)

			req := httptest.NewRecorder()
			zr.ServeHTTP(req, r)
			require.Equal(t, 202, req.Code)

			assert.Eventually(t, func() bool {
				allTraces := next.AllTraces()
				return len(allTraces) != 0
			}, 2*time.Second, 10*time.Millisecond)
		})
	}
}

func TestReceiverInvalidContentType(t *testing.T) {
	body := `{ invalid json `

	r := httptest.NewRequest("POST", "/api/v2/spans",
		bytes.NewBuffer([]byte(body)))
	r.Header.Add("content-type", "application/json")

	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "",
		},
	}
	zr, err := newReceiver(cfg, consumertest.NewNop(), receivertest.NewNopCreateSettings())
	require.NoError(t, err)

	req := httptest.NewRecorder()
	zr.ServeHTTP(req, r)

	require.Equal(t, 400, req.Code)
	require.Equal(t, "invalid character 'i' looking for beginning of object key string\n", req.Body.String())
}

func TestReceiverConsumerError(t *testing.T) {
	body, err := os.ReadFile(zipkinV2Single)
	require.NoError(t, err)

	r := httptest.NewRequest("POST", "/api/v2/spans", bytes.NewBuffer(body))
	r.Header.Add("content-type", "application/json")

	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:9411",
		},
	}
	zr, err := newReceiver(cfg, consumertest.NewErr(errors.New("consumer error")), receivertest.NewNopCreateSettings())
	require.NoError(t, err)

	req := httptest.NewRecorder()
	zr.ServeHTTP(req, r)

	require.Equal(t, 500, req.Code)
	require.Equal(t, "\"Internal Server Error\"", req.Body.String())
}

func TestReceiverConsumerPermanentError(t *testing.T) {
	body, err := os.ReadFile(zipkinV2Single)
	require.NoError(t, err)

	r := httptest.NewRequest("POST", "/api/v2/spans", bytes.NewBuffer(body))
	r.Header.Add("content-type", "application/json")

	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:9411",
		},
	}
	zr, err := newReceiver(cfg, consumertest.NewErr(consumererror.NewPermanent(errors.New("consumer error"))), receivertest.NewNopCreateSettings())
	require.NoError(t, err)

	req := httptest.NewRecorder()
	zr.ServeHTTP(req, r)

	require.Equal(t, 400, req.Code)
	require.Equal(t, "\"Bad Request\"", req.Body.String())
}

func thriftExample() []byte {
	now := time.Now().Unix()
	zSpans := []*zipkincore.Span{
		{
			TraceID: 1,
			Name:    "test",
			ID:      2,
			BinaryAnnotations: []*zipkincore.BinaryAnnotation{
				{
					Key:   "http.path",
					Value: []byte("/"),
				},
			},
			Timestamp: &now,
		},
	}

	return zipkin2.SerializeThrift(context.TODO(), zSpans)
}

func compressGzip(body []byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(body)
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

func compressZlib(body []byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)

	_, err := zw.Write(body)
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

func TestConvertSpansToTraceSpans_JSONWithoutSerivceName(t *testing.T) {
	blob, err := os.ReadFile("./testdata/sample2.json")
	require.NoError(t, err, "Failed to read sample JSON file: %v", err)
	zi := newTestZipkinReceiver()
	reqs, err := zi.v2ToTraceSpans(blob, nil)
	require.NoError(t, err, "Failed to parse convert Zipkin spans in JSON to Trace spans: %v", err)

	require.Equal(t, 1, reqs.ResourceSpans().Len(), "Expecting only one request since all spans share same node/localEndpoint: %v", reqs.ResourceSpans().Len())

	// Expecting 1 non-nil spans
	require.Equal(t, 1, reqs.SpanCount(), "Incorrect non-nil spans count")
}

func TestConvertSpansToTraceSpans_JSONWithSimpleError(t *testing.T) {
	blob, err := os.ReadFile("./testdata/sample3.json")
	require.NoError(t, err, "Failed to read sample JSON file: %v", err)
	zi := newTestZipkinReceiver()
	reqs, err := zi.v2ToTraceSpans(blob, nil)
	require.NoError(t, err, "Failed to parse convert Zipkin spans in JSON to Trace spans: %v", err)

	require.Equal(t, 1, reqs.ResourceSpans().Len(), "Expecting only one request since all spans share same node/localEndpoint: %v", reqs.ResourceSpans().Len())

	// Expecting 1 non-nil spans
	require.Equal(t, 1, reqs.SpanCount(), "Incorrect non-nil spans count")

	require.Equal(t, ptrace.StatusCodeError, reqs.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Status().Code())

	_, exists := reqs.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Get("error")
	require.False(t, exists, "Error attribute should be removed when error is simply set to \"true\".")
}

func TestConvertSpansToTraceSpans_JSONWithErrorMessage(t *testing.T) {
	blob, err := os.ReadFile("./testdata/sample4.json")
	require.NoError(t, err, "Failed to read sample JSON file: %v", err)
	zi := newTestZipkinReceiver()
	reqs, err := zi.v2ToTraceSpans(blob, nil)
	require.NoError(t, err, "Failed to parse convert Zipkin spans in JSON to Trace spans: %v", err)

	require.Equal(t, 1, reqs.ResourceSpans().Len(), "Expecting only one request since all spans share same node/localEndpoint: %v", reqs.ResourceSpans().Len())

	// Expecting 1 non-nil spans
	require.Equal(t, 1, reqs.SpanCount(), "Incorrect non-nil spans count")

	require.Equal(t, ptrace.StatusCodeError, reqs.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Status().Code(),
		"Error code should be set to the proper error status since the trace had an error tag.")

	errorMessage, exists := reqs.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Get("error")
	require.True(t, exists, "Given span should have an error attribute with the received span's error message.")
	require.Equal(t, "Non-basic error message", errorMessage.AsString(), "Error message should be retained in the span.")
}

func TestReceiverConvertsStringsToTypes(t *testing.T) {
	body, err := os.ReadFile(zipkinV2Single)
	require.NoError(t, err, "Failed to read sample JSON file: %v", err)

	r := httptest.NewRequest("POST", "/api/v2/spans", bytes.NewBuffer(body))
	r.Header.Add("content-type", "application/json")

	next := new(consumertest.TracesSink)
	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "",
		},
		ParseStringTags: true,
	}
	zr, err := newReceiver(cfg, next, receivertest.NewNopCreateSettings())
	require.NoError(t, err)

	req := httptest.NewRecorder()
	zr.ServeHTTP(req, r)
	require.Equal(t, 202, req.Code)

	require.Eventually(t, func() bool {
		allTraces := next.AllTraces()
		return len(allTraces) != 0
	}, 2*time.Second, 10*time.Millisecond)

	td := next.AllTraces()[0]
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	expected := map[string]any{
		"cache_hit":            true,
		"ping_count":           int64(25),
		"timeout":              12.3,
		"clnt/finagle.version": "6.45.0",
		"http.path":            "/api",
		"http.status_code":     int64(500),
		"net.host.ip":          "7::80:807f",
		"peer.service":         "backend",
		"net.peer.ip":          "192.168.99.101",
		"net.peer.port":        int64(9000),
	}

	assert.EqualValues(t, expected, span.Attributes().AsRaw())
}

func TestFromBytesWithNoTimestamp(t *testing.T) {
	noTimestampBytes, err := os.ReadFile(zipkinV2NoTimestamp)
	require.NoError(t, err, "Failed to read sample JSON file: %v", err)

	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "",
		},
		ParseStringTags: true,
	}
	zi, err := newReceiver(cfg, consumertest.NewNop(), receivertest.NewNopCreateSettings())
	require.NoError(t, err)

	hdr := make(http.Header)
	hdr.Set("Content-Type", "application/json")

	// under the hood this calls V2SpansToInternalTraces, the
	// method we want to test, and this is a better end to end
	// representation of what happens for the notimestamp case.
	traces, err := zi.v2ToTraceSpans(noTimestampBytes, hdr)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	gs := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.NotNil(t, gs.StartTimestamp)
	assert.NotNil(t, gs.EndTimestamp)

	// missing timestamp and duration in the incoming zipkin v2 json
	// are handled and converted to unix time zero in the internal span
	// format.
	fakeStartTimestamp := gs.StartTimestamp().AsTime().UnixNano()
	assert.Equal(t, int64(0), fakeStartTimestamp)

	fakeEndTimestamp := gs.StartTimestamp().AsTime().UnixNano()
	assert.Equal(t, int64(0), fakeEndTimestamp)

	wasAbsent, mapContainedKey := gs.Attributes().Get("otel.zipkin.absentField.startTime")
	assert.True(t, mapContainedKey)
	assert.True(t, wasAbsent.Bool())
}
