// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/baggage"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	model_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/model/v1"
)

// tracesUnmarshaller deserializes the message body.
type tracesUnmarshaller interface {
	// unmarshal the amqp-message into traces.
	// Only valid traces are produced or error is returned
	unmarshal(message *inboundMessage) (ptrace.Traces, error)
}

// newUnmarshalleer returns a new unmarshaller ready for message unmarshalling
func newTracesUnmarshaller(logger *zap.Logger, metrics *opencensusMetrics) tracesUnmarshaller {
	return &solaceTracesUnmarshaller{
		logger:  logger,
		metrics: metrics,
		// v1 unmarshaller is implemented by solaceMessageUnmarshallerV1
		receiveUnmarshallerV1: &brokerTraceReceiveUnmarshallerV1{
			logger:  logger,
			metrics: metrics,
		},
	}
}

// solaceTracesUnmarshaller implements tracesUnmarshaller.
type solaceTracesUnmarshaller struct {
	logger                *zap.Logger
	metrics               *opencensusMetrics
	receiveUnmarshallerV1 tracesUnmarshaller
}

var (
	errUpgradeRequired = errors.New("unsupported trace message, upgrade required")
	errUnknownTopic    = errors.New("unknown topic")
	errEmptyPayload    = errors.New("no binary attachment")
)

// unmarshal will unmarshal an *solaceMessage into ptrace.Traces.
// It will make a decision based on the version of the message which unmarshalling strategy to use.
// For now, only receive v1 messages are used.
func (u *solaceTracesUnmarshaller) unmarshal(message *inboundMessage) (ptrace.Traces, error) {
	const (
		topicPrefix       = "_telemetry/"
		receiveSpanPrefix = "broker/trace/receive/"
		v1Suffix          = "v1"
	)
	if message.Properties == nil || message.Properties.To == nil {
		// no topic
		u.logger.Error("Received message with no topic")
		return ptrace.Traces{}, errUnknownTopic
	}
	var topic string = *message.Properties.To
	// Multiplex the topic string. For now we only have a single type handled
	if strings.HasPrefix(topic, topicPrefix) {
		// we are a telemetry strng
		if strings.HasPrefix(topic[len(topicPrefix):], receiveSpanPrefix) {
			// we are handling a receive span, validate the version is v1
			if strings.HasSuffix(topic, v1Suffix) {
				return u.receiveUnmarshallerV1.unmarshal(message)
			}
			// otherwise we are an unknown version
			u.logger.Error("Received message with unsupported receive span version, an upgrade is required", zap.String("topic", *message.Properties.To))
		} else {
			u.logger.Error("Received message with unsupported topic, an upgrade is required", zap.String("topic", *message.Properties.To))
		}
		// if we don't know the type, we must upgrade
		return ptrace.Traces{}, errUpgradeRequired
	}
	// unknown topic, do not require an upgrade
	u.logger.Error("Received message with unknown topic", zap.String("topic", *message.Properties.To))
	return ptrace.Traces{}, errUnknownTopic
}

type brokerTraceReceiveUnmarshallerV1 struct {
	logger  *zap.Logger
	metrics *opencensusMetrics
}

// unmarshal implements tracesUnmarshaller.unmarshal
func (u *brokerTraceReceiveUnmarshallerV1) unmarshal(message *inboundMessage) (ptrace.Traces, error) {
	spanData, err := u.unmarshalToSpanData(message)
	if err != nil {
		return ptrace.Traces{}, err
	}
	traces := ptrace.NewTraces()
	u.populateTraces(spanData, traces)
	return traces, nil
}

// unmarshalToSpanData will consume an solaceMessage and unmarshal it into a SpanData.
// Returns an error if one occurred.
func (u *brokerTraceReceiveUnmarshallerV1) unmarshalToSpanData(message *inboundMessage) (*model_v1.SpanData, error) {
	var data = message.GetData()
	if len(data) == 0 {
		return nil, errEmptyPayload
	}
	var spanData model_v1.SpanData
	if err := proto.Unmarshal(data, &spanData); err != nil {
		return nil, err
	}
	return &spanData, nil
}

