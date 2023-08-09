// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"github.com/IBM/sarama"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/splitObjs"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type pdataLogsMarshaler struct {
	marshaler plog.Marshaler
	encoding  string
}

func (p pdataLogsMarshaler) Marshal(ld plog.Logs, config *Config) ([]*sarama.ProducerMessage, error) {
	bts, err := p.marshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	return []*sarama.ProducerMessage{
		{
			Topic: config.Topic,
			Value: sarama.ByteEncoder(bts),
		},
	}, nil
}

func (p pdataLogsMarshaler) Encoding() string {
	return p.encoding
}

func (p pdataLogsMarshaler) cutLogs(ld plog.Logs, maxBytesSizeWithoutCommonData int) ([]plog.Logs, error) {
	if maxBytesSizeWithoutCommonData <= 0 {
		return []plog.Logs{ld}, nil
	}

	// 1. check ld need to cut by it size
	bytes, err := p.marshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}

	if len(bytes) <= maxBytesSizeWithoutCommonData {
		return []plog.Logs{ld}, nil
	}

	// 2. cut ld  90*3/100
	// 2.1 cutSize = (max_bytes_size / total_ld_size) * totalLogRecordsNum = (max_bytes_size * totalLogRecordsNum) / total_ld_size
	//cutSize := int(float64(maxProducerMsgBytesSize) / float64(len(bytes)) * float64(logRecordsNum(ld)))
	cutSize := (maxBytesSizeWithoutCommonData * logRecordsNum(ld)) / len(bytes)
	if cutSize == 0 {
		if len(bytes) > maxBytesSizeWithoutCommonData {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
		}
		return []plog.Logs{ld}, nil
	}

	// 2.2 cut logs to logsSlice
	return p.cutLogsByMaxByte(cutSize, ld, maxBytesSizeWithoutCommonData)
}

func (p pdataLogsMarshaler) cutLogsByMaxByte(splitSize int, ld plog.Logs, maxByte int) (dest []plog.Logs, err error) {
	if logRecordsNum(ld) <= splitSize {
		return p.cutLogsByMaxByte(splitSize/2, ld, maxByte)
	}

	split := splitObjs.SplitLogs(splitSize, ld)

	if logRecordsBytes(split, p) > maxByte {
		// check logRecordsNum == 1
		if logRecordsNum(split) == 1 {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
		}
		left, err := p.cutLogsByMaxByte(splitSize/2, split, maxByte)
		if err != nil {
			return nil, err
		}
		dest = append(dest, left...)
	} else {
		dest = append(dest, split)
	}

	if logRecordsBytes(ld, p) > maxByte {
		if logRecordsNum(ld) == 1 {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
		}
		right, err := p.cutLogsByMaxByte(splitSize, ld, maxByte)
		if err != nil {
			return nil, err
		}
		dest = append(dest, right...)
	} else {
		dest = append(dest, ld)
	}
	return dest, nil

}

func logRecordsNum(ld plog.Logs) (num int) {
	for x := 0; x < ld.ResourceLogs().Len(); x++ {
		for y := 0; y < ld.ResourceLogs().At(x).ScopeLogs().Len(); y++ {
			num += ld.ResourceLogs().At(x).ScopeLogs().At(y).LogRecords().Len()
		}
	}
	return num
}

