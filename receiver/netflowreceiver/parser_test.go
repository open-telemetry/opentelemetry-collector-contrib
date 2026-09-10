// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver

import (
	"net/netip"
	"testing"

	flowpb "github.com/netsampler/goflow2/v2/pb"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestGetProtoName(t *testing.T) {
	tests := []struct {
		proto uint32
		want  string
	}{
		{proto: 1, want: "icmp"},
		{proto: 6, want: "tcp"},
		{proto: 17, want: "udp"},
		{proto: 58, want: "ipv6-icmp"},
		{proto: 132, want: "sctp"},
		{proto: 0, want: "hopopt"},
		{proto: 400, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := getTransportName(tt.proto)
			if got != tt.want {
				t.Errorf("getProtoName(%d) = %s; want %s", tt.proto, got, tt.want)
			}
		})
	}
}

func TestConvertToOtel(t *testing.T) {
	pm := &protoproducer.ProtoProducerMessage{
		FlowMessage: flowpb.FlowMessage{
			SrcAddr:             netip.MustParseAddr("192.168.1.1").AsSlice(),
			SrcPort:             0,
			DstAddr:             netip.MustParseAddr("192.168.1.2").AsSlice(),
			DstPort:             2055,
			SamplerAddress:      netip.MustParseAddr("192.168.1.100").AsSlice(),
			Type:                3,
			Etype:               0x800,
			Proto:               6,
			Bytes:               100,
			Packets:             1,
			TimeReceivedNs:      1000000000,
			TimeFlowStartNs:     1000000100,
			TimeFlowEndNs:       1000000200,
			SequenceNum:         1,
			SamplingRate:        1,
			TcpFlags:            1,
			InIf:                5,
			OutIf:               10,
			IpTos:               46,
			IpTtl:               64,
			IpFlags:             2,
			FragmentId:          1000,
			FragmentOffset:      0,
			Ipv6FlowLabel:       0,
			IcmpType:            8,
			IcmpCode:            0,
			SrcMac:              0x001122334455,
			DstMac:              0xaabbccddeeff,
			SrcVlan:             100,
			DstVlan:             200,
			VlanId:              100,
			NextHop:             netip.MustParseAddr("10.0.0.1").AsSlice(),
			NextHopAs:           65001,
			SrcAs:               65000,
			DstAs:               65001,
			BgpNextHop:          netip.MustParseAddr("10.0.0.2").AsSlice(),
			SrcNet:              24,
			DstNet:              32,
			ForwardingStatus:    64,
			ObservationDomainId: 1,
			ObservationPointId:  2,
		},
	}

	record := plog.NewLogRecord()
	err := addMessageAttributes(pm, &record)
	if err != nil {
		t.Errorf("TestConvertToOtel() error = %v", err)
		return
	}

	assert.Equal(t, int64(1000000100), record.Timestamp().AsTime().UnixNano())
	assert.Equal(t, int64(1000000000), record.ObservedTimestamp().AsTime().UnixNano())

	expectedAttributes := pcommon.NewMap()
	expectedAttributes.PutStr("source.address", "192.168.1.1")
	expectedAttributes.PutInt("source.port", 0)
	expectedAttributes.PutStr("destination.address", "192.168.1.2")
	expectedAttributes.PutInt("destination.port", 2055)
	expectedAttributes.PutStr("network.transport", getTransportName(6))
	expectedAttributes.PutStr("network.type", getEtypeName(0x800))
	expectedAttributes.PutInt("flow.io.bytes", 100)
	expectedAttributes.PutInt("flow.io.packets", 1)
	expectedAttributes.PutStr("flow.type", getFlowTypeName(3))
	expectedAttributes.PutInt("flow.sequence_num", 1)
	expectedAttributes.PutInt("flow.time_received", 1000000000)
	expectedAttributes.PutInt("flow.start", 1000000100)
	expectedAttributes.PutInt("flow.end", 1000000200)
	expectedAttributes.PutInt("flow.sampling_rate", 1)
	expectedAttributes.PutStr("flow.sampler_address", "192.168.1.100")
	expectedAttributes.PutInt("flow.tcp_flags", 1)
	expectedAttributes.PutInt("flow.in_if", 5)
	expectedAttributes.PutInt("flow.out_if", 10)
	expectedAttributes.PutInt("flow.ip_tos", 46)
	expectedAttributes.PutInt("flow.ip_ttl", 64)
	expectedAttributes.PutInt("flow.ip_flags", 2)
	expectedAttributes.PutInt("flow.fragment_id", 1000)
	expectedAttributes.PutInt("flow.fragment_offset", 0)
	expectedAttributes.PutInt("flow.ipv6_flow_label", 0)
	expectedAttributes.PutInt("flow.icmp_type", 8)
	expectedAttributes.PutInt("flow.icmp_code", 0)
	expectedAttributes.PutStr("flow.src_mac", "00:11:22:33:44:55")
	expectedAttributes.PutStr("flow.dst_mac", "aa:bb:cc:dd:ee:ff")
	expectedAttributes.PutInt("flow.src_vlan", 100)
	expectedAttributes.PutInt("flow.dst_vlan", 200)
	expectedAttributes.PutInt("flow.vlan_id", 100)
	expectedAttributes.PutStr("flow.next_hop", "10.0.0.1")
	expectedAttributes.PutInt("flow.next_hop_as", 65001)
	expectedAttributes.PutInt("flow.src_as", 65000)
	expectedAttributes.PutInt("flow.dst_as", 65001)
	expectedAttributes.PutStr("flow.bgp_next_hop", "10.0.0.2")
	expectedAttributes.PutInt("flow.src_net", 24)
	expectedAttributes.PutInt("flow.dst_net", 32)
	expectedAttributes.PutInt("flow.forwarding_status", 64)
	expectedAttributes.PutInt("flow.observation_domain_id", 1)
	expectedAttributes.PutInt("flow.observation_point_id", 2)

	assert.Equal(t, expectedAttributes, record.Attributes())
}