// createSpan will create a new Span from the given traces and map the given SpanData to the span.
// This will set all required fields such as name version, trace and span ID, parent span ID (if applicable),
// timestamps, errors and states.
func (u *brokerTraceReceiveUnmarshallerV1) populateTraces(spanData *model_v1.SpanData, traces ptrace.Traces) {
	// Append new resource span and map any attributes
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	u.mapResourceSpanAttributes(spanData, resourceSpan.Resource().Attributes())
	instrLibrarySpans := resourceSpan.ScopeSpans().AppendEmpty()
	// Create a new span
	clientSpan := instrLibrarySpans.Spans().AppendEmpty()
	// map the basic span data
	u.mapClientSpanData(spanData, clientSpan)
	// map all span attributes
	u.mapClientSpanAttributes(spanData, clientSpan.Attributes())
	// map all events
	u.mapEvents(spanData, clientSpan)
}

func (u *brokerTraceReceiveUnmarshallerV1) mapResourceSpanAttributes(spanData *model_v1.SpanData, attrMap pcommon.Map) {
	const (
		routerNameAttrKey     = "service.name"
		messageVpnNameAttrKey = "service.instance.id"
		solosVersionAttrKey   = "service.version"
	)
	attrMap.PutStr(routerNameAttrKey, spanData.RouterName)
	if spanData.MessageVpnName != nil {
		attrMap.PutStr(messageVpnNameAttrKey, *spanData.MessageVpnName)
	}
	attrMap.PutStr(solosVersionAttrKey, spanData.SolosVersion)
}

func (u *brokerTraceReceiveUnmarshallerV1) mapClientSpanData(spanData *model_v1.SpanData, clientSpan ptrace.Span) {
	const clientSpanName = "(topic) receive"

	// client span constants
	clientSpan.SetName(clientSpanName)
	// SPAN_KIND_CONSUMER == 5
	clientSpan.SetKind(5)

	// map trace ID
	var traceID [16]byte
	copy(traceID[:16], spanData.TraceId)
	clientSpan.SetTraceID(traceID)
	// map span ID
	var spanID [8]byte
	copy(spanID[:8], spanData.SpanId)
	clientSpan.SetSpanID(spanID)
	// conditional parent-span-id
	if len(spanData.ParentSpanId) == 8 {
		var parentSpanID [8]byte
		copy(parentSpanID[:8], spanData.ParentSpanId)
		clientSpan.SetParentSpanID(parentSpanID)
	}

	// timestamps
	clientSpan.SetStartTimestamp(pcommon.Timestamp(spanData.GetStartTimeUnixNano()))
	clientSpan.SetEndTimestamp(pcommon.Timestamp(spanData.GetEndTimeUnixNano()))
	// status
	if spanData.ErrorDescription != "" {
		clientSpan.Status().SetCode(ptrace.StatusCodeError)
		clientSpan.Status().SetMessage(spanData.ErrorDescription)
	}
	// trace state
	if spanData.TraceState != nil {
		clientSpan.TraceState().FromRaw(*spanData.TraceState)
	}
}

