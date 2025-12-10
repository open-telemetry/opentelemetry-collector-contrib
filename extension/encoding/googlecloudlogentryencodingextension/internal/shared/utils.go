// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package shared // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"

import (
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// protocolNames are needed to know the name of the protocol number given by the field
// protocol in a flow log record.
// See https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml.
var protocolNames = map[uint32]string{
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
	55:  "mobile",
	56:  "tlsp",
	57:  "skip",
	58:  "ipv6-icmp",
	59:  "ipv6-nonxt",
	60:  "ipv6-opts",
	62:  "cftp",
	64:  "sat-expak",
	65:  "kryptolan",
	66:  "rvd",
	67:  "ippc",
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
	84:  "ttp",
	85:  "nsfnet-igp",
	86:  "dgp",
	87:  "tcf",
	88:  "eigrp",
	89:  "ospf",
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
	146: "homa",
	147: "bit-emu",
	255: "reserved",
}

// ProtocolName returns the textual protocol name for the provided protocol number.
func ProtocolName(protocol uint32) (string, bool) {
	name, ok := protocolNames[protocol]
	return name, ok
}

// strToInt converts a string representation of a number into a 64-bit integer.
// The string must contain a valid base-10 integer. If parsing fails, the function
// returns -1 and an error describing the failure.
func strToInt(numberStr string) (int64, error) {
	num, err := strconv.ParseInt(numberStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to convert string %q to int64", numberStr)
	}
	return num, nil
}

// AddStrAsInt parses a string value into an integer and adds it to the attributes map.
// If the input string is empty, the function does nothing and returns nil.
// Returns an error if the string cannot be parsed as an integer.
func AddStrAsInt(field, value string, attributes pcommon.Map) error {
	if value == "" {
		return nil
	}
	n, err := strToInt(value)
	if err != nil {
		return err
	}
	attributes.PutInt(field, n)
	return nil
}

// PutStr places value in the attributes map, if not empty
func PutStr(field, value string, attributes pcommon.Map) {
	if value != "" {
		attributes.PutStr(field, value)
	}
}

// PutInt places value in the attributes map, if not nil
func PutInt(field string, value *int64, attributes pcommon.Map) {
	if value != nil {
		attributes.PutInt(field, *value)
	}
}

// PutBool places the value in the attributes map, if not nil
func PutBool(field string, value *bool, attributes pcommon.Map) {
	if value != nil {
		attributes.PutBool(field, *value)
	}
}

// PutDouble places the value in the attributes map, if not nil
func PutDouble(field string, value *float64, attributes pcommon.Map) {
	if value != nil {
		attributes.PutDouble(field, *value)
	}
}

// PutStrIfNotPresent places value in the attributes map if not empty and not already present
// Returns an error if the attribute is already present with a different value.
func PutStrIfNotPresent(field, value string, attributes pcommon.Map) (bool, error) {
	if value == "" {
		return false, nil
	}
	val, exists := attributes.Get(field)
	if !exists {
		attributes.PutStr(field, value)
		return true, nil
	}
	if val.Str() != value {
		return false, fmt.Errorf("attribute %q already present with different value: existing=%q, new=%q", field, val.Str(), value)
	}
	return false, nil
}

// PutDurationAsSeconds parses a duration string (e.g., "0.063s") and stores it as seconds (float64).
// If the input string is empty, the function does nothing and returns nil.
// Returns an error if the string cannot be parsed as a duration.
func PutDurationAsSeconds(field, value string, attributes pcommon.Map) error {
	if value == "" {
		return nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("failed to parse duration %q: %w", value, err)
	}
	attributes.PutDouble(field, d.Seconds())
	return nil
}
