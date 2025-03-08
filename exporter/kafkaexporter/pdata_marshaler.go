// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.uber.org/multierr"
)

type pdataLogsMarshaler struct {
	marshaler              plog.Marshaler
	encoding               string
	partitionedByResources bool
}

func (p pdataLogsMarshaler) Marshal(ld plog.Logs, topic string) ([]*sarama.ProducerMessage, error) {
	var msgs []*sarama.ProducerMessage
	if p.partitionedByResources {
		logs := ld.ResourceLogs()

		for i := 0; i < logs.Len(); i++ {
			resourceMetrics := logs.At(i)
			hash := pdatautil.MapHash(resourceMetrics.Resource().Attributes())

			newLogs := plog.NewLogs()
			resourceMetrics.CopyTo(newLogs.ResourceLogs().AppendEmpty())

			bts, err := p.marshaler.MarshalLogs(newLogs)
			if err != nil {
				return nil, err
			}
			msgs = append(msgs, &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(bts),
				Key:   sarama.ByteEncoder(hash[:]),
			})
		}
	} else {
		bts, err := p.marshaler.MarshalLogs(ld)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(bts),
		})
	}
	return msgs, nil
}

func (p pdataLogsMarshaler) Encoding() string {
	return p.encoding
}

func newPdataLogsMarshaler(marshaler plog.Marshaler, encoding string, partitionedByResources bool) LogsMarshaler {
	return pdataLogsMarshaler{
		marshaler:              marshaler,
		encoding:               encoding,
		partitionedByResources: partitionedByResources,
	}
}

type pdataMetricsMarshaler struct {
	marshaler              pmetric.Marshaler
	encoding               string
	partitionedByResources bool
}

func (p pdataMetricsMarshaler) Marshal(ld pmetric.Metrics, topic string) ([]*sarama.ProducerMessage, error) {
	var msgs []*sarama.ProducerMessage
	if p.partitionedByResources {
		metrics := ld.ResourceMetrics()

		for i := 0; i < metrics.Len(); i++ {
			resourceMetrics := metrics.At(i)
			hash := pdatautil.MapHash(resourceMetrics.Resource().Attributes())

			newMetrics := pmetric.NewMetrics()
			resourceMetrics.CopyTo(newMetrics.ResourceMetrics().AppendEmpty())

			bts, err := p.marshaler.MarshalMetrics(newMetrics)
			if err != nil {
				return nil, err
			}
			msgs = append(msgs, &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(bts),
				Key:   sarama.ByteEncoder(hash[:]),
			})
		}
	} else {
		bts, err := p.marshaler.MarshalMetrics(ld)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(bts),
		})
	}

	return msgs, nil
}

func (p pdataMetricsMarshaler) Encoding() string {
	return p.encoding
}

func newPdataMetricsMarshaler(marshaler pmetric.Marshaler, encoding string, partitionedByResources bool) MetricsMarshaler {
	return &pdataMetricsMarshaler{
		marshaler:              marshaler,
		encoding:               encoding,
		partitionedByResources: partitionedByResources,
	}
}

type pdataTracesMarshaler struct {
	marshaler            ptrace.Marshaler
	encoding             string
	partitionedByTraceID bool
	maxMessageBytes      int
}

