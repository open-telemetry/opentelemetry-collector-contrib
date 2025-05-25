// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"errors"
	"net/netip"
	"time"

	"github.com/netsampler/goflow2/v2/producer"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

var (

	// https://www.iana.org/assignments/ieee-802-numbers/ieee-802-numbers.xhtml#ieee-802-numbers-1
	etypeNames = map[uint32]string{
		0x806:  "arp",
		0x800:  "ipv4",
		0x814c: "snmp",
		0x86dd: "ipv6",
		0x8847: "mpls",
		0x888e: "eapol",
		0x88cc: "lldp",
		0x88e5: "macsec",
		0x88f5: "mvrp",
		0x88f7: "ptp",
		0xa0ed: "6lowpan",
	}

	// https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml
	transportProtocolNames = map[uint32]string{
		0:   "hopopt",
		1:   "icmp",
		2:   "igmp",
		3:   "ggp",
		4:   "ipv4",
		5:   "st",
		6:   "tcp",
		7:   "cbt",
		8:   "egp",
		9:   "igp",
		10:  "bbn-rcc-mon",
		11:  "nvp-ii",
		12:  "pup",
		13:  "argus",
		14:  "emcon",
		15:  "xnet",
		16:  "chaos",
		17:  "udp",
		18:  "mux",
		19:  "dcn-meas",
		20:  "hmp",
		21:  "prm",
		22:  "xns-idp",
		23:  "trunk-1",
		24:  "trunk-2",
		25:  "leaf-1",
		26:  "leaf-2",
		27:  "rdp",
		28:  "irtp",
		29:  "iso-tp4",
		30:  "netblt",
		31:  "mfe-nsp",
		32:  "merit-inp",
		33:  "dccp",
		34:  "3pc",
		35:  "idpr",
		36:  "xtp",
		37:  "ddp",
		38:  "idpr-cmtp",
		39:  "tp++",
		40:  "il",
		41:  "ipv6",
		42:  "sdrp",
		43:  "ipv6-route",
		44:  "ipv6-frag",
		45:  "idrp",
		46:  "rsvp",
		47:  "gre",
		48:  "dsr",
		49:  "bna",
		50:  "esp",
		51:  "ah",
		52:  "i-nlsp",
		53:  "swipe",
		54:  "narp",
		55:  "min-ipv4",
		56:  "tlsp",
		57:  "skip",
		58:  "ipv6-icmp",
		59:  "ipv6-nonxt",
		60:  "ipv6-opts",
		61:  "any-host-internal-protocol",
		62:  "cftp",
		63:  "any-local-network",
		64:  "sat-expak",
		65:  "kryptolan",
		66:  "rvd",
		67:  "ippc",
		68:  "any-distributed-file-system",
		69:  "sat-mon",
		70:  "visa",
		71:  "ipcv",
		72:  "cpnx",
		73:  "cphb",
		74:  "wsn",
		75:  "pvp",
		76:  "br-sat-mon",
		77:  "sun-nd",
		78:  "wb-mon",
		79:  "wb-expak",
		80:  "iso-ip",
		81:  "vmtp",
		82:  "secure-vmtp",
		83:  "vines",
		84:  "iptm",
		85:  "nsfnet-igp",
		86:  "dgp",
		87:  "tcf",
		88:  "eigrp",
		89:  "ospfigp",
		90:  "sprite-rpc",
		91:  "larp",
		92:  "mtp",
		93:  "ax.25",
		94:  "ipip",
		95:  "micp",
		96:  "scc-sp",
		97:  "etherip",
		98:  "encap",
		99:  "any-private-encryption-scheme",
		100: "gmtp",
		101: "ifmp",
		102: "pnni",
		103: "pim",
		104: "aris",
		105: "scps",
		106: "qnx",
		107: "a/n",
		108: "ipcomp",
		109: "snp",
		110: "compaq-peer",
		111: "ipx-in-ip",
		112: "vrrp",
		113: "pgm",
		114: "any-0-hop-protocol",
		115: "l2tp",
		116: "ddx",
		117: "iatp",
		118: "stp",
		119: "srp",
		120: "uti",
		121: "smp",
		122: "sm",
		123: "ptp",
		124: "isis over ipv4",
		125: "fire",
		126: "crtp",
		127: "crudp",
		128: "sscopmce",
		129: "iplt",
		130: "sps",
		131: "pipe",
		132: "sctp",
		133: "fc",
		134: "rsvp-e2e-ignore",
		135: "mobility header",
		136: "udplite",
		137: "mpls-in-ip",
		138: "manet",
		139: "hip",
		140: "shim6",
		141: "wesp",
		142: "rohc",
		143: "ethernet",
		144: "aggfrag",
		145: "nsh",
	}

	flowTypeNames = map[int32]string{
		0: "unknown",
		1: "sflow_5",
		2: "netflow_v5",
		3: "netflow_v9",
		4: "ipfix",
	}
)