func TestEmptyConvertToOtel(t *testing.T) {
	pm := &protoproducer.ProtoProducerMessage{}

	record := plog.NewLogRecord()
	err := addMessageAttributes(pm, &record)
	if err != nil {
		t.Errorf("TestConvertToOtel() error = %v", err)
		return
	}

	assert.Equal(t, int64(0), record.Timestamp().AsTime().UnixNano())
	assert.Equal(t, int64(0), record.ObservedTimestamp().AsTime().UnixNano())

	expectedAttributes := pcommon.NewMap()
	expectedAttributes.PutStr("source.address", "invalid IP")
	expectedAttributes.PutInt("source.port", 0)
	expectedAttributes.PutStr("destination.address", "invalid IP")
	expectedAttributes.PutInt("destination.port", 0)
	expectedAttributes.PutStr("network.transport", "hopopt")
	expectedAttributes.PutStr("network.type", "unknown")
	expectedAttributes.PutInt("flow.io.bytes", 0)
	expectedAttributes.PutInt("flow.io.packets", 0)
	expectedAttributes.PutStr("flow.type", "unknown")
	expectedAttributes.PutInt("flow.sequence_num", 0)
	expectedAttributes.PutInt("flow.time_received", 0)
	expectedAttributes.PutInt("flow.start", 0)
	expectedAttributes.PutInt("flow.end", 0)
	expectedAttributes.PutInt("flow.sampling_rate", 0)
	expectedAttributes.PutStr("flow.sampler_address", "invalid IP")
	expectedAttributes.PutInt("flow.tcp_flags", 0)
	expectedAttributes.PutInt("flow.in_if", 0)
	expectedAttributes.PutInt("flow.out_if", 0)
	expectedAttributes.PutInt("flow.ip_tos", 0)
	expectedAttributes.PutInt("flow.ip_ttl", 0)
	expectedAttributes.PutInt("flow.ip_flags", 0)
	expectedAttributes.PutInt("flow.fragment_id", 0)
	expectedAttributes.PutInt("flow.fragment_offset", 0)
	expectedAttributes.PutInt("flow.ipv6_flow_label", 0)
	expectedAttributes.PutInt("flow.icmp_type", 0)
	expectedAttributes.PutInt("flow.icmp_code", 0)
	expectedAttributes.PutStr("flow.src_mac", "00:00:00:00:00:00")
	expectedAttributes.PutStr("flow.dst_mac", "00:00:00:00:00:00")
	expectedAttributes.PutInt("flow.src_vlan", 0)
	expectedAttributes.PutInt("flow.dst_vlan", 0)
	expectedAttributes.PutInt("flow.vlan_id", 0)
	expectedAttributes.PutInt("flow.next_hop_as", 0)
	expectedAttributes.PutInt("flow.src_as", 0)
	expectedAttributes.PutInt("flow.dst_as", 0)
	expectedAttributes.PutInt("flow.src_net", 0)
	expectedAttributes.PutInt("flow.dst_net", 0)
	expectedAttributes.PutInt("flow.forwarding_status", 0)
	expectedAttributes.PutInt("flow.observation_domain_id", 0)
	expectedAttributes.PutInt("flow.observation_point_id", 0)

	assert.Equal(t, expectedAttributes, record.Attributes())
}

func TestFormatMAC(t *testing.T) {
	tests := []struct {
		mac  uint64
		want string
	}{
		{mac: 0x000000000000, want: "00:00:00:00:00:00"},
		{mac: 0x001122334455, want: "00:11:22:33:44:55"},
		{mac: 0xaabbccddeeff, want: "aa:bb:cc:dd:ee:ff"},
		{mac: 0xffffffffffff, want: "ff:ff:ff:ff:ff:ff"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, formatMAC(tt.mac))
		})
	}
}
