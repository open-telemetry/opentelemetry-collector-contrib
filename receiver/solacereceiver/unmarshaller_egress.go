// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"encoding/hex"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	egress_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/model/egress/v1"
)

type brokerTraceEgressUnmarshallerV1 struct {
	logger  *zap.Logger
	metrics *opencensusMetrics
}

func (u *brokerTraceEgressUnmarshallerV1) unmarshal(message *inboundMessage) (ptrace.Traces, error) {
	spanData, err := u.unmarshalToSpanData(message)
	if err != nil {
		return ptrace.Traces{}, err
	}
	traces := ptrace.NewTraces()
	u.populateTraces(spanData, traces)
	return traces, nil
}

func (u *brokerTraceEgressUnmarshallerV1) unmarshalToSpanData(message *inboundMessage) (*egress_v1.SpanData, error) {
	var data = message.GetData()
	if len(data) == 0 {
		return nil, errEmptyPayload
	}
	var spanData egress_v1.SpanData
	if err := proto.Unmarshal(data, &spanData); err != nil {
		return nil, err
	}
	return &spanData, nil
}

func (u *brokerTraceEgressUnmarshallerV1) populateTraces(spanData *egress_v1.SpanData, traces ptrace.Traces) {
	// Append new resource span and map any attributes
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	u.mapResourceSpanAttributes(spanData, resourceSpan.Resource().Attributes())
	instrLibrarySpans := resourceSpan.ScopeSpans().AppendEmpty()
	for _, egressSpanData := range spanData.EgressSpans {
		u.mapEgressSpan(egressSpanData, instrLibrarySpans.Spans())
	}
}

func (u *brokerTraceEgressUnmarshallerV1) mapResourceSpanAttributes(spanData *egress_v1.SpanData, attrMap pcommon.Map) {
	setResourceSpanAttributes(attrMap, spanData.RouterName, spanData.SolosVersion, spanData.MessageVpnName)
}

func (u *brokerTraceEgressUnmarshallerV1) mapEgressSpan(spanData *egress_v1.SpanData_EgressSpan, clientSpans ptrace.SpanSlice) {
	if spanData.GetSendSpan() != nil {
		clientSpan := clientSpans.AppendEmpty()
		u.mapEgressSpanCommon(spanData, clientSpan)
		u.mapSendSpan(spanData.GetSendSpan(), clientSpan)
		if transactionEvent := spanData.GetTransactionEvent(); transactionEvent != nil {
			u.mapTransactionEvent(transactionEvent, clientSpan.Events().AppendEmpty())
		}
	} else {
		// unknown span type, drop the span
		u.logger.Warn("Received egress span with unknown span type, is the collector out of date?")
		u.metrics.recordDroppedEgressSpan()
	}
}

func (u *brokerTraceEgressUnmarshallerV1) mapEgressSpanCommon(spanData *egress_v1.SpanData_EgressSpan, clientSpan ptrace.Span) {
	var traceID [16]byte
	copy(traceID[:16], spanData.TraceId)
	clientSpan.SetTraceID(traceID)
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
	if spanData.ErrorDescription != nil {
		clientSpan.Status().SetCode(ptrace.StatusCodeError)
		clientSpan.Status().SetMessage(*spanData.ErrorDescription)
	}
}

