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
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
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
			SrcAddr:         netip.MustParseAddr("192.168.1.1").AsSlice(),
			SrcPort:         0,
			DstAddr:         netip.MustParseAddr("192.168.1.2").AsSlice(),
			DstPort:         2055,
			SamplerAddress:  netip.MustParseAddr("192.168.1.100").AsSlice(),
			Type:            3,
			Etype:           0x800,
			Proto:           6,
			Bytes:           100,
			Packets:         1,
			TimeReceivedNs:  1000000000,
			TimeFlowStartNs: 1000000100,
			TimeFlowEndNs:   1000000200,
			SequenceNum:     1,
			SamplingRate:    1,
			TcpFlags:        1,
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
	expectedAttributes.PutStr(string(semconv.SourceAddressKey), "192.168.1.1")
	expectedAttributes.PutInt(string(semconv.SourcePortKey), 0)
	expectedAttributes.PutStr(string(semconv.DestinationAddressKey), "192.168.1.2")
	expectedAttributes.PutInt(string(semconv.DestinationPortKey), 2055)
	expectedAttributes.PutStr(string(semconv.NetworkTransportKey), getTransportName(6))
	expectedAttributes.PutStr(string(semconv.NetworkTypeKey), getEtypeName(0x800))
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
	expectedAttributes.PutStr(string(semconv.SourceAddressKey), "invalid IP")
	expectedAttributes.PutInt(string(semconv.SourcePortKey), 0)
	expectedAttributes.PutStr(string(semconv.DestinationAddressKey), "invalid IP")
	expectedAttributes.PutInt(string(semconv.DestinationPortKey), 0)
	expectedAttributes.PutStr(string(semconv.NetworkTransportKey), "hopopt")
	expectedAttributes.PutStr(string(semconv.NetworkTypeKey), "unknown")
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

	assert.Equal(t, expectedAttributes, record.Attributes())
}
