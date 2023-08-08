// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

func (p pdataTracesMarshaler) Marshal(td ptrace.Traces, config *Config) ([]*sarama.ProducerMessage, error) {
	tracesSlice, err := p.cutTraces(td, config)
	if err != nil {
		return nil, err
	}

	var messagesSlice []*sarama.ProducerMessage

	for _, traces := range tracesSlice {
		tracesData, err := p.marshaler.MarshalTraces(traces)
		if err != nil {
			return nil, err
		}

		message := &sarama.ProducerMessage{
			Topic: config.Topic,
			Value: sarama.ByteEncoder(tracesData),
		}
		messagesSlice = append(messagesSlice, message)
	}

	return messagesSlice, nil
}

func (p pdataTracesMarshaler) Encoding() string {
	return p.encoding
}

func newPdataTracesMarshaler(marshaler ptrace.Marshaler, encoding string) TracesMarshaler {
	return pdataTracesMarshaler{
		marshaler: marshaler,
		encoding:  encoding,
	}
}

func (p pdataTracesMarshaler) cutTraces(td ptrace.Traces, config *Config) ([]ptrace.Traces, error) {
	// 1. check td need to cut by it size
	bytes, err := p.marshaler.MarshalTraces(td)
	if err != nil {
		return nil, err
	}

	// 不用cut
	if config.Producer.MaxMessageBytes == 0 {
		return []ptrace.Traces{td}, nil
	}

	maxBytesSize := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)

	if len(bytes) <= maxBytesSize {
		return []ptrace.Traces{td}, nil
	}

	// 2. cut td
	// 2.1 cutSize = total_td_size/max_bytes_size
	cutSize := len(bytes) / maxBytesSize

	// 2.2 cut traces to tracesSlice
	return p.cutTracesByMaxByte(cutSize, td, maxBytesSize)
}

func (p pdataTracesMarshaler) cutTracesByMaxByte(splitSize int, td ptrace.Traces, maxByte int) (dest []ptrace.Traces, err error) {
	if tracesSpansNum(td) <= splitSize {
		return p.cutTracesByMaxByte(splitSize/2, td, maxByte)
	}

	split := splitTraces(splitSize, td)

	if tracesSpansBytes(split, p) > maxByte {
		// check spansNum == 1
		if tracesSpansNum(split) == 1 {
			return nil, errSingleResourcesSpansMessageSizeOverMaxMsgByte
		}
		left, err := p.cutTracesByMaxByte(splitSize/2, split, maxByte)
		if err != nil {
			return nil, err
		}
		dest = append(dest, left...)
	} else {
		dest = append(dest, split)
	}

	if tracesSpansBytes(td, p) > maxByte {
		if tracesSpansNum(td) == 1 {
			return nil, errSingleResourcesSpansMessageSizeOverMaxMsgByte
		}
		right, err := p.cutTracesByMaxByte(splitSize, td, maxByte)
		if err != nil {
			return nil, err
		}
		dest = append(dest, right...)
	} else {
		dest = append(dest, td)
	}
	return dest, nil

}

func tracesSpansNum(td ptrace.Traces) (num int) {
	for x := 0; x < td.ResourceSpans().Len(); x++ {
		for y := 0; y < td.ResourceSpans().At(x).ScopeSpans().Len(); y++ {
			num += td.ResourceSpans().At(x).ScopeSpans().At(y).Spans().Len()
		}
	}
	return num
}

func tracesSpansBytes(td ptrace.Traces, p pdataTracesMarshaler) int {
	bytes, err := p.marshaler.MarshalTraces(td)
	if err != nil {
		return 0
	}
	return len(bytes)
}

func getBlankProducerMessageSize(config *Config) int {
	msg := sarama.ProducerMessage{}
	return msg.ByteSize(config.Producer.protoVersion)
}
