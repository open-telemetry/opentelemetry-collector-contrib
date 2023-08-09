package kafkaexporter

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/testdata"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"testing"
)

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
