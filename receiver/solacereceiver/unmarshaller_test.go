// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver

import (
	"errors"
	"testing"

	"github.com/Azure/go-amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	egress_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/model/egress/v1"
	receive_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/model/receive/v1"
)

// Validate entire unmarshal flow
func TestSolaceMessageUnmarshallerUnmarshal(t *testing.T) {
	validReceiveTopicVersion := "_telemetry/broker/trace/receive/v1"
	validEgressTopicVersion := "_telemetry/broker/trace/egress/v1"
	invalidReceiveTopicVersion := "_telemetry/broker/trace/receive/v2"
	invalidTelemetryTopic := "_telemetry/broker/trace/somethingNew"
	invalidTopicString := "some unknown topic string that won't be valid"

	tests := []struct {
		name    string
		message *amqp.Message
		want    *ptrace.Traces
		err     error
	}{
		{
			name: "Unknown Topic Stirng",
			message: &inboundMessage{
				Properties: &amqp.MessageProperties{
					To: &invalidTopicString,
				},
			},
			err: errUnknownTopic,
		},
		{
			name: "Bad Topic Version",
			message: &inboundMessage{
				Properties: &amqp.MessageProperties{
					To: &invalidReceiveTopicVersion,
				},
			},
			err: errUpgradeRequired,
		},
		{
			name: "Unknown Telemetry Topic",
			message: &inboundMessage{
				Properties: &amqp.MessageProperties{
					To: &invalidTelemetryTopic,
				},
			},
			err: errUpgradeRequired,
		},
		{
			name: "No Message Properties",
			message: &inboundMessage{
				Properties: nil,
			},
			err: errUnknownTopic,
		},
		{
			name: "No Topic String",
			message: &inboundMessage{
				Properties: &amqp.MessageProperties{
					To: nil,
				},
			},
			err: errUnknownTopic,
		},
		{
			name: "Empty Message Data with Receive topic",
			message: &amqp.Message{
				Data: [][]byte{{}},
				Properties: &amqp.MessageProperties{
					To: &validReceiveTopicVersion,
				},
			},
			err: errEmptyPayload,
		},
		{
			name: "Invalid Message Data with Receive topic",
			message: &amqp.Message{
				Data: [][]byte{{1, 2, 3, 4, 5}},
				Properties: &amqp.MessageProperties{
					To: &validReceiveTopicVersion,
				},
			},
			err: errors.New("cannot parse invalid wire-format data"),
		},
		{
			name: "Empty Message Data with Egress topic",
			message: &amqp.Message{
				Data: [][]byte{{}},
				Properties: &amqp.MessageProperties{
					To: &validEgressTopicVersion,
				},
			},
			err: errEmptyPayload,
		},
		{
			name: "Invalid Message Data with Egress topic",
			message: &amqp.Message{
				Data: [][]byte{{1, 2, 3, 4, 5}},
				Properties: &amqp.MessageProperties{
					To: &validEgressTopicVersion,
				},
			},
			err: errors.New("cannot parse invalid wire-format data"),
		},
		{
			name: "Valid Receive Message Data",
			message: &amqp.Message{
				Data: [][]byte{func() []byte {
					// TODO capture binary data of this directly, ie. real world data.
					var (
						protocolVersion      = "5.0"
						applicationMessageID = "someMessageID"
						correlationID        = "someConversationID"
						priority             = uint32(1)
						ttl                  = int64(86000)
						routerName           = "someRouterName"
						vpnName              = "someVpnName"
						replyToTopic         = "someReplyToTopic"
						topic                = "someTopic"
					)
					validData, err := proto.Marshal(&receive_v1.SpanData{
						TraceId:                             []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
						SpanId:                              []byte{7, 6, 5, 4, 3, 2, 1, 0},
						StartTimeUnixNano:                   1234567890,
						EndTimeUnixNano:                     2234567890,
						RouterName:                          routerName,
						MessageVpnName:                      &vpnName,
						SolosVersion:                        "10.0.0",
						Protocol:                            "MQTT",
						ProtocolVersion:                     &protocolVersion,
						ApplicationMessageId:                &applicationMessageID,
						CorrelationId:                       &correlationID,
						DeliveryMode:                        receive_v1.SpanData_DIRECT,
						BinaryAttachmentSize:                1000,
						XmlAttachmentSize:                   200,
						MetadataSize:                        34,
						ClientUsername:                      "someClientUsername",
						ClientName:                          "someClient1234",
						Topic:                               topic,
						ReplyToTopic:                        &replyToTopic,
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
						UserProperties: map[string]*receive_v1.SpanData_UserPropertyValue{
							"special_key": {
								Value: &receive_v1.SpanData_UserPropertyValue_BoolValue{
									BoolValue: true,
								},
							},
						},
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
						TransactionEvent: &receive_v1.SpanData_TransactionEvent{
							TimeUnixNano: 123456789,
							Type:         receive_v1.SpanData_TransactionEvent_SESSION_TIMEOUT,
							Initiator:    receive_v1.SpanData_TransactionEvent_CLIENT,
							TransactionId: &receive_v1.SpanData_TransactionEvent_LocalId{
								LocalId: &receive_v1.SpanData_TransactionEvent_LocalTransactionId{
									TransactionId: 12345,
									SessionId:     67890,
									SessionName:   "my-session-name",
								},
							},
						},
					})
					require.NoError(t, err)
					return validData
				}()},
				Properties: &amqp.MessageProperties{
					To: &validReceiveTopicVersion,
				},
			},
			want: func() *ptrace.Traces {
				traces := ptrace.NewTraces()
				resource := traces.ResourceSpans().AppendEmpty()
				populateAttributes(t, resource.Resource().Attributes(), map[string]interface{}{
					"service.name":        "someRouterName",
					"service.instance.id": "someVpnName",
					"service.version":     "10.0.0",
				})
				instrumentation := resource.ScopeSpans().AppendEmpty()
				span := instrumentation.Spans().AppendEmpty()
				span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
				span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
				span.SetStartTimestamp(1234567890)
				span.SetEndTimestamp(2234567890)
				// expect some constants
				span.SetKind(5)
				span.SetName("(topic) receive")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				spanAttrs := span.Attributes()
				populateAttributes(t, spanAttrs, map[string]interface{}{
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
					"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
					"messaging.solace.dropped_application_message_properties": false,
					"messaging.solace.delivery_mode":                          "direct",
					"net.host.ip":                                             "1.2.3.4",
					"net.host.port":                                           int64(55555),
					"net.peer.ip":                                             "2345:425:2ca1::567:5673:23b5",
					"net.peer.port":                                           int64(12345),
					"messaging.solace.user_properties.special_key":            true,
				})
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "sometopic enqueue", 2345678, map[string]interface{}{
					"messaging.solace.destination_type":     "topic-endpoint",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "session_timeout", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
				return &traces
			}(),
		},
		{
			name: "Valid Egress Message Data",
			message: &amqp.Message{
				Data: [][]byte{func() []byte {
					// TODO capture binary data of this directly, ie. real world data.
					var (
						routerName = "someRouterName"
						vpnName    = "someVpnName"
					)
					validData, err := proto.Marshal(&egress_v1.SpanData{
						RouterName:     routerName,
						MessageVpnName: &vpnName,
						SolosVersion:   "10.0.0",
						EgressSpans: func() []*egress_v1.SpanData_EgressSpan {
							spans := make([]*egress_v1.SpanData_EgressSpan, len(validEgressSpans))
							i := 0
							for _, spanRef := range validEgressSpans {
								span := spanRef.in
								spans[i] = span
								i++
							}
							return spans
						}(),
					})
					require.NoError(t, err)
					return validData
				}()},
				Properties: &amqp.MessageProperties{
					To: &validEgressTopicVersion,
				},
			},
			want: func() *ptrace.Traces {
				traces := ptrace.NewTraces()
				resource := traces.ResourceSpans().AppendEmpty()
				populateAttributes(t, resource.Resource().Attributes(), map[string]interface{}{
					"service.name":        "someRouterName",
					"service.instance.id": "someVpnName",
					"service.version":     "10.0.0",
				})
				instrumentation := resource.ScopeSpans().AppendEmpty()
				// first send span
				for _, spanRef := range validEgressSpans {
					span := spanRef.out
					newSpan := instrumentation.Spans().AppendEmpty()
					span.CopyTo(newSpan)
				}
				return &traces
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTracesUnmarshaller(zap.NewNop(), newTestMetrics(t))
			traces, err := u.unmarshal(tt.message)
			if tt.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.err.Error())
			} else {
				assert.NoError(t, err)
			}

			if tt.want != nil {
				require.NotNil(t, traces)
				require.Equal(t, 1, traces.ResourceSpans().Len())
				expectedResource := tt.want.ResourceSpans().At(0)
				resource := traces.ResourceSpans().At(0)
				assert.Equal(t, expectedResource.Resource().Attributes().AsRaw(), resource.Resource().Attributes().AsRaw())
				require.Equal(t, 1, resource.ScopeSpans().Len())
				expectedInstrumentation := expectedResource.ScopeSpans().At(0)
				instrumentation := resource.ScopeSpans().At(0)
				assert.Equal(t, expectedInstrumentation.Scope(), instrumentation.Scope())
				require.Equal(t, expectedInstrumentation.Spans().Len(), instrumentation.Spans().Len())
				for i := 0; i < expectedInstrumentation.Spans().Len(); i++ {
					expectedSpan := expectedInstrumentation.Spans().At(i)
					span := instrumentation.Spans().At(i)
					compareSpans(t, expectedSpan, span)
				}
			} else {
				assert.Equal(t, ptrace.Traces{}, traces)
			}
		})
	}
}