func logRecordsBytes(ld plog.Logs, p pdataLogsMarshaler) int {
	bytes, err := p.marshaler.MarshalLogs(ld)
	if err != nil {
		return 0
	}
	return len(bytes)
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

func (p pdataMetricsMarshaler) Marshal(ld pmetric.Metrics, config *Config) ([]*sarama.ProducerMessage, error) {
	bts, err := p.marshaler.MarshalMetrics(ld)
	if err != nil {
		return nil, err
	}
	return []*sarama.ProducerMessage{
		{
			Topic: config.Topic,
			Value: sarama.ByteEncoder(bts),
		},
	}, nil
}

func (p pdataMetricsMarshaler) Encoding() string {
	return p.encoding
}

func (p pdataMetricsMarshaler) cutMetrics(md pmetric.Metrics, maxBytesSizeWithoutCommonData int) ([]pmetric.Metrics, error) {
	if maxBytesSizeWithoutCommonData <= 0 {
		return []pmetric.Metrics{md}, nil
	}

	// 1. check md need to cut by it size
	bytes, err := p.marshaler.MarshalMetrics(md)
	if err != nil {
		return nil, err
	}

	if len(bytes) <= maxBytesSizeWithoutCommonData {
		return []pmetric.Metrics{md}, nil
	}

	// 2. cut md  90*3/100
	// 2.1 cutSize = (max_bytes_size / total_md_size) * totalMetricNum = (max_bytes_size * totalMetricNum) / total_md_size
	//cutSize := int(float64(maxProducerMsgBytesSize) / float64(len(bytes)) * float64(metricsNum(md)))
	cutSize := (maxBytesSizeWithoutCommonData * metricsNum(md)) / len(bytes)
	if cutSize == 0 {
		if len(bytes) > maxBytesSizeWithoutCommonData {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
		}
		return []pmetric.Metrics{md}, nil
	}

	// 2.2 cut metrics to metricsSlice
	return p.cutMetricsByMaxByte(cutSize, md, maxBytesSizeWithoutCommonData)
}

func (p pdataMetricsMarshaler) cutMetricsByMaxByte(splitSize int, md pmetric.Metrics, maxByte int) (dest []pmetric.Metrics, err error) {
	if metricsNum(md) <= splitSize {
		return p.cutMetricsByMaxByte(splitSize/2, md, maxByte)
	}

	split := splitObjs.SplitMetrics(splitSize, md)

	if metricsBytes(split, p) > maxByte {
		// check metricsNum == 1
		if metricsNum(split) == 1 {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
		}
		left, err := p.cutMetricsByMaxByte(splitSize/2, split, maxByte)
		if err != nil {
			return nil, err
		}
		dest = append(dest, left...)
	} else {
		dest = append(dest, split)
	}

	if metricsBytes(md, p) > maxByte {
		if metricsNum(md) == 1 {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
		}
		right, err := p.cutMetricsByMaxByte(splitSize, md, maxByte)
		if err != nil {
			return nil, err
		}
		dest = append(dest, right...)
	} else {
		dest = append(dest, md)
	}
	return dest, nil

}

func metricsNum(md pmetric.Metrics) (num int) {
	for x := 0; x < md.ResourceMetrics().Len(); x++ {
		for y := 0; y < md.ResourceMetrics().At(x).ScopeMetrics().Len(); y++ {
			num += md.ResourceMetrics().At(x).ScopeMetrics().At(y).Metrics().Len()
		}
	}
	return num
}

func metricsBytes(md pmetric.Metrics, p pdataMetricsMarshaler) int {
	bytes, err := p.marshaler.MarshalMetrics(md)
	if err != nil {
		return 0
	}
	return len(bytes)
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
	maxBytesSizeWithoutCommonData := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)

	tracesSlice, err := p.cutTraces(td, maxBytesSizeWithoutCommonData)
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

func (p pdataTracesMarshaler) cutTraces(td ptrace.Traces, maxBytesSizeWithoutCommonData int) ([]ptrace.Traces, error) {
	if maxBytesSizeWithoutCommonData <= 0 {
		return []ptrace.Traces{td}, nil
	}

	// 1. check td need to cut by it size
	bytes, err := p.marshaler.MarshalTraces(td)
	if err != nil {
		return nil, err
	}

	if len(bytes) <= maxBytesSizeWithoutCommonData {
		return []ptrace.Traces{td}, nil
	}

	// 2. cut td  90*3/100
	// 2.1 cutSize = (max_bytes_size / total_td_size) * totalSpanNum = (max_bytes_size * totalSpanNum) / total_td_size
	//cutSize := int(float64(maxProducerMsgBytesSize) / float64(len(bytes)) * float64(tracesSpansNum(td)))
	cutSize := (maxBytesSizeWithoutCommonData * tracesSpansNum(td)) / len(bytes)
	if cutSize == 0 {
		if len(bytes) > maxBytesSizeWithoutCommonData {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
		}
		return []ptrace.Traces{td}, nil
	}

	// 2.2 cut traces to tracesSlice
	return p.cutTracesByMaxByte(cutSize, td, maxBytesSizeWithoutCommonData)
}

func (p pdataTracesMarshaler) cutTracesByMaxByte(splitSize int, td ptrace.Traces, maxByte int) (dest []ptrace.Traces, err error) {
	if tracesSpansNum(td) <= splitSize {
		return p.cutTracesByMaxByte(splitSize/2, td, maxByte)
	}

	split := splitObjs.SplitTraces(splitSize, td)

	if tracesSpansBytes(split, p) > maxByte {
		// check spansNum == 1
		if tracesSpansNum(split) == 1 {
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
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
			return nil, errSingleKafkaProducerMessageSizeOverMaxMsgByte
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
