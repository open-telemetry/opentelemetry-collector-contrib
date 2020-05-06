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
	"testing"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformEmptySpan(t *testing.T) {
	transform := new(transformer)
	_, err := transform.Span(nil)
	assert.Error(t, err)
}

func TestTransformSpan(t *testing.T) {
	now := time.Unix(100, 0)
	tests := []struct {
		in   *tracepb.Span
		want telemetry.Span
	}{
		{
			in: &tracepb.Span{
				Status: &tracepb.Status{},
			},
			want: telemetry.Span{
				ID:          "0000000000000000",
				TraceID:     "00000000000000000000000000000000",
				ServiceName: "test-service",
				Attributes: map[string]interface{}{
					"collector.name":    name,
					"collector.version": version,
					"resource":          "R1",
				},
			},
		},
		{
			in: &tracepb.Span{
				TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:    &tracepb.TruncatableString{Value: "root"},
				Status:  &tracepb.Status{},
			},
			want: telemetry.Span{
				ID:          "0000000000000001",
				TraceID:     "01010101010101010101010101010101",
				Name:        "root",
				ServiceName: "test-service",
				Attributes: map[string]interface{}{
					"collector.name":    name,
					"collector.version": version,
					"resource":          "R1",
				},
			},
		},
		{
			in: &tracepb.Span{
				TraceId:      []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:       []byte{0, 0, 0, 0, 0, 0, 0, 2},
				ParentSpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:         &tracepb.TruncatableString{Value: "client"},
				Status:       &tracepb.Status{},
			},
			want: telemetry.Span{
				ID:          "0000000000000002",
				TraceID:     "01010101010101010101010101010101",
				Name:        "client",
				ParentID:    "0000000000000001",
				ServiceName: "test-service",
				Attributes: map[string]interface{}{
					"collector.name":    name,
					"collector.version": version,
					"resource":          "R1",
				},
			},
		},
		{
			in: &tracepb.Span{
				TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 3},
				Name:    &tracepb.TruncatableString{Value: "error"},
				Status:  &tracepb.Status{Code: 1},
			},
			want: telemetry.Span{
				ID:          "0000000000000003",
				TraceID:     "01010101010101010101010101010101",
				Name:        "error",
				ServiceName: "test-service",
				Attributes: map[string]interface{}{
					"collector.name":    name,
					"collector.version": version,
					"error":             true,
					"resource":          "R1",
				},
			},
		},
		{
			in: &tracepb.Span{
				TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 4},
				Name:    &tracepb.TruncatableString{Value: "attrs"},
				Status:  &tracepb.Status{},
				Attributes: &tracepb.Span_Attributes{
					AttributeMap: map[string]*tracepb.AttributeValue{
						"empty": nil,
						"prod": &tracepb.AttributeValue{
							Value: &tracepb.AttributeValue_BoolValue{true},
						},
						"weight": &tracepb.AttributeValue{
							Value: &tracepb.AttributeValue_IntValue{10},
						},
						"score": &tracepb.AttributeValue{
							Value: &tracepb.AttributeValue_DoubleValue{99.8},
						},
						"user": &tracepb.AttributeValue{
							Value: &tracepb.AttributeValue_StringValue{
								&tracepb.TruncatableString{Value: "alice"},
							},
						},
					},
				},
			},
			want: telemetry.Span{
				ID:          "0000000000000004",
				TraceID:     "01010101010101010101010101010101",
				Name:        "attrs",
				ServiceName: "test-service",
				Attributes: map[string]interface{}{
					"collector.name":    name,
					"collector.version": version,
					"resource":          "R1",
					"prod":              true,
					"weight":            int64(10),
					"score":             float64(99.8),
					"user":              "alice",
				},
			},
		},
		{
			in: &tracepb.Span{
				TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 5},
				Name:    &tracepb.TruncatableString{Value: "with time"},
				Status:  &tracepb.Status{},
				StartTime: &timestamp.Timestamp{
					Seconds: now.Unix(),
				},
				EndTime: &timestamp.Timestamp{
					Seconds: now.Add(time.Second * 5).Unix(),
				},
			},
			want: telemetry.Span{
				ID:          "0000000000000005",
				TraceID:     "01010101010101010101010101010101",
				Name:        "with time",
				Timestamp:   now,
				Duration:    time.Second * 5,
				ServiceName: "test-service",
				Attributes: map[string]interface{}{
					"collector.name":    name,
					"collector.version": version,
					"resource":          "R1",
				},
			},
		},
	}

	transform := &transformer{
		ServiceName: "test-service",
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
	}
	for _, test := range tests {
		got, err := transform.Span(test.in)
		require.NoError(t, err)
		assert.Equal(t, test.want, got)
	}
}
