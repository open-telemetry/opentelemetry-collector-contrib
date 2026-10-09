// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

const (
	trapTimeWindow = 150 * time.Second
	maxTrapEngines = 4096
)

// trapEngineTime holds the non-authoritative engine's time synchronization state.
// It is accessed only by the UDP receive goroutine.
type trapEngineTime struct {
	boots    uint32
	latest   uint32
	received time.Time
}

func admitTrap(cfg *TrapsConfig, packet *gosnmp.SnmpPacket, engines map[string]trapEngineTime, now time.Time) error {
	var version string
	switch packet.Version {
	case gosnmp.Version1:
		version = "v1"
	case gosnmp.Version2c:
		version = "v2c"
	case gosnmp.Version3:
		version = "v3"
	default:
		return errors.New("unsupported SNMP notification version")
	}
	if !slices.Contains(cfg.Versions, version) {
		return errors.New("SNMP notification version is not enabled")
	}
	if packet.Version == gosnmp.Version1 {
		if packet.PDUType != gosnmp.Trap {
			return errors.New("SNMPv1 PDU is not a trap")
		}
	} else if packet.PDUType != gosnmp.SNMPv2Trap && packet.PDUType != gosnmp.InformRequest {
		return errors.New("SNMP PDU is not a notification")
	}
	if packet.Version != gosnmp.Version3 {
		if len(cfg.Communities) == 0 {
			return nil
		}
		for _, community := range cfg.Communities {
			if string(community) == packet.Community {
				return nil
			}
		}
		return errors.New("SNMP notification community is not allowed")
	}
	if cfg.V3 == nil || packet.SecurityModel != gosnmp.UserSecurityModel {
		return errors.New("SNMPv3 USM is not configured")
	}
	// An INFORM makes the receiver authoritative. Supporting it requires a
	// persistent local engine identity and engine boots state, unlike traps.
	if packet.PDUType == gosnmp.InformRequest {
		return errors.New("SNMPv3 informs are not supported")
	}
	params, ok := packet.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if !ok || params.UserName != cfg.V3.User {
		return errors.New("SNMPv3 notification user is not allowed")
	}
	level := packet.MsgFlags & gosnmp.AuthPriv
	if level == 2 {
		return errors.New("SNMPv3 privacy requires authentication")
	}
	var minimum gosnmp.SnmpV3MsgFlags
	switch strings.ToLower(cfg.V3.SecurityLevel) {
	case "auth_no_priv":
		minimum = gosnmp.AuthNoPriv
	case "auth_priv":
		minimum = gosnmp.AuthPriv
	}
	if level < minimum {
		return errors.New("SNMPv3 notification security level is below the configured minimum")
	}
	if len(params.AuthoritativeEngineID) < 5 || len(params.AuthoritativeEngineID) > 32 {
		return errors.New("SNMPv3 authoritative engine ID must contain 5 to 32 bytes")
	}
	if level&gosnmp.AuthNoPriv != 0 {
		return checkTrapTime(engines, params, now)
	}
	return nil
}

func checkTrapTime(engines map[string]trapEngineTime, params *gosnmp.UsmSecurityParameters, now time.Time) error {
	boots, engineTime := params.AuthoritativeEngineBoots, params.AuthoritativeEngineTime
	if boots >= math.MaxInt32 || engineTime > math.MaxInt32 {
		return errors.New("SNMPv3 notification is outside the USM time window")
	}
	key := params.AuthoritativeEngineID
	previous, exists := engines[key]
	if !exists {
		// Refuse additional engines instead of evicting synchronized state and
		// allowing an old authenticated message to establish a fresh window.
		if len(engines) >= maxTrapEngines {
			return errors.New("SNMPv3 notification engine limit reached")
		}
		engines[key] = trapEngineTime{boots: boots, latest: engineTime, received: now}
		return nil
	}
	if boots < previous.boots {
		return errors.New("SNMPv3 notification is outside the USM time window")
	}
	// RFC 3414 section 3.2 step 7b: only a newer engine boot or a later
	// authenticated engine time may re-synchronize the local clock.
	if boots > previous.boots || engineTime > previous.latest {
		engines[key] = trapEngineTime{boots: boots, latest: engineTime, received: now}
		return nil
	}
	expected := time.Duration(previous.latest)*time.Second + now.Sub(previous.received)
	if expected-time.Duration(engineTime)*time.Second > trapTimeWindow {
		return errors.New("SNMPv3 notification is outside the USM time window")
	}
	return nil
}

