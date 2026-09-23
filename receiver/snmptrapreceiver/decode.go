// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver"

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gosnmp/gosnmp"
)

const (
	snmpTrapOID = "1.3.6.1.6.3.1.1.4.1.0"
	sysUpTime   = "1.3.6.1.2.1.1.3.0"
)

// DecodeOptions control optional fields on the record.
type DecodeOptions struct {
	IncludeCommunity bool
	Translator       Translator
}

// Decode turns a gosnmp trap/inform packet into a Record.
func Decode(packet *gosnmp.SnmpPacket, addr *net.UDPAddr, now time.Time, opts DecodeOptions) Record {
	tr := opts.Translator
	if tr == nil {
		tr = NoopTranslator{}
	}

	rec := Record{
		Time:     now,
		Version:  versionName(packet.Version),
		PDUType:  pduKind(packet.PDUType),
		Varbinds: make([]Varbind, 0, len(packet.Variables)),
	}
	if addr != nil {
		rec.Source = addr.IP.String()
	}
	if opts.IncludeCommunity && packet.Community != "" {
		rec.Community = packet.Community
	}
	if packet.Version == gosnmp.Version3 {
		if packet.ContextName != "" {
			rec.ContextName = packet.ContextName
		}
		if packet.ContextEngineID != "" {
			rec.EngineID = fmt.Sprintf("%x", packet.ContextEngineID)
		}
	}

	if packet.Version == gosnmp.Version1 {
		rec.Enterprise = NormalizeOID(packet.Enterprise)
		gt, st := packet.GenericTrap, packet.SpecificTrap
		rec.GenericTrap = &gt
		rec.SpecificTrap = &st
		rec.SysUpTime = packet.Timestamp
		if packet.AgentAddress != "" {
			rec.AgentAddress = packet.AgentAddress
		}
		if oid := v1TrapOID(packet); oid != "" {
			setTrapIdentity(&rec, oid, tr)
		}
	}

	for _, v := range packet.Variables {
		oid := NormalizeOID(v.Name)
		vb := Varbind{
			OID:  oid,
			Type: typeName(v.Type),
		}
		if l := lookup(tr, oid); l.Name != oid {
			vb.Name = l.Name
		}

		switch v.Type {
		case gosnmp.ObjectIdentifier:
			val := NormalizeOID(stringify(v.Value))
			if l := lookup(tr, val); l.Name != "" {
				vb.Value = l.Name
			} else {
				vb.Value = val
			}
			if oid == snmpTrapOID || oid == strings.TrimSuffix(snmpTrapOID, ".0") {
				setTrapIdentity(&rec, val, tr)
				continue
			}
		default:
			vb.Value = formatValue(v)
		}

		if oid == sysUpTime {
			if n, ok := asUint(v.Value); ok {
				rec.SysUpTime = n
			}
		}
		rec.Varbinds = append(rec.Varbinds, vb)
	}

	if rec.TrapName == "" && rec.TrapOID != "" {
		rec.TrapName = rec.TrapOID
	}
	return rec
}

func setTrapIdentity(rec *Record, oid string, tr Translator) {
	oid = NormalizeOID(oid)
	rec.TrapOID = oid
	l := lookup(tr, oid)
	rec.TrapName = l.Name
	rec.MIB = l.MIB
}

func lookup(tr Translator, oid string) Lookup {
	oid = NormalizeOID(oid)
	if tr == nil || oid == "" {
		return Lookup{Name: oid}
	}
	l, err := tr.Lookup(oid)
	if err != nil || strings.TrimSpace(l.Name) == "" {
		return Lookup{Name: oid}
	}
	return Lookup{Name: l.Name, MIB: l.MIB}
}

// NormalizeOID strips a leading dot.
func NormalizeOID(oid string) string {
	return strings.TrimPrefix(strings.TrimSpace(oid), ".")
}

func v1TrapOID(packet *gosnmp.SnmpPacket) string {
	// RFC 2576 §3.1
	if packet.GenericTrap >= 0 && packet.GenericTrap < 6 {
		return "1.3.6.1.6.3.1.1.5." + strconv.Itoa(packet.GenericTrap+1)
	}
	if packet.GenericTrap == 6 {
		ent := NormalizeOID(packet.Enterprise)
		if ent == "" {
			return ""
		}
		return ent + ".0." + strconv.Itoa(packet.SpecificTrap)
	}
	return ""
}

func versionName(v gosnmp.SnmpVersion) string {
	switch v {
	case gosnmp.Version1:
		return "1"
	case gosnmp.Version2c:
		return "2c"
	case gosnmp.Version3:
		return "3"
	default:
		return strconv.Itoa(int(v))
	}
}

func pduKind(t gosnmp.PDUType) string {
	switch t {
	case gosnmp.InformRequest:
		return "inform"
	case gosnmp.Trap, gosnmp.SNMPv2Trap:
		return "trap"
	default:
		if t == 0 {
			return "trap"
		}
		return fmt.Sprintf("%d", t)
	}
}

func typeName(t gosnmp.Asn1BER) string {
	switch t {
	case gosnmp.Boolean:
		return "Boolean"
	case gosnmp.Integer:
		return "Integer"
	case gosnmp.BitString:
		return "BitString"
	case gosnmp.OctetString:
		return "OctetString"
	case gosnmp.Null:
		return "Null"
	case gosnmp.ObjectIdentifier:
		return "OID"
	case gosnmp.IPAddress:
		return "IPAddress"
	case gosnmp.Counter32:
		return "Counter32"
	case gosnmp.Gauge32:
		return "Gauge32"
	case gosnmp.TimeTicks:
		return "TimeTicks"
	case gosnmp.Opaque:
		return "Opaque"
	case gosnmp.NsapAddress:
		return "NsapAddress"
	case gosnmp.Counter64:
		return "Counter64"
	case gosnmp.Uinteger32:
		return "Uinteger32"
	case gosnmp.NoSuchObject:
		return "NoSuchObject"
	case gosnmp.NoSuchInstance:
		return "NoSuchInstance"
	case gosnmp.EndOfMibView:
		return "EndOfMibView"
	default:
		return fmt.Sprintf("type_%d", t)
	}
}

func formatValue(v gosnmp.SnmpPDU) any {
	switch val := v.Value.(type) {
	case []byte:
		if v.Type == gosnmp.IPAddress {
			if ip := net.IP(val); len(ip) == 4 || len(ip) == 16 {
				return ip.String()
			}
		}
		if utf8.Valid(val) {
			return string(val)
		}
		return hex.EncodeToString(val)
	case string:
		if v.Type == gosnmp.ObjectIdentifier {
			return NormalizeOID(val)
		}
		return val
	case net.IP:
		return val.String()
	default:
		return v.Value
	}
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}

func asUint(v any) (uint, bool) {
	switch n := v.(type) {
	case uint:
		return n, true
	case uint32:
		return uint(n), true
	case uint64:
		return uint(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	case int32:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	default:
		return 0, false
	}
}
