package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"errors"
	"net/netip"
	"time"

	"github.com/netsampler/goflow2/v2/producer"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
)

var (
	etypeName = map[uint32]string{
		0x806:  "ARP",
		0x800:  "IPv4",
		0x86dd: "IPv6",
	}
	protoName = map[uint32]string{
		1:   "ICMP",
		6:   "TCP",
		17:  "UDP",
		58:  "ICMPv6",
		132: "SCTP",
	}

	flowTypeName = map[int32]string{
		0: "UNKNOWN",
		1: "SFLOW_5",
		2: "NETFLOW_V5",
		3: "NETFLOW_V9",
		4: "IPFIX",
	}
)

type NetworkAddress struct {
	Address string `json:"address,omitempty"`
	Port    uint32 `json:"port,omitempty"`
}

type Flow struct {
	Type           string    `json:"type,omitempty"`
	TimeReceived   time.Time `json:"time_received,omitempty"`
	Start          time.Time `json:"start,omitempty"`
	End            time.Time `json:"end,omitempty"`
	SequenceNum    uint32    `json:"sequence_num,omitempty"`
	SamplingRate   uint64    `json:"sampling_rate,omitempty"`
	SamplerAddress string    `json:"sampler_address,omitempty"`
}

type Protocol struct {
	Name []byte `json:"name,omitempty"` // Layer 7
}

type NetworkIO struct {
	Bytes   uint64 `json:"bytes,omitempty"`
	Packets uint64 `json:"packets,omitempty"`
}

type OtelNetworkMessage struct {
	Source      NetworkAddress `json:"source,omitempty"`
	Destination NetworkAddress `json:"destination,omitempty"`
	Transport   string         `json:"transport,omitempty"` // Layer 4
	Type        string         `json:"type,omitempty"`      // Layer 3
	IO          NetworkIO      `json:"io,omitempty"`
	Flow        Flow           `json:"flow,omitempty"`
}

func getEtypeName(etype uint32) string {
	if name, ok := etypeName[etype]; ok {
		return name
	}
	return "unknown"
}

func getProtoName(proto uint32) string {
	if name, ok := protoName[proto]; ok {
		return name
	}
	return "unknown"
}

func getFlowTypeName(flowType int32) string {
	if name, ok := flowTypeName[flowType]; ok {
		return name
	}
	return "unknown"
}

// convertToOtel converts a ProtoProducerMessage to an OtelNetworkMessage
func convertToOtel(m producer.ProducerMessage) (*OtelNetworkMessage, error) {

	// we know msg is ProtoProducerMessage because that is the parent producer
	pm, ok := m.(*protoproducer.ProtoProducerMessage)
	if !ok {
		return nil, errors.New("message is not ProtoProducerMessage")
	}

	// Parse IP addresses bytes to netip.Addr
	srcAddr, _ := netip.AddrFromSlice(pm.SrcAddr)
	dstAddr, _ := netip.AddrFromSlice(pm.DstAddr)
	samplerAddr, _ := netip.AddrFromSlice(pm.SamplerAddress)

	// Time the receiver received the message
	receivedTime := time.Unix(0, int64(pm.TimeReceivedNs))
	startTime := time.Unix(0, int64(pm.TimeFlowStartNs))
	endTime := time.Unix(0, int64(pm.TimeFlowEndNs))

	// Construct the actual log record based on the otel semantic conventions
	// see https://opentelemetry.io/docs/specs/semconv/general/attributes/
	otelMessage := OtelNetworkMessage{
		Source: NetworkAddress{
			Address: srcAddr.String(),
			Port:    pm.SrcPort,
		},
		Destination: NetworkAddress{
			Address: dstAddr.String(),
			Port:    pm.DstPort,
		},
		Type:      getEtypeName(pm.Etype), // Layer 3
		Transport: getProtoName(pm.Proto), // Layer 4
		IO: NetworkIO{
			Bytes:   pm.Bytes,
			Packets: pm.Packets,
		},
		Flow: Flow{
			Type:           getFlowTypeName(int32(pm.Type)),
			TimeReceived:   receivedTime,
			Start:          startTime,
			End:            endTime,
			SequenceNum:    pm.SequenceNum,
			SamplingRate:   pm.SamplingRate,
			SamplerAddress: samplerAddr.String(),
		},
	}

	return &otelMessage, nil

}
