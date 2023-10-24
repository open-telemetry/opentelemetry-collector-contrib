// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License. language governing permissions and
// limitations under the License.
package sflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sflowreceiver"

import (
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type SFlowRecord interface{}

type SFlowPacket interface{}

type Dot1Q struct {
	Priority       uint8  `json:"priority"`
	DropEligible   bool   `json:"dropEligible"`
	VLANIdentifier uint16 `json:"vlanIdentifier"`
	Type           string `json:"type"`
}

type Ethernet struct {
	SrcMAC       string `json:"srcMac"`
	DstMAC       string `json:"dstMac"`
	EthernetType string `json:"ethernetType"`
}

type IPv4 struct {
	Version    uint8  `json:"version"`
	IHL        uint8  `json:"ihl"`
	TOS        uint8  `json:"tos"`
	Length     uint16 `json:"length"`
	Id         uint16 `json:"id"`
	Flags      string `json:"flags"`
	FragOffset uint16 `json:"fragOffset"`
	TTL        uint8  `json:"ttl"`
	Protocol   string `json:"protocol"`
	Checksum   uint16 `json:"checksum"`
	SrcIP      net.IP `json:"srcIP"`
	DstIP      net.IP `json:"dstIP"`
}

type SFlowRawPacketFlowRecord struct {
	Header SFlowPacket `json:"header"`
}

type SflowSample struct {
	EnterpriseID          string        `json:"enterpriseID"`
	Format                string        `json:"format"`
	SampleLength          uint32        `json:"sampleLength"`
	SourceIDClass         string        `json:"sflowSourceFormat"`
	SourceIDIndex         string        `json:"sflowSourceValue"`
	SamplingRate          uint32        `json:"samplingRate"`
	SamplePool            uint32        `json:"samplePool"`
	Dropped               uint32        `json:"dropped"`
	InputInterfaceFormat  uint32        `json:"inputInterfaceFormat"`
	InputInterface        uint32        `json:"inputInterface"`
	OutputInterfaceFormat uint32        `json:"outputInterfaceFormat"`
	OutputInterface       uint32        `json:"outputInterface"`
	RecordCount           uint32        `json:"recordCount"`
	Records               []SFlowRecord `json:"record"`
}

type SFlowData struct {
	Version        uint32        `json:"version"`
	AgentIP        string        `json:"agentIP"`
	SubAgentID     uint32        `json:"subAgentID"`
	SequenceNumber uint32        `json:"sequenceNumber"`
	AgentUptime    uint32        `json:"agentUpTime"`
	SampleCount    uint32        `json:"sampleCount"`
	SFlowSample    []SflowSample `json:"sflow"`
}

func DecodeSFlowPacket(byteData []byte) *SFlowData {
	// Parse the sFlow packet.
	packet := gopacket.NewPacket(byteData, layers.LayerTypeSFlow, gopacket.Default)

	// Create an SFlowData struct to store the parsed data.
	var sFlowData SFlowData

	// Extract data from the parsed sFlow packet.
	if sFlowLayer := packet.Layer(layers.LayerTypeSFlow); sFlowLayer != nil {
		sFlow, _ := sFlowLayer.(*layers.SFlowDatagram)

		for idx, sample := range sFlow.FlowSamples {
			sFlowData.SFlowSample = append(sFlowData.SFlowSample, SflowSample{
				EnterpriseID:          sample.EnterpriseID.String(),
				Format:                sample.Format.String(),
				SampleLength:          sample.SampleLength,
				SourceIDClass:         sample.SourceIDClass.String(),
				SamplingRate:          sample.SamplingRate,
				SamplePool:            sample.SamplePool,
				Dropped:               sample.Dropped,
				InputInterfaceFormat:  sample.InputInterfaceFormat,
				InputInterface:        sample.InputInterface,
				OutputInterfaceFormat: sample.OutputInterfaceFormat,
				OutputInterface:       sample.OutputInterface,
				RecordCount:           sample.RecordCount,
			})
			for _, record := range sample.Records {
				switch record.(type) {
				case layers.SFlowRawPacketFlowRecord:
					fr := record.(layers.SFlowRawPacketFlowRecord)
					for _, hl := range fr.Header.Layers() {
						switch hl.(type) {
						case *layers.Ethernet:
							mac := hl.(*layers.Ethernet)
							sFlowData.SFlowSample[idx].Records = append(sFlowData.SFlowSample[idx].Records, SFlowRawPacketFlowRecord{
								Header: Ethernet{
									SrcMAC:       mac.SrcMAC.String(),
									DstMAC:       mac.DstMAC.String(),
									EthernetType: mac.EthernetType.String(),
								},
							})
						case *layers.Dot1Q:
							dot1q := hl.(*layers.Dot1Q)
							sFlowData.SFlowSample[idx].Records = append(sFlowData.SFlowSample[idx].Records, SFlowRawPacketFlowRecord{
								Header: Dot1Q{
									Priority:       dot1q.Priority,
									DropEligible:   dot1q.DropEligible,
									VLANIdentifier: dot1q.VLANIdentifier,
									Type:           dot1q.Type.String(),
								},
							})
						case *layers.IPv4:
							ip := hl.(*layers.IPv4)
							sFlowData.SFlowSample[idx].Records = append(sFlowData.SFlowSample[idx].Records, SFlowRawPacketFlowRecord{
								Header: IPv4{
									Version:    ip.Version,
									IHL:        ip.IHL,
									TOS:        ip.TOS,
									TTL:        ip.TTL,
									Length:     ip.Length,
									Id:         ip.Id,
									SrcIP:      ip.SrcIP,
									DstIP:      ip.DstIP,
									Flags:      ip.Flags.String(),
									FragOffset: ip.FragOffset,
									Protocol:   ip.Protocol.String(),
									Checksum:   ip.Checksum,
								},
							})
						case *layers.UDP:
						case *layers.VXLAN:
						case *layers.TCP:
						}
					}

				case layers.SFlowExtendedSwitchFlowRecord:
				case layers.SFlowExtendedRouterFlowRecord:
				case layers.SFlowExtendedGatewayFlowRecord:
				case layers.SFlowEthernetFrameFlowRecord:
				case layers.SFlowBaseFlowRecord:
				default:
					// Type not supported
				}
			}
		}

		// Map sFlow data to the SFlowData struct.
		sFlowData.Version = uint32(sFlow.DatagramVersion)
		sFlowData.AgentIP = sFlow.AgentAddress.String()
		sFlowData.AgentUptime = sFlow.AgentUptime
		sFlowData.SampleCount = sFlow.SampleCount

	}
	return &sFlowData
}