// mapAttributes takes a set of attributes from SpanData and maps them to ClientSpan.Attributes().
// Will also copy any user properties stored in the SpanData with a best effort approach.
func (u *brokerTraceReceiveUnmarshallerV1) mapClientSpanAttributes(spanData *model_v1.SpanData, attrMap pcommon.Map) {
	// constant attributes
	const (
		systemAttrKey      = "messaging.system"
		systemAttrValue    = "SolacePubSub+"
		operationAttrKey   = "messaging.operation"
		operationAttrValue = "receive"
	)
	attrMap.PutStr(systemAttrKey, systemAttrValue)
	attrMap.PutStr(operationAttrKey, operationAttrValue)
	// attributes from spanData
	const (
		protocolAttrKey                    = "messaging.protocol"
		protocolVersionAttrKey             = "messaging.protocol_version"
		messageIDAttrKey                   = "messaging.message_id"
		conversationIDAttrKey              = "messaging.conversation_id"
		payloadSizeBytesAttrKey            = "messaging.message_payload_size_bytes"
		destinationAttrKey                 = "messaging.destination"
		clientUsernameAttrKey              = "messaging.solace.client_username"
		clientNameAttrKey                  = "messaging.solace.client_name"
		replicationGroupMessageIDAttrKey   = "messaging.solace.replication_group_message_id"
		priorityAttrKey                    = "messaging.solace.priority"
		ttlAttrKey                         = "messaging.solace.ttl"
		dmqEligibleAttrKey                 = "messaging.solace.dmq_eligible"
		droppedEnqueueEventsSuccessAttrKey = "messaging.solace.dropped_enqueue_events_success"
		droppedEnqueueEventsFailedAttrKey  = "messaging.solace.dropped_enqueue_events_failed"
		replyToAttrKey                     = "messaging.solace.reply_to_topic"
		receiveTimeAttrKey                 = "messaging.solace.broker_receive_time_unix_nano"
		droppedUserPropertiesAttrKey       = "messaging.solace.dropped_application_message_properties"
		deliveryModeAttrKey                = "messaging.solace.delivery_mode"
		hostIPAttrKey                      = "net.host.ip"
		hostPortAttrKey                    = "net.host.port"
		peerIPAttrKey                      = "net.peer.ip"
		peerPortAttrKey                    = "net.peer.port"
	)
	attrMap.PutStr(protocolAttrKey, spanData.Protocol)
	if spanData.ProtocolVersion != nil {
		attrMap.PutStr(protocolVersionAttrKey, *spanData.ProtocolVersion)
	}
	if spanData.ApplicationMessageId != nil {
		attrMap.PutStr(messageIDAttrKey, *spanData.ApplicationMessageId)
	}
	if spanData.CorrelationId != nil {
		attrMap.PutStr(conversationIDAttrKey, *spanData.CorrelationId)
	}
	attrMap.PutInt(payloadSizeBytesAttrKey, int64(spanData.BinaryAttachmentSize+spanData.XmlAttachmentSize+spanData.MetadataSize))
	attrMap.PutStr(clientUsernameAttrKey, spanData.ClientUsername)
	attrMap.PutStr(clientNameAttrKey, spanData.ClientName)
	attrMap.PutInt(receiveTimeAttrKey, spanData.BrokerReceiveTimeUnixNano)
	attrMap.PutStr(destinationAttrKey, spanData.Topic)

	var deliveryMode string
	switch spanData.DeliveryMode {
	case model_v1.SpanData_DIRECT:
		deliveryMode = "direct"
	case model_v1.SpanData_NON_PERSISTENT:
		deliveryMode = "non_persistent"
	case model_v1.SpanData_PERSISTENT:
		deliveryMode = "persistent"
	default:
		deliveryMode = fmt.Sprintf("Unknown Delivery Mode (%s)", spanData.DeliveryMode.String())
		u.logger.Warn(fmt.Sprintf("Received span with unknown delivery mode %s", spanData.DeliveryMode))
		u.metrics.recordRecoverableUnmarshallingError()
	}
	attrMap.PutStr(deliveryModeAttrKey, deliveryMode)

	rgmid := u.rgmidToString(spanData.ReplicationGroupMessageId)
	if len(rgmid) > 0 {
		attrMap.PutStr(replicationGroupMessageIDAttrKey, rgmid)
	}

	if spanData.Priority != nil {
		attrMap.PutInt(priorityAttrKey, int64(*spanData.Priority))
	}
	if spanData.Ttl != nil {
		attrMap.PutInt(ttlAttrKey, *spanData.Ttl)
	}
	if spanData.ReplyToTopic != nil {
		attrMap.PutStr(replyToAttrKey, *spanData.ReplyToTopic)
	}
	attrMap.PutBool(dmqEligibleAttrKey, spanData.DmqEligible)
	attrMap.PutInt(droppedEnqueueEventsSuccessAttrKey, int64(spanData.DroppedEnqueueEventsSuccess))
	attrMap.PutInt(droppedEnqueueEventsFailedAttrKey, int64(spanData.DroppedEnqueueEventsFailed))

	// The IPs are now optional meaning we will not incluude them if they are zero length
	hostIPLen := len(spanData.HostIp)
	if hostIPLen == 4 || hostIPLen == 16 {
		attrMap.PutStr(hostIPAttrKey, net.IP(spanData.HostIp).String())
		attrMap.PutInt(hostPortAttrKey, int64(spanData.HostPort))
	} else {
		u.logger.Debug("Host ip not included", zap.Int("length", hostIPLen))
	}

	peerIPLen := len(spanData.PeerIp)
	if peerIPLen == 4 || peerIPLen == 16 {
		attrMap.PutStr(peerIPAttrKey, net.IP(spanData.PeerIp).String())
		attrMap.PutInt(peerPortAttrKey, int64(spanData.PeerPort))
	} else {
		u.logger.Debug("Peer IP not included", zap.Int("length", peerIPLen))
	}

	if spanData.Baggage != nil {
		err := u.unmarshalBaggage(attrMap, *spanData.Baggage)
		if err != nil {
			u.logger.Warn("Received malformed baggage string in span data")
			u.metrics.recordRecoverableUnmarshallingError()
		}
	}

	attrMap.PutBool(droppedUserPropertiesAttrKey, spanData.DroppedApplicationMessageProperties)
	for key, value := range spanData.UserProperties {
		if value != nil {
			u.insertUserProperty(attrMap, key, value.Value)
		}
	}
}

