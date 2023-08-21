// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

type pdataLogsMarshaler struct {
	marshaler plog.Marshaler
	encoding  string
}

func (p pdataLogsMarshaler) Marshal(ld plog.Logs, topic string) ([]*sarama.ProducerMessage, error) {
	bts, err := p.marshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	return []*sarama.ProducerMessage{
		{
			Topic: topic,
			Value: sarama.ByteEncoder(bts),
		},
	}, nil
}

func (p pdataLogsMarshaler) Encoding() string {
	return p.encoding
}

func newPdataLogsMarshaler(marshaler plog.Marshaler, encoding string) LogsMarshaler {
	return pdataLogsMarshaler{
		marshaler: marshaler,
		encoding:  encoding,
	}
}

type pdataMetricsMarshaler struct {
	marshaler pmetric.Marshaler
	encoding  string
}

func (p pdataMetricsMarshaler) Marshal(ld pmetric.Metrics, topic string) ([]*sarama.ProducerMessage, error) {
	bts, err := p.marshaler.MarshalMetrics(ld)
	if err != nil {
		return nil, err
	}
	return []*sarama.ProducerMessage{
		{
			Topic: topic,
			Value: sarama.ByteEncoder(bts),
		},
	}, nil
}

func (p pdataMetricsMarshaler) Encoding() string {
	return p.encoding
}

func newPdataMetricsMarshaler(marshaler pmetric.Marshaler, encoding string) MetricsMarshaler {
	return pdataMetricsMarshaler{
		marshaler: marshaler,
		encoding:  encoding,
	}
}

type pdataTracesMarshaler struct {
	marshaler ptrace.Marshaler
	encoding  string
}

func (p pdataTracesMarshaler) Marshal(td ptrace.Traces, topic string) ([]*sarama.ProducerMessage, error) {
	bts, err := p.marshaler.MarshalTraces(td)
	if err != nil {
		return nil, err
	}
	return []*sarama.ProducerMessage{
		{
			Topic: topic,
			Value: sarama.ByteEncoder(bts),
		},
	}, nil
}

func (p pdataTracesMarshaler) Encoding() string {
	return p.encoding
}

func (p pdataTracesMarshaler) KeyData() string {
	return "none"
}

type pdataTracesMarshalerByTraceId pdataTracesMarshaler

func (p pdataTracesMarshalerByTraceId) Marshal(td ptrace.Traces, topic string) ([]*sarama.ProducerMessage, error) {
	var messages []*sarama.ProducerMessage

	for _, tracesById := range batchpersignal.SplitTraces(td) {
		bts, err := p.marshaler.MarshalTraces(tracesById)
		if err != nil {
			return nil, err
		}
		var traceID = tracesById.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
		key := traceutil.TraceIDToHexOrEmptyString(traceID)
		messages = append(messages, &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(bts),
			Key:   sarama.ByteEncoder(key),
		})
	}
	return messages, nil
}

func (p pdataTracesMarshalerByTraceId) Encoding() string {
	return p.encoding
}

func (p pdataTracesMarshalerByTraceId) KeyData() string {
	return "traceID"
}

func newPdataTracesMarshaler(marshaler ptrace.Marshaler, encoding string) TracesMarshaler {
	return pdataTracesMarshaler{
		marshaler: marshaler,
		encoding:  encoding,
	}
}

func newPdataTracesMarshalerByTraceId(marshaler ptrace.Marshaler, encoding string) TracesMarshaler {
	return pdataTracesMarshalerByTraceId{
		marshaler: marshaler,
		encoding:  encoding,
	}
}
