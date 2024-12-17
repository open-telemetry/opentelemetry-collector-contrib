// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver

import (
	"net/netip"
	"testing"
	"time"

	flowpb "github.com/netsampler/goflow2/v2/pb"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/assert"
)

func TestGetProtoName(t *testing.T) {
	tests := []struct {
		proto uint32
		want  string
	}{
		{proto: 1, want: "ICMP"},
		{proto: 6, want: "TCP"},
		{proto: 17, want: "UDP"},
		{proto: 58, want: "ICMPv6"},
		{proto: 132, want: "SCTP"},
		{proto: 0, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := getProtoName(tt.proto)
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
			TimeFlowStartNs: 1000000000,
			TimeFlowEndNs:   1000000100,
			SequenceNum:     1,
			SamplingRate:    1,
		},
	}

	otel, err := convertToOtel(pm)
	if err != nil {
		t.Errorf("convertToOtel() error = %v", err)
		return
	}
	assert.Equal(t, "192.168.1.1", otel.Source.Address)
	assert.Equal(t, uint32(0), otel.Source.Port)
	assert.Equal(t, "192.168.1.2", otel.Destination.Address)
	assert.Equal(t, uint32(2055), otel.Destination.Port)
	assert.Equal(t, uint64(100), otel.IO.Bytes)
	assert.Equal(t, uint64(1), otel.IO.Packets)
	assert.Equal(t, "TCP", otel.Transport)
	assert.Equal(t, "IPv4", otel.Type)
	assert.Equal(t, "NETFLOW_V9", otel.Flow.Type)
	assert.Equal(t, "192.168.1.100", otel.Flow.SamplerAddress)
	assert.Equal(t, uint32(1), otel.Flow.SequenceNum)
	assert.Equal(t, uint64(1), otel.Flow.SamplingRate)
	assert.Equal(t, time.Unix(0, 1000000000), otel.Flow.Start)
	assert.Equal(t, time.Unix(0, 1000000100), otel.Flow.End)
	assert.Equal(t, time.Unix(0, 1000000000), otel.Flow.TimeReceived)
}

func TestEmptyConvertToOtel(t *testing.T) {
	pm := &protoproducer.ProtoProducerMessage{}

	otel, err := convertToOtel(pm)
	if err != nil {
		t.Errorf("convertToOtel() error = %v", err)
		return
	}
	assert.Equal(t, "invalid IP", otel.Source.Address)
	assert.Equal(t, uint32(0), otel.Source.Port)
	assert.Equal(t, "invalid IP", otel.Destination.Address)
	assert.Equal(t, uint32(0), otel.Destination.Port)
	assert.Equal(t, uint64(0), otel.IO.Bytes)
	assert.Equal(t, uint64(0), otel.IO.Packets)
	assert.Equal(t, "unknown", otel.Transport)
	assert.Equal(t, "unknown", otel.Type)
	assert.Equal(t, "UNKNOWN", otel.Flow.Type)
	assert.Equal(t, "invalid IP", otel.Flow.SamplerAddress)
	assert.Equal(t, uint32(0), otel.Flow.SequenceNum)
	assert.Equal(t, uint64(0), otel.Flow.SamplingRate)
	assert.Equal(t, time.Unix(0, 0), otel.Flow.Start)
	assert.Equal(t, time.Unix(0, 0), otel.Flow.End)
	assert.Equal(t, time.Unix(0, 0), otel.Flow.TimeReceived)
}
