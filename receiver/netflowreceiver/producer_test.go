package netflowreceiver

import (
	"testing"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
	flowpb "github.com/netsampler/goflow2/v2/pb"
	"github.com/netsampler/goflow2/v2/producer"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
		FlowSets: []interface{}{
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

	otelLogsProducer := newOtelLogsProducer(protoProducer, consumertest.NewNop())
	messages, err := otelLogsProducer.Produce(message, &producer.ProduceArgs{})
	require.NoError(t, err)
	require.NotNil(t, messages)
	assert.Equal(t, 1, len(messages))

	pm, ok := messages[0].(*protoproducer.ProtoProducerMessage)
	assert.True(t, ok)
	assert.Equal(t, flowpb.FlowMessage_NETFLOW_V9, pm.Type)
	assert.Equal(t, uint64(1), pm.Packets)
	assert.Equal(t, uint32(256), pm.ObservationDomainId)
	assert.Equal(t, uint32(838987416), pm.SequenceNum)
}
