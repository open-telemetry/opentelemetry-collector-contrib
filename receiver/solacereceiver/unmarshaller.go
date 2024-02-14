// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
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
		egressUnmarshallerV1: &brokerTraceEgressUnmarshallerV1{
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
	egressUnmarshallerV1  tracesUnmarshaller
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
		egressSpanPrefix  = "broker/trace/egress/"
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
		} else { // make lint happy, wants two boolean expressions to be written as a switch?!
			if strings.HasPrefix(topic[len(topicPrefix):], egressSpanPrefix) {
				if strings.HasSuffix(topic, v1Suffix) {
					return u.egressUnmarshallerV1.unmarshal(message)
				}
			} else {
				u.logger.Error("Received message with unsupported topic, an upgrade is required", zap.String("topic", *message.Properties.To))
			}
		}
		// if we don't know the type, we must upgrade
		return ptrace.Traces{}, errUpgradeRequired
	}
	// unknown topic, do not require an upgrade
	u.logger.Error("Received message with unknown topic", zap.String("topic", *message.Properties.To))
	return ptrace.Traces{}, errUnknownTopic
}

// common helper functions used by all unmarshallers

// Endpoint types
const (
	queueKind         = "queue"
	topicEndpointKind = "topic-endpoint"
)

// Transaction event keys
const (
	transactionInitiatorEventKey    = "messaging.solace.transaction_initiator"
	transactionIDEventKey           = "messaging.solace.transaction_id"
	transactedSessionNameEventKey   = "messaging.solace.transacted_session_name"
	transactedSessionIDEventKey     = "messaging.solace.transacted_session_id"
	transactionErrorMessageEventKey = "messaging.solace.transaction_error_message"
	transactionXIDEventKey          = "messaging.solace.transaction_xid"
)

// span keys
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

// constant attributes
const (
	systemAttrKey    = "messaging.system"
	systemAttrValue  = "SolacePubSub+"
	operationAttrKey = "messaging.operation"
)

func setResourceSpanAttributes(attrMap pcommon.Map, routerName, version string, messageVpnName *string) {
	const (
		routerNameAttrKey     = "service.name"
		messageVpnNameAttrKey = "service.instance.id"
		solosVersionAttrKey   = "service.version"
	)
	attrMap.PutStr(routerNameAttrKey, routerName)
	attrMap.PutStr(solosVersionAttrKey, version)
	if messageVpnName != nil {
		attrMap.PutStr(messageVpnNameAttrKey, *messageVpnName)
	}
}
