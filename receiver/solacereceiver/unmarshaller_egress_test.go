// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package solacereceiver

import (
	"fmt"
	"testing"

	"github.com/Azure/go-amqp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	egress_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/model/egress/v1"
)

// msgWithAdditionalSpan is an EgressSpanMsg with one EgressSpan of type 1 (future proofing)
// Used to test that if we receive a oneof that does not support our given trace then we will be able to handle it.
// We must do this with protobuf data since we must ensure that the protobuf library handles this case as expected when unmarshalling.
var msgWithAdditionalSpan = []byte{10, 48, 10, 16, 1, 2, 3, 4, 5, 6, 7, 8, 9, 8, 7, 6, 5, 4, 3, 2, 18, 8, 1, 2, 3, 4, 5, 6, 7, 8, 26, 8, 1, 2, 3, 4, 5, 6, 7, 8, 74, 8, 32, 3, 10, 4, 116, 101, 115, 116, 18, 10, 118, 109, 114, 45, 49, 51, 51, 45, 53, 51, 26, 7, 100, 101, 102, 97, 117, 108, 116}

func TestMsgWithUnknownOneof(t *testing.T) {
	unmarshallerV1 := newTestEgressV1Unmarshaller(t)
	spanData, err := unmarshallerV1.unmarshalToSpanData(amqp.NewMessage(msgWithAdditionalSpan))
	assert.NoError(t, err)
	// expect one egress span
	assert.Len(t, spanData.EgressSpans, 1)
	assert.Nil(t, spanData.EgressSpans[0].GetSendSpan())
}

