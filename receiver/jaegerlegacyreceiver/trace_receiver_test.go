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

package jaegerlegacyreceiver

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/jaegertracing/jaeger/tchannel/agent/app/reporter/tchannel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/jaeger-lib/metrics"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"
)

const jaegerReceiver = "jaeger_receiver_test"

func TestTraceSource(t *testing.T) {
	jr, err := New(jaegerReceiver, &Configuration{}, nil, zap.NewNop())
	assert.NoError(t, err, "should not have failed to create the Jaeger receiver")
	require.NotNil(t, jr)
}

func TestPortsNotOpen(t *testing.T) {
	// an empty config should result in no open ports
	config := &Configuration{}

	sink := new(exportertest.SinkTraceExporterOld)

	jr, err := New(jaegerReceiver, config, sink, zap.NewNop())
	assert.NoError(t, err, "should not have failed to create a new receiver")
	defer jr.Shutdown(context.Background())

	assert.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()), "should not have failed to start trace reception")

	// there is a race condition here that we're ignoring.
	//  this test may occasionally pass incorrectly, but it will not fail incorrectly
	//  TODO: consider adding a way for a receiver to asynchronously signal that is ready to receive spans to eliminate races/arbitrary waits
	l, err := net.Listen("tcp", "localhost:14267")
	assert.NoError(t, err, "should have been able to listen on 14267.  jaeger receiver incorrectly started thrift_tchannel")

	if l != nil {
		l.Close()
	}
}

func TestThriftTChannelReception(t *testing.T) {
	port := testutil.GetAvailablePort(t)
	config := &Configuration{
		CollectorThriftPort: int(port),
	}
	sink := new(exportertest.SinkTraceExporterOld)

	jr, err := New(jaegerReceiver, config, sink, zap.NewNop())
	assert.NoError(t, err, "should not have failed to create a new receiver")
	defer jr.Shutdown(context.Background())

	assert.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()), "should not have failed to start trace reception")
	t.Log("StartTraceReception")

	b := tchannel.NewBuilder()
	b.CollectorHostPorts = []string{fmt.Sprintf("localhost:%d", port)}

	p, err := tchannel.NewCollectorProxy(b, metrics.NullFactory, zap.NewNop())
	assert.NoError(t, err, "should not have failed to create collector proxy")

	now := time.Unix(1542158650, 536343000).UTC()
	d10min := 10 * time.Minute
	d2sec := 2 * time.Second
	nowPlus10min := now.Add(d10min)
	nowPlus10min2sec := now.Add(d10min).Add(d2sec)

	want := expectedTraceData(now, nowPlus10min, nowPlus10min2sec)
	batch, err := jaegerthrifthttpexporter.OCProtoToJaegerThrift(want[0])
	assert.NoError(t, err, "should not have failed proto/thrift translation")

	//confirm port is open before attempting
	err = testutil.WaitForPort(t, port)
	assert.NoError(t, err, "WaitForPort failed")

	err = p.GetReporter().EmitBatch(batch)
	assert.NoError(t, err, "should not have failed to emit batch")

	got := sink.AllTraces()
	assert.Len(t, batch.Spans, len(want[0].Spans), "got a conflicting amount of spans")

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Mismatched responses\n-Got +Want:\n\t%s", diff)
	}
}

func expectedTraceData(t1, t2, t3 time.Time) []consumerdata.TraceData {
	traceID := []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}
	parentSpanID := []byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18}
	childSpanID := []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}

	return []consumerdata.TraceData{
		{
			Node: &commonpb.Node{
				ServiceInfo: &commonpb.ServiceInfo{Name: "issaTest"},
				LibraryInfo: &commonpb.LibraryInfo{},
				Identifier:  &commonpb.ProcessIdentifier{},
				Attributes: map[string]string{
					"bool":   "true",
					"string": "yes",
					"int64":  "10000000",
				},
			},
			Spans: []*tracepb.Span{
				{
					TraceId:      traceID,
					SpanId:       childSpanID,
					ParentSpanId: parentSpanID,
					Name:         &tracepb.TruncatableString{Value: "DBSearch"},
					StartTime:    TimeToTimestamp(t1),
					EndTime:      TimeToTimestamp(t2),
					Status: &tracepb.Status{
						Code:    trace.StatusCodeNotFound,
						Message: "Stale indices",
					},
					Attributes: &tracepb.Span_Attributes{
						AttributeMap: map[string]*tracepb.AttributeValue{
							"error": {
								Value: &tracepb.AttributeValue_BoolValue{BoolValue: true},
							},
						},
					},
					Links: &tracepb.Span_Links{
						Link: []*tracepb.Span_Link{
							{
								TraceId: traceID,
								SpanId:  parentSpanID,
								Type:    tracepb.Span_Link_PARENT_LINKED_SPAN,
							},
						},
					},
				},
				{
					TraceId:   traceID,
					SpanId:    parentSpanID,
					Name:      &tracepb.TruncatableString{Value: "ProxyFetch"},
					StartTime: TimeToTimestamp(t2),
					EndTime:   TimeToTimestamp(t3),
					Status: &tracepb.Status{
						Code:    trace.StatusCodeInternal,
						Message: "Frontend crash",
					},
					Attributes: &tracepb.Span_Attributes{
						AttributeMap: map[string]*tracepb.AttributeValue{
							"error": {
								Value: &tracepb.AttributeValue_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			SourceFormat: "jaeger",
		},
	}
}

// TimeToTimestamp converts a time.Time to a timestamp.Timestamp pointer.
func TimeToTimestamp(t time.Time) *timestamp.Timestamp {
	if t.IsZero() {
		return nil
	}
	nanoTime := t.UnixNano()
	return &timestamp.Timestamp{
		Seconds: nanoTime / 1e9,
		Nanos:   int32(nanoTime % 1e9),
	}
}