// common helpers

func compareSpans(t *testing.T, expected, actual ptrace.Span) {
	assert.Equal(t, expected.Name(), actual.Name())
	assert.Equal(t, expected.TraceID(), actual.TraceID())
	assert.Equal(t, expected.SpanID(), actual.SpanID())
	assert.Equal(t, expected.ParentSpanID(), actual.ParentSpanID())
	assert.Equal(t, expected.StartTimestamp(), actual.StartTimestamp())
	assert.Equal(t, expected.EndTimestamp(), actual.EndTimestamp())
	assert.Equal(t, expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
	require.Equal(t, expected.Events().Len(), actual.Events().Len())
	for i := 0; i < expected.Events().Len(); i++ {
		lessFunc := func(a, b ptrace.SpanEvent) bool {
			return a.Name() < b.Name() // choose any comparison here
		}
		ee := expected.Events()
		ee.Sort(lessFunc)
		expectedEvent := ee.At(i)

		ae := actual.Events()
		ae.Sort(lessFunc)
		actualEvent := ae.At(i)
		assert.Equal(t, expectedEvent.Name(), actualEvent.Name())
		assert.Equal(t, expectedEvent.Timestamp(), actualEvent.Timestamp())
		assert.Equal(t, expectedEvent.Attributes().AsRaw(), actualEvent.Attributes().AsRaw())
	}
}

func populateEvent(t *testing.T, span ptrace.Span, name string, timestamp uint64, attributes map[string]interface{}) {
	spanEvent := span.Events().AppendEmpty()
	spanEvent.SetName(name)
	spanEvent.SetTimestamp(pcommon.Timestamp(timestamp))
	populateAttributes(t, spanEvent.Attributes(), attributes)
}

func populateAttributes(t *testing.T, attrMap pcommon.Map, attributes map[string]interface{}) {
	for key, val := range attributes {
		switch casted := val.(type) {
		case string:
			attrMap.PutStr(key, casted)
		case int64:
			attrMap.PutInt(key, casted)
		case int:
			attrMap.PutInt(key, int64(casted))
		case bool:
			attrMap.PutBool(key, casted)
		default:
			require.Fail(t, "Test setup issue: unknown type, could not insert data")
		}
	}
}
