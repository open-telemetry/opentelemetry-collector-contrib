// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

const (
	trapIdentityOID = "1.3.6.1.6.3.1.1.4.1.0"
	trapUptimeOID   = "1.3.6.1.2.1.1.3.0"
)

// decodeTrap preserves the decoded notification in a structured body. Uptime is
// relative to device startup, so only the observed timestamp is populated.
func decodeTrap(packet *gosnmp.SnmpPacket, sender *net.UDPAddr, received time.Time, includeCommunity bool) (plog.Logs, error) {
	if packet == nil {
		return plog.Logs{}, errors.New("nil SNMP notification")
	}
	var version, kind, oid string
	var uptime uint64
	switch packet.Version {
	case gosnmp.Version1:
		version = "v1"
		if packet.PDUType != gosnmp.Trap {
			return plog.Logs{}, errors.New("SNMPv1 notification requires a Trap PDU")
		}
		kind = "trap"
		uptime = uint64(packet.Timestamp)
		if uptime > math.MaxUint32 {
			return plog.Logs{}, fmt.Errorf("invalid SNMPv1 notification uptime: %d", uptime)
		}
		switch {
		case packet.GenericTrap >= 0 && packet.GenericTrap < 6:
			oid = "1.3.6.1.6.3.1.1.5." + strconv.Itoa(packet.GenericTrap+1)
		case packet.GenericTrap == 6 && packet.SpecificTrap >= 0:
			enterprise, err := trapOID(packet.Enterprise)
			if err != nil {
				return plog.Logs{}, fmt.Errorf("invalid enterprise-specific trap OID: %w", err)
			}
			// RFC 3584 section 3.1 maps enterpriseSpecific to enterprise.0.specific.
			oid = enterprise + ".0." + strconv.Itoa(packet.SpecificTrap)
		default:
			return plog.Logs{}, fmt.Errorf("invalid SNMPv1 generic/specific trap: %d/%d", packet.GenericTrap, packet.SpecificTrap)
		}
	case gosnmp.Version2c, gosnmp.Version3:
		version = "v2c"
		if packet.Version == gosnmp.Version3 {
			version = "v3"
		}
		switch packet.PDUType {
		case gosnmp.SNMPv2Trap:
			kind = "trap"
		case gosnmp.InformRequest:
			kind = "inform"
		default:
			return plog.Logs{}, fmt.Errorf("unsupported notification PDU: %s", packet.PDUType)
		}
		// RFC 3416 requires sysUpTime.0 and snmpTrapOID.0 as the first two bindings.
		if len(packet.Variables) < 2 || strings.TrimPrefix(packet.Variables[0].Name, ".") != trapUptimeOID ||
			packet.Variables[0].Type != gosnmp.TimeTicks || strings.TrimPrefix(packet.Variables[1].Name, ".") != trapIdentityOID ||
			packet.Variables[1].Type != gosnmp.ObjectIdentifier {
			return plog.Logs{}, errors.New("notification must begin with sysUpTime.0 and snmpTrapOID.0 bindings")
		}
		var ok bool
		uptime, ok = trapUnsigned(packet.Variables[0].Value)
		if !ok || uptime > math.MaxUint32 {
			return plog.Logs{}, fmt.Errorf("invalid notification uptime: %v", packet.Variables[0].Value)
		}
		identity, ok := packet.Variables[1].Value.(string)
		if !ok {
			return plog.Logs{}, fmt.Errorf("notification OID has value type %T", packet.Variables[1].Value)
		}
		var err error
		oid, err = trapOID(identity)
		if err != nil {
			return plog.Logs{}, fmt.Errorf("invalid notification OID: %w", err)
		}
	default:
		return plog.Logs{}, fmt.Errorf("unsupported SNMP version: %d", packet.Version)
	}

	logs := plog.NewLogs()
	scope := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	scope.Scope().SetName(metadata.ScopeName)
	record := scope.LogRecords().AppendEmpty()
	record.SetObservedTimestamp(pcommon.NewTimestampFromTime(received))
	attrs := record.Attributes()
	attrs.PutStr("snmp.version", version)
	attrs.PutStr("snmp.pdu.type", kind)
	attrs.PutStr("snmp.trap.oid", oid)
	if sender != nil {
		attrs.PutStr("network.peer.address", sender.IP.String())
		attrs.PutInt("network.peer.port", int64(sender.Port))
	}
	body := record.Body().SetEmptyMap()
	body.PutStr("version", version)
	body.PutStr("pdu_type", kind)
	body.PutStr("trap_oid", oid)
	body.PutInt("sys_up_time", int64(uptime))
	if includeCommunity && packet.Community != "" {
		body.PutStr("community", packet.Community)
	}
	if packet.Version == gosnmp.Version1 {
		body.PutStr("enterprise", strings.TrimPrefix(packet.Enterprise, "."))
		body.PutStr("agent_address", packet.AgentAddress)
		body.PutInt("generic_trap", int64(packet.GenericTrap))
		body.PutInt("specific_trap", int64(packet.SpecificTrap))
	}
	if packet.Version == gosnmp.Version3 {
		body.PutEmptyBytes("context_engine_id").FromRaw([]byte(packet.ContextEngineID))
		body.PutStr("context_name", packet.ContextName)
	}
	bindings := body.PutEmptySlice("varbinds")
	bindings.EnsureCapacity(len(packet.Variables))
	for i, binding := range packet.Variables {
		name, err := trapOID(binding.Name)
		if err != nil {
			return plog.Logs{}, fmt.Errorf("varbind %d has invalid OID: %w", i, err)
		}
		entry := bindings.AppendEmpty().SetEmptyMap()
		entry.PutStr("oid", name)
		entry.PutStr("type", binding.Type.String())
		if err := trapValue(entry.PutEmpty("value"), binding); err != nil {
			return plog.Logs{}, fmt.Errorf("varbind %d (%s, %s): %w", i, name, binding.Type, err)
		}
	}
	return logs, nil
}

