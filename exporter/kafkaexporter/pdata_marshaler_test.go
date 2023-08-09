package kafkaexporter

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/testdata"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"testing"
)

func TestSplitLogs_maxLogRecordsByteSize_success(t *testing.T) {
	maxMessageBytes := 1000
	p := pdataLogsMarshaler{
		marshaler: &plog.ProtoMarshaler{},
		encoding:  defaultEncoding,
	}
	config := &Config{Topic: "topic", Producer: Producer{MaxMessageBytes: maxMessageBytes}}
	spanNumList := []int{5, 10, 15, 20}
	for i, spanNum := range spanNumList {
		td := testdata.GenerateLogs(spanNum)
		beforeCutSize := logRecordsBytes(td, p)
		maxBytesSizeWithoutCommonData := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)
		split, err := p.cutLogs(td, maxBytesSizeWithoutCommonData)
		assert.NoError(t, err)

		totalSizeByte := 0

		cutSpans := 0
		for j, trace := range split {
			size := logRecordsBytes(trace, p)
			fmt.Printf("logRecord: %d, size: %v\n", j, size)
			totalSizeByte += size
			cutSpans += logRecordsNum(trace)
		}

		if beforeCutSize <= maxMessageBytes {
			assert.Equal(t, beforeCutSize, totalSizeByte)
		} else {
			// cut will add more common data
			assert.Less(t, beforeCutSize, totalSizeByte)
		}

		assert.Equal(t, spanNumList[i], cutSpans)
		fmt.Println("---------------------")
	}
}

func TestSplitLogs_maxLogRecordsByteSize_bigSingleSpanError(t *testing.T) {
	maxMessageBytes := 100
	p := pdataLogsMarshaler{
		marshaler: &plog.ProtoMarshaler{},
		encoding:  defaultEncoding,
	}
	config := &Config{Topic: "topic", Producer: Producer{MaxMessageBytes: maxMessageBytes}}
	td := testdata.GenerateLogs(1)
	singleSpanSize := logRecordsBytes(td, p)
	// singleLogRecordSize big then maxMessageBytes
	assert.Greater(t, singleSpanSize, maxMessageBytes)

	maxBytesSizeWithoutCommonData := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)
	split, err := p.cutLogs(td, maxBytesSizeWithoutCommonData)
	assert.Error(t, err)
	assert.EqualError(t, err, errSingleKafkaProducerMessageSizeOverMaxMsgByte.Error())
	assert.Nil(t, split)
}

func TestSplitMetrics_maxMetricsByteSize_success(t *testing.T) {
	maxMessageBytes := 1000
	p := pdataMetricsMarshaler{
		marshaler: &pmetric.ProtoMarshaler{},
		encoding:  defaultEncoding,
	}
	config := &Config{Topic: "topic", Producer: Producer{MaxMessageBytes: maxMessageBytes}}
	spanNumList := []int{5, 10, 15, 20}
	for i, spanNum := range spanNumList {
		td := testdata.GenerateMetrics(spanNum)
		beforeCutSize := metricsBytes(td, p)
		maxBytesSizeWithoutCommonData := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)
		split, err := p.cutMetrics(td, maxBytesSizeWithoutCommonData)
		assert.NoError(t, err)

		totalSizeByte := 0

		cutSpans := 0
		for j, trace := range split {
			size := metricsBytes(trace, p)
			fmt.Printf("trace: %d, size: %v\n", j, size)
			totalSizeByte += size
			cutSpans += metricsNum(trace)
		}

		if beforeCutSize <= maxMessageBytes {
			assert.Equal(t, beforeCutSize, totalSizeByte)
		} else {
			// cut will add more common data
			assert.Less(t, beforeCutSize, totalSizeByte)
		}

		assert.Equal(t, spanNumList[i], cutSpans)
		fmt.Println("---------------------")
	}
}

func TestSplitMetrics_maxMetricsByteSize_bigSingleSpanError(t *testing.T) {
	maxMessageBytes := 100
	p := pdataMetricsMarshaler{
		marshaler: &pmetric.ProtoMarshaler{},
		encoding:  defaultEncoding,
	}
	config := &Config{Topic: "topic", Producer: Producer{MaxMessageBytes: maxMessageBytes}}
	td := testdata.GenerateMetrics(1)
	singleSpanSize := metricsBytes(td, p)
	// singleSpanSize big then maxMessageBytes
	assert.Greater(t, singleSpanSize, maxMessageBytes)

	maxBytesSizeWithoutCommonData := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)
	split, err := p.cutMetrics(td, maxBytesSizeWithoutCommonData)
	assert.Error(t, err)
	assert.EqualError(t, err, errSingleKafkaProducerMessageSizeOverMaxMsgByte.Error())
	assert.Nil(t, split)
}

func TestSplitTraces_maxSpansByteSize_success(t *testing.T) {
	maxMessageBytes := 1000
	p := pdataTracesMarshaler{
		marshaler: &ptrace.ProtoMarshaler{},
		encoding:  defaultEncoding,
	}
	config := &Config{Topic: "topic", Producer: Producer{MaxMessageBytes: maxMessageBytes}}
	spanNumList := []int{5, 10, 15, 20}
	for i, spanNum := range spanNumList {
		td := testdata.GenerateTraces(spanNum)
		beforeCutSize := tracesSpansBytes(td, p)
		maxBytesSizeWithoutCommonData := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)
		split, err := p.cutTraces(td, maxBytesSizeWithoutCommonData)
		assert.NoError(t, err)

		totalSizeByte := 0

		cutSpans := 0
		for j, trace := range split {
			size := tracesSpansBytes(trace, p)
			fmt.Printf("trace: %d, size: %v\n", j, size)
			totalSizeByte += size
			cutSpans += tracesSpansNum(trace)
		}

		if beforeCutSize <= maxMessageBytes {
			assert.Equal(t, beforeCutSize, totalSizeByte)
		} else {
			// cut will add more common data
			assert.Less(t, beforeCutSize, totalSizeByte)
		}

		assert.Equal(t, spanNumList[i], cutSpans)
		fmt.Println("---------------------")
	}
}

func TestSplitTraces_maxSpansByteSize_bigSingleSpanError(t *testing.T) {
	maxMessageBytes := 100
	p := pdataTracesMarshaler{
		marshaler: &ptrace.ProtoMarshaler{},
		encoding:  defaultEncoding,
	}
	config := &Config{Topic: "topic", Producer: Producer{MaxMessageBytes: maxMessageBytes}}
	td := testdata.GenerateTraces(1)
	singleSpanSize := tracesSpansBytes(td, p)
	// singleSpanSize big then maxMessageBytes
	assert.Greater(t, singleSpanSize, maxMessageBytes)

	maxBytesSizeWithoutCommonData := config.Producer.MaxMessageBytes - getBlankProducerMessageSize(config)
	split, err := p.cutTraces(td, maxBytesSizeWithoutCommonData)
	assert.Error(t, err)
	assert.EqualError(t, err, errSingleKafkaProducerMessageSizeOverMaxMsgByte.Error())
	assert.Nil(t, split)
}
