// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadatatest"
	move_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/model/move/v1"
)

func TestMoveUnmarshallerMapResourceSpan(t *testing.T) {
	tests := []struct {
		name                        string
		spanData                    *move_v1.SpanData
		want                        map[string]any
		expectedUnmarshallingErrors int64
	}{
		{
			name:     "Map to generated field values when not nresent",
			spanData: &move_v1.SpanData{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, tel := newTestMoveV1Unmarshaller(t)
			actual := pcommon.NewMap()
			u.mapResourceSpanAttributes(tt.spanData, actual)
			version, _ := actual.Get("service.version") // make sure we generated a uuid version
			assert.NotEmpty(t, version)
			serviceName, _ := actual.Get("service.name") // make sure we are generating a uuid name
			assert.NotEmpty(t, serviceName)
			if tt.expectedUnmarshallingErrors > 0 {
				metadatatest.AssertEqualSolacereceiverRecoverableUnmarshallingErrors(t, tel, []metricdata.DataPoint[int64]{
					{
						Value: tt.expectedUnmarshallingErrors,
					},
				}, metricdatatest.IgnoreTimestamp())
			}
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
			u, _ := newTestMoveV1Unmarshaller(t)
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
			"messaging.system":         "SolacePubSub+",
			"messaging.operation.name": "move",
			"messaging.operation.type": "move",
		}
		maps.Copy(base, attributes)
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

	// test partition numbers
	someSourcePartitionNumber := uint32(123)
	someDestinationPartitionNumber := uint32(456)

	tests := []struct {
		name                        string
		spanData                    *move_v1.SpanData
		want                        ptrace.Span
		expectedUnmarshallingErrors int64
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
				"messaging.operation.name":          "move",
				"messaging.operation.type":          "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.type": "queue",
			}, "sourceQueue move"),
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
				"messaging.operation.name":          "move",
				"messaging.operation.type":          "move",
				"messaging.source.name":             "0123456789abcdef0123456789abcdeg",
				"messaging.solace.source.kind":      "topic-endpoint",
				"messaging.destination.name":        "2123456789abcdef0123456789abcdeg",
				"messaging.solace.destination.type": "topic-endpoint",
			}, "0123456789abcdef0123456789abcdeg move"),
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
				"messaging.operation.name":          "move",
				"messaging.operation.type":          "move",
				"messaging.source.name":             "#P2P/QTMP/myQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.type": "queue",
			}, "(anonymous) move"),
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
				"messaging.solace.destination.type": "topic-endpoint",
			}, "(anonymous) move"),
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
				"messaging.solace.destination.type": "queue",
			}, "(unknown) move"),
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
			}, "0123456789abcdef0123456789abcde_source move"),
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
				TypeInfo:                   &move_v1.SpanData_TtlExpiredInfo{},
				ReplicationGroupMessageId:  []byte{0x01, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
				SourcePartitionNumber:      &someSourcePartitionNumber,
				DestinationPartitionNumber: &someDestinationPartitionNumber,
			}),
			want: getSpan(map[string]any{
				"messaging.system":                              "SolacePubSub+",
				"messaging.operation.name":                      "move",
				"messaging.operation.type":                      "move",
				"messaging.source.name":                         "sourceQueue",
				"messaging.solace.source.kind":                  "queue",
				"messaging.destination.name":                    "destQueue",
				"messaging.solace.destination.type":             "queue",
				"messaging.solace.operation.reason":             "ttl_expired",
				"messaging.solace.replication_group_message_id": "rmid1:00010-40910192431-40516479-90a9c4e1",
				"messaging.solace.source.partition_number":      123,
				"messaging.solace.destination.partition_number": 456,
			}, "sourceQueue move"),
		},
		{
			name:                        "With No Valid Attributes",
			spanData:                    getMoveSpan(&move_v1.SpanData{}),
			want:                        getSpan(map[string]any{}, "(unknown) move"),
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
				"messaging.operation.name":          "move",
				"messaging.operation.type":          "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.type": "queue",
				"messaging.solace.operation.reason": "ttl_expired",
			}, "sourceQueue move"),
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
				"messaging.operation.name":          "move",
				"messaging.operation.type":          "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.type": "queue",
				"messaging.solace.operation.reason": "rejected_nack",
			}, "sourceQueue move"),
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
				"messaging.operation.name":          "move",
				"messaging.operation.type":          "move",
				"messaging.source.name":             "sourceQueue",
				"messaging.solace.source.kind":      "queue",
				"messaging.destination.name":        "destQueue",
				"messaging.solace.destination.type": "queue",
				"messaging.solace.operation.reason": "max_redeliveries_exceeded",
			}, "sourceQueue move"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, tel := newTestMoveV1Unmarshaller(t)
			actual := ptrace.NewSpan()
			u.mapClientSpanData(tt.spanData, actual)
			compareSpans(t, tt.want, actual)
			if tt.expectedUnmarshallingErrors > 0 {
				metadatatest.AssertEqualSolacereceiverRecoverableUnmarshallingErrors(t, tel, []metricdata.DataPoint[int64]{
					{
						Value:      tt.expectedUnmarshallingErrors,
						Attributes: u.metricAttrs,
					},
				}, metricdatatest.IgnoreTimestamp())
			}
		})
	}
}

func newTestMoveV1Unmarshaller(t *testing.T) (*brokerTraceMoveUnmarshallerV1, *componenttest.Telemetry) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting
	builder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)
	metricAttr := attribute.NewSet(attribute.String("receiver_name", metadata.Type.String()))
	return &brokerTraceMoveUnmarshallerV1{zap.NewNop(), builder, metricAttr}, tel
}