func (p *pdataTracesMarshaler) processTrace(
	trace ptrace.Traces,
	topic string,
	key []byte,
	messages *[]*sarama.ProducerMessage,
	messageChunks *[]*ProducerMessageChunks,
	packetSize *int) error {

	if trace.ResourceSpans().Len() == 0 {
		return nil
	}

	bts, err := p.marshaler.MarshalTraces(trace)
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(bts),
		Key:   sarama.ByteEncoder(key),
	}

	// Deriving version from sarama library
	// https://github.com/IBM/sarama/blob/main/async_producer.go#L454
	currentMsgSize := msg.ByteSize(2)

	if (*packetSize + currentMsgSize) <= p.maxMessageBytes {
		*packetSize += currentMsgSize
		*messages = append(*messages, msg)
		return nil
	}

	if len(*messages) > 0 {
		*messageChunks = append(*messageChunks, &ProducerMessageChunks{*messages})
		*messages = make([]*sarama.ProducerMessage, 0)
		*packetSize = 0
	}

	// If current message itself exceeds limit, split it
	if currentMsgSize > p.maxMessageBytes {
		remaining := trace
		for {
			splitPoint, err := p.findSplitPoint(remaining)
			if err != nil {
				return fmt.Errorf("failed to find split point: %w", err)
			}

			// If we can't split further (single span exceeds limit)
			if splitPoint < 1 {
				return fmt.Errorf("single span size %d exceeds maximum message size %d", currentMsgSize, p.maxMessageBytes)
			}

			// Split and process first half
			firstHalf, secondHalf := p.splitTraceBySpans(remaining, 0, splitPoint)

			bts, err := p.marshaler.MarshalTraces(firstHalf)
			if err != nil {
				return fmt.Errorf("failed to marshal first half: %w", err)
			}

			splitMsg := &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(bts),
				Key:   sarama.ByteEncoder(key),
			}

			*messageChunks = append(*messageChunks, &ProducerMessageChunks{[]*sarama.ProducerMessage{splitMsg}})

			remaining = secondHalf
			if remaining.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len() == 0 {
				break
			}

			bts, err = p.marshaler.MarshalTraces(remaining)
			if err != nil {
				return fmt.Errorf("failed to marshal remaining half: %w", err)
			}

			remainingSize := (&sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(bts),
				Key:   sarama.ByteEncoder(key),
			}).ByteSize(2)

			if remainingSize <= p.maxMessageBytes {
				*messages = append(*messages, &sarama.ProducerMessage{
					Topic: topic,
					Value: sarama.ByteEncoder(bts),
					Key:   sarama.ByteEncoder(key),
				})
				*packetSize = remainingSize
				break
			}
		}
		return nil
	}

	*messages = append(*messages, msg)
	*packetSize = currentMsgSize
	return nil
}

func (p *pdataTracesMarshaler) Marshal(td ptrace.Traces, topic string) ([]*ProducerMessageChunks, error) {
	if td.ResourceSpans().Len() == 0 {
		return []*ProducerMessageChunks{}, nil
	}

	if p.maxMessageBytes <= 0 {
		return nil, fmt.Errorf("maxMessageBytes must be positive, got %d", p.maxMessageBytes)
	}

	var messageChunks []*ProducerMessageChunks
	var messages []*sarama.ProducerMessage
	var packetSize int
	var errs error

	if p.partitionedByTraceID {
		traces := batchpersignal.SplitTraces(td)
		if len(traces) == 0 {
			return []*ProducerMessageChunks{}, nil
		}

		for _, trace := range traces {
			if trace.ResourceSpans().Len() == 0 ||
				trace.ResourceSpans().At(0).ScopeSpans().Len() == 0 ||
				trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len() == 0 {
				continue
			}

			key := traceutil.TraceIDToHexOrEmptyString(trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
			if err := p.processTrace(trace, topic, []byte(key), &messages, &messageChunks, &packetSize); err != nil {
				errs = multierr.Append(errs, fmt.Errorf("failed to process trace with ID %s: %w", key, err))
				continue
			}
		}
	} else {
		if err := p.processTrace(td, topic, nil, &messages, &messageChunks, &packetSize); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to process batch: %w", err))
		}
	}

	// Add any remaining messages as final chunk
	if len(messages) > 0 {
		messageChunks = append(messageChunks, &ProducerMessageChunks{messages})
	}

	return messageChunks, errs
}

func (p *pdataTracesMarshaler) Encoding() string {
	return p.encoding
}

func newPdataTracesMarshaler(marshaler ptrace.Marshaler, encoding string, partitionedByTraceID bool, maxMessageBytes int) TracesMarshaler {
	return &pdataTracesMarshaler{
		marshaler:            marshaler,
		encoding:             encoding,
		partitionedByTraceID: partitionedByTraceID,
		maxMessageBytes:      maxMessageBytes,
	}
}