// trapOID validates numeric OIDs and removes the optional leading dot.
func trapOID(raw string) (string, error) {
	oid := strings.TrimPrefix(raw, ".")
	arcs := strings.Split(oid, ".")
	if len(arcs) < 2 || len(arcs) > 128 {
		return "", fmt.Errorf("invalid OID %q", raw)
	}
	var first uint64
	for i, arc := range arcs {
		if arc == "" || strings.HasPrefix(arc, "+") {
			return "", fmt.Errorf("invalid OID %q", raw)
		}
		n, err := strconv.ParseUint(arc, 10, 32)
		if err != nil || (i == 0 && n > 2) || (i == 1 && first < 2 && n > 39) {
			return "", fmt.Errorf("invalid OID %q", raw)
		}
		if i == 0 {
			first = n
		}
		arcs[i] = strconv.FormatUint(n, 10)
	}
	return strings.Join(arcs, "."), nil
}

func trapValue(dst pcommon.Value, binding gosnmp.SnmpPDU) error {
	switch binding.Type {
	case gosnmp.Null, gosnmp.NoSuchObject, gosnmp.NoSuchInstance, gosnmp.EndOfMibView:
		if binding.Value == nil {
			return nil
		}
	case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32, gosnmp.Counter64:
		if n, ok := trapUnsigned(binding.Value); ok {
			if binding.Type == gosnmp.Counter64 {
				// OTLP has signed integers only. Preserve the full uint64 range.
				dst.SetStr(strconv.FormatUint(n, 10))
				return nil
			}
			if n <= math.MaxUint32 {
				dst.SetInt(int64(n))
				return nil
			}
		}
	case gosnmp.Integer:
		switch n := binding.Value.(type) {
		case int:
			dst.SetInt(int64(n))
			return nil
		case int32:
			dst.SetInt(int64(n))
			return nil
		case int64:
			dst.SetInt(n)
			return nil
		}
	case gosnmp.OctetString, gosnmp.Opaque, gosnmp.NsapAddress:
		switch raw := binding.Value.(type) {
		case []byte:
			dst.SetEmptyBytes().FromRaw(raw)
			return nil
		case string:
			dst.SetEmptyBytes().FromRaw([]byte(raw))
			return nil
		}
	case gosnmp.ObjectIdentifier:
		if raw, ok := binding.Value.(string); ok {
			oid, err := trapOID(raw)
			if err != nil {
				return err
			}
			dst.SetStr(oid)
			return nil
		}
	case gosnmp.IPAddress:
		var ip net.IP
		switch raw := binding.Value.(type) {
		case string:
			ip = net.ParseIP(raw)
		case []byte:
			ip = net.IP(raw)
		case net.IP:
			ip = raw
		}
		if ip.To4() != nil || ip.To16() != nil {
			dst.SetStr(ip.String())
			return nil
		}
	case gosnmp.OpaqueFloat, gosnmp.OpaqueDouble:
		var n float64
		switch value := binding.Value.(type) {
		case float32:
			n = float64(value)
		case float64:
			n = value
		default:
			return fmt.Errorf("unexpected floating-point value type %T", binding.Value)
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			dst.SetStr(strconv.FormatFloat(n, 'g', -1, 64))
		} else {
			dst.SetDouble(n)
		}
		return nil
	case gosnmp.Boolean:
		if value, ok := binding.Value.(bool); ok {
			dst.SetBool(value)
			return nil
		}
	case gosnmp.BitString:
		if value, ok := binding.Value.(gosnmp.BitStringValue); ok {
			if value.BitLength < 0 || value.BitLength > len(value.Bytes)*8 {
				return fmt.Errorf("invalid bit string length %d", value.BitLength)
			}
			bits := dst.SetEmptyMap()
			bits.PutEmptyBytes("bytes").FromRaw(value.Bytes)
			bits.PutInt("bit_length", int64(value.BitLength))
			return nil
		}
	}
	return fmt.Errorf("unsupported or invalid %s value of type %T", binding.Type, binding.Value)
}

func trapUnsigned(value any) (uint64, bool) {
	switch n := value.(type) {
	case uint:
		return uint64(n), true
	case uint32:
		return uint64(n), true
	case uint64:
		return n, true
	case int:
		return uint64(n), n >= 0
	case int32:
		return uint64(n), n >= 0
	case int64:
		return uint64(n), n >= 0
	default:
		return 0, false
	}
}