func getEtypeName(etype uint32) string {
	if name, ok := etypeNames[etype]; ok {
		return name
	}
	return "unknown"
}

func getTransportName(proto uint32) string {
	if name, ok := transportProtocolNames[proto]; ok {
		return name
	}
	return "unknown"
}

func getFlowTypeName(flowType int32) string {
	if name, ok := flowTypeNames[flowType]; ok {
		return name
	}
	return "unknown"
}

// addMessageAttributes parses the message attributes and adds them to the log record
func addMessageAttributes(m producer.ProducerMessage, r *plog.LogRecord) error {
	// we know msg is ProtoProducerMessage because that is the parent producer
	pm, ok := m.(*protoproducer.ProtoProducerMessage)
	if !ok {
		return errors.New("this flow message is not ProtoProducerMessage, this is not expected")
	}

	// Parse IP addresses bytes to netip.Addr
	srcAddr, _ := netip.AddrFromSlice(pm.SrcAddr)
	dstAddr, _ := netip.AddrFromSlice(pm.DstAddr)
	samplerAddr, _ := netip.AddrFromSlice(pm.SamplerAddress)

	// Time the receiver received the message
	receivedTime := time.Unix(0, int64(pm.TimeReceivedNs))
	startTime := time.Unix(0, int64(pm.TimeFlowStartNs))

	r.SetObservedTimestamp(pcommon.NewTimestampFromTime(receivedTime))
	r.SetTimestamp(pcommon.NewTimestampFromTime(startTime))

	// Source and destination attributes
	r.Attributes().PutStr(string(semconv.SourceAddressKey), srcAddr.String())
	r.Attributes().PutInt(string(semconv.SourcePortKey), int64(pm.SrcPort))
	r.Attributes().PutStr(string(semconv.DestinationAddressKey), dstAddr.String())
	r.Attributes().PutInt(string(semconv.DestinationPortKey), int64(pm.DstPort))

	// Network attributes
	r.Attributes().PutStr(string(semconv.NetworkTransportKey), getTransportName(pm.Proto))
	r.Attributes().PutStr(string(semconv.NetworkTypeKey), getEtypeName(pm.Etype))

	// There is no semconv as of today for these
	r.Attributes().PutInt("flow.io.bytes", int64(pm.Bytes))
	r.Attributes().PutInt("flow.io.packets", int64(pm.Packets))
	r.Attributes().PutStr("flow.type", getFlowTypeName(int32(pm.Type)))
	r.Attributes().PutInt("flow.sequence_num", int64(pm.SequenceNum))
	r.Attributes().PutInt("flow.time_received", int64(pm.TimeReceivedNs))
	r.Attributes().PutInt("flow.start", int64(pm.TimeFlowStartNs))
	r.Attributes().PutInt("flow.end", int64(pm.TimeFlowEndNs))
	r.Attributes().PutInt("flow.sampling_rate", int64(pm.SamplingRate))
	r.Attributes().PutStr("flow.sampler_address", samplerAddr.String())

	return nil
}