// findSplitPoint uses binary search to find the maximum number of spans that can fit within maxMessageBytes
func (p *pdataTracesMarshaler) findSplitPoint(trace ptrace.Traces) (int, error) {
	if trace.ResourceSpans().Len() == 0 ||
		trace.ResourceSpans().At(0).ScopeSpans().Len() == 0 {
		return 0, fmt.Errorf("trace contains no spans")
	}

	spans := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	totalSpans := spans.Len()

	if totalSpans == 0 {
		return 0, fmt.Errorf("scope contains no spans")
	}

	// If single span, check if it fits
	if totalSpans == 1 {
		testTrace := ptrace.NewTraces()
		trace.ResourceSpans().At(0).Resource().CopyTo(testTrace.ResourceSpans().AppendEmpty().Resource())
		scope := testTrace.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
		trace.ResourceSpans().At(0).ScopeSpans().At(0).Scope().CopyTo(scope.Scope())
		spans.At(0).CopyTo(scope.Spans().AppendEmpty())

		bts, err := p.marshaler.MarshalTraces(testTrace)
		if err != nil {
			return 0, err
		}

		if (&sarama.ProducerMessage{Value: sarama.ByteEncoder(bts)}).ByteSize(2) <= p.maxMessageBytes {
			return 1, nil
		}
		return 0, fmt.Errorf("single span exceeds maximum message size")
	}

	left := 1
	right := totalSpans

	// Binary search to find the largest number of spans that fits within maxMessageBytes
	for left < right {
		mid := (left + right + 1) / 2

		testTrace := ptrace.NewTraces()
		trace.ResourceSpans().At(0).Resource().CopyTo(testTrace.ResourceSpans().AppendEmpty().Resource())
		scope := testTrace.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
		trace.ResourceSpans().At(0).ScopeSpans().At(0).Scope().CopyTo(scope.Scope())

		for i := 0; i < mid; i++ {
			spans.At(i).CopyTo(scope.Spans().AppendEmpty())
		}

		bts, err := p.marshaler.MarshalTraces(testTrace)
		if err != nil {
			return 0, err
		}

		msgSize := (&sarama.ProducerMessage{Value: sarama.ByteEncoder(bts)}).ByteSize(2)

		if msgSize <= p.maxMessageBytes {
			left = mid
		} else {
			right = mid - 1
		}
	}

	if left == 0 {
		return 0, fmt.Errorf("cannot find valid split point")
	}

	return left, nil
}

// splitTraceBySpans splits a trace into two parts based on span indices [0, splitPoint) and [splitPoint, end]
func (p *pdataTracesMarshaler) splitTraceBySpans(trace ptrace.Traces, low, high int) (ptrace.Traces, ptrace.Traces) {
	firstHalf := ptrace.NewTraces()
	secondHalf := ptrace.NewTraces()

	// Get the original resource spans
	rSpans := trace.ResourceSpans()
	if rSpans.Len() == 0 {
		return firstHalf, secondHalf
	}

	// Copy resource to both traces
	rSpans.At(0).Resource().CopyTo(firstHalf.ResourceSpans().AppendEmpty().Resource())
	rSpans.At(0).Resource().CopyTo(secondHalf.ResourceSpans().AppendEmpty().Resource())

	// For each scope spans in the original trace
	originalScope := rSpans.At(0).ScopeSpans()
	for i := 0; i < originalScope.Len(); i++ {
		scope := originalScope.At(i)
		spans := scope.Spans()

		// Create scope spans in both halves
		firstScope := firstHalf.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
		secondScope := secondHalf.ResourceSpans().At(0).ScopeSpans().AppendEmpty()

		// Copy scope information
		scope.Scope().CopyTo(firstScope.Scope())
		scope.Scope().CopyTo(secondScope.Scope())

		// Split spans between first and second half
		for j := 0; j < spans.Len(); j++ {
			if j < low || j >= high {
				spans.At(j).CopyTo(secondScope.Spans().AppendEmpty())
			} else {
				spans.At(j).CopyTo(firstScope.Spans().AppendEmpty())
			}
		}
	}

	return firstHalf, secondHalf
}
