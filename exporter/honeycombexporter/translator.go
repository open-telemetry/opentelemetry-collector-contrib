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

package honeycombexporter

import (
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"google.golang.org/grpc/codes"
)

// spanAttributesToMap converts an opencensus proto Span_Attributes object into a map
// of strings to generic types usable for sending events to honeycomb.
func spanAttributesToMap(spanAttrs *tracepb.Span_Attributes) map[string]interface{} {
	var attrs map[string]interface{}

	if spanAttrs == nil {
		return attrs
	}

	attrMap := spanAttrs.GetAttributeMap()
	if attrMap == nil {
		return attrs
	}

	attrs = make(map[string]interface{}, len(attrMap))

	for key, attributeValue := range attrMap {
		switch val := attributeValue.Value.(type) {
		case *tracepb.AttributeValue_StringValue:
			attrs[key] = attributeValueAsString(attributeValue)
		case *tracepb.AttributeValue_BoolValue:
			attrs[key] = val.BoolValue
		case *tracepb.AttributeValue_IntValue:
			attrs[key] = val.IntValue
		case *tracepb.AttributeValue_DoubleValue:
			attrs[key] = val.DoubleValue
		}
	}
	return attrs
}

// hasRemoteParent returns true if the this span is a child of a span in a different process.
func hasRemoteParent(span *tracepb.Span) bool {
	if sameProcess := span.GetSameProcessAsParentSpan(); sameProcess != nil {
		return !sameProcess.Value
	}
	return false
}

// timestampToTime converts a protobuf timestamp into a time.Time.
func timestampToTime(ts *timestamp.Timestamp) (t time.Time) {
	if ts == nil {
		return
	}
	return time.Unix(ts.Seconds, int64(ts.Nanos))
}

// getStatusCode returns the status code
func getStatusCode(status *tracepb.Status) int32 {
	if status != nil {
		return int32(codes.Code(status.Code))
	}

	return int32(codes.OK)
}

// getStatusMessage returns the status message as a string
func getStatusMessage(status *tracepb.Status) string {
	if status != nil {
		if len(status.GetMessage()) > 0 {
			return status.GetMessage()
		}
	}

	return codes.OK.String()
}

// getTraceLevelfFields extracts the node and resource fields from TraceData that
// should be added as underlays on every span in a trace.
func getTraceLevelFields(td consumerdata.TraceData) map[string]interface{} {
	fields := make(map[string]interface{})

	// we want to get the service_name and some opencensus fields that are set when
	// run as an agent from the node.
	nodeFields := func(node *commonpb.Node, fields map[string]interface{}) {
		if node == nil {
			return
		}
		if node.ServiceInfo != nil {
			fields["service_name"] = node.ServiceInfo.Name
		}
		if process := node.GetIdentifier(); process != nil {
			if len(process.HostName) != 0 {
				fields["process.hostname"] = process.HostName
			}
			fields["process.pid"] = process.Pid
			fields["opencensus.start_timestamp"] = timestampToTime(process.StartTimestamp)
		}
	}

	// all resource labels should be applied as trace level fields
	resourceFields := func(resource *resourcepb.Resource, fields map[string]interface{}) {
		if resource == nil {
			return
		}
		resourceType := resource.GetType()
		if resourceType != "" {
			fields["opencensus.resourcetype"] = resourceType
		}
		for k, v := range resource.GetLabels() {
			fields[k] = v
		}
	}

	nodeFields(td.Node, fields)
	resourceFields(td.Resource, fields)

	if sourceFormat := td.SourceFormat; len(sourceFormat) > 0 {
		fields["source_format"] = td.SourceFormat
	}

	return fields
}

// attributeValueAsString converts a opencensus proto AttributeValue object into a string
func attributeValueAsString(val *tracepb.AttributeValue) string {
	if wrapper := val.GetStringValue(); wrapper != nil {
		return wrapper.GetValue()
	}

	return ""
}

// truncatableStringAsString just returns the value part as a string. "" if s is nil
func truncatableStringAsString(s *tracepb.TruncatableString) string {
	if s != nil {
		return s.GetValue()
	}
	return ""
}
