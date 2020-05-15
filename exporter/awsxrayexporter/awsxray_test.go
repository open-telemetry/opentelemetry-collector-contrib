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
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
)

func TestTraceExport(t *testing.T) {
	traceExporter := initializeTraceExporter()
	ctx := context.Background()
	td := constructSpanData()
	err := traceExporter.ConsumeTraceData(ctx, td)
	assert.Nil(t, err)
}

func initializeTraceExporter() component.TraceExporterOld {
	os.Setenv("AWS_ACCESS_KEY_ID", "AKIASSWVJUY4PZXXXXXX")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "XYrudg2H87u+ADAAq19Wqx3D41a09RsTXXXXXXXX")
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	os.Setenv("AWS_REGION", "us-east-1")
	logger := zap.NewNop()
	factory := Factory{}
	config := factory.CreateDefaultConfig()
	config.(*Config).Region = "us-east-1"
	config.(*Config).LocalMode = true
	mconn := new(mockConn)
	mconn.sn, _ = getDefaultSession(logger)
	traceExporter, err := NewTraceExporter(config, logger, mconn)
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
	labels[semconventions.AttributeServiceName] = "signup_aggregator"
	labels[semconventions.AttributeContainerName] = "signup_aggregator"
	labels[semconventions.AttributeContainerImage] = "otel/signupaggregator"
	labels[semconventions.AttributeContainerTag] = "v1"
	labels[semconventions.AttributeCloudProvider] = "aws"
	labels[semconventions.AttributeCloudAccount] = "999999998"
	labels[semconventions.AttributeCloudRegion] = "us-west-2"
	labels[semconventions.AttributeCloudZone] = "us-west-1b"
	return &resourcepb.Resource{
		Type:   "container",
		Labels: labels,
	}
}

func constructHTTPClientSpan() *tracepb.Span {
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeHTTP
	attributes[semconventions.AttributeHTTPMethod] = "GET"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[semconventions.AttributeHTTPStatusCode] = 200
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      newTraceID(),
		SpanId:       newSegmentID(),
		ParentSpanId: newSegmentID(),
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
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeHTTP
	attributes[semconventions.AttributeHTTPMethod] = "GET"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[semconventions.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[semconventions.AttributeHTTPStatusCode] = 200
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      newTraceID(),
		SpanId:       newSegmentID(),
		ParentSpanId: newSegmentID(),
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

func newTraceID() []byte {
	var r [16]byte
	epoch := time.Now().Unix()
	binary.BigEndian.PutUint32(r[0:4], uint32(epoch))
	_, err := rand.Read(r[4:])
	if err != nil {
		panic(err)
	}
	return r[:]
}

func newSegmentID() []byte {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return r[:]
}
