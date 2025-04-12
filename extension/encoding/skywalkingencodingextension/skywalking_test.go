// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingencodingextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/protobuf/proto"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func mockGrpcTraceSegment() *agent.SegmentObject {
	return &agent.SegmentObject{
		TraceId:         "de5980b8-fce3-4a37-aab9-b4ac3af7eedd",
		TraceSegmentId:  "de5980b8-fce3-4a37-aab9-b4ac3af7eedd.33.16560607369950066",
		Service:         "demo-segment",
		ServiceInstance: "demo-instance",
		IsSizeLimited:   false,
		Spans: []*agent.SpanObject{
			{
				SpanId:        1,
				ParentSpanId:  0,
				StartTime:     time.Now().Unix(),
				EndTime:       time.Now().Unix() + 10,
				OperationName: "operation/hello",
				Peer:          "127.0.0.1:6666",
				SpanType:      agent.SpanType_Entry,
				SpanLayer:     agent.SpanLayer_Http,
				ComponentId:   1,
				IsError:       false,
				SkipAnalysis:  false,
				Tags: []*common.KeyStringValuePair{
					{
						Key:   "foo-key",
						Value: "bar-value",
					},
				},
				Logs: []*agent.Log{
					{
						Time: time.Now().Unix(),
						Data: []*common.KeyStringValuePair{
							{
								Key:   "log-foo",
								Value: "log-bar",
							},
						},
					},
				},
				Refs: []*agent.SegmentReference{
					{
						RefType:                  agent.RefType_CrossThread,
						TraceId:                  "de5980b8-fce3-4a37-aab9-b4ac3af7eddd",
						ParentTraceSegmentId:     "de5980b8-fce3-4a37-aab9-b4ac3af7eddd.33.16560607369950066",
						ParentSpanId:             0,
						ParentService:            "parent-foo-service",
						ParentServiceInstance:    "parent-foo-instance",
						ParentEndpoint:           "parent-foo-endpoint",
						NetworkAddressUsedAtPeer: "127.0.0.1:6666",
					},
				},
			},
			{
				SpanId:        2,
				ParentSpanId:  1,
				StartTime:     time.Now().Unix(),
				EndTime:       time.Now().Unix() + 20,
				OperationName: "operation/bar",
				Peer:          "127.0.0.1:6666",
				SpanType:      agent.SpanType_Local,
				SpanLayer:     agent.SpanLayer_Http,
				ComponentId:   2,
				IsError:       false,
				SkipAnalysis:  false,
				Tags: []*common.KeyStringValuePair{
					{
						Key:   "bar-key",
						Value: "bar-value",
					},
				},
				Logs: []*agent.Log{
					{
						Time: time.Now().Unix(),
						Data: []*common.KeyStringValuePair{
							{
								Key:   "bar-key",
								Value: "bar-value",
							},
						},
					},
				},
			},
		},
	}
}

func TestUnmarshalSegment(t *testing.T) {
	swSegment := mockGrpcTraceSegment()
	segmentBytes, err := proto.Marshal(swSegment)

	require.NoError(t, err)

	tests := []struct {
		unmarshaler ptrace.Unmarshaler
		encoding    string
		bytes       []byte
	}{
		{
			unmarshaler: skywalkingProtobufTrace{},
			encoding:    "skywalking_proto",
			bytes:       segmentBytes,
		},
	}
	for _, test := range tests {
		t.Run(test.encoding, func(t *testing.T) {
			got, err := test.unmarshaler.UnmarshalTraces(test.bytes)
			require.NoError(t, err)
			assert.Equal(t, 1, got.ResourceSpans().Len())
			assert.Equal(t, 3, got.ResourceSpans().At(0).Resource().Attributes().Len())
			assert.Equal(t, 2, got.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
			assert.Equal(t, "de5980b8fce34a37aab9b4ac3af7eedd", got.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID().String())
		})
	}
}
