// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
)

// tracesUnmarshaller deserializes the message body.
type tracesUnmarshaller interface {
	// unmarshal the amqp-message into traces.
	// Only valid traces are produced or error is returned
	unmarshal(message *inboundMessage) (ptrace.Traces, error)
}

// newTracesUnmarshaller returns a new unmarshaller ready for message unmarshalling
func newTracesUnmarshaller(logger *zap.Logger, telemetryBuilder *metadata.TelemetryBuilder, metricAttrs attribute.Set) tracesUnmarshaller {
	return &solaceTracesUnmarshaller{
		logger:           logger,
		telemetryBuilder: telemetryBuilder,
		// v1 unmarshaller is implemented by solaceMessageUnmarshallerV1
		moveUnmarshallerV1: &brokerTraceMoveUnmarshallerV1{
			logger:           logger,
			telemetryBuilder: telemetryBuilder,
			metricAttrs:      metricAttrs,
		},
		receiveUnmarshallerV1: &brokerTraceReceiveUnmarshallerV1{
			logger:           logger,
			telemetryBuilder: telemetryBuilder,
			metricAttrs:      metricAttrs,
		},
		egressUnmarshallerV1: &brokerTraceEgressUnmarshallerV1{
			logger:           logger,
			telemetryBuilder: telemetryBuilder,
			metricAttrs:      metricAttrs,
		},
	}
}

// solaceTracesUnmarshaller implements tracesUnmarshaller.
type solaceTracesUnmarshaller struct {
	logger                *zap.Logger
	telemetryBuilder      *metadata.TelemetryBuilder
	moveUnmarshallerV1    tracesUnmarshaller
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
		moveSpanPrefix    = "broker/trace/move/"
		receiveSpanPrefix = "broker/trace/receive/"
		egressSpanPrefix  = "broker/trace/egress/"
		v1Suffix          = "v1"
	)
	if message.Properties == nil || message.Properties.To == nil {
		// no topic
		u.logger.Error("Received message with no topic")
		return ptrace.Traces{}, errUnknownTopic
	}
	topic := *message.Properties.To
	// Multiplex the topic string. For now we only have a single type handled
	if strings.HasPrefix(topic, topicPrefix) {
		// we are a telemetry string
		if strings.HasPrefix(topic[len(topicPrefix):], moveSpanPrefix) {
			// we are handling a move span, validate the version is v1
			if strings.HasSuffix(topic, v1Suffix) {
				return u.moveUnmarshallerV1.unmarshal(message)
			}
			// otherwise we are an unknown version
			u.logger.Error("Received message with unsupported move span version, an upgrade is required", zap.String("topic", *message.Properties.To))
		} else {
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
					// otherwise we are an unknown version
					u.logger.Error("Received message with unsupported egress span version, an upgrade is required", zap.String("topic", *message.Properties.To))
				} else {
					u.logger.Error("Received message with unsupported topic, an upgrade is required", zap.String("topic", *message.Properties.To))
				}
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
	protocolAttrKey                    = "network.protocol.name"
	protocolVersionAttrKey             = "network.protocol.version"
	messageIDAttrKey                   = "messaging.message.id"
	conversationIDAttrKey              = "messaging.message.conversation_id"
	messageBodySizeBytesAttrKey        = "messaging.message.body.size"
	messageEnvelopeSizeBytesAttrKey    = "messaging.message.envelope.size"
	destinationNameAttrKey             = "messaging.destination.name"
	destinationTypeAttrKey             = "messaging.solace.destination.type"
	clientUsernameAttrKey              = "messaging.solace.client_username"
	clientNameAttrKey                  = "messaging.solace.client_name"
	partitionNumberKey                 = "messaging.solace.partition_number"
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
	hostIPAttrKey                      = "server.address"
	hostPortAttrKey                    = "server.port"
	peerIPAttrKey                      = "network.peer.address"
	peerPortAttrKey                    = "network.peer.port"
)

// constant attributes
const (
	systemAttrKey        = "messaging.system"
	systemAttrValue      = "SolacePubSub+"
	operationNameAttrKey = "messaging.operation.name"
	operationTypeAttrKey = "messaging.operation.type"
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

func rgmidToString(rgmid []byte, otelMetricAttrs attribute.Set, telemetryBuilder *metadata.TelemetryBuilder, logger *zap.Logger) string {
	// rgmid[0] is the version of the rgmid
	if len(rgmid) != 17 || rgmid[0] != 1 {
		// may be cases where the rgmid is empty or nil, len(rgmid) will return 0 if nil
		if len(rgmid) > 0 {
			logger.Warn("Received invalid length or version for rgmid", zap.Int8("version", int8(rgmid[0])), zap.Int("length", len(rgmid)))
			telemetryBuilder.SolacereceiverRecoverableUnmarshallingErrors.Add(context.Background(), 1, metric.WithAttributeSet(otelMetricAttrs))
		}
		return hex.EncodeToString(rgmid)
	}
	rgmidEncoded := make([]byte, 32)
	hex.Encode(rgmidEncoded, rgmid[1:])
	// format: rmid1:aaaaa-bbbbbbbbbbb-cccccccc-dddddddd
	rgmidString := "rmid1:" + string(rgmidEncoded[0:5]) + "-" + string(rgmidEncoded[5:16]) + "-" + string(rgmidEncoded[16:24]) + "-" + string(rgmidEncoded[24:32])
	return rgmidString
}
