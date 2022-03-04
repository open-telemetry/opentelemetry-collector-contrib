// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skywalking

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func TestSwProtoToTraces(t *testing.T) {
	swSpan := mockGrpcTraceSegment(1)
	td := ToOtlpTraces(swSpan)
	fmt.Printf("converted trace data: %#v\n", td)
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