// mapEvents maps all events contained in SpanData to relevant events within clientSpan.Events()
func (u *brokerTraceReceiveUnmarshallerV1) mapEvents(spanData *model_v1.SpanData, clientSpan ptrace.Span) {
	// handle enqueue events
	for _, enqueueEvent := range spanData.EnqueueEvents {
		u.mapEnqueueEvent(enqueueEvent, clientSpan.Events())
	}

	// handle transaction events
	if transactionEvent := spanData.TransactionEvent; transactionEvent != nil {
		u.mapTransactionEvent(transactionEvent, clientSpan.Events())
	}
}

// mapEnqueueEvent maps a SpanData_EnqueueEvent to a ClientSpan.Event
func (u *brokerTraceReceiveUnmarshallerV1) mapEnqueueEvent(enqueueEvent *model_v1.SpanData_EnqueueEvent, clientSpanEvents ptrace.SpanEventSlice) {
	const (
		enqueueEventSuffix               = " enqueue" // Final should be `<dest> enqueue`
		messagingDestinationTypeEventKey = "messaging.solace.destination_type"
		statusMessageEventKey            = "messaging.solace.enqueue_error_message"
		rejectsAllEnqueuesKey            = "messaging.solace.rejects_all_enqueues"
		partitionNumberKey               = "messaging.solace.partition_number"
		queueKind                        = "queue"
		topicEndpointKind                = "topic-endpoint"
	)
	var destinationName string
	var destinationType string
	switch casted := enqueueEvent.Dest.(type) {
	case *model_v1.SpanData_EnqueueEvent_TopicEndpointName:
		destinationName = casted.TopicEndpointName
		destinationType = topicEndpointKind
	case *model_v1.SpanData_EnqueueEvent_QueueName:
		destinationName = casted.QueueName
		destinationType = queueKind
	default:
		u.logger.Warn(fmt.Sprintf("Unknown destination type %T", casted))
		u.metrics.recordRecoverableUnmarshallingError()
		return
	}
	clientEvent := clientSpanEvents.AppendEmpty()
	clientEvent.SetName(destinationName + enqueueEventSuffix)
	clientEvent.SetTimestamp(pcommon.Timestamp(enqueueEvent.TimeUnixNano))
	clientEvent.Attributes().EnsureCapacity(3)
	clientEvent.Attributes().PutStr(messagingDestinationTypeEventKey, destinationType)
	clientEvent.Attributes().PutBool(rejectsAllEnqueuesKey, enqueueEvent.RejectsAllEnqueues)
	if enqueueEvent.ErrorDescription != nil {
		clientEvent.Attributes().PutStr(statusMessageEventKey, enqueueEvent.GetErrorDescription())
	}
	if enqueueEvent.PartitionNumber != nil {
		clientEvent.Attributes().PutInt(partitionNumberKey, int64(*enqueueEvent.PartitionNumber))
	}
}

