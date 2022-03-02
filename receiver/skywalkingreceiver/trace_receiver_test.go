// Copyright  OpenTelemetry Authors
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

package skywalkingreceiver

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

var (
	skywalkingReceiver = config.NewComponentIDWithName("skywalking", "receiver_test")
)

func TestTraceSource(t *testing.T) {
	set := componenttest.NewNopReceiverCreateSettings()
	jr := newSkywalkingReceiver(skywalkingReceiver, &configuration{}, nil, set)
	require.NotNil(t, jr)
}

func TestStartAndShutdown(t *testing.T) {
	port := 12800
	config := &configuration{
		CollectorHTTPPort: port,
		CollectorHTTPSettings: confighttp.HTTPServerSettings{
			Endpoint: fmt.Sprintf(":%d", port),
		},
	}
	sink := new(consumertest.TracesSink)

	set := componenttest.NewNopReceiverCreateSettings()
	sr := newSkywalkingReceiver(skywalkingReceiver, config, sink, set)

	require.NoError(t, sr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, sr.Shutdown(context.Background())) })

}

func TestGRPCReception(t *testing.T) {
	config := &configuration{
		CollectorGRPCPort: 11800, // that's the only one used by this test
	}

	sink := new(consumertest.TracesSink)

	set := componenttest.NewNopReceiverCreateSettings()

	swReceiver := newSkywalkingReceiver(skywalkingReceiver, config, sink, set)

	require.NoError(t, swReceiver.Start(context.Background(), componenttest.NewNopHost()))

	t.Cleanup(func() { require.NoError(t, swReceiver.Shutdown(context.Background())) })

	conn, err := grpc.Dial(fmt.Sprintf("0.0.0.0:%d", config.CollectorGRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	data := mockGrpcTraceSegment(1)
	segmentCollection := &agent.SegmentCollection{
		Segments: []*agent.SegmentObject{
			data,
		},
	}

	//skywalking agent client send trace data to otel/skywalkingreceiver
	client := agent.NewTraceSegmentReportServiceClient(conn)
	commands, err := client.CollectInSync(context.Background(), segmentCollection)
	if err != nil {
		t.Fatalf("cannot send data in sync mode: %v", err)
	}
	// verify
	assert.NoError(t, err, "send skywalking segment successful.")
	assert.NotNil(t, commands)
}

func mockGrpcTraceSegment(sequence int) *agent.SegmentObject {
	seq := strconv.Itoa(sequence)
	return &agent.SegmentObject{
		TraceId:         "trace" + seq,
		TraceSegmentId:  "trace-segment" + seq,
		Service:         "demo-service" + seq,
		ServiceInstance: "demo-instance" + seq,
		IsSizeLimited:   false,
		Spans: []*agent.SpanObject{
			{
				SpanId:        1,
				ParentSpanId:  0,
				StartTime:     time.Now().Unix(),
				EndTime:       time.Now().Unix() + 10,
				OperationName: "operation" + seq,
				Peer:          "127.0.0.1:6666",
				SpanType:      agent.SpanType_Entry,
				SpanLayer:     agent.SpanLayer_Http,
				ComponentId:   1,
				IsError:       false,
				SkipAnalysis:  false,
				Tags: []*common.KeyStringValuePair{
					{
						Key:   "mock-key" + seq,
						Value: "mock-value" + seq,
					},
				},
				Logs: []*agent.Log{
					{
						Time: time.Now().Unix(),
						Data: []*common.KeyStringValuePair{
							{
								Key:   "log-key" + seq,
								Value: "log-value" + seq,
							},
						},
					},
				},
				Refs: []*agent.SegmentReference{
					{
						RefType:                  agent.RefType_CrossThread,
						TraceId:                  "trace" + seq,
						ParentTraceSegmentId:     "parent-trace-segment" + seq,
						ParentSpanId:             0,
						ParentService:            "parent" + seq,
						ParentServiceInstance:    "parent" + seq,
						ParentEndpoint:           "parent" + seq,
						NetworkAddressUsedAtPeer: "127.0.0.1:6666",
					},
				},
			},
		},
	}
}