// checkTrapEnvelope checks privacy against the wire representation before gosnmp
// decrypts it. gosnmp accepts a plaintext scoped PDU even when the privacy flag
// is set, so checking decoded MsgFlags alone would not enforce encryption.
func checkTrapEnvelope(wire []byte) error {
	tag, message, rest, err := trapTLV(wire)
	if err != nil || tag != byte(gosnmp.Sequence) || len(rest) != 0 {
		return errors.New("invalid SNMP message envelope")
	}
	tag, version, message, err := trapTLV(message)
	if err != nil || tag != byte(gosnmp.Integer) || len(version) != 1 {
		return errors.New("invalid SNMP message version")
	}
	if version[0] != byte(gosnmp.Version3) {
		return nil
	}
	tag, header, message, err := trapTLV(message)
	if err != nil || tag != byte(gosnmp.Sequence) {
		return errors.New("invalid SNMPv3 message header")
	}
	for range 2 {
		tag, _, header, err = trapTLV(header)
		if err != nil || tag != byte(gosnmp.Integer) {
			return errors.New("invalid SNMPv3 message header")
		}
	}
	tag, flags, header, err := trapTLV(header)
	if err != nil || tag != byte(gosnmp.OctetString) || len(flags) != 1 || flags[0]&0xf8 != 0 {
		return errors.New("invalid SNMPv3 message flags")
	}
	tag, _, header, err = trapTLV(header)
	if err != nil || tag != byte(gosnmp.Integer) || len(header) != 0 {
		return errors.New("invalid SNMPv3 message security model")
	}
	tag, _, message, err = trapTLV(message)
	if err != nil || tag != byte(gosnmp.OctetString) {
		return errors.New("invalid SNMPv3 message security parameters")
	}
	tag, _, rest, err = trapTLV(message)
	if err != nil || len(rest) != 0 {
		return errors.New("invalid SNMPv3 scoped PDU")
	}
	wantTag := byte(gosnmp.Sequence)
	if flags[0]&2 != 0 {
		wantTag = byte(gosnmp.OctetString)
	}
	if tag != wantTag {
		return errors.New("SNMPv3 scoped PDU does not match the message privacy flag")
	}
	return nil
}

// trapTLV reads a definite-length BER field without allocating. Bounds are
// checked before indexing or converting a field length to int.
func trapTLV(wire []byte) (byte, []byte, []byte, error) {
	if len(wire) < 2 {
		return 0, nil, nil, errors.New("truncated BER field")
	}
	tag := wire[0]
	length := uint64(wire[1])
	wire = wire[2:]
	if length&0x80 != 0 {
		count := int(length & 0x7f)
		if count == 0 || count == 127 || len(wire) < count {
			return 0, nil, nil, errors.New("invalid BER field length")
		}
		// RFC 3417 permits extra definite-length octets, including leading
		// zeros. Bound every step by the available value bytes so a long
		// encoding cannot overflow the accumulator or describe missing data.
		limit := uint64(len(wire) - count)
		length = 0
		for _, b := range wire[:count] {
			if length > limit>>8 {
				return 0, nil, nil, errors.New("truncated BER field value")
			}
			length = length<<8 | uint64(b)
			if length > limit {
				return 0, nil, nil, errors.New("truncated BER field value")
			}
		}
		wire = wire[count:]
	}
	if length > uint64(len(wire)) {
		return 0, nil, nil, errors.New("truncated BER field value")
	}
	return tag, wire[:int(length)], wire[int(length):], nil
}

// safeUnmarshalTrap validates USM outside the external parser's panic recovery.
// Only the gosnmp call in unmarshalTrapPacket contains dependency panics and
// sanitizes dependency errors; receiver-owned validation errors remain useful.
func safeUnmarshalTrap(parser *trapParser, wire []byte) (*gosnmp.SnmpPacket, error) {
	return parser.UnmarshalTrap(wire, true)
}