// mapTransactionEvent maps a SpanData_TransactionEvent to a ClientSpan.Event
func (u *brokerTraceReceiveUnmarshallerV1) mapTransactionEvent(transactionEvent *model_v1.SpanData_TransactionEvent, clientSpanEvents ptrace.SpanEventSlice) {
	const (
		transactionInitiatorEventKey    = "messaging.solace.transaction_initiator"
		transactionIDEventKey           = "messaging.solace.transaction_id"
		transactedSessionNameEventKey   = "messaging.solace.transacted_session_name"
		transactedSessionIDEventKey     = "messaging.solace.transacted_session_id"
		transactionErrorMessageEventKey = "messaging.solace.transaction_error_message"
		transactionXIDEventKey          = "messaging.solace.transaction_xid"
	)
	// map the transaction type to a name
	var name string
	switch transactionEvent.GetType() {
	case model_v1.SpanData_TransactionEvent_COMMIT:
		name = "commit"
	case model_v1.SpanData_TransactionEvent_ROLLBACK:
		name = "rollback"
	case model_v1.SpanData_TransactionEvent_END:
		name = "end"
	case model_v1.SpanData_TransactionEvent_PREPARE:
		name = "prepare"
	case model_v1.SpanData_TransactionEvent_SESSION_TIMEOUT:
		name = "session_timeout"
	case model_v1.SpanData_TransactionEvent_ROLLBACK_ONLY:
		name = "rollback_only"
	default:
		// Set the name to the unknown transaction event type to ensure forward compat.
		name = fmt.Sprintf("Unknown Transaction Event (%s)", transactionEvent.GetType().String())
		u.logger.Warn(fmt.Sprintf("Received span with unknown transaction event %s", transactionEvent.GetType()))
		u.metrics.recordRecoverableUnmarshallingError()
	}
	clientEvent := clientSpanEvents.AppendEmpty()
	clientEvent.SetName(name)
	clientEvent.SetTimestamp(pcommon.Timestamp(transactionEvent.TimeUnixNano))
	// map initiator enums to expected initiator strings
	var initiator string
	switch transactionEvent.GetInitiator() {
	case model_v1.SpanData_TransactionEvent_CLIENT:
		initiator = "client"
	case model_v1.SpanData_TransactionEvent_ADMIN:
		initiator = "administrator"
	case model_v1.SpanData_TransactionEvent_BROKER:
		initiator = "broker"
	default:
		initiator = fmt.Sprintf("Unknown Transaction Initiator (%s)", transactionEvent.GetInitiator().String())
		u.logger.Warn(fmt.Sprintf("Received span with unknown transaction initiator %s", transactionEvent.GetInitiator()))
		u.metrics.recordRecoverableUnmarshallingError()
	}
	clientEvent.Attributes().PutStr(transactionInitiatorEventKey, initiator)
	// conditionally set the error description if one occurred, otherwise omit
	if transactionEvent.ErrorDescription != nil {
		clientEvent.Attributes().PutStr(transactionErrorMessageEventKey, transactionEvent.GetErrorDescription())
	}
	// map the transaction type/id
	transactionID := transactionEvent.GetTransactionId()
	switch casted := transactionID.(type) {
	case *model_v1.SpanData_TransactionEvent_LocalId:
		clientEvent.Attributes().PutInt(transactionIDEventKey, int64(casted.LocalId.TransactionId))
		clientEvent.Attributes().PutStr(transactedSessionNameEventKey, casted.LocalId.SessionName)
		clientEvent.Attributes().PutInt(transactedSessionIDEventKey, int64(casted.LocalId.SessionId))
	case *model_v1.SpanData_TransactionEvent_Xid_:
		// format xxxxxxxx-yyyyyyyy-zzzzzzzz where x is FormatID (hex rep of int32), y is BranchQualifier and z is GlobalID, hex encoded.
		xidString := fmt.Sprintf("%08x", casted.Xid.FormatId) + "-" +
			hex.EncodeToString(casted.Xid.BranchQualifier) + "-" + hex.EncodeToString(casted.Xid.GlobalId)
		clientEvent.Attributes().PutStr(transactionXIDEventKey, xidString)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown transaction ID type %T", transactionID))
		u.metrics.recordRecoverableUnmarshallingError()
	}
}

func (u *brokerTraceReceiveUnmarshallerV1) rgmidToString(rgmid []byte) string {
	// rgmid[0] is the version of the rgmid
	if len(rgmid) != 17 || rgmid[0] != 1 {
		// may be cases where the rgmid is empty or nil, len(rgmid) will return 0 if nil
		if len(rgmid) > 0 {
			u.logger.Warn("Received invalid length or version for rgmid", zap.Int8("version", int8(rgmid[0])), zap.Int("length", len(rgmid)))
			u.metrics.recordRecoverableUnmarshallingError()
		}
		return hex.EncodeToString(rgmid)
	}
	rgmidEncoded := make([]byte, 32)
	hex.Encode(rgmidEncoded, rgmid[1:])
	// format: rmid1:aaaaa-bbbbbbbbbbb-cccccccc-dddddddd
	rgmidString := "rmid1:" + string(rgmidEncoded[0:5]) + "-" + string(rgmidEncoded[5:16]) + "-" + string(rgmidEncoded[16:24]) + "-" + string(rgmidEncoded[24:32])
	return rgmidString
}

