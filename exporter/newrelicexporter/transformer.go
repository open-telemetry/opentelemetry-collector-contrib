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

package newrelicexporter

import (
	"errors"
	"sync"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opencensus.io/trace"
)

var transformers = sync.Pool{
	New: func() interface{} { return new(transformer) },
}

type transformer struct {
	ServiceName string
	Resource    *resourcepb.Resource
}

var (
	emptySpan   telemetry.Span
	emptySpanID trace.SpanID
)

func (t *transformer) Span(span *tracepb.Span) (telemetry.Span, error) {
	if span == nil {
		return emptySpan, errors.New("empty span")
	}

	startTime := t.Timestamp(span.StartTime)

	var (
		traceID          trace.TraceID
		spanID, parentID trace.SpanID
	)

	copy(traceID[:], span.GetTraceId())
	copy(spanID[:], span.GetSpanId())
	copy(parentID[:], span.GetParentSpanId())
	sp := telemetry.Span{
		ID:          spanID.String(),
		TraceID:     traceID.String(),
		Name:        t.TruncatableString(span.GetName()),
		Timestamp:   startTime,
		Duration:    t.Timestamp(span.EndTime).Sub(startTime),
		ServiceName: t.ServiceName,
		Attributes:  t.SpanAttributes(span),
	}

	if parentID != emptySpanID {
		sp.ParentID = parentID.String()
	}

	return sp, nil
}

func (t *transformer) TruncatableString(ts *tracepb.TruncatableString) string {
	if ts == nil {
		return ""
	}
	return ts.Value
}

func (t *transformer) SpanAttributes(span *tracepb.Span) map[string]interface{} {

	length := 2

	isErr := t.isError(span.Status.Code)
	if isErr {
		length++
	}

	if t.Resource != nil {
		length += len(t.Resource.Labels)
	}

	if span.Attributes != nil {
		length += len(span.Attributes.AttributeMap)
	}

	attrs := make(map[string]interface{}, length)

	// Any existing error attribute will override this.
	if isErr {
		attrs["error"] = true
	}

	if t.Resource != nil {
		for k, v := range t.Resource.Labels {
			attrs[k] = v
		}
	}

	if span.Attributes != nil {
		for key, attr := range span.Attributes.AttributeMap {
			if attr == nil || attr.Value == nil {
				continue
			}
			// Default to skipping if unknown type.
			switch v := attr.Value.(type) {
			case *tracepb.AttributeValue_BoolValue:
				attrs[key] = v.BoolValue
			case *tracepb.AttributeValue_IntValue:
				attrs[key] = v.IntValue
			case *tracepb.AttributeValue_DoubleValue:
				attrs[key] = v.DoubleValue
			case *tracepb.AttributeValue_StringValue:
				attrs[key] = t.TruncatableString(v.StringValue)
			}
		}
	}

	// Default attributes to tell New Relic about this collector.
	// (overrides any existing)
	attrs["collector.name"] = name
	attrs["collector.version"] = version

	return attrs
}

func (t *transformer) Timestamp(ts *timestamp.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return time.Unix(ts.Seconds, int64(ts.Nanos))
}

func (t *transformer) isError(code int32) bool {
	if code == 0 {
		return false
	}
	return true
}