func TestEgressUnmarshallerMapResourceSpan(t *testing.T) {
	var (
		routerName = "someRouterName"
		vpnName    = "someVpnName"
		version    = "10.0.0"
	)
	tests := []struct {
		name                        string
		spanData                    *egress_v1.SpanData
		want                        map[string]any
		expectedUnmarshallingErrors any
	}{
		{
			name: "Maps All Fields When Present",
			spanData: &egress_v1.SpanData{
				RouterName:     routerName,
				MessageVpnName: &vpnName,
				SolosVersion:   version,
			},
			want: map[string]any{
				"service.name":        routerName,
				"service.instance.id": vpnName,
				"service.version":     version,
			},
		},
		{
			name:     "Does Not Map Fields When Not Present",
			spanData: &egress_v1.SpanData{},
			want: map[string]any{
				"service.version": "",
				"service.name":    "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestEgressV1Unmarshaller(t)
			actual := pcommon.NewMap()
			u.mapResourceSpanAttributes(tt.spanData, actual)
			assert.Equal(t, tt.want, actual.AsRaw())
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

var (
	protocolVersion  = "5.0"
	protocolVersion2 = "1.0"
	protocolVersion3 = "3.0"
	someError        = "someErrorOccurred"
	someOtherError   = "someOtherErrorOccurred"
)

// validEgressSpans is valid data used as test data
var validEgressSpans = []struct {
	in  *egress_v1.SpanData_EgressSpan
	out ptrace.Span
}{
	{
		&egress_v1.SpanData_EgressSpan{
			TraceId:           []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SpanId:            []byte{7, 6, 5, 4, 3, 2, 1, 0},
			StartTimeUnixNano: 234567890,
			EndTimeUnixNano:   234567890,
			TypeData: &egress_v1.SpanData_EgressSpan_SendSpan{
				SendSpan: &egress_v1.SpanData_SendSpan{
					Protocol:               "SMF",
					ProtocolVersion:        &protocolVersion3,
					ConsumerClientUsername: "clientUsername",
					ConsumerClientName:     "clientName",
					Source: &egress_v1.SpanData_SendSpan_QueueName{
						QueueName: "someQueue",
					},
					Outcome:     egress_v1.SpanData_SendSpan_FLOW_UNBOUND,
					ReplayedMsg: false,
				},
			},
		},
		func() ptrace.Span {
			span := ptrace.NewSpan()
			span.SetName("someQueue send")
			span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
			span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
			span.SetStartTimestamp(234567890)
			span.SetEndTimestamp(234567890)
			span.SetKind(4)
			spanAttrs := span.Attributes()
			spanAttrs.PutStr("messaging.system", "SolacePubSub+")
			spanAttrs.PutStr("messaging.operation", "send")
			spanAttrs.PutStr("messaging.protocol", "SMF")
			spanAttrs.PutStr("messaging.protocol_version", "3.0")
			spanAttrs.PutStr("messaging.source.name", "someQueue")
			spanAttrs.PutStr("messaging.source.kind", "queue")
			spanAttrs.PutStr("messaging.solace.client_username", "clientUsername")
			spanAttrs.PutStr("messaging.solace.client_name", "clientName")
			spanAttrs.PutBool("messaging.solace.message_replayed", false)
			spanAttrs.PutStr("messaging.solace.send.outcome", "flow unbound")
			return span
		}(),
	},
	{
		&egress_v1.SpanData_EgressSpan{
			TraceId:           []byte{1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SpanId:            []byte{7, 6, 5, 4, 3, 2, 1, 1},
			StartTimeUnixNano: 1234567890,
			EndTimeUnixNano:   2234567890,
			ErrorDescription:  &someError,
			TypeData: &egress_v1.SpanData_EgressSpan_SendSpan{
				SendSpan: &egress_v1.SpanData_SendSpan{
					Protocol:               "MQTT",
					ProtocolVersion:        &protocolVersion,
					ConsumerClientUsername: "someClientUsername",
					ConsumerClientName:     "someClient1234",
					Source: &egress_v1.SpanData_SendSpan_QueueName{
						QueueName: "queueName",
					},
					Outcome:     egress_v1.SpanData_SendSpan_ACCEPTED,
					ReplayedMsg: false,
				},
			},
			TransactionEvent: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 123456789,
				Type:         egress_v1.SpanData_TransactionEvent_SESSION_TIMEOUT,
				Initiator:    egress_v1.SpanData_TransactionEvent_BROKER,
				TransactionId: &egress_v1.SpanData_TransactionEvent_LocalId{
					LocalId: &egress_v1.SpanData_TransactionEvent_LocalTransactionId{
						TransactionId: 12345,
						SessionId:     67890,
						SessionName:   "my-session-name",
					},
				},
				ErrorDescription: &someOtherError,
			},
		}, func() ptrace.Span {
			span := ptrace.NewSpan()
			span.SetName("queueName send")
			span.SetTraceID([16]byte{1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
			span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 1})
			span.SetStartTimestamp(1234567890)
			span.SetEndTimestamp(2234567890)
			span.SetKind(4)
			span.Status().SetCode(ptrace.StatusCodeError)
			span.Status().SetMessage("someErrorOccurred")
			spanAttrs := span.Attributes()
			spanAttrs.PutStr("messaging.system", "SolacePubSub+")
			spanAttrs.PutStr("messaging.operation", "send")
			spanAttrs.PutStr("messaging.protocol", "MQTT")
			spanAttrs.PutStr("messaging.protocol_version", "5.0")
			spanAttrs.PutStr("messaging.source.name", "queueName")
			spanAttrs.PutStr("messaging.source.kind", "queue")
			spanAttrs.PutStr("messaging.solace.client_username", "someClientUsername")
			spanAttrs.PutStr("messaging.solace.client_name", "someClient1234")
			spanAttrs.PutBool("messaging.solace.message_replayed", false)
			spanAttrs.PutStr("messaging.solace.send.outcome", "accepted")
			txnEvent := span.Events().AppendEmpty()
			txnEvent.SetName("session_timeout")
			txnEvent.SetTimestamp(123456789)
			txnEventAttrs := txnEvent.Attributes()
			txnEventAttrs.PutStr("messaging.solace.transaction_initiator", "broker")
			txnEventAttrs.PutInt("messaging.solace.transaction_id", 12345)
			txnEventAttrs.PutStr("messaging.solace.transacted_session_name", "my-session-name")
			txnEventAttrs.PutInt("messaging.solace.transacted_session_id", 67890)
			txnEventAttrs.PutStr("messaging.solace.transaction_error_message", "someOtherErrorOccurred")
			return span
		}(),
	},
	{
		&egress_v1.SpanData_EgressSpan{
			TraceId:           []byte{1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 31},
			SpanId:            []byte{0, 1, 2, 3, 4, 5, 6, 7},
			ParentSpanId:      []byte{7, 6, 5, 4, 3, 2, 1, 0},
			StartTimeUnixNano: 4234567890,
			EndTimeUnixNano:   5234567890,
			TypeData: &egress_v1.SpanData_EgressSpan_SendSpan{
				SendSpan: &egress_v1.SpanData_SendSpan{
					Protocol:               "AMQP",
					ProtocolVersion:        &protocolVersion2,
					ConsumerClientUsername: "someOtherClientUsername",
					ConsumerClientName:     "someOtherClient1234",
					Source: &egress_v1.SpanData_SendSpan_TopicEndpointName{
						TopicEndpointName: "topicEndpointName",
					},
					Outcome:     egress_v1.SpanData_SendSpan_REJECTED,
					ReplayedMsg: true,
				},
			},
			TransactionEvent: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 223456789,
				Type:         egress_v1.SpanData_TransactionEvent_END,
				Initiator:    egress_v1.SpanData_TransactionEvent_CLIENT,
				TransactionId: &egress_v1.SpanData_TransactionEvent_Xid_{
					Xid: &egress_v1.SpanData_TransactionEvent_Xid{
						FormatId:        123,
						BranchQualifier: []byte{0, 8, 20, 254},
						GlobalId:        []byte{128, 64, 32, 16, 8, 4, 2, 1, 0},
					},
				},
			},
		}, func() ptrace.Span {
			// second send span
			span := ptrace.NewSpan()
			span.SetName("topicEndpointName send")
			span.SetTraceID([16]byte{1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 31})
			span.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
			span.SetParentSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
			span.SetStartTimestamp(4234567890)
			span.SetEndTimestamp(5234567890)
			span.SetKind(4)
			spanAttrs := span.Attributes()
			spanAttrs.PutStr("messaging.system", "SolacePubSub+")
			spanAttrs.PutStr("messaging.operation", "send")
			spanAttrs.PutStr("messaging.protocol", "AMQP")
			spanAttrs.PutStr("messaging.protocol_version", "1.0")
			spanAttrs.PutStr("messaging.source.name", "topicEndpointName")
			spanAttrs.PutStr("messaging.source.kind", "topic-endpoint")
			spanAttrs.PutStr("messaging.solace.client_username", "someOtherClientUsername")
			spanAttrs.PutStr("messaging.solace.client_name", "someOtherClient1234")
			spanAttrs.PutBool("messaging.solace.message_replayed", true)
			spanAttrs.PutStr("messaging.solace.send.outcome", "rejected")
			txnEvent := span.Events().AppendEmpty()
			txnEvent.SetName("end")
			txnEvent.SetTimestamp(223456789)
			txnAttrs := txnEvent.Attributes()
			txnAttrs.PutStr("messaging.solace.transaction_initiator", "client")
			txnAttrs.PutStr("messaging.solace.transaction_xid", "0000007b-000814fe-804020100804020100")
			return span
		}(),
	},
}

func TestEgressUnmarshallerEgressSpan(t *testing.T) {
	type testCase struct {
		name                        string
		spanData                    *egress_v1.SpanData_EgressSpan
		want                        *ptrace.Span
		expectedUnmarshallingErrors any
	}
	tests := []testCase{
		{
			name: "No typed span",
			spanData: &egress_v1.SpanData_EgressSpan{
				TraceId:           []byte{1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 31},
				SpanId:            []byte{0, 1, 2, 3, 4, 5, 6, 7},
				ParentSpanId:      []byte{7, 6, 5, 4, 3, 2, 1, 0},
				StartTimeUnixNano: 4234567890,
				EndTimeUnixNano:   5234567890,
				// no typed span type
			},
		},
	}
	var i = 1
	for _, dataRef := range validEgressSpans {
		name := "valid span " + fmt.Sprint(i)
		i++
		want := dataRef.out
		spanData := dataRef.in
		tests = append(tests, testCase{
			name:     name,
			spanData: spanData,
			want:     &want,
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestEgressV1Unmarshaller(t)
			actual := ptrace.NewSpanSlice()
			u.mapEgressSpan(tt.spanData, actual)
			if tt.want != nil {
				assert.Equal(t, 1, actual.Len())
				compareSpans(t, *tt.want, actual.At(0))
			} else {
				assert.Equal(t, 0, actual.Len())
				validateMetric(t, u.metrics.views.droppedEgressSpans, 1)
			}
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

func TestEgressUnmarshallerSendSpanAttributes(t *testing.T) {
	// creates a base attribute map that additional data can be added to
	// does not include outcome or source. Attributes will override all fields in base
	getSpan := func(attributes map[string]any, name string) ptrace.Span {
		base := map[string]any{
			"messaging.system":                  "SolacePubSub+",
			"messaging.operation":               "send",
			"messaging.protocol":                "MQTT",
			"messaging.protocol_version":        "5.0",
			"messaging.solace.client_username":  "someUser",
			"messaging.solace.client_name":      "someName",
			"messaging.solace.message_replayed": false,
			"messaging.solace.send.outcome":     "accepted",
		}
		for key, val := range attributes {
			base[key] = val
		}
		span := ptrace.NewSpan()
		err := span.Attributes().FromRaw(base)
		assert.NoError(t, err)
		span.SetName(name)
		span.SetKind(ptrace.SpanKindProducer)
		return span
	}
	// sets the common fields from getAttributes
	getSendSpan := func(base *egress_v1.SpanData_SendSpan) *egress_v1.SpanData_SendSpan {
		protocolVersion := "5.0"
		base.Protocol = "MQTT"
		base.ProtocolVersion = &protocolVersion
		base.ConsumerClientUsername = "someUser"
		base.ConsumerClientName = "someName"
		return base
	}
	tests := []struct {
		name                        string
		spanData                    *egress_v1.SpanData_SendSpan
		want                        ptrace.Span
		expectedUnmarshallingErrors any
	}{
		{
			name: "With Queue source",
			spanData: getSendSpan(&egress_v1.SpanData_SendSpan{
				Source: &egress_v1.SpanData_SendSpan_QueueName{
					QueueName: "someQueue",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.source.name": "someQueue",
				"messaging.source.kind": "queue",
			}, "someQueue send"),
		},
		{
			name: "With Topic Endpoint source",
			spanData: getSendSpan(&egress_v1.SpanData_SendSpan{
				Source: &egress_v1.SpanData_SendSpan_TopicEndpointName{
					TopicEndpointName: "0123456789abcdef0123456789abcdeg",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.source.name": "0123456789abcdef0123456789abcdeg",
				"messaging.source.kind": "topic-endpoint",
			}, "0123456789abcdef0123456789abcdeg send"),
		},
		{
			name: "With Anonymous Queue source",
			spanData: getSendSpan(&egress_v1.SpanData_SendSpan{
				Source: &egress_v1.SpanData_SendSpan_QueueName{
					QueueName: "#P2P/QTMP/myQueue",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.source.name": "#P2P/QTMP/myQueue",
				"messaging.source.kind": "queue",
			}, "(anonymous) send"),
		},
		{
			name: "With Anonymous Topic Endpoint source",
			spanData: getSendSpan(&egress_v1.SpanData_SendSpan{
				Source: &egress_v1.SpanData_SendSpan_TopicEndpointName{
					TopicEndpointName: "0123456789abcdef0123456789abcdef",
				},
			}),
			want: getSpan(map[string]any{
				"messaging.source.name": "0123456789abcdef0123456789abcdef",
				"messaging.source.kind": "topic-endpoint",
			}, "(anonymous) send"),
		},
		{
			name:                        "With Unknown Endpoint source",
			spanData:                    getSendSpan(&egress_v1.SpanData_SendSpan{}),
			want:                        getSpan(map[string]any{}, "(unknown) send"),
			expectedUnmarshallingErrors: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestEgressV1Unmarshaller(t)
			actual := ptrace.NewSpan()
			u.mapSendSpan(tt.spanData, actual)
			compareSpans(t, tt.want, actual)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
	// test the various outcomes
	outcomes := map[egress_v1.SpanData_SendSpan_Outcome]string{
		egress_v1.SpanData_SendSpan_ACCEPTED:                  "accepted",
		egress_v1.SpanData_SendSpan_REJECTED:                  "rejected",
		egress_v1.SpanData_SendSpan_RELEASED:                  "released",
		egress_v1.SpanData_SendSpan_DELIVERY_FAILED:           "delivery failed",
		egress_v1.SpanData_SendSpan_FLOW_UNBOUND:              "flow unbound",
		egress_v1.SpanData_SendSpan_TRANSACTION_COMMIT:        "transaction commit",
		egress_v1.SpanData_SendSpan_TRANSACTION_COMMIT_FAILED: "transaction commit failed",
		egress_v1.SpanData_SendSpan_TRANSACTION_ROLLBACK:      "transaction rollback",
	}
	for outcomeKey, outcomeName := range outcomes {
		t.Run("With outcome "+outcomeName, func(t *testing.T) {
			u := newTestEgressV1Unmarshaller(t)
			expected := getSpan(map[string]any{
				"messaging.source.name":         "someQueue",
				"messaging.source.kind":         "queue",
				"messaging.solace.send.outcome": outcomeName,
			}, "someQueue send")
			spanData := getSendSpan(&egress_v1.SpanData_SendSpan{
				Source: &egress_v1.SpanData_SendSpan_QueueName{
					QueueName: "someQueue",
				},
				Outcome: outcomeKey,
			})
			actual := ptrace.NewSpan()
			u.mapSendSpan(spanData, actual)
			compareSpans(t, expected, actual)
		})
	}
}

func TestEgressUnmarshallerTransactionEvent(t *testing.T) {
	someErrorString := "some error"
	tests := []struct {
		name                 string
		spanData             *egress_v1.SpanData_TransactionEvent
		populateExpectedSpan func(span ptrace.Span)
		unmarshallingErrors  any
	}{
		{ // Local Transaction
			name: "Local Transaction Event",
			spanData: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 123456789,
				Type:         egress_v1.SpanData_TransactionEvent_COMMIT,
				Initiator:    egress_v1.SpanData_TransactionEvent_CLIENT,
				TransactionId: &egress_v1.SpanData_TransactionEvent_LocalId{
					LocalId: &egress_v1.SpanData_TransactionEvent_LocalTransactionId{
						TransactionId: 12345,
						SessionId:     67890,
						SessionName:   "my-session-name",
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "commit", 123456789, map[string]any{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
			},
		},
		{
			name: "Local Transaction Event with Session Timeout",
			spanData: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 123456789,
				Type:         egress_v1.SpanData_TransactionEvent_SESSION_TIMEOUT,
				Initiator:    egress_v1.SpanData_TransactionEvent_CLIENT,
				TransactionId: &egress_v1.SpanData_TransactionEvent_LocalId{
					LocalId: &egress_v1.SpanData_TransactionEvent_LocalTransactionId{
						TransactionId: 12345,
						SessionId:     67890,
						SessionName:   "my-session-name",
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "session_timeout", 123456789, map[string]any{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
			},
		},
		{
			name: "Local Transaction Event with Rollback Only",
			spanData: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 123456789,
				Type:         egress_v1.SpanData_TransactionEvent_ROLLBACK_ONLY,
				Initiator:    egress_v1.SpanData_TransactionEvent_CLIENT,
				TransactionId: &egress_v1.SpanData_TransactionEvent_LocalId{
					LocalId: &egress_v1.SpanData_TransactionEvent_LocalTransactionId{
						TransactionId: 12345,
						SessionId:     67890,
						SessionName:   "my-session-name",
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "rollback_only", 123456789, map[string]any{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
			},
		},
		{ // XA transaction
			name: "XA Transaction Event",
			spanData: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 123456789,
				Type:         egress_v1.SpanData_TransactionEvent_END,
				Initiator:    egress_v1.SpanData_TransactionEvent_ADMIN,
				TransactionId: &egress_v1.SpanData_TransactionEvent_Xid_{
					Xid: &egress_v1.SpanData_TransactionEvent_Xid{
						FormatId:        123,
						BranchQualifier: []byte{0, 8, 20, 254},
						GlobalId:        []byte{128, 64, 32, 16, 8, 4, 2, 1, 0},
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "end", 123456789, map[string]any{
					"messaging.solace.transaction_initiator": "administrator",
					"messaging.solace.transaction_xid":       "0000007b-000814fe-804020100804020100",
				})
			},
		},
		{ // XA Transaction with no branch qualifier or global ID and with an error
			name: "XA Transaction Event with nil fields and error",
			spanData: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 123456789,
				Type:         egress_v1.SpanData_TransactionEvent_PREPARE,
				Initiator:    egress_v1.SpanData_TransactionEvent_BROKER,
				TransactionId: &egress_v1.SpanData_TransactionEvent_Xid_{
					Xid: &egress_v1.SpanData_TransactionEvent_Xid{
						FormatId:        123,
						BranchQualifier: nil,
						GlobalId:        nil,
					},
				},
				ErrorDescription: &someErrorString,
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "prepare", 123456789, map[string]any{
					"messaging.solace.transaction_initiator":     "broker",
					"messaging.solace.transaction_xid":           "0000007b--",
					"messaging.solace.transaction_error_message": someErrorString,
				})
			},
		},
		{ // Type of transaction not handled
			name: "Unknown Transaction Type and no ID",
			spanData: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano: 123456789,
				Type:         egress_v1.SpanData_TransactionEvent_Type(12345),
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "Unknown Transaction Event (12345)", 123456789, map[string]any{
					"messaging.solace.transaction_initiator": "client",
				})
			},
			unmarshallingErrors: 2,
		},
		{ // Type of ID not handled, type of initiator not handled
			name: "Unknown Transaction Initiator and no ID",
			spanData: &egress_v1.SpanData_TransactionEvent{
				TimeUnixNano:  123456789,
				Type:          egress_v1.SpanData_TransactionEvent_ROLLBACK,
				Initiator:     egress_v1.SpanData_TransactionEvent_Initiator(12345),
				TransactionId: nil,
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "rollback", 123456789, map[string]any{
					"messaging.solace.transaction_initiator": "Unknown Transaction Initiator (12345)",
				})
			},
			unmarshallingErrors: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestEgressV1Unmarshaller(t)
			expected := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			tt.populateExpectedSpan(expected)
			actual := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			u.mapTransactionEvent(tt.spanData, actual.Events().AppendEmpty())
			// order is nondeterministic for attributes, so we must sort to get a valid comparison
			compareSpans(t, expected, actual)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.unmarshallingErrors)
		})
	}
}

func newTestEgressV1Unmarshaller(t *testing.T) *brokerTraceEgressUnmarshallerV1 {
	m := newTestMetrics(t)
	return &brokerTraceEgressUnmarshallerV1{zap.NewNop(), m}
}
