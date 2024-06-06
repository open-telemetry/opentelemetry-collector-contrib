// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	move_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/model/move/v1"
)

func TestMoveUnmarshallerMapResourceSpan(t *testing.T) {
	tests := []struct {
		name                        string
		spanData                    *move_v1.SpanData
		want                        map[string]any
		expectedUnmarshallingErrors any
	}{
		{
			name:     "Map to generated field values when not nresent",
			spanData: &move_v1.SpanData{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestMoveV1Unmarshaller(t)
			actual := pcommon.NewMap()
			u.mapResourceSpanAttributes(actual)
			version, _ := actual.Get("service.version") // make sure we generated a uuid version
			assert.NotEmpty(t, version)
			serviceName, _ := actual.Get("service.name") // make sure we are generating a uuid name
			assert.NotEmpty(t, serviceName)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

// Tests the received span to traces mappings
// Includes all required opentelemetry fields such as trace ID, span ID, etc.
func TestMoveUnmarshallerMapClientSpanData(t *testing.T) {
	tests := []struct {
		name string
		data *move_v1.SpanData
		want func(ptrace.Span)
	}{
		// no parent span Id and source/destination fields
		{
			name: "Without Optional Fields",
			data: &move_v1.SpanData{
				TraceId:           []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
				SpanId:            []byte{7, 6, 5, 4, 3, 2, 1, 0},
				StartTimeUnixNano: 1234567890,
				EndTimeUnixNano:   2234567890,
			},
			want: func(span ptrace.Span) {
				span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
				span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
				span.SetStartTimestamp(1234567890)
				span.SetEndTimestamp(2234567890)
				// expect some constants
				span.SetKind(ptrace.SpanKindInternal)
				span.Status().SetCode(ptrace.StatusCodeUnset)
			},
		},
		// parent span Id and no source/destination fields
		{
			name: "With Optional Fields",
			data: &move_v1.SpanData{
				TraceId:           []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
				SpanId:            []byte{7, 6, 5, 4, 3, 2, 1, 0},
				StartTimeUnixNano: 1234567890,
				EndTimeUnixNano:   2234567890,
				ParentSpanId:      []byte{15, 14, 13, 12, 11, 10, 9, 8},
			},
			want: func(span ptrace.Span) {
				span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
				span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
				span.SetStartTimestamp(1234567890)
				span.SetEndTimestamp(2234567890)
				span.SetParentSpanID([8]byte{15, 14, 13, 12, 11, 10, 9, 8})
				// expect some constants
				span.SetKind(ptrace.SpanKindInternal)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestMoveV1Unmarshaller(t)
			actual := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			u.mapMoveSpanTracingInfo(tt.data, actual) // map the tracing information
			// u.mapClientSpanData(tt.data, actual)      // other span attributes
			expected := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			tt.want(expected)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestMoveUnmarshallerMapClientSpanAttributes(t *testing.T) {
	getSpan := func(attributes map[string]any, name string) ptrace.Span {
		base := map[string]any{
			"messaging.system":    "SolacePubSub+",
			"messaging.operation": "move",
		}
		for key, val := range attributes {
			base[key] = val
		}
		span := ptrace.NewSpan()
		err := span.Attributes().FromRaw(base)
		assert.NoError(t, err)
		span.SetName(name)
		span.SetKind(ptrace.SpanKindInternal)
		return span
	}
	// sets the common fields from getAttributes
	getMoveSpan := func(base *move_v1.SpanData) *move_v1.SpanData {
		// just return the base back
		return base
	}

	tests := []struct {
		name                        string
		spanData                    *move_v1.SpanData
		want                        ptrace.Span
		expectedUnmarshallingErrors any
	}{
		{
			name: "With source and destination Queue endpoints",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceQueueName{
					SourceQueueName: "sourceQueue",
				},
				Destination: &move_v1.SpanData_DestinationQueueName{
					DestinationQueueName: "destQueue",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.system":                  "SolacePubSub+",
				"messaging.operation":               "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.kind": "queue",
			}, "(queue: \"sourceQueue\") move"),
			expectedUnmarshallingErrors: 1, // for the TypeInfo validation
		},
		{
			name: "With source and destination Topic endpoints",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceTopicEndpointName{
					SourceTopicEndpointName: "0123456789abcdef0123456789abcdeg",
				},
				Destination: &move_v1.SpanData_DestinationTopicEndpointName{
					DestinationTopicEndpointName: "2123456789abcdef0123456789abcdeg",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.system":                  "SolacePubSub+",
				"messaging.operation":               "move",
				"messaging.source.name":             "0123456789abcdef0123456789abcdeg",
				"messaging.solace.source.kind":      "topic-endpoint",
				"messaging.destination.name":        "2123456789abcdef0123456789abcdeg",
				"messaging.solace.destination.kind": "topic-endpoint",
			}, "(topic: \"0123456789abcdef0123456789abcdeg\") move"),
			expectedUnmarshallingErrors: 1, // for the TypeInfo validation
		},

		{
			name: "With Anonymous Source Queue Endpoint",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceQueueName{
					SourceQueueName: "#P2P/QTMP/myQueue",
				},
				Destination: &move_v1.SpanData_DestinationQueueName{
					DestinationQueueName: "destQueue",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.system":                  "SolacePubSub+",
				"messaging.operation":               "move",
				"messaging.source.name":             "#P2P/QTMP/myQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.kind": "queue",
			}, "(queue: \"(anonymous)\") move"),
			expectedUnmarshallingErrors: 1, // for the TypeInfo validation
		},
		{
			name: "With Anonymous Source Topic Endpoint",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceTopicEndpointName{
					SourceTopicEndpointName: "0123456789abcdef0123456789abcdef",
				},
				Destination: &move_v1.SpanData_DestinationTopicEndpointName{
					DestinationTopicEndpointName: "2123456789abcdef0123456789ab_dest",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.source.name":             "0123456789abcdef0123456789abcdef",
				"messaging.solace.source.kind":      "topic-endpoint",
				"messaging.destination.name":        "2123456789abcdef0123456789ab_dest",
				"messaging.solace.destination.kind": "topic-endpoint",
			}, "(topic: \"(anonymous)\") move"),
			expectedUnmarshallingErrors: 1, // for the TypeInfo validation
		},
		{
			name: "With Unknown Source Endpoint",
			spanData: getMoveSpan(&move_v1.SpanData{
				Destination: &move_v1.SpanData_DestinationQueueName{
					DestinationQueueName: "destQueue",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.kind": "queue",
			}, "((unknown): \"(unknown)\") move"),
			expectedUnmarshallingErrors: 2, // for destination and TypeInfo field validations
		},
		{
			name: "With Unknown Destination Endpoint",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceTopicEndpointName{
					SourceTopicEndpointName: "0123456789abcdef0123456789abcde_source",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.source.name":        "0123456789abcdef0123456789abcde_source",
				"messaging.solace.source.kind": "topic-endpoint",
			}, "(topic: \"0123456789abcdef0123456789abcde_source\") move"),
			expectedUnmarshallingErrors: 2, // for destination and TypeInfo field validations
		},
		{
			name: "With All Valid Attributes",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceQueueName{
					SourceQueueName: "sourceQueue",
				},
				Destination: &move_v1.SpanData_DestinationQueueName{
					DestinationQueueName: "destQueue",
				},
				TypeInfo: &move_v1.SpanData_TtlExpiredInfo{},
			}),
			want: getSpan(map[string]any{
				"messaging.system":                  "SolacePubSub+",
				"messaging.operation":               "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.kind": "queue",
				"messaging.solace.operation.reason": "ttl_expired",
			}, "(queue: \"sourceQueue\") move"),
		},
		{
			name:                        "With No Valid Attributes",
			spanData:                    getMoveSpan(&move_v1.SpanData{}),
			want:                        getSpan(map[string]any{}, "((unknown): \"(unknown)\") move"),
			expectedUnmarshallingErrors: 3, // for all fields validations
		},

		// Tests for the Move Reason
		{
			name: "With All Valid Attributes and Ttl Expired reason",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceQueueName{
					SourceQueueName: "sourceQueue",
				},
				Destination: &move_v1.SpanData_DestinationQueueName{
					DestinationQueueName: "destQueue",
				},
				TypeInfo: &move_v1.SpanData_TtlExpiredInfo{},
			}),
			want: getSpan(map[string]any{
				"messaging.system":                  "SolacePubSub+",
				"messaging.operation":               "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.kind": "queue",
				"messaging.solace.operation.reason": "ttl_expired",
			}, "(queue: \"sourceQueue\") move"),
		},
		{
			name: "With All Valid Attributes and Rejected Outcome reason",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceQueueName{
					SourceQueueName: "sourceQueue",
				},
				Destination: &move_v1.SpanData_DestinationQueueName{
					DestinationQueueName: "destQueue",
				},
				TypeInfo: &move_v1.SpanData_RejectedOutcomeInfo{},
			}),
			want: getSpan(map[string]any{
				"messaging.system":                  "SolacePubSub+",
				"messaging.operation":               "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.kind": "queue",
				"messaging.solace.operation.reason": "rejected_nack",
			}, "(queue: \"sourceQueue\") move"),
		},
		{
			name: "With All Valid Attributes and Max Redeliveries Exceeded reason",
			spanData: getMoveSpan(&move_v1.SpanData{
				Source: &move_v1.SpanData_SourceQueueName{
					SourceQueueName: "sourceQueue",
				},
				Destination: &move_v1.SpanData_DestinationQueueName{
					DestinationQueueName: "destQueue",
				},
				TypeInfo: &move_v1.SpanData_MaxRedeliveriesInfo{},
			}),
			want: getSpan(map[string]any{
				"messaging.system":                  "SolacePubSub+",
				"messaging.operation":               "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.kind": "queue",
				"messaging.solace.operation.reason": "max_redeliveries_exceeded",
			}, "(queue: \"sourceQueue\") move"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestMoveV1Unmarshaller(t)
			actual := ptrace.NewSpan()
			u.mapClientSpanData(tt.spanData, actual)
			compareSpans(t, tt.want, actual)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

func newTestMoveV1Unmarshaller(t *testing.T) *brokerTraceMoveUnmarshallerV1 {
	m := newTestMetrics(t)
	return &brokerTraceMoveUnmarshallerV1{zap.NewNop(), m}
}
