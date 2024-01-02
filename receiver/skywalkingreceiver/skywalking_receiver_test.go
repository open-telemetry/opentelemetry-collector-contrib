// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

var (
	skywalkingReceiver = component.NewIDWithName("skywalking", "receiver_test")
)

var traceJSON = []byte(`
	[{
	"traceId": "a12ff60b-5807-463b-a1f8-fb1c8608219e",
	"serviceInstance": "User_Service_Instance_Name",
	"spans": [{
		"operationName": "/ingress",
		"startTime": 1588664577013,
		"endTime": 1588664577028,
		"spanType": 0,
		"spanId": 1,
		"isError": false,
		"parentSpanId": 0,
		"componentId": 6000,
		"peer": "upstream service",
		"spanLayer": 3
	}, {
		"operationName": "/ingress",
		"startTime": 1588664577013,
		"tags": [{
			"key": "http.method",
			"value": "GET"
		}, {
			"key": "http.params",
			"value": "http://localhost/ingress"
		}],
		"endTime": 1588664577028,
		"spanType": 1,
		"spanId": 0,
		"parentSpanId": -1,
		"isError": false,
		"spanLayer": 3,
		"componentId": 6000
	}],
	"service": "User_Service_Name",
	"traceSegmentId": "a12ff60b-5807-463b-a1f8-fb1c8608219e"
}]`)

func TestTraceSource(t *testing.T) {
	set := receivertest.NewNopCreateSettings()
	set.ID = skywalkingReceiver
	mockSwReceiver := newSkywalkingReceiver(&configuration{}, set)
	err := mockSwReceiver.registerTraceConsumer(nil)
	assert.Equal(t, err, component.ErrNilNextConsumer)
	require.NotNil(t, mockSwReceiver)
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

	set := receivertest.NewNopCreateSettings()
	set.ID = skywalkingReceiver
	sr := newSkywalkingReceiver(config, set)
	err := sr.registerTraceConsumer(sink)
	require.NoError(t, err)
	require.NoError(t, sr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, sr.Shutdown(context.Background())) })

}

func TestGRPCReception(t *testing.T) {
	config := &configuration{
		CollectorGRPCPort: 11800, // that's the only one used by this test
	}

	sink := new(consumertest.TracesSink)

	set := receivertest.NewNopCreateSettings()
	set.ID = skywalkingReceiver
	mockSwReceiver := newSkywalkingReceiver(config, set)
	err := mockSwReceiver.registerTraceConsumer(sink)
	require.NoError(t, err)
	require.NoError(t, mockSwReceiver.Start(context.Background(), componenttest.NewNopHost()))

	t.Cleanup(func() { require.NoError(t, mockSwReceiver.Shutdown(context.Background())) })
	conn, err := grpc.Dial(fmt.Sprintf("0.0.0.0:%d", config.CollectorGRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	segmentCollection := &agent.SegmentCollection{
		Segments: []*agent.SegmentObject{
			mockGrpcTraceSegment(1),
		},
	}

	// skywalking agent client send trace data to otel/skywalkingreceiver
	client := agent.NewTraceSegmentReportServiceClient(conn)
	commands, err := client.CollectInSync(context.Background(), segmentCollection)
	if err != nil {
		t.Fatalf("cannot send data in sync mode: %v", err)
	}
	// verify
	assert.NoError(t, err, "send skywalking segment successful.")
	assert.NotNil(t, commands)
}

func TestHttpReception(t *testing.T) {
	config := &configuration{
		CollectorHTTPPort: 12800,
		CollectorHTTPSettings: confighttp.HTTPServerSettings{
			Endpoint: fmt.Sprintf(":%d", 12800),
		},
	}

	sink := new(consumertest.TracesSink)

	set := receivertest.NewNopCreateSettings()
	set.ID = skywalkingReceiver
	mockSwReceiver := newSkywalkingReceiver(config, set)
	err := mockSwReceiver.registerTraceConsumer(sink)
	require.NoError(t, err)
	require.NoError(t, mockSwReceiver.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, mockSwReceiver.Shutdown(context.Background())) })
	req, err := http.NewRequest("POST", "http://127.0.0.1:12800/v3/segments", bytes.NewBuffer(traceJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(req)
	// http client send trace data to otel/skywalkingreceiver
	if err != nil {
		t.Fatalf("cannot send data in sync mode: %v", err)
	}
	// verify
	assert.NoError(t, err, "send skywalking segment successful.")
	assert.NotNil(t, response)

}

func mockGrpcTraceSegment(sequence int) *agent.SegmentObject {
	seq := strconv.Itoa(sequence)
	return &agent.SegmentObject{
		TraceId:         "trace" + seq,
		TraceSegmentId:  "trace-segment" + seq,
		Service:         "demo-segmentReportService" + seq,
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
			{
				SpanId:        2,
				ParentSpanId:  1,
				StartTime:     time.Now().Unix(),
				EndTime:       time.Now().Unix() + 20,
				OperationName: "operation" + seq,
				Peer:          "127.0.0.1:6666",
				SpanType:      agent.SpanType_Local,
				SpanLayer:     agent.SpanLayer_Http,
				ComponentId:   2,
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
			},
		},
	}
}
