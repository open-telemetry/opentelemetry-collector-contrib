// Copyright The OpenTelemetry Authors
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

package utils

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/stretchr/testify/assert"
)

// TestGetRootFromCompleteTrace ensures that GetRoot returns a root span or fails gracefully from a complete trace chunk
// From: https://github.com/DataDog/datadog-agent/blob/eb819b117fba4d57ccc35f1a74d4618b57daf8aa/pkg/trace/traceutil/trace_test.go#L15
func TestGetRootFromCompleteTrace(t *testing.T) {
	assert := assert.New(t)

	var trace *pb.APITrace
	trace = *pb.APITrace{
		TraceID:   uint64(1234),
		Spans:     []*pb.Span{},
		StartTime: 0,
		EndTime:   0,
	}

			

	// append(pb.Span{TraceID: uint64(1234), SpanID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
	// 		pb.Span{TraceID: uint64(1234), SpanID: uint64(12342), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
	// 		pb.Span{TraceID: uint64(1234), SpanID: uint64(12343), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
	// 		pb.Span{TraceID: uint64(1234), SpanID: uint64(12344), ParentID: uint64(12342), Service: "s2", Name: "n2", Resource: ""},
	// 		pb.Span{TraceID: uint64(1234), SpanID: uint64(12345), ParentID: uint64(12344), Service: "s2", Name: "n2", Resource: ""},
	// 	}
	// trace := *pb.APITrace{
	// 	&pb.Span{TraceID: uint64(1234), SpanID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
	// 	&pb.Span{TraceID: uint64(1234), SpanID: uint64(12342), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
	// 	&pb.Span{TraceID: uint64(1234), SpanID: uint64(12343), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
	// 	&pb.Span{TraceID: uint64(1234), SpanID: uint64(12344), ParentID: uint64(12342), Service: "s2", Name: "n2", Resource: ""},
	// 	&pb.Span{TraceID: uint64(1234), SpanID: uint64(12345), ParentID: uint64(12344), Service: "s2", Name: "n2", Resource: ""},
	// }

	assert.Equal(GetRoot(trace).SpanID, uint64(12341))
}

// TestGetRootFromPartialTrace ensures that GetRoot returns a root span or fails gracefully from a partial trace chunk
// From: https://github.com/DataDog/datadog-agent/blob/eb819b117fba4d57ccc35f1a74d4618b57daf8aa/pkg/trace/traceutil/trace_test.go#L29
// func TestGetRootFromPartialTrace(t *testing.T) {
// 	assert := assert.New(t)

// 	trace := *pb.APITrace{
// 		&pb.Span{TraceID: uint64(1234), SpanID: uint64(12341), ParentID: uint64(12340), Service: "s1", Name: "n1", Resource: ""},
// 		&pb.Span{TraceID: uint64(1234), SpanID: uint64(12342), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
// 		&pb.Span{TraceID: uint64(1234), SpanID: uint64(12343), ParentID: uint64(12342), Service: "s2", Name: "n2", Resource: ""},
// 	}

// 	assert.Equal(GetRoot(trace).SpanID, uint64(12341))
// }
