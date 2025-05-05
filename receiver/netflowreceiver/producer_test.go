// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
	flowpb "github.com/netsampler/goflow2/v2/pb"
	"github.com/netsampler/goflow2/v2/producer"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestProduce(t *testing.T) {
	// list of netflow.DataFlowSet
	message := &netflow.NFv9Packet{
		Version:        9,
		Count:          1,
		SystemUptime:   0xb3bff683,
		UnixSeconds:    0x618aa3a8,
		SequenceNumber: 838987416,
		SourceId:       256,
		FlowSets: []any{
			netflow.DataFlowSet{
				FlowSetHeader: netflow.FlowSetHeader{
					Id:     260,
					Length: 1372,
				},
				Records: []netflow.DataRecord{
					{
						Values: []netflow.DataField{
							{
								PenProvided: false,
								Type:        2,
								Pen:         0,
								Value:       []uint8{0x00, 0x00, 0x00, 0x01},
							},
						},
					},
				},
			},
		},
	}

	cfgProducer := &protoproducer.ProducerConfig{}
	cfgm, err := cfgProducer.Compile() // converts configuration into a format that can be used by a protobuf producer
	require.NoError(t, err)
	// We use a goflow2 proto producer to produce messages using protobuf format
	protoProducer, err := protoproducer.CreateProtoProducer(cfgm, protoproducer.CreateSamplingSystem)
	require.NoError(t, err)

	otelLogsProducer := newOtelLogsProducer(protoProducer, consumertest.NewNop(), zap.NewNop(), false)
	messages, err := otelLogsProducer.Produce(message, &producer.ProduceArgs{})
	require.NoError(t, err)
	require.NotNil(t, messages)
	assert.Len(t, messages, 1)

	pm, ok := messages[0].(*protoproducer.ProtoProducerMessage)
	assert.True(t, ok)
	assert.Equal(t, flowpb.FlowMessage_NETFLOW_V9, pm.Type)
	assert.Equal(t, uint64(1), pm.Packets)
	assert.Equal(t, uint32(256), pm.ObservationDomainId)
	assert.Equal(t, uint32(838987416), pm.SequenceNum)
}

func TestProduceRaw(t *testing.T) {
	// list of netflow.DataFlowSet
	message := &netflow.NFv9Packet{
		Version:        9,
		Count:          1,
		SystemUptime:   0xb3bff683,
		UnixSeconds:    0x618aa3a8,
		SequenceNumber: 838987416,
		SourceId:       256,
		FlowSets: []any{
			netflow.DataFlowSet{
				FlowSetHeader: netflow.FlowSetHeader{
					Id:     260,
					Length: 1372,
				},
				Records: []netflow.DataRecord{
					{
						Values: []netflow.DataField{
							{
								PenProvided: false,
								Type:        2,
								Pen:         0,
								Value:       []uint8{0x00, 0x00, 0x00, 0x01},
							},
						},
					},
					{
						Values: []netflow.DataField{
							{
								PenProvided: false,
								Type:        2,
								Pen:         0,
								Value:       []uint8{0x00, 0x00, 0x00, 0x02},
							},
						},
					},
					{
						Values: []netflow.DataField{
							{
								PenProvided: false,
								Type:        2,
								Pen:         0,
								Value:       []uint8{0x00, 0x00, 0x00, 0x03},
							},
						},
					},
				},
			},
		},
	}

	cfgProducer := &protoproducer.ProducerConfig{}
	cfgm, err := cfgProducer.Compile()
	require.NoError(t, err)

	protoProducer, err := protoproducer.CreateProtoProducer(cfgm, protoproducer.CreateSamplingSystem)
	require.NoError(t, err)

	sink := &consumertest.LogsSink{}
	otelLogsProducer := newOtelLogsProducer(protoProducer, sink, zap.NewNop(), true)

	messages, err := otelLogsProducer.Produce(message, &producer.ProduceArgs{})
	require.NoError(t, err)
	require.NotNil(t, messages)
	assert.Len(t, messages, 3)

	logs := sink.AllLogs()
	require.Len(t, logs, 1)
	records := logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 3, records.Len()) // Should have one record per flow record

	// Each record should be a raw string representation of the ProducerMessage
	for i := 0; i < 3; i++ {
		record := records.At(i)
		msg := messages[i]
		assert.Equal(t, fmt.Sprintf("%+v", msg), record.Body().Str())
	}
}

// This PanicProducer replaces the ProtoProducer, to simulate it producing a panic
type PanicProducer struct{}

func (m *PanicProducer) Produce(_ any, _ *producer.ProduceArgs) ([]producer.ProducerMessage, error) {
	panic("producer panic!")
}

func (m *PanicProducer) Close() {}

func (m *PanicProducer) Commit(_ []producer.ProducerMessage) {}

func TestProducerPanic(t *testing.T) {
	// Create a mock logger that can capture logged messages
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(observedZapCore)

	// Create a mock consumer
	mockConsumer := consumertest.NewNop()

	// Wrap a PanicProducer (instead of ProtoProducer) in the OtelLogsProducerWrapper
	wrapper := newOtelLogsProducer(&PanicProducer{}, mockConsumer, logger, false)

	// Call Produce which should recover from panic
	messages, err := wrapper.Produce(nil, &producer.ProduceArgs{
		SamplerAddress: netip.MustParseAddr("127.0.0.1"),
	})

	// Verify that no error is returned (since panic was recovered)
	assert.NoError(t, err)
	assert.Empty(t, messages)

	// Verify that the error was logged
	log := observedLogs.All()[0]
	assert.Equal(t, "unexpected error processing the message", log.Message)
	assert.Equal(t, "producer panic!", log.ContextMap()["error"])
}
