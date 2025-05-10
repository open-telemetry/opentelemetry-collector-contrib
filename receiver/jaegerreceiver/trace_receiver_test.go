// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerreceiver

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/jaegertracing/jaeger-idl/proto-gen/api_v2"
	jaegerthrift "github.com/jaegertracing/jaeger-idl/thrift-gen/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/metadata"
)

var jaegerReceiver = component.MustNewIDWithName("jaeger", "receiver_test")

func TestTraceSource(t *testing.T) {
	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerReceiver, Protocols{}, nil, set)
	require.NoError(t, err)
	require.NotNil(t, jr)
}

func jaegerBatchToHTTPBody(b *jaegerthrift.Batch) (*http.Request, error) {
	body, err := thrift.NewTSerializer().Write(context.Background(), b)
	if err != nil {
		return nil, err
	}
	r := httptest.NewRequest(http.MethodPost, "/api/traces", bytes.NewReader(body))
	r.Header.Add("content-type", "application/x-thrift")
	return r, nil
}

func TestThriftHTTPBodyDecode(t *testing.T) {
	jr := jReceiver{}
	batch := &jaegerthrift.Batch{
		Process: jaegerthrift.NewProcess(),
		Spans:   []*jaegerthrift.Span{jaegerthrift.NewSpan()},
	}
	r, err := jaegerBatchToHTTPBody(batch)
	require.NoError(t, err, "failed to prepare http body")

	gotBatch, hErr := jr.decodeThriftHTTPBody(r)
	require.Nil(t, hErr, "failed to decode http body")
	assert.Equal(t, batch, gotBatch)
}

func TestReception(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	// 1. Create the Jaeger receiver aka "server"
	config := Protocols{
		ThriftHTTP: &confighttp.ServerConfig{
			Endpoint: addr,
		},
	}
	sink := new(consumertest.TracesSink)

	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerReceiver, config, sink, set)
	require.NoError(t, err)

	require.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	// 2. Then send spans to the Jaeger receiver.
	_, port, _ := net.SplitHostPort(addr)
	collectorAddr := fmt.Sprintf("http://localhost:%s/api/traces", port)
	td := generateTraceData()
	batches := jaeger.ProtoFromTraces(td)
	for _, batch := range batches {
		require.NoError(t, sendToCollector(collectorAddr, modelToThrift(batch)))
	}

	assert.NoError(t, err, "should not have failed to create the Jaeger OpenCensus exporter")

	gotTraces := sink.AllTraces()
	assert.Len(t, gotTraces, 1)

	assert.Equal(t, td, gotTraces[0])
}

func TestPortsNotOpen(t *testing.T) {
	// an empty config should result in no open ports
	config := Protocols{}

	sink := new(consumertest.TracesSink)

	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerReceiver, config, sink, set)
	require.NoError(t, err)

	require.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	// there is a race condition here that we're ignoring.
	//  this test may occasionally pass incorrectly, but it will not fail incorrectly
	//  TODO: consider adding a way for a receiver to asynchronously signal that is ready to receive spans to eliminate races/arbitrary waits
	l, err := net.Listen("tcp", "localhost:14250")
	assert.NoError(t, err, "should have been able to listen on 14250.  jaeger receiver incorrectly started grpc")
	if l != nil {
		l.Close()
	}

	l, err = net.Listen("tcp", "localhost:14268")
	assert.NoError(t, err, "should have been able to listen on 14268.  jaeger receiver incorrectly started thrift_http")
	if l != nil {
		l.Close()
	}
}

func TestGRPCReception(t *testing.T) {
	// prepare
	config := Protocols{
		GRPC: &configgrpc.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  testutil.GetAvailableLocalAddress(t),
				Transport: confignet.TransportTypeTCP,
			},
		},
	}
	sink := new(consumertest.TracesSink)

	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerReceiver, config, sink, set)
	require.NoError(t, err)

	require.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	conn, err := grpc.NewClient(config.GRPC.NetAddr.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	cl := api_v2.NewCollectorServiceClient(conn)

	now := time.Unix(1542158650, 536343000).UTC()
	d10min := 10 * time.Minute
	d2sec := 2 * time.Second
	nowPlus10min := now.Add(d10min)
	nowPlus10min2sec := now.Add(d10min).Add(d2sec)

	// test
	req := grpcFixture(t, now, d10min, d2sec)
	resp, err := cl.PostSpans(context.Background(), req, grpc.WaitForReady(true))

	// verify
	assert.NoError(t, err, "should not have failed to post spans")
	assert.NotNil(t, resp, "response should not have been nil")

	gotTraces := sink.AllTraces()
	assert.Len(t, gotTraces, 1)
	want := expectedTraceData(now, nowPlus10min, nowPlus10min2sec)

	assert.Len(t, req.Batch.Spans, want.SpanCount(), "got a conflicting amount of spans")

	assert.Equal(t, want, gotTraces[0])
}

