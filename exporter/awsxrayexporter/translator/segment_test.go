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

package translator

import (
	"fmt"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClientSpanWithRpcComponent(t *testing.T) {
	spanName := "/widgets"
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = RpcComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "ipv6"
	attributes[PeerIpv6Attribute] = "2607:f8b0:4000:80c::2004"
	attributes[PeerPortAttribute] = "9443"
	attributes[TargetAttribute] = spanName
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(nil, spanName, 0, "OK", attributes, labels)

	segment := MakeSegment(spanName, span)

	assert.NotNil(t, segment)
	assert.Equal(t, spanName, segment.Name)
	w := borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
}

func constructClientSpan(parentSpanId []byte, name string, code int32, message string, attributes map[string]interface{}, rscLabels map[string]string) *tracepb.Span {
	var (
		traceId        = newTraceID()
		spanId         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	return &tracepb.Span{
		TraceId:      traceId,
		SpanId:       spanId,
		ParentSpanId: parentSpanId,
		Name:         &tracepb.TruncatableString{Value: name},
		Kind:         tracepb.Span_CLIENT,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code:    code,
			Message: message,
		},
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: spanAttributes,
		},
		Resource: &resourcepb.Resource{
			Type:   "container",
			Labels: rscLabels,
		},
	}
}

func constructSpanAttributes(attributes map[string]interface{}) map[string]*tracepb.AttributeValue {
	attrs := make(map[string]*tracepb.AttributeValue)
	for key, value := range attributes {
		valType := reflect.TypeOf(value)
		var attrVal tracepb.AttributeValue
		if valType.Kind() == reflect.Int {
			attrVal = tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
				IntValue: int64(value.(int)),
			}}
		} else if valType.Kind() == reflect.Int64 {
			attrVal = tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
				IntValue: value.(int64),
			}}
		} else {
			attrVal = tracepb.AttributeValue{Value: &tracepb.AttributeValue_StringValue{
				StringValue: &tracepb.TruncatableString{Value: fmt.Sprintf("%v", value)},
			}}
		}
		attrs[key] = &attrVal
	}
	return attrs
}

func constructDefaultResourceLabels() map[string]string {
	labels := make(map[string]string)
	labels[ServiceNameAttribute] = "signup_aggregator"
	labels[ServiceVersionAttribute] = "1.1.12"
	labels[ContainerNameAttribute] = "signup_aggregator"
	labels[ContainerImageAttribute] = "otel/signupaggregator"
	labels[ContainerTagAttribute] = "v1"
	labels[K8sClusterAttribute] = "production"
	labels[K8sNamespaceAttribute] = "default"
	labels[K8sDeploymentAttribute] = "signup_aggregator"
	labels[K8sPodAttribute] = "signup_aggregator-x82ufje83"
	labels[CloudProviderAttribute] = "aws"
	labels[CloudAccountAttribute] = "123456789"
	labels[CloudRegionAttribute] = "us-east-1"
	labels[CloudZoneAttribute] = "us-east-1c"
	return labels
}

func convertTimeToTimestamp(t time.Time) *timestamp.Timestamp {
	if t.IsZero() {
		return nil
	}
	nanoTime := t.UnixNano()
	return &timestamp.Timestamp{
		Seconds: nanoTime / 1e9,
		Nanos:   int32(nanoTime % 1e9),
	}
}
