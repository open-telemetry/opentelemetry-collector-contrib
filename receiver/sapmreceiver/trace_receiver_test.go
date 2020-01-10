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
	"encoding/binary"
	"fmt"
	"net/http"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/jaegertracing/jaeger/model"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exportertest"
	tracetranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace"
	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/sapmprotocol"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
)

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
					StartTime:    &timestamp.Timestamp{Seconds: t1.UnixNano() / 1e9, Nanos: int32(t1.UnixNano() % 1e9)},
					EndTime:      &timestamp.Timestamp{Seconds: t2.UnixNano() / 1e9, Nanos: int32(t2.UnixNano() % 1e9)},
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
					StartTime: &timestamp.Timestamp{Seconds: t2.UnixNano() / 1e9, Nanos: int32(t2.UnixNano() % 1e9)},
					EndTime:   &timestamp.Timestamp{Seconds: t3.UnixNano() / 1e9, Nanos: int32(t3.UnixNano() % 1e9)},
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
			SourceFormat: "sapm",
		},
	}
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
func sendSapm(endpoint string, sapm *splunksapm.PostSpansRequest, zipped bool) (*http.Response, error) {
	// marshal the sapm
	reqBytes, err := proto.Marshal(sapm)
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
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s%s", endpoint, sapmprotocol.TraceEndpointV2), bytes.NewReader(reqBytes))
	req.Header.Set(sapmprotocol.ContentTypeHeaderName, sapmprotocol.ContentTypeHeaderValue)

	// set headers for gzip
	if zipped {
		req.Header.Set(sapmprotocol.ContentEncodingHeaderName, sapmprotocol.GZipEncodingHeaderValue)
		req.Header.Set(sapmprotocol.AcceptEncodingHeaderName, sapmprotocol.GZipEncodingHeaderValue)
	}

	// send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("failed to send request to receiver %v", resp)
	}

	return resp, nil
}

func TestReception(t *testing.T) {
	now := time.Unix(1542158650, 536343000).UTC()
	nowPlus10min := now.Add(10 * time.Minute)
	nowPlus10min2sec := now.Add(10 * time.Minute).Add(2 * time.Second)

	type args struct {
		config *Config
		sapm   *splunksapm.PostSpansRequest
		zipped bool
	}
	tests := []struct {
		name string
		args args
		want []consumerdata.TraceData
	}{
		{
			name: "receive uncompressed sapm",
			args: args{
				// 1. Create the SAPM receiver aka "server"
				config: &Config{
					ReceiverSettings: configmodels.ReceiverSettings{Endpoint: defaultEndpoint},
				},
				sapm:   &splunksapm.PostSpansRequest{Batches: []*model.Batch{grpcFixture(now, time.Minute*10, time.Second*2)}},
				zipped: false,
			},
			want: expectedTraceData(now, nowPlus10min, nowPlus10min2sec),
		},
		{
			name: "receive compressed sapm",
			args: args{
				config: &Config{
					ReceiverSettings: configmodels.ReceiverSettings{Endpoint: defaultEndpoint},
				},
				sapm:   &splunksapm.PostSpansRequest{Batches: []*model.Batch{grpcFixture(now, time.Minute*10, time.Second*2)}},
				zipped: true,
			},
			want: expectedTraceData(now, nowPlus10min, nowPlus10min2sec),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			sink := new(exportertest.SinkTraceExporter)

			sr, err := New(context.Background(), zap.NewNop(), tt.args.config, sink)
			defer sr.Shutdown()
			assert.NoError(t, err, "should not have failed to create the SAPM receiver")
			t.Log("Starting")

			mh := component.NewMockHost()
			err = sr.Start(mh)
			assert.NoError(t, err, "should not have failed to start trace reception")
			t.Log("Trace Reception Started")

			t.Log("Sending Sapm Request")
			var resp *http.Response
			resp, err = sendSapm(tt.args.config.Endpoint, tt.args.sapm, tt.args.zipped)
			assert.NoError(t, err, fmt.Sprintf("should not have failed when sending sapm %v", resp))
			t.Log("SAPM Request Received")

			// retrieve received traces
			got := sink.AllTraces()

			// compare what we got to what we wanted
			t.Log("Comparing expected data to trace data")
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("Mismatched responses\n-Got +Want:\n\t%s", diff)
			}

		})
	}
}
