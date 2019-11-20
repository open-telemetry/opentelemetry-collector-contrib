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

package awsxrayexporter

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/translator"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestTraceExport(t *testing.T) {
	traceExporter := initializeTraceExporter()
	ctx := context.Background()
	td := constructSpanData()
	err := traceExporter.ConsumeTraceData(ctx, td)
	assert.Nil(t, err)
}

func initializeTraceExporter() exporter.TraceExporter {
	logger := zap.NewNop()
	factory := Factory{}
	traceExporter, err := factory.CreateTraceExporter(logger, factory.CreateDefaultConfig())
	if err != nil {
		panic(err)
	}
	return traceExporter
}

func constructSpanData() consumerdata.TraceData {
	resource := constructResource()
	spans := make([]*tracepb.Span, 2)
	spans[0] = constructHTTPClientSpan()
	spans[0].Resource = resource
	spans[1] = constructHTTPServerSpan()
	spans[1].Resource = resource
	return consumerdata.TraceData{
		Node:         nil,
		Resource:     resource,
		Spans:        spans,
		SourceFormat: "oc",
	}
}

func constructResource() *resourcepb.Resource {
	labels := make(map[string]string)
	labels[translator.ServiceNameAttribute] = "signup_aggregator"
	labels[translator.ServiceVersionAttribute] = "1.1.12"
	labels[translator.ContainerNameAttribute] = "signup_aggregator"
	labels[translator.ContainerImageAttribute] = "otel/signupaggregator"
	labels[translator.ContainerTagAttribute] = "v1"
	labels[translator.CloudProviderAttribute] = "aws"
	labels[translator.CloudAccountAttribute] = "999999998"
	labels[translator.CloudRegionAttribute] = "us-west-2"
	labels[translator.CloudZoneAttribute] = "us-west-1b"
	return &resourcepb.Resource{
		Type:   "container",
		Labels: labels,
	}
}

func constructHTTPClientSpan() *tracepb.Span {
	attributes := make(map[string]interface{})
	attributes[translator.ComponentAttribute] = translator.HTTPComponentType
	attributes[translator.MethodAttribute] = "GET"
	attributes[translator.URLAttribute] = "https://api.example.com/users/junit"
	attributes[translator.StatusCodeAttribute] = 200
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      translator.NewTraceID(),
		SpanId:       translator.NewSegmentID(),
		ParentSpanId: translator.NewSegmentID(),
		Name:         &tracepb.TruncatableString{Value: "/users/junit"},
		Kind:         tracepb.Span_CLIENT,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code:    0,
			Message: "OK",
		},
		SameProcessAsParentSpan: &wrappers.BoolValue{Value: false},
		Tracestate: &tracepb.Span_Tracestate{
			Entries: []*tracepb.Span_Tracestate_Entry{
				{Key: "foo", Value: "bar"},
				{Key: "a", Value: "b"},
			},
		},
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: spanAttributes,
		},
	}
}

func constructHTTPServerSpan() *tracepb.Span {
	attributes := make(map[string]interface{})
	attributes[translator.ComponentAttribute] = translator.HTTPComponentType
	attributes[translator.MethodAttribute] = "GET"
	attributes[translator.URLAttribute] = "https://api.example.com/users/junit"
	attributes[translator.UserAgentAttribute] = "PostmanRuntime/7.16.3"
	attributes[translator.ClientIPAttribute] = "192.168.15.32"
	attributes[translator.StatusCodeAttribute] = 200
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      translator.NewTraceID(),
		SpanId:       translator.NewSegmentID(),
		ParentSpanId: translator.NewSegmentID(),
		Name:         &tracepb.TruncatableString{Value: "/users/junit"},
		Kind:         tracepb.Span_SERVER,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code:    0,
			Message: "OK",
		},
		SameProcessAsParentSpan: &wrappers.BoolValue{Value: false},
		Tracestate: &tracepb.Span_Tracestate{
			Entries: []*tracepb.Span_Tracestate_Entry{
				{Key: "foo", Value: "bar"},
				{Key: "a", Value: "b"},
			},
		},
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: spanAttributes,
		},
	}
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