// unmarshalBaggage will unmarshal a baggage string
// See spec https://github.com/open-telemetry/opentelemetry-go/blob/v1.11.1/baggage/baggage.go
func (u *brokerTraceReceiveUnmarshallerV1) unmarshalBaggage(toMap pcommon.Map, baggageString string) error {
	const (
		baggageValuePrefix    = "messaging.solace.message.baggage."
		baggageMetadataPrefix = "messaging.solace.message.baggage_metadata."
		propertyDelimiter     = ";"
	)
	bg, err := baggage.Parse(baggageString)
	if err != nil {
		return err
	}
	// we got a valid baggage string, assume everything else is valid
	for _, member := range bg.Members() {
		toMap.PutStr(baggageValuePrefix+member.Key(), member.Value())
		// member.Properties copies, we should cache
		properties := member.Properties()
		if len(properties) > 0 {
			// Re-encode the properties and save them as a parameter
			var propertyString strings.Builder
			propertyString.WriteString(properties[0].String())
			for i := 1; i < len(properties); i++ {
				propertyString.WriteString(propertyDelimiter + properties[i].String())
			}
			toMap.PutStr(baggageMetadataPrefix+member.Key(), propertyString.String())
		}
	}
	return nil
}

// insertUserProperty will instert a user property value with the given key to an attribute if possible.
// Since AttributeMap only supports int64 integer types, uint64 data may be misrepresented.
func (u *brokerTraceReceiveUnmarshallerV1) insertUserProperty(toMap pcommon.Map, key string, value interface{}) {
	const (
		// userPropertiesPrefixAttrKey is the key used to prefix all user properties
		userPropertiesAttrKeyPrefix = "messaging.solace.user_properties."
	)
	k := userPropertiesAttrKeyPrefix + key
	switch v := value.(type) {
	case *model_v1.SpanData_UserPropertyValue_NullValue:
		toMap.PutEmpty(k)
	case *model_v1.SpanData_UserPropertyValue_BoolValue:
		toMap.PutBool(k, v.BoolValue)
	case *model_v1.SpanData_UserPropertyValue_DoubleValue:
		toMap.PutDouble(k, v.DoubleValue)
	case *model_v1.SpanData_UserPropertyValue_ByteArrayValue:
		toMap.PutEmptyBytes(k).FromRaw(v.ByteArrayValue)
	case *model_v1.SpanData_UserPropertyValue_FloatValue:
		toMap.PutDouble(k, float64(v.FloatValue))
	case *model_v1.SpanData_UserPropertyValue_Int8Value:
		toMap.PutInt(k, int64(v.Int8Value))
	case *model_v1.SpanData_UserPropertyValue_Int16Value:
		toMap.PutInt(k, int64(v.Int16Value))
	case *model_v1.SpanData_UserPropertyValue_Int32Value:
		toMap.PutInt(k, int64(v.Int32Value))
	case *model_v1.SpanData_UserPropertyValue_Int64Value:
		toMap.PutInt(k, v.Int64Value)
	case *model_v1.SpanData_UserPropertyValue_Uint8Value:
		toMap.PutInt(k, int64(v.Uint8Value))
	case *model_v1.SpanData_UserPropertyValue_Uint16Value:
		toMap.PutInt(k, int64(v.Uint16Value))
	case *model_v1.SpanData_UserPropertyValue_Uint32Value:
		toMap.PutInt(k, int64(v.Uint32Value))
	case *model_v1.SpanData_UserPropertyValue_Uint64Value:
		toMap.PutInt(k, int64(v.Uint64Value))
	case *model_v1.SpanData_UserPropertyValue_StringValue:
		toMap.PutStr(k, v.StringValue)
	case *model_v1.SpanData_UserPropertyValue_DestinationValue:
		toMap.PutStr(k, v.DestinationValue)
	case *model_v1.SpanData_UserPropertyValue_CharacterValue:
		toMap.PutStr(k, string(rune(v.CharacterValue)))
	default:
		u.logger.Warn(fmt.Sprintf("Unknown user property type: %T", v))
		u.metrics.recordRecoverableUnmarshallingError()
	}
}
