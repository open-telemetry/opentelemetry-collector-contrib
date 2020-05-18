// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stackdriverexporter

import (
	"context"
	"net"
	"testing"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	cloudtracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	"google.golang.org/grpc"
)

func mustTS(t time.Time) *timestamp.Timestamp {
	tt, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}
	return tt
}

type testServer struct {
	ch chan *cloudtracepb.BatchWriteSpansRequest
}

func (ts *testServer) BatchWriteSpans(ctx context.Context, r *cloudtracepb.BatchWriteSpansRequest) (*empty.Empty, error) {
	go func() {
		ts.ch <- r
	}()
	return &empty.Empty{}, nil
}

// Creates a new span.
func (ts *testServer) CreateSpan(context.Context, *cloudtracepb.Span) (*cloudtracepb.Span, error) {
	return nil, nil
}

func TestStackdriverExport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := grpc.NewServer()

	reqCh := make(chan *cloudtracepb.BatchWriteSpansRequest)

	cloudtracepb.RegisterTraceServiceServer(srv, &testServer{ch: reqCh})

	lis, err := net.Listen("tcp", ":8080")
	defer func() {
		_ = lis.Close()
	}()
	require.NoError(t, err)

	go srv.Serve(lis)

	sde, err := newStackdriverTraceExporter(&Config{
		ProjectID:   "idk",
		Endpoint:    "127.0.0.1:8080",
		UseInsecure: true,
	})
	require.NoError(t, err)

	testTime := time.Now()

	td := consumerdata.TraceData{
		Resource: &resourcepb.Resource{},
		Spans: []*tracepb.Span{
			{
				Name: &tracepb.TruncatableString{
					Value: "foobar",
				},
				StartTime: mustTS(testTime),
			},
		},
	}

	err = sde.ConsumeTraceData(ctx, td)
	assert.NoError(t, err)

	select {
	case <-time.After(10 * time.Second):
		t.Errorf("test timed out")
	case r := <-reqCh:
		assert.Len(t, r.Spans, 1)
		assert.Equal(t, "foobar", r.Spans[0].GetDisplayName().GetValue())
		assert.Equal(t, mustTS(testTime), r.Spans[0].StartTime)
	}
}
