// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/baggage"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	receive_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/model/receive/v1"
)

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
func (u *brokerTraceReceiveUnmarshallerV1) unmarshalToSpanData(message *inboundMessage) (*receive_v1.SpanData, error) {
	var data = message.GetData()
	if len(data) == 0 {
		return nil, errEmptyPayload
	}
	var spanData receive_v1.SpanData
	if err := proto.Unmarshal(data, &spanData); err != nil {
		return nil, err
	}
	return &spanData, nil
}

// createSpan will create a new Span from the given traces and map the given SpanData to the span.
// This will set all required fields such as name version, trace and span ID, parent span ID (if applicable),
// timestamps, errors and states.
func (u *brokerTraceReceiveUnmarshallerV1) populateTraces(spanData *receive_v1.SpanData, traces ptrace.Traces) {
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

func (u *brokerTraceReceiveUnmarshallerV1) mapResourceSpanAttributes(spanData *receive_v1.SpanData, attrMap pcommon.Map) {
	setResourceSpanAttributes(attrMap, spanData.RouterName, spanData.SolosVersion, spanData.MessageVpnName)
}

func (u *brokerTraceReceiveUnmarshallerV1) mapClientSpanData(spanData *receive_v1.SpanData, clientSpan ptrace.Span) {
	const clientSpanName = "(topic) receive"

	// client span constants
	clientSpan.SetName(clientSpanName)
	// SPAN_KIND_CONSUMER == 5
	clientSpan.SetKind(ptrace.SpanKindConsumer)

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
func (u *brokerTraceReceiveUnmarshallerV1) mapClientSpanAttributes(spanData *receive_v1.SpanData, attrMap pcommon.Map) {
	// receive operation
	const operationAttrValue = "receive"
	attrMap.PutStr(systemAttrKey, systemAttrValue)
	attrMap.PutStr(operationAttrKey, operationAttrValue)

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
	case receive_v1.SpanData_DIRECT:
		deliveryMode = "direct"
	case receive_v1.SpanData_NON_PERSISTENT:
		deliveryMode = "non_persistent"
	case receive_v1.SpanData_PERSISTENT:
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
func (u *brokerTraceReceiveUnmarshallerV1) mapEvents(spanData *receive_v1.SpanData, clientSpan ptrace.Span) {
	// handle enqueue events
	for _, enqueueEvent := range spanData.EnqueueEvents {
		u.mapEnqueueEvent(enqueueEvent, clientSpan.Events())
	}

	// handle transaction events
	if transactionEvent := spanData.TransactionEvent; transactionEvent != nil {
		u.mapTransactionEvent(transactionEvent, clientSpan.Events().AppendEmpty())
	}
}

// mapEnqueueEvent maps a SpanData_EnqueueEvent to a ClientSpan.Event
func (u *brokerTraceReceiveUnmarshallerV1) mapEnqueueEvent(enqueueEvent *receive_v1.SpanData_EnqueueEvent, clientSpanEvents ptrace.SpanEventSlice) {
	const (
		enqueueEventSuffix               = " enqueue" // Final should be `<dest> enqueue`
		messagingDestinationTypeEventKey = "messaging.solace.destination_type"
		statusMessageEventKey            = "messaging.solace.enqueue_error_message"
		rejectsAllEnqueuesKey            = "messaging.solace.rejects_all_enqueues"
		partitionNumberKey               = "messaging.solace.partition_number"
	)
	var destinationName string
	var destinationType string
	switch casted := enqueueEvent.Dest.(type) {
	case *receive_v1.SpanData_EnqueueEvent_TopicEndpointName:
		destinationName = casted.TopicEndpointName
		destinationType = topicEndpointKind
	case *receive_v1.SpanData_EnqueueEvent_QueueName:
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
func (u *brokerTraceReceiveUnmarshallerV1) mapTransactionEvent(transactionEvent *receive_v1.SpanData_TransactionEvent, clientEvent ptrace.SpanEvent) {
	// map the transaction type to a name
	var name string
	switch transactionEvent.GetType() {
	case receive_v1.SpanData_TransactionEvent_COMMIT:
		name = "commit"
	case receive_v1.SpanData_TransactionEvent_ROLLBACK:
		name = "rollback"
	case receive_v1.SpanData_TransactionEvent_END:
		name = "end"
	case receive_v1.SpanData_TransactionEvent_PREPARE:
		name = "prepare"
	case receive_v1.SpanData_TransactionEvent_SESSION_TIMEOUT:
		name = "session_timeout"
	case receive_v1.SpanData_TransactionEvent_ROLLBACK_ONLY:
		name = "rollback_only"
	default:
		// Set the name to the unknown transaction event type to ensure forward compat.
		name = fmt.Sprintf("Unknown Transaction Event (%s)", transactionEvent.GetType().String())
		u.logger.Warn(fmt.Sprintf("Received span with unknown transaction event %s", transactionEvent.GetType()))
		u.metrics.recordRecoverableUnmarshallingError()
	}
	clientEvent.SetName(name)
	clientEvent.SetTimestamp(pcommon.Timestamp(transactionEvent.TimeUnixNano))
	// map initiator enums to expected initiator strings
	var initiator string
	switch transactionEvent.GetInitiator() {
	case receive_v1.SpanData_TransactionEvent_CLIENT:
		initiator = "client"
	case receive_v1.SpanData_TransactionEvent_ADMIN:
		initiator = "administrator"
	case receive_v1.SpanData_TransactionEvent_BROKER:
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
	case *receive_v1.SpanData_TransactionEvent_LocalId:
		clientEvent.Attributes().PutInt(transactionIDEventKey, int64(casted.LocalId.TransactionId))
		clientEvent.Attributes().PutStr(transactedSessionNameEventKey, casted.LocalId.SessionName)
		clientEvent.Attributes().PutInt(transactedSessionIDEventKey, int64(casted.LocalId.SessionId))
	case *receive_v1.SpanData_TransactionEvent_Xid_:
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
func (u *brokerTraceReceiveUnmarshallerV1) insertUserProperty(toMap pcommon.Map, key string, value any) {
	const (
		// userPropertiesPrefixAttrKey is the key used to prefix all user properties
		userPropertiesAttrKeyPrefix = "messaging.solace.user_properties."
	)
	k := userPropertiesAttrKeyPrefix + key
	switch v := value.(type) {
	case *receive_v1.SpanData_UserPropertyValue_NullValue:
		toMap.PutEmpty(k)
	case *receive_v1.SpanData_UserPropertyValue_BoolValue:
		toMap.PutBool(k, v.BoolValue)
	case *receive_v1.SpanData_UserPropertyValue_DoubleValue:
		toMap.PutDouble(k, v.DoubleValue)
	case *receive_v1.SpanData_UserPropertyValue_ByteArrayValue:
		toMap.PutEmptyBytes(k).FromRaw(v.ByteArrayValue)
	case *receive_v1.SpanData_UserPropertyValue_FloatValue:
		toMap.PutDouble(k, float64(v.FloatValue))
	case *receive_v1.SpanData_UserPropertyValue_Int8Value:
		toMap.PutInt(k, int64(v.Int8Value))
	case *receive_v1.SpanData_UserPropertyValue_Int16Value:
		toMap.PutInt(k, int64(v.Int16Value))
	case *receive_v1.SpanData_UserPropertyValue_Int32Value:
		toMap.PutInt(k, int64(v.Int32Value))
	case *receive_v1.SpanData_UserPropertyValue_Int64Value:
		toMap.PutInt(k, v.Int64Value)
	case *receive_v1.SpanData_UserPropertyValue_Uint8Value:
		toMap.PutInt(k, int64(v.Uint8Value))
	case *receive_v1.SpanData_UserPropertyValue_Uint16Value:
		toMap.PutInt(k, int64(v.Uint16Value))
	case *receive_v1.SpanData_UserPropertyValue_Uint32Value:
		toMap.PutInt(k, int64(v.Uint32Value))
	case *receive_v1.SpanData_UserPropertyValue_Uint64Value:
		toMap.PutInt(k, int64(v.Uint64Value))
	case *receive_v1.SpanData_UserPropertyValue_StringValue:
		toMap.PutStr(k, v.StringValue)
	case *receive_v1.SpanData_UserPropertyValue_DestinationValue:
		toMap.PutStr(k, v.DestinationValue)
	case *receive_v1.SpanData_UserPropertyValue_CharacterValue:
		toMap.PutStr(k, string(rune(v.CharacterValue)))
	default:
		u.logger.Warn(fmt.Sprintf("Unknown user property type: %T", v))
		u.metrics.recordRecoverableUnmarshallingError()
	}
}
