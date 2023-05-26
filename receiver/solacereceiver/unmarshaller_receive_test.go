// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	receive_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/model/receive/v1"
)

func TestReceiveUnmarshallerMapResourceSpan(t *testing.T) {
	var (
		routerName = "someRouterName"
		vpnName    = "someVpnName"
		version    = "10.0.0"
	)
	tests := []struct {
		name                        string
		spanData                    *receive_v1.SpanData
		want                        map[string]interface{}
		expectedUnmarshallingErrors interface{}
	}{
		{
			name: "Maps All Fields When Present",
			spanData: &receive_v1.SpanData{
				RouterName:     routerName,
				MessageVpnName: &vpnName,
				SolosVersion:   version,
			},
			want: map[string]interface{}{
				"service.name":        routerName,
				"service.instance.id": vpnName,
				"service.version":     version,
			},
		},
		{
			name:     "Does Not Map Fields When Not Present",
			spanData: &receive_v1.SpanData{},
			want: map[string]interface{}{
				"service.version": "",
				"service.name":    "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestReceiveV1Unmarshaller(t)
			actual := pcommon.NewMap()
			u.mapResourceSpanAttributes(tt.spanData, actual)
			assert.Equal(t, tt.want, actual.AsRaw())
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

// Tests the received span to traces mappings
// Includes all required opentelemetry fields such as trace ID, span ID, etc.
func TestReceiveUnmarshallerMapClientSpanData(t *testing.T) {
	someTraceState := "some trace status"
	tests := []struct {
		name string
		data *receive_v1.SpanData
		want func(ptrace.Span)
	}{
		// no trace state no status no parent span
		{
			name: "Without Optional Fields",
			data: &receive_v1.SpanData{
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
				span.SetKind(5)
				span.SetName("(topic) receive")
				span.Status().SetCode(ptrace.StatusCodeUnset)
			},
		},
		// trace state status and parent span
		{
			name: "With Optional Fields",
			data: &receive_v1.SpanData{
				TraceId:           []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
				SpanId:            []byte{7, 6, 5, 4, 3, 2, 1, 0},
				StartTimeUnixNano: 1234567890,
				EndTimeUnixNano:   2234567890,
				ParentSpanId:      []byte{15, 14, 13, 12, 11, 10, 9, 8},
				TraceState:        &someTraceState,
				ErrorDescription:  "some error",
			},
			want: func(span ptrace.Span) {
				span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
				span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
				span.SetStartTimestamp(1234567890)
				span.SetEndTimestamp(2234567890)
				span.SetParentSpanID([8]byte{15, 14, 13, 12, 11, 10, 9, 8})
				span.TraceState().FromRaw(someTraceState)
				span.Status().SetCode(ptrace.StatusCodeError)
				span.Status().SetMessage("some error")
				// expect some constants
				span.SetKind(5)
				span.SetName("(topic) receive")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestReceiveV1Unmarshaller(t)
			actual := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			u.mapClientSpanData(tt.data, actual)
			expected := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			tt.want(expected)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestReceiveUnmarshallerMapClientSpanAttributes(t *testing.T) {
	var (
		protocolVersion      = "5.0"
		applicationMessageID = "someMessageID"
		correlationID        = "someConversationID"
		replyToTopic         = "someReplyToTopic"
		baggageString        = `someKey=someVal;someProp=someOtherThing,someOtherKey=someOtherVal;someProp=NewProp123;someOtherProp=AnotherProp192`
		invalidBaggageString = `someKey"=someVal;someProp=someOtherThing`
		priority             = uint32(1)
		ttl                  = int64(86000)
	)

	tests := []struct {
		name                        string
		spanData                    *receive_v1.SpanData
		want                        map[string]interface{}
		expectedUnmarshallingErrors interface{}
	}{
		{
			name: "With All Valid Attributes",
			spanData: &receive_v1.SpanData{
				Protocol:                            "MQTT",
				ProtocolVersion:                     &protocolVersion,
				ApplicationMessageId:                &applicationMessageID,
				CorrelationId:                       &correlationID,
				BinaryAttachmentSize:                1000,
				XmlAttachmentSize:                   200,
				MetadataSize:                        34,
				ClientUsername:                      "someClientUsername",
				ClientName:                          "someClient1234",
				ReplyToTopic:                        &replyToTopic,
				DeliveryMode:                        receive_v1.SpanData_PERSISTENT,
				Topic:                               "someTopic",
				ReplicationGroupMessageId:           []byte{0x01, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
				Priority:                            &priority,
				Ttl:                                 &ttl,
				DmqEligible:                         true,
				DroppedEnqueueEventsSuccess:         42,
				DroppedEnqueueEventsFailed:          24,
				HostIp:                              []byte{1, 2, 3, 4},
				HostPort:                            55555,
				PeerIp:                              []byte{35, 69, 4, 37, 44, 161, 0, 0, 0, 0, 5, 103, 86, 115, 35, 181},
				PeerPort:                            12345,
				BrokerReceiveTimeUnixNano:           1357924680,
				DroppedApplicationMessageProperties: false,
				Baggage:                             &baggageString,
				UserProperties: map[string]*receive_v1.SpanData_UserPropertyValue{
					"special_key": {
						Value: &receive_v1.SpanData_UserPropertyValue_BoolValue{
							BoolValue: true,
						},
					},
				},
			},
			want: map[string]interface{}{
				"messaging.system":                                        "SolacePubSub+",
				"messaging.operation":                                     "receive",
				"messaging.protocol":                                      "MQTT",
				"messaging.protocol_version":                              "5.0",
				"messaging.message_id":                                    "someMessageID",
				"messaging.conversation_id":                               "someConversationID",
				"messaging.message_payload_size_bytes":                    int64(1234),
				"messaging.destination":                                   "someTopic",
				"messaging.solace.client_username":                        "someClientUsername",
				"messaging.solace.client_name":                            "someClient1234",
				"messaging.solace.replication_group_message_id":           "rmid1:00010-40910192431-40516479-90a9c4e1",
				"messaging.solace.priority":                               int64(1),
				"messaging.solace.ttl":                                    int64(86000),
				"messaging.solace.dmq_eligible":                           true,
				"messaging.solace.dropped_enqueue_events_success":         int64(42),
				"messaging.solace.dropped_enqueue_events_failed":          int64(24),
				"messaging.solace.reply_to_topic":                         "someReplyToTopic",
				"messaging.solace.delivery_mode":                          "persistent",
				"net.host.ip":                                             "1.2.3.4",
				"net.host.port":                                           int64(55555),
				"net.peer.ip":                                             "2345:425:2ca1::567:5673:23b5",
				"net.peer.port":                                           int64(12345),
				"messaging.solace.user_properties.special_key":            true,
				"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
				"messaging.solace.dropped_application_message_properties": false,
				"messaging.solace.message.baggage.someKey":                "someVal",
				"messaging.solace.message.baggage_metadata.someKey":       "someProp=someOtherThing",
				"messaging.solace.message.baggage.someOtherKey":           `someOtherVal`,
				"messaging.solace.message.baggage_metadata.someOtherKey":  "someProp=NewProp123;someOtherProp=AnotherProp192",
			},
		},
		{
			name: "With Only Required Fields",
			spanData: &receive_v1.SpanData{
				Protocol:                            "MQTT",
				BinaryAttachmentSize:                1000,
				XmlAttachmentSize:                   200,
				MetadataSize:                        34,
				ClientUsername:                      "someClientUsername",
				ClientName:                          "someClient1234",
				Topic:                               "someTopic",
				DeliveryMode:                        receive_v1.SpanData_NON_PERSISTENT,
				DmqEligible:                         true,
				DroppedEnqueueEventsSuccess:         42,
				DroppedEnqueueEventsFailed:          24,
				HostIp:                              []byte{1, 2, 3, 4},
				HostPort:                            55555,
				PeerIp:                              []byte{35, 69, 4, 37, 44, 161, 0, 0, 0, 0, 5, 103, 86, 115, 35, 181},
				PeerPort:                            12345,
				BrokerReceiveTimeUnixNano:           1357924680,
				DroppedApplicationMessageProperties: true,
				UserProperties: map[string]*receive_v1.SpanData_UserPropertyValue{
					"special_key": nil,
				},
			},
			want: map[string]interface{}{
				"messaging.system":                                        "SolacePubSub+",
				"messaging.operation":                                     "receive",
				"messaging.protocol":                                      "MQTT",
				"messaging.message_payload_size_bytes":                    int64(1234),
				"messaging.destination":                                   "someTopic",
				"messaging.solace.client_username":                        "someClientUsername",
				"messaging.solace.client_name":                            "someClient1234",
				"messaging.solace.dmq_eligible":                           true,
				"messaging.solace.delivery_mode":                          "non_persistent",
				"messaging.solace.dropped_enqueue_events_success":         int64(42),
				"messaging.solace.dropped_enqueue_events_failed":          int64(24),
				"net.host.ip":                                             "1.2.3.4",
				"net.host.port":                                           int64(55555),
				"net.peer.ip":                                             "2345:425:2ca1::567:5673:23b5",
				"net.peer.port":                                           int64(12345),
				"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
				"messaging.solace.dropped_application_message_properties": true,
			},
		},
		{
			name: "With Some Invalid Fields",
			spanData: &receive_v1.SpanData{
				Protocol:                            "MQTT",
				BinaryAttachmentSize:                1000,
				XmlAttachmentSize:                   200,
				MetadataSize:                        34,
				ClientUsername:                      "someClientUsername",
				ClientName:                          "someClient1234",
				Topic:                               "someTopic",
				DeliveryMode:                        receive_v1.SpanData_DeliveryMode(1000),
				DmqEligible:                         true,
				DroppedEnqueueEventsSuccess:         42,
				DroppedEnqueueEventsFailed:          24,
				HostPort:                            55555,
				PeerPort:                            12345,
				BrokerReceiveTimeUnixNano:           1357924680,
				DroppedApplicationMessageProperties: true,
				Baggage:                             &invalidBaggageString,
				UserProperties: map[string]*receive_v1.SpanData_UserPropertyValue{
					"special_key": nil,
				},
			},
			// we no longer expect the port when the IP is not present
			want: map[string]interface{}{
				"messaging.system":                                        "SolacePubSub+",
				"messaging.operation":                                     "receive",
				"messaging.protocol":                                      "MQTT",
				"messaging.message_payload_size_bytes":                    int64(1234),
				"messaging.destination":                                   "someTopic",
				"messaging.solace.client_username":                        "someClientUsername",
				"messaging.solace.client_name":                            "someClient1234",
				"messaging.solace.dmq_eligible":                           true,
				"messaging.solace.delivery_mode":                          "Unknown Delivery Mode (1000)",
				"messaging.solace.dropped_enqueue_events_success":         int64(42),
				"messaging.solace.dropped_enqueue_events_failed":          int64(24),
				"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
				"messaging.solace.dropped_application_message_properties": true,
			},
			// Invalid delivery mode, missing IPs, invalid baggage string
			expectedUnmarshallingErrors: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestReceiveV1Unmarshaller(t)
			actual := pcommon.NewMap()
			u.mapClientSpanAttributes(tt.spanData, actual)
			assert.Equal(t, tt.want, actual.AsRaw())
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

// Validate that all event types are properly handled and appended into the span data
func TestReceiveUnmarshallerEvents(t *testing.T) {
	someErrorString := "some error"
	somePartitionNumber := uint32(345)
	tests := []struct {
		name                 string
		spanData             *receive_v1.SpanData
		populateExpectedSpan func(span ptrace.Span)
		unmarshallingErrors  interface{}
	}{
		{ // don't expect any events when none are present in the span data
			name:                 "No Events",
			spanData:             &receive_v1.SpanData{},
			populateExpectedSpan: func(span ptrace.Span) {},
		},
		{ // when an enqueue event is present, expect it to be added to the span events
			name: "Enqueue Event Queue",
			spanData: &receive_v1.SpanData{
				EnqueueEvents: []*receive_v1.SpanData_EnqueueEvent{
					{
						Dest:            &receive_v1.SpanData_EnqueueEvent_QueueName{QueueName: "somequeue"},
						TimeUnixNano:    123456789,
						PartitionNumber: &somePartitionNumber,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
					"messaging.solace.partition_number":     345,
				})
			},
		},
		{ // when a topic endpoint enqueue event is present, expect it to be added to the span events
			name: "Enqueue Event Topic Endpoint",
			spanData: &receive_v1.SpanData{
				EnqueueEvents: []*receive_v1.SpanData_EnqueueEvent{
					{
						Dest:               &receive_v1.SpanData_EnqueueEvent_TopicEndpointName{TopicEndpointName: "sometopic"},
						TimeUnixNano:       123456789,
						ErrorDescription:   &someErrorString,
						RejectsAllEnqueues: true,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "sometopic enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":      "topic-endpoint",
					"messaging.solace.enqueue_error_message": someErrorString,
					"messaging.solace.rejects_all_enqueues":  true,
				})
			},
		},
		{ // when a both a queue and topic endpoint enqueue event is present, expect it to be added to the span events
			name: "Enqueue Event Queue and Topic Endpoint",
			spanData: &receive_v1.SpanData{
				EnqueueEvents: []*receive_v1.SpanData_EnqueueEvent{
					{
						Dest:         &receive_v1.SpanData_EnqueueEvent_QueueName{QueueName: "somequeue"},
						TimeUnixNano: 123456789,
					},
					{
						Dest:         &receive_v1.SpanData_EnqueueEvent_TopicEndpointName{TopicEndpointName: "sometopic"},
						TimeUnixNano: 2345678,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "sometopic enqueue", 2345678, map[string]interface{}{
					"messaging.solace.destination_type":     "topic-endpoint",
					"messaging.solace.rejects_all_enqueues": false,
				})
			},
		},
		{ // when an enqueue event does not have a valid dest (ie. nil)
			name: "Enqueue Event no Dest",
			spanData: &receive_v1.SpanData{
				EnqueueEvents: []*receive_v1.SpanData_EnqueueEvent{
					{
						Dest:         nil,
						TimeUnixNano: 123456789,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {},
			unmarshallingErrors:  1,
		},
		{ // Local Transaction
			name: "Local Transaction Event",
			spanData: &receive_v1.SpanData{
				TransactionEvent: &receive_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         receive_v1.SpanData_TransactionEvent_COMMIT,
					Initiator:    receive_v1.SpanData_TransactionEvent_CLIENT,
					TransactionId: &receive_v1.SpanData_TransactionEvent_LocalId{
						LocalId: &receive_v1.SpanData_TransactionEvent_LocalTransactionId{
							TransactionId: 12345,
							SessionId:     67890,
							SessionName:   "my-session-name",
						},
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "commit", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
			},
		},
		{ // XA transaction
			name: "XA Transaction Event",
			spanData: &receive_v1.SpanData{
				TransactionEvent: &receive_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         receive_v1.SpanData_TransactionEvent_END,
					Initiator:    receive_v1.SpanData_TransactionEvent_ADMIN,
					TransactionId: &receive_v1.SpanData_TransactionEvent_Xid_{
						Xid: &receive_v1.SpanData_TransactionEvent_Xid{
							FormatId:        123,
							BranchQualifier: []byte{0, 8, 20, 254},
							GlobalId:        []byte{128, 64, 32, 16, 8, 4, 2, 1, 0},
						},
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "end", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator": "administrator",
					"messaging.solace.transaction_xid":       "0000007b-000814fe-804020100804020100",
				})
			},
		},
		{ // XA Transaction with no branch qualifier or global ID and with an error
			name: "XA Transaction Event with nil fields and error",
			spanData: &receive_v1.SpanData{
				TransactionEvent: &receive_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         receive_v1.SpanData_TransactionEvent_PREPARE,
					Initiator:    receive_v1.SpanData_TransactionEvent_BROKER,
					TransactionId: &receive_v1.SpanData_TransactionEvent_Xid_{
						Xid: &receive_v1.SpanData_TransactionEvent_Xid{
							FormatId:        123,
							BranchQualifier: nil,
							GlobalId:        nil,
						},
					},
					ErrorDescription: &someErrorString,
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "prepare", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":     "broker",
					"messaging.solace.transaction_xid":           "0000007b--",
					"messaging.solace.transaction_error_message": someErrorString,
				})
			},
		},
		{ // Type of transaction not handled
			name: "Unknown Transaction Type and no ID",
			spanData: &receive_v1.SpanData{
				TransactionEvent: &receive_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         receive_v1.SpanData_TransactionEvent_Type(12345),
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "Unknown Transaction Event (12345)", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator": "client",
				})
			},
			unmarshallingErrors: 2,
		},
		{ // Type of ID not handled, type of initiator not handled
			name: "Unknown Transaction Initiator and no ID",
			spanData: &receive_v1.SpanData{
				TransactionEvent: &receive_v1.SpanData_TransactionEvent{
					TimeUnixNano:  123456789,
					Type:          receive_v1.SpanData_TransactionEvent_ROLLBACK,
					Initiator:     receive_v1.SpanData_TransactionEvent_Initiator(12345),
					TransactionId: nil,
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "rollback", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator": "Unknown Transaction Initiator (12345)",
				})
			},
			unmarshallingErrors: 2,
		},
		{ // when a both a queue and topic endpoint enqueue event is present, expect it to be added to the span events
			name: "Multiple Events",
			spanData: &receive_v1.SpanData{
				EnqueueEvents: []*receive_v1.SpanData_EnqueueEvent{
					{
						Dest:         &receive_v1.SpanData_EnqueueEvent_QueueName{QueueName: "somequeue"},
						TimeUnixNano: 123456789,
					},
					{
						Dest:               &receive_v1.SpanData_EnqueueEvent_TopicEndpointName{TopicEndpointName: "sometopic"},
						TimeUnixNano:       2345678,
						RejectsAllEnqueues: true,
					},
				},
				TransactionEvent: &receive_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         receive_v1.SpanData_TransactionEvent_ROLLBACK_ONLY,
					Initiator:    receive_v1.SpanData_TransactionEvent_CLIENT,
					TransactionId: &receive_v1.SpanData_TransactionEvent_LocalId{
						LocalId: &receive_v1.SpanData_TransactionEvent_LocalTransactionId{
							TransactionId: 12345,
							SessionId:     67890,
							SessionName:   "my-session-name",
						},
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "sometopic enqueue", 2345678, map[string]interface{}{
					"messaging.solace.destination_type":     "topic-endpoint",
					"messaging.solace.rejects_all_enqueues": true,
				})
				populateEvent(t, span, "rollback_only", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestReceiveV1Unmarshaller(t)
			expected := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			tt.populateExpectedSpan(expected)
			actual := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			u.mapEvents(tt.spanData, actual)
			// order is nondeterministic for attributes, so we must sort to get a valid comparison
			compareSpans(t, expected, actual)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.unmarshallingErrors)
		})
	}
}

func TestReceiveUnmarshallerRGMID(t *testing.T) {
	tests := []struct {
		name     string
		in       []byte
		expected string
		numErr   interface{}
	}{
		{
			name:     "Valid RGMID",
			in:       []byte{0x01, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
			expected: "rmid1:00010-40910192431-40516479-90a9c4e1",
		},
		{
			name:     "Bad RGMID Version",
			in:       []byte{0x02, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
			expected: "0200010409101924314051647990a9c4e1", // expect default behavior of hex dump
			numErr:   1,
		},
		{
			name:     "Bad RGMID length",
			in:       []byte{0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
			expected: "00010409101924314051647990a9c4e1", // expect default behavior of hex dump
			numErr:   1,
		},
		{
			name:     "Nil RGMID",
			in:       nil,
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestReceiveV1Unmarshaller(t)
			actual := u.rgmidToString(tt.in)
			assert.Equal(t, tt.expected, actual)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.numErr)
		})
	}
}

func TestReceiveUnmarshallerReceiveBaggageString(t *testing.T) {
	testCases := []struct {
		name     string
		baggage  string
		expected func(pcommon.Map)
		errStr   string
	}{
		{
			name:    "Valid baggage",
			baggage: `someKey=someVal`,
			expected: func(m pcommon.Map) {
				assert.NoError(t, m.FromRaw(map[string]interface{}{
					"messaging.solace.message.baggage.someKey": "someVal",
				}))
			},
		},
		{
			name:    "Valid baggage with properties",
			baggage: `someKey=someVal;someProp=someOtherThing,someOtherKey=someOtherVal;someProp=NewProp123;someOtherProp=AnotherProp192`,
			expected: func(m pcommon.Map) {
				assert.NoError(t, m.FromRaw(map[string]interface{}{
					"messaging.solace.message.baggage.someKey":               "someVal",
					"messaging.solace.message.baggage_metadata.someKey":      "someProp=someOtherThing",
					"messaging.solace.message.baggage.someOtherKey":          `someOtherVal`,
					"messaging.solace.message.baggage_metadata.someOtherKey": "someProp=NewProp123;someOtherProp=AnotherProp192",
				}))
			},
		},
		{
			name:    "Invalid baggage",
			baggage: `someKey"=someVal;someProp=someOtherThing`,
			errStr:  "invalid key",
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%T", testCase.name), func(t *testing.T) {
			actual := pcommon.NewMap()
			u := newTestReceiveV1Unmarshaller(t)
			err := u.unmarshalBaggage(actual, testCase.baggage)
			if testCase.errStr == "" {
				assert.Nil(t, err)
			} else {
				assert.ErrorContains(t, err, testCase.errStr)
			}
			if testCase.expected != nil {
				expected := pcommon.NewMap()
				testCase.expected(expected)
				assert.Equal(t, expected.AsRaw(), actual.AsRaw())
			} else {
				// assert we didn't add anything if we don't have a result map
				assert.Equal(t, 0, actual.Len())
			}
		})
	}
}

func TestReceiveUnmarshallerInsertUserProperty(t *testing.T) {
	emojiVal := 0xf09f92a9
	testCases := []struct {
		data         interface{}
		expectedType pcommon.ValueType
		validate     func(val pcommon.Value)
	}{
		{
			&receive_v1.SpanData_UserPropertyValue_NullValue{},
			pcommon.ValueTypeEmpty,
			nil,
		},
		{
			&receive_v1.SpanData_UserPropertyValue_BoolValue{BoolValue: true},
			pcommon.ValueTypeBool,
			func(val pcommon.Value) {
				assert.Equal(t, true, val.Bool())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_DoubleValue{DoubleValue: 12.34},
			pcommon.ValueTypeDouble,
			func(val pcommon.Value) {
				assert.Equal(t, float64(12.34), val.Double())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_ByteArrayValue{ByteArrayValue: []byte{1, 2, 3, 4}},
			pcommon.ValueTypeBytes,
			func(val pcommon.Value) {
				assert.Equal(t, []byte{1, 2, 3, 4}, val.Bytes().AsRaw())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_FloatValue{FloatValue: 12.34},
			pcommon.ValueTypeDouble,
			func(val pcommon.Value) {
				assert.Equal(t, float64(float32(12.34)), val.Double())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Int8Value{Int8Value: 8},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(8), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Int16Value{Int16Value: 16},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(16), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Int32Value{Int32Value: 32},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(32), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Int64Value{Int64Value: 64},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(64), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Uint8Value{Uint8Value: 8},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(8), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Uint16Value{Uint16Value: 16},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(16), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Uint32Value{Uint32Value: 32},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(32), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_Uint64Value{Uint64Value: 64},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(64), val.Int())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_StringValue{StringValue: "hello world"},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, "hello world", val.Str())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_DestinationValue{DestinationValue: "some_dest"},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, "some_dest", val.Str())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_CharacterValue{CharacterValue: 0x61},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, "a", val.Str())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_CharacterValue{CharacterValue: 0xe68080},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, string(rune(0xe68080)), val.Str())
			},
		},
		{
			&receive_v1.SpanData_UserPropertyValue_CharacterValue{CharacterValue: 0xf09f92a9},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, string(rune(emojiVal)), val.Str())
			},
		},
	}

	unmarshaller := &brokerTraceReceiveUnmarshallerV1{
		logger: zap.NewNop(),
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%T", testCase.data), func(t *testing.T) {
			const key = "some-property"
			attributeMap := pcommon.NewMap()
			unmarshaller.insertUserProperty(attributeMap, key, testCase.data)
			actual, ok := attributeMap.Get("messaging.solace.user_properties." + key)
			require.True(t, ok)
			assert.Equal(t, testCase.expectedType, actual.Type())
			if testCase.validate != nil {
				testCase.validate(actual)
			}
		})
	}
}

func TestSolaceMessageReceiveUnmarshallerV1InsertUserPropertyUnsupportedType(t *testing.T) {
	u := newTestReceiveV1Unmarshaller(t)
	const key = "some-property"
	attributeMap := pcommon.NewMap()
	u.insertUserProperty(attributeMap, key, "invalid data type")
	_, ok := attributeMap.Get("messaging.solace.user_properties." + key)
	assert.False(t, ok)
	validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, 1)
}

func newTestReceiveV1Unmarshaller(t *testing.T) *brokerTraceReceiveUnmarshallerV1 {
	m := newTestMetrics(t)
	return &brokerTraceReceiveUnmarshallerV1{zap.NewNop(), m}
}