func (u *brokerTraceEgressUnmarshallerV1) mapSendSpan(sendSpan *egress_v1.SpanData_SendSpan, span ptrace.Span) {
	const (
		sourceNameKey = "messaging.source.name"
		sourceKindKey = "messaging.source.kind"
		replayedKey   = "messaging.solace.message_replayed"
		outcomeKey    = "messaging.solace.send.outcome"
	)
	const (
		sendSpanOperation = "send"
		sendNameSuffix    = " send"
		unknownSendName   = "(unknown)"
		anonymousSendName = "(anonymous)"
	)
	// hard coded to producer span
	span.SetKind(ptrace.SpanKindProducer)

	attributes := span.Attributes()
	attributes.PutStr(systemAttrKey, systemAttrValue)
	attributes.PutStr(operationAttrKey, sendSpanOperation)
	attributes.PutStr(protocolAttrKey, sendSpan.Protocol)
	if sendSpan.ProtocolVersion != nil {
		attributes.PutStr(protocolVersionAttrKey, *sendSpan.ProtocolVersion)
	}
	// we don't fatal out when we don't have a valid kind, instead just log and increment stats
	var name string
	switch casted := sendSpan.Source.(type) {
	case *egress_v1.SpanData_SendSpan_TopicEndpointName:
		if isAnonymousTopicEndpoint(casted.TopicEndpointName) {
			name = anonymousSendName
		} else {
			name = casted.TopicEndpointName
		}
		attributes.PutStr(sourceNameKey, casted.TopicEndpointName)
		attributes.PutStr(sourceKindKey, topicEndpointKind)
	case *egress_v1.SpanData_SendSpan_QueueName:
		if isAnonymousQueue(casted.QueueName) {
			name = anonymousSendName
		} else {
			name = casted.QueueName
		}
		attributes.PutStr(sourceNameKey, casted.QueueName)
		attributes.PutStr(sourceKindKey, queueKind)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown source type %T", casted))
		u.metrics.recordRecoverableUnmarshallingError()
		name = unknownSendName
	}
	span.SetName(name + sendNameSuffix)

	attributes.PutStr(clientUsernameAttrKey, sendSpan.ConsumerClientUsername)
	attributes.PutStr(clientNameAttrKey, sendSpan.ConsumerClientName)
	attributes.PutBool(replayedKey, sendSpan.ReplayedMsg)

	var outcome string
	switch sendSpan.Outcome {
	case egress_v1.SpanData_SendSpan_ACCEPTED:
		outcome = "accepted"
	case egress_v1.SpanData_SendSpan_REJECTED:
		outcome = "rejected"
	case egress_v1.SpanData_SendSpan_RELEASED:
		outcome = "released"
	case egress_v1.SpanData_SendSpan_DELIVERY_FAILED:
		outcome = "delivery failed"
	case egress_v1.SpanData_SendSpan_FLOW_UNBOUND:
		outcome = "flow unbound"
	case egress_v1.SpanData_SendSpan_TRANSACTION_COMMIT:
		outcome = "transaction commit"
	case egress_v1.SpanData_SendSpan_TRANSACTION_COMMIT_FAILED:
		outcome = "transaction commit failed"
	case egress_v1.SpanData_SendSpan_TRANSACTION_ROLLBACK:
		outcome = "transaction rollback"
	}
	attributes.PutStr(outcomeKey, outcome)
}

// maps a transaction event. We cannot reuse the code in receive unmarshaller since
// the protobuf model is different and the return type for things like type and initiator would not work in an interface
func (u *brokerTraceEgressUnmarshallerV1) mapTransactionEvent(transactionEvent *egress_v1.SpanData_TransactionEvent, clientEvent ptrace.SpanEvent) {
	// map the transaction type to a name
	var name string
	switch transactionEvent.GetType() {
	case egress_v1.SpanData_TransactionEvent_COMMIT:
		name = "commit"
	case egress_v1.SpanData_TransactionEvent_ROLLBACK:
		name = "rollback"
	case egress_v1.SpanData_TransactionEvent_END:
		name = "end"
	case egress_v1.SpanData_TransactionEvent_PREPARE:
		name = "prepare"
	case egress_v1.SpanData_TransactionEvent_SESSION_TIMEOUT:
		name = "session_timeout"
	case egress_v1.SpanData_TransactionEvent_ROLLBACK_ONLY:
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
	case egress_v1.SpanData_TransactionEvent_CLIENT:
		initiator = "client"
	case egress_v1.SpanData_TransactionEvent_ADMIN:
		initiator = "administrator"
	case egress_v1.SpanData_TransactionEvent_BROKER:
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
	case *egress_v1.SpanData_TransactionEvent_LocalId:
		clientEvent.Attributes().PutInt(transactionIDEventKey, int64(casted.LocalId.TransactionId))
		clientEvent.Attributes().PutStr(transactedSessionNameEventKey, casted.LocalId.SessionName)
		clientEvent.Attributes().PutInt(transactedSessionIDEventKey, int64(casted.LocalId.SessionId))
	case *egress_v1.SpanData_TransactionEvent_Xid_:
		// format xxxxxxxx-yyyyyyyy-zzzzzzzz where x is FormatID (hex rep of int32), y is BranchQualifier and z is GlobalID, hex encoded.
		xidString := fmt.Sprintf("%08x", casted.Xid.FormatId) + "-" +
			hex.EncodeToString(casted.Xid.BranchQualifier) + "-" + hex.EncodeToString(casted.Xid.GlobalId)
		clientEvent.Attributes().PutStr(transactionXIDEventKey, xidString)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown transaction ID type %T", transactionID))
		u.metrics.recordRecoverableUnmarshallingError()
	}
}

func isAnonymousQueue(name string) bool {
	// all anonymous queues start with the prefix #P2P/QTMP
	const anonymousQueuePrefix = "#P2P/QTMP"
	return strings.HasPrefix(name, anonymousQueuePrefix)
}

func isAnonymousTopicEndpoint(name string) bool {
	// all anonymous topic endpoints are made up of hex strings of length 32
	if len(name) != 32 {
		return false
	}
	for _, c := range []byte(name) { // []byte casting is more efficient in this loop
		// check if we are outside 0-9 AND outside a-f
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