func TestGRPCReceptionWithTLS(t *testing.T) {
	// prepare
	tlsCreds := &configtls.ServerConfig{
		Config: configtls.Config{
			CertFile: filepath.Join("testdata", "server.crt"),
			KeyFile:  filepath.Join("testdata", "server.key"),
		},
	}

	grpcServerSettings := &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  testutil.GetAvailableLocalAddress(t),
			Transport: confignet.TransportTypeTCP,
		},
		TLSSetting: tlsCreds,
	}

	config := Protocols{
		GRPC: grpcServerSettings,
	}
	sink := new(consumertest.TracesSink)

	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerReceiver, config, sink, set)
	require.NoError(t, err)

	require.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	creds, err := credentials.NewClientTLSFromFile(filepath.Join("testdata", "server.crt"), "localhost")
	require.NoError(t, err)
	conn, err := grpc.NewClient(grpcServerSettings.NetAddr.Endpoint, grpc.WithTransportCredentials(creds))
	require.NoError(t, err)
	defer conn.Close()

	cl := api_v2.NewCollectorServiceClient(conn)

	now := time.Now()
	d10min := 10 * time.Minute
	d2sec := 2 * time.Second
	nowPlus10min := now.Add(d10min)
	nowPlus10min2sec := now.Add(d10min).Add(d2sec)

	// test
	req := grpcFixture(t, now, d10min, d2sec)
	resp, err := cl.PostSpans(context.Background(), req, grpc.WaitForReady(true))

	// verify
	assert.NoError(t, err, "should not have failed to post spans")
	assert.NotNil(t, resp, "response should not have been nil")

	gotTraces := sink.AllTraces()
	assert.Len(t, gotTraces, 1)
	want := expectedTraceData(now, nowPlus10min, nowPlus10min2sec)

	assert.Len(t, req.Batch.Spans, want.SpanCount(), "got a conflicting amount of spans")
	assert.Equal(t, want, gotTraces[0])
}

func expectedTraceData(t1, t2, t3 time.Time) ptrace.Traces {
	traceID := pcommon.TraceID(
		[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
	parentSpanID := pcommon.SpanID([8]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18})
	childSpanID := pcommon.SpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "issaTest")
	rs.Resource().Attributes().PutBool("bool", true)
	rs.Resource().Attributes().PutStr("string", "yes")
	rs.Resource().Attributes().PutInt("int64", 10000000)
	spans := rs.ScopeSpans().AppendEmpty().Spans()

	span0 := spans.AppendEmpty()
	span0.SetSpanID(childSpanID)
	span0.SetParentSpanID(parentSpanID)
	span0.SetTraceID(traceID)
	span0.SetName("DBSearch")
	span0.SetStartTimestamp(pcommon.NewTimestampFromTime(t1))
	span0.SetEndTimestamp(pcommon.NewTimestampFromTime(t2))
	span0.Status().SetCode(ptrace.StatusCodeError)
	span0.Status().SetMessage("Stale indices")

	span1 := spans.AppendEmpty()
	span1.SetSpanID(parentSpanID)
	span1.SetTraceID(traceID)
	span1.SetName("ProxyFetch")
	span1.SetStartTimestamp(pcommon.NewTimestampFromTime(t2))
	span1.SetEndTimestamp(pcommon.NewTimestampFromTime(t3))
	span1.Status().SetCode(ptrace.StatusCodeError)
	span1.Status().SetMessage("Frontend crash")

	return traces
}

func grpcFixture(t *testing.T, t1 time.Time, d1, d2 time.Duration) *api_v2.PostSpansRequest {
	traceID := model.TraceID{}
	require.NoError(t, traceID.Unmarshal([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}))

	parentSpanID := model.NewSpanID(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18}))
	childSpanID := model.NewSpanID(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}))

	return &api_v2.PostSpansRequest{
		Batch: model.Batch{
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
						model.String(string(conventions.OTelStatusDescriptionKey), "Stale indices"),
						model.Int64(string(conventions.OTelStatusCodeKey), int64(ptrace.StatusCodeError)),
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
						model.String(string(conventions.OTelStatusDescriptionKey), "Frontend crash"),
						model.Int64(string(conventions.OTelStatusCodeKey), int64(ptrace.StatusCodeError)),
						model.Bool("error", true),
					},
				},
			},
		},
	}
}

func TestSampling(t *testing.T) {
	config := Protocols{
		GRPC: &configgrpc.ServerConfig{NetAddr: confignet.AddrConfig{
			Endpoint:  testutil.GetAvailableLocalAddress(t),
			Transport: confignet.TransportTypeTCP,
		}},
	}
	sink := new(consumertest.TracesSink)

	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerReceiver, config, sink, set)
	require.NoError(t, err)

	require.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	conn, err := grpc.NewClient(config.GRPC.NetAddr.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer conn.Close()

	cl := api_v2.NewSamplingManagerClient(conn)

	resp, err := cl.GetSamplingStrategy(context.Background(), &api_v2.SamplingStrategyParameters{
		ServiceName: "foo",
	})
	assert.Error(t, err, "expect: unknown service jaeger.api_v2.SamplingManager")
	assert.Nil(t, resp)
}

func TestConsumeThriftTrace(t *testing.T) {
	tests := []struct {
		batch    *jaegerthrift.Batch
		numSpans int
	}{
		{
			batch: nil,
		},
		{
			batch:    &jaegerthrift.Batch{Spans: []*jaegerthrift.Span{{}}},
			numSpans: 1,
		},
	}
	for _, test := range tests {
		numSpans, err := consumeTraces(context.Background(), test.batch, consumertest.NewNop())
		require.NoError(t, err)
		assert.Equal(t, test.numSpans, numSpans)
	}
}

func sendToCollector(endpoint string, batch *jaegerthrift.Batch) error {
	buf, err := thrift.NewTSerializer().Write(context.Background(), batch)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-thrift")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to upload traces; HTTP status code: %d", resp.StatusCode)
	}
	return nil
}
