// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
	move_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/model/move/v1"
)

type brokerTraceMoveUnmarshallerV1 struct {
	logger           *zap.Logger
	telemetryBuilder *metadata.TelemetryBuilder
	metricAttrs      attribute.Set // other Otel attributes (to add to the metrics)
}

// unmarshal implements tracesUnmarshaller.unmarshal
func (u *brokerTraceMoveUnmarshallerV1) unmarshal(message *inboundMessage) (ptrace.Traces, error) {
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
func (u *brokerTraceMoveUnmarshallerV1) unmarshalToSpanData(message *inboundMessage) (*move_v1.SpanData, error) {
	data := message.GetData()
	if len(data) == 0 {
		return nil, errEmptyPayload
	}
	var spanData move_v1.SpanData
	if err := proto.Unmarshal(data, &spanData); err != nil {
		return nil, err
	}
	return &spanData, nil
}

// populateTraces will create a new Span from the given traces and map the given SpanData to the span.
// This will set all required fields such as name version, trace and span ID, parent span ID (if applicable),
// timestamps, errors and states.
func (u *brokerTraceMoveUnmarshallerV1) populateTraces(spanData *move_v1.SpanData, traces ptrace.Traces) {
	// Append new resource span and map any attributes
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	u.mapResourceSpanAttributes(spanData, resourceSpan.Resource().Attributes())
	instrLibrarySpans := resourceSpan.ScopeSpans().AppendEmpty()
	// Create a new span
	clientSpan := instrLibrarySpans.Spans().AppendEmpty()
	// map the tracing data for the span
	u.mapMoveSpanTracingInfo(spanData, clientSpan)
	// map the basic span data
	u.mapClientSpanData(spanData, clientSpan)
}

func (u *brokerTraceMoveUnmarshallerV1) mapResourceSpanAttributes(spanData *move_v1.SpanData, attrMap pcommon.Map) {
	setResourceSpanAttributes(attrMap, spanData.RouterName, spanData.SolosVersion, spanData.MessageVpnName)
}

func (u *brokerTraceMoveUnmarshallerV1) mapMoveSpanTracingInfo(spanData *move_v1.SpanData, span ptrace.Span) {
	// hard coded to internal span
	// SPAN_KIND_CONSUMER == 1
	span.SetKind(ptrace.SpanKindInternal)

	// map trace ID
	var traceID [16]byte
	copy(traceID[:16], spanData.TraceId)
	span.SetTraceID(traceID)
	// map span ID
	var spanID [8]byte
	copy(spanID[:8], spanData.SpanId)
	span.SetSpanID(spanID)
	// conditional parent-span-id
	if len(spanData.ParentSpanId) == 8 {
		var parentSpanID [8]byte
		copy(parentSpanID[:8], spanData.ParentSpanId)
		span.SetParentSpanID(parentSpanID)
	}
	// timestamps
	span.SetStartTimestamp(pcommon.Timestamp(spanData.GetStartTimeUnixNano()))
	span.SetEndTimestamp(pcommon.Timestamp(spanData.GetEndTimeUnixNano()))
}

func (u *brokerTraceMoveUnmarshallerV1) mapClientSpanData(moveSpan *move_v1.SpanData, span ptrace.Span) {
	const (
		sourceNameKey                 = "messaging.source.name"
		sourceKindKey                 = "messaging.solace.source.kind"
		destinationNameKey            = "messaging.destination.name"
		moveOperationReasonKey        = "messaging.solace.operation.reason"
		sourcePartitionNumberKey      = "messaging.solace.source.partition_number"
		destinationPartitionNumberKey = "messaging.solace.destination.partition_number"
	)
	const (
		spanOperationName     = "move"
		spanOperationType     = "move"
		moveNameSuffix        = " move"
		unknownEndpointName   = "(unknown)"
		anonymousEndpointName = "(anonymous)"
	)
	// Delete Info reasons
	const (
		ttlExpired              = "ttl_expired"
		rejectedNack            = "rejected_nack"
		maxRedeliveriesExceeded = "max_redeliveries_exceeded"
	)

	attributes := span.Attributes()
	attributes.PutStr(systemAttrKey, systemAttrValue)
	attributes.PutStr(operationNameAttrKey, spanOperationName)
	attributes.PutStr(operationTypeAttrKey, spanOperationType)

	// map the replication group ID for the move span
	rgmid := rgmidToString(moveSpan.ReplicationGroupMessageId, u.metricAttrs, u.telemetryBuilder, u.logger)
	if len(rgmid) > 0 {
		attributes.PutStr(replicationGroupMessageIDAttrKey, rgmid)
	}

	// source queue partition number
	if moveSpan.SourcePartitionNumber != nil {
		attributes.PutInt(sourcePartitionNumberKey, int64(*moveSpan.SourcePartitionNumber))
	}

	// destination queue partition number
	if moveSpan.DestinationPartitionNumber != nil {
		attributes.PutInt(destinationPartitionNumberKey, int64(*moveSpan.DestinationPartitionNumber))
	}

	// set source endpoint information
	// don't fatal out when we receive invalid endpoint name, instead just log and increment stats
	var sourceEndpointName string
	switch casted := moveSpan.Source.(type) {
	case *move_v1.SpanData_SourceTopicEndpointName:
		if isAnonymousTopicEndpoint(casted.SourceTopicEndpointName) {
			sourceEndpointName = anonymousEndpointName
		} else {
			sourceEndpointName = casted.SourceTopicEndpointName
		}
		attributes.PutStr(sourceNameKey, casted.SourceTopicEndpointName)
		attributes.PutStr(sourceKindKey, topicEndpointKind)
	case *move_v1.SpanData_SourceQueueName:
		if isAnonymousQueue(casted.SourceQueueName) {
			sourceEndpointName = anonymousEndpointName
		} else {
			sourceEndpointName = casted.SourceQueueName
		}
		attributes.PutStr(sourceNameKey, casted.SourceQueueName)
		attributes.PutStr(sourceKindKey, queueKind)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown source endpoint type %T", casted))
		u.telemetryBuilder.SolacereceiverRecoverableUnmarshallingErrors.Add(context.Background(), 1, metric.WithAttributeSet(u.metricAttrs))
		sourceEndpointName = unknownEndpointName
	}
	span.SetName(sourceEndpointName + moveNameSuffix)

	// set destination endpoint information
	// don't fatal out when we receive invalid endpoint name, instead just log and increment stats
	switch casted := moveSpan.Destination.(type) {
	case *move_v1.SpanData_DestinationTopicEndpointName:
		attributes.PutStr(destinationNameKey, casted.DestinationTopicEndpointName)
		attributes.PutStr(destinationTypeAttrKey, topicEndpointKind)
	case *move_v1.SpanData_DestinationQueueName:
		attributes.PutStr(destinationNameKey, casted.DestinationQueueName)
		attributes.PutStr(destinationTypeAttrKey, queueKind)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown endpoint type %T", casted))
		u.telemetryBuilder.SolacereceiverRecoverableUnmarshallingErrors.Add(context.Background(), 1, metric.WithAttributeSet(u.metricAttrs))
	}

	// do not fatal out when we don't have a valid move reason name
	// instead just log and increment stats
	switch casted := moveSpan.TypeInfo.(type) {
	// caused by expired ttl on message
	case *move_v1.SpanData_TtlExpiredInfo:
		attributes.PutStr(moveOperationReasonKey, ttlExpired)
	// caused by consumer N(ack)ing with Rejected outcome
	case *move_v1.SpanData_RejectedOutcomeInfo:
		attributes.PutStr(moveOperationReasonKey, rejectedNack)
	// caused by max redelivery reached/exceeded
	case *move_v1.SpanData_MaxRedeliveriesInfo:
		attributes.PutStr(moveOperationReasonKey, maxRedeliveriesExceeded)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown move reason info type %T", casted))
		u.telemetryBuilder.SolacereceiverRecoverableUnmarshallingErrors.Add(context.Background(), 1, metric.WithAttributeSet(u.metricAttrs))
	}
}
