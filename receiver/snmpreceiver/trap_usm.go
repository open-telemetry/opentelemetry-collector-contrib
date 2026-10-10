// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// trapParser is owned by the receive goroutine. USM keys remain local to this
// receiver: gosnmp's process-wide password cache otherwise retains a derived
// privacy key for every engine ID, even before it authenticates the sender.
type trapParser struct {
	cfg     *TrapsConfig
	plain   gosnmp.GoSNMP
	user    string
	minimum gosnmp.SnmpV3MsgFlags
	auth    gosnmp.SnmpV3AuthProtocol
	privacy gosnmp.SnmpV3PrivProtocol
	authKey []byte
	privKey []byte
	engines map[string]trapUSMKeys
}

type trapUSMKeys struct {
	auth    []byte
	privacy []byte
}

type trapUSMEnvelope struct {
	flags           gosnmp.SnmpV3MsgFlags
	engine          string
	user            string
	boots           uint32
	engineTime      uint32
	digest          []byte
	digestAt        int
	digestHeaderLen int
	privacy         []byte
}

func newTrapParser(cfg *TrapsConfig) (*trapParser, error) {
	parser := &trapParser{
		cfg: cfg, plain: gosnmp.GoSNMP{Version: gosnmp.Version2c},
		auth: gosnmp.NoAuth, privacy: gosnmp.NoPriv,
		engines: make(map[string]trapUSMKeys),
	}
	if cfg.V3 == nil {
		return parser, nil
	}
	parser.user = cfg.V3.User
	switch strings.ToLower(cfg.V3.SecurityLevel) {
	case "no_auth_no_priv":
	case "auth_no_priv":
		parser.minimum = gosnmp.AuthNoPriv
		parser.auth = getAuthProtocol(cfg.V3.AuthType)
	case "auth_priv":
		parser.minimum = gosnmp.AuthPriv
		parser.auth = getAuthProtocol(cfg.V3.AuthType)
		parser.privacy = getPrivacyProtocol(cfg.V3.PrivacyType)
	default:
		return nil, errors.New("invalid traps.v3.security_level")
	}
	if parser.auth != gosnmp.NoAuth {
		parser.authKey = trapPasswordKey(parser.auth.HashType(), []byte(cfg.V3.AuthPassword))
	}
	if parser.privacy != gosnmp.NoPriv {
		parser.privKey = trapPasswordKey(parser.auth.HashType(), []byte(cfg.V3.PrivacyPassword))
	}
	return parser, nil
}

func (p *trapParser) UnmarshalTrap(wire []byte, useResponseSecurityParameters bool) (*gosnmp.SnmpPacket, error) {
	if err := checkTrapEnvelope(wire); err != nil {
		return nil, err
	}
	_, message, _, _ := trapTLV(wire)
	_, version, _, _ := trapTLV(message)
	if version[0] != byte(gosnmp.Version3) {
		parsingWire, err := normalizeTrapInformForParsing(wire)
		if err != nil {
			return nil, err
		}
		return unmarshalTrapPacket(&p.plain, parsingWire, useResponseSecurityParameters)
	}
	if p.cfg.V3 == nil || !slices.Contains(p.cfg.Versions, "v3") {
		return nil, errors.New("SNMPv3 USM is not configured")
	}
	envelope, err := readTrapUSM(wire)
	if err != nil {
		return nil, err
	}
	level := envelope.flags & gosnmp.AuthPriv
	if level == 2 || level < p.minimum {
		return nil, errors.New("SNMPv3 notification security level is not allowed")
	}
	if envelope.user != p.user {
		return nil, errors.New("SNMPv3 notification user is not allowed")
	}
	keys, cached := p.engines[envelope.engine]
	if level&gosnmp.AuthNoPriv != 0 {
		if p.auth == gosnmp.NoAuth || len(envelope.digest) != trapDigestSize(p.auth) {
			return nil, errors.New("invalid SNMPv3 authentication parameters")
		}
		if envelope.boots == math.MaxInt32 {
			return nil, errors.New("SNMPv3 notification is outside the USM time window")
		}
		if !cached {
			keys.auth = trapLocalizeKey(p.auth.HashType(), p.authKey, envelope.engine)
		}
		// Authenticate the original message, with only its digest value zeroed.
		// No input-dependent password hashing or cache insertion precedes this.
		if !verifyTrapDigest(wire, envelope, p.auth.HashType(), keys.auth) {
			return nil, errors.New("SNMPv3 notification is not authentic")
		}
		if !cached && len(p.engines) >= maxTrapEngines {
			return nil, errors.New("SNMPv3 notification engine limit reached")
		}
	} else if len(envelope.digest) != 0 || len(envelope.privacy) != 0 {
		return nil, errors.New("invalid unauthenticated SNMPv3 security parameters")
	}
	if level == gosnmp.AuthPriv {
		if p.privacy == gosnmp.NoPriv || len(envelope.privacy) != 8 {
			return nil, errors.New("invalid SNMPv3 privacy parameters")
		}
		if !cached {
			keys.privacy = trapPrivacyKey(p.auth.HashType(), p.privacy, p.privKey, envelope.engine)
		}
	} else if len(envelope.privacy) != 0 {
		return nil, errors.New("unexpected SNMPv3 privacy parameters")
	}
	parsingWire := wire
	if level&gosnmp.AuthNoPriv != 0 {
		parsingWire, err = normalizeTrapUSMForParsing(wire, envelope, p.auth.HashType(), keys.auth)
		if err != nil {
			return nil, err
		}
	}
	// A fresh parser and exact prelocalized engine identity keep gosnmp from
	// deriving keys or writing its global cache, including during InitSecurityKeys.
	params := &gosnmp.UsmSecurityParameters{
		UserName: p.user, AuthoritativeEngineID: envelope.engine,
		AuthenticationProtocol: gosnmp.NoAuth, PrivacyProtocol: gosnmp.NoPriv,
	}
	if level&gosnmp.AuthNoPriv != 0 {
		params.AuthenticationProtocol, params.SecretKey = p.auth, keys.auth
	}
	if level == gosnmp.AuthPriv {
		params.PrivacyProtocol, params.PrivacyKey = p.privacy, keys.privacy
	}
	parser := &gosnmp.GoSNMP{
		Version: gosnmp.Version3, MsgFlags: envelope.flags,
		SecurityModel: gosnmp.UserSecurityModel, SecurityParameters: params,
	}
	packet, err := unmarshalTrapPacket(parser, parsingWire, useResponseSecurityParameters)
	if err == nil && level&gosnmp.AuthNoPriv != 0 {
		// Return the authenticated sender's digest, rather than the signature
		// used only to accommodate the dependency's BER parsing restriction.
		if decoded, ok := packet.SecurityParameters.(*gosnmp.UsmSecurityParameters); ok {
			decoded.AuthenticationParameters = string(envelope.digest)
		}
	}
	if err == nil && level&gosnmp.AuthNoPriv != 0 && packet.PDUType == gosnmp.SNMPv2Trap && !cached {
		p.engines[envelope.engine] = keys
	}
	return packet, err
}

// readTrapUSM validates the six USM fields before gosnmp can act on an engine ID.
// The envelope has already been checked. Digest offsets refer to the original
// wire message, including the enclosing OCTET STRING and SEQUENCE headers.
func readTrapUSM(wire []byte) (trapUSMEnvelope, error) {
	var result trapUSMEnvelope
	_, message, _, _ := trapTLV(wire)
	_, _, message, _ = trapTLV(message)
	_, header, message, _ := trapTLV(message)
	for range 2 {
		_, _, header, _ = trapTLV(header)
	}
	_, flags, header, _ := trapTLV(header)
	result.flags = gosnmp.SnmpV3MsgFlags(flags[0])
	_, model, _, _ := trapTLV(header)
	securityModel, err := trapUSMInteger(model)
	if err != nil || securityModel != uint32(gosnmp.UserSecurityModel) {
		return result, errors.New("unsupported SNMPv3 security model")
	}
	_, security, rest, _ := trapTLV(message)
	securityAt := len(wire) - len(rest) - len(security)
	tag, fields, rest, err := trapTLV(security)
	if err != nil || tag != byte(gosnmp.Sequence) || len(rest) != 0 {
		return result, errors.New("invalid SNMPv3 USM sequence")
	}
	var values [6][]byte
	for i := range values {
		var field []byte
		encodedLength := len(fields)
		tag, field, fields, err = trapTLV(fields)
		want := byte(gosnmp.OctetString)
		if i == 1 || i == 2 {
			want = byte(gosnmp.Integer)
		}
		if err != nil || tag != want {
			return result, errors.New("invalid SNMPv3 USM field")
		}
		values[i] = field
		if i == 4 {
			result.digestHeaderLen = encodedLength - len(fields) - len(field)
			result.digestAt = securityAt + len(security) - len(fields) - len(field)
		}
	}
	if len(fields) != 0 || len(values[0]) < 5 || len(values[0]) > 32 || len(values[3]) == 0 || len(values[3]) > 32 {
		return result, errors.New("invalid SNMPv3 engine identity or user")
	}
	result.boots, err = trapUSMInteger(values[1])
	if err != nil {
		return result, err
	}
	result.engineTime, err = trapUSMInteger(values[2])
	if err != nil {
		return result, err
	}
	result.engine, result.user = string(values[0]), string(values[3])
	result.digest, result.privacy = values[4], values[5]
	return result, nil
}

// normalizeTrapUSMForParsing runs only after authenticating the original wire.
// RFC 3417 section 8 permits extra definite-length octets, but gosnmp blanks a
// digest assuming a two-byte TLV header. Shorten that header in a parsing copy
// and sign the copy with the verified key so its own authentication still runs.
// The scoped PDU, including encrypted bytes, is preserved without modification.
func normalizeTrapUSMForParsing(wire []byte, envelope trapUSMEnvelope, hash crypto.Hash, key []byte) ([]byte, error) {
	if envelope.digestHeaderLen == 2 {
		return wire, nil
	}
	_, message, _, _ := trapTLV(wire)
	_, _, rest, _ := trapTLV(message)
	_, _, rest, _ = trapTLV(rest)
	prefix := message[:len(message)-len(rest)]
	_, security, scoped, _ := trapTLV(rest)
	_, fields, _, _ := trapTLV(security)
	digestField := fields
	for range 4 {
		_, _, digestField, _ = trapTLV(digestField)
	}
	_, _, afterDigest, _ := trapTLV(digestField)
	usm := append([]byte(nil), fields[:len(fields)-len(digestField)]...)
	usm = append(usm, byte(gosnmp.OctetString), byte(len(envelope.digest)))
	usm = append(usm, make([]byte, len(envelope.digest))...)
	usm = append(usm, afterDigest...)
	message = append([]byte(nil), prefix...)
	message = appendTrapTLV(message, byte(gosnmp.OctetString), appendTrapTLV(nil, byte(gosnmp.Sequence), usm))
	message = append(message, scoped...)
	parsingWire := appendTrapTLV(nil, byte(gosnmp.Sequence), message)
	parsingEnvelope, err := readTrapUSM(parsingWire)
	if err != nil {
		return nil, err
	}
	copy(parsingEnvelope.digest, calculateTrapDigest(parsingWire, parsingEnvelope, hash, key))
	return parsingWire, nil
}

func trapUSMInteger(value []byte) (uint32, error) {
	if len(value) == 0 || len(value) > 8 || value[0]&0x80 != 0 {
		return 0, errors.New("invalid SNMPv3 USM integer")
	}
	var number uint64
	for _, b := range value {
		number = number<<8 | uint64(b)
	}
	if number > math.MaxInt32 {
		return 0, errors.New("SNMPv3 USM integer exceeds its range")
	}
	return uint32(number), nil
}

// trapPasswordKey implements RFC 3414 appendix A.2. A reusable block avoids
// allocating for each iteration of the required one-megabyte password stream.
func trapPasswordKey(hash crypto.Hash, password []byte) []byte {
	if len(password) == 0 {
		return nil
	}
	digest := hash.New()
	var block [64]byte
	position := 0
	for count := 0; count < 1048576; count += len(block) {
		for i := range block {
			block[i] = password[position]
			position = (position + 1) % len(password)
		}
		_, _ = digest.Write(block[:])
	}
	return digest.Sum(nil)
}

// RFC 3414 section 2.6: Kul = H(Ku || engineID || Ku).
func trapLocalizeKey(hash crypto.Hash, master []byte, engine string) []byte {
	digest := hash.New()
	_, _ = digest.Write(master)
	_, _ = digest.Write([]byte(engine))
	_, _ = digest.Write(master)
	return digest.Sum(nil)
}

func trapPrivacyKey(hash crypto.Hash, protocol gosnmp.SnmpV3PrivProtocol, master []byte, engine string) []byte {
	key := trapLocalizeKey(hash, master, engine)
	length := 16
	switch protocol {
	case gosnmp.DES:
		return key
	case gosnmp.AES192, gosnmp.AES192C:
		length = 24
	case gosnmp.AES256, gosnmp.AES256C:
		length = 32
	}
	if len(key) < length {
		switch protocol {
		case gosnmp.AES192, gosnmp.AES256:
			// Blumenthal extension hashes the localized key once.
			digest := hash.New()
			_, _ = digest.Write(key)
			key = append(key, digest.Sum(nil)...)
		default:
			// Reeder extension treats the localized key as a password. This
			// expensive work occurs only after a sender has authenticated.
			extended := trapLocalizeKey(hash, trapPasswordKey(hash, key), engine)
			key = append(key, extended...)
		}
	}
	return key[:length]
}

func trapDigestSize(protocol gosnmp.SnmpV3AuthProtocol) int {
	switch protocol {
	case gosnmp.MD5, gosnmp.SHA:
		return 12
	case gosnmp.SHA224:
		return 16
	case gosnmp.SHA256:
		return 24
	case gosnmp.SHA384:
		return 32
	case gosnmp.SHA512:
		return 48
	default:
		return 0
	}
}

func verifyTrapDigest(wire []byte, envelope trapUSMEnvelope, hash crypto.Hash, key []byte) bool {
	return hmac.Equal(calculateTrapDigest(wire, envelope, hash, key), envelope.digest)
}

func calculateTrapDigest(wire []byte, envelope trapUSMEnvelope, hash crypto.Hash, key []byte) []byte {
	digest := hmac.New(hash.New, key)
	_, _ = digest.Write(wire[:envelope.digestAt])
	var zeros [48]byte
	_, _ = digest.Write(zeros[:len(envelope.digest)])
	_, _ = digest.Write(wire[envelope.digestAt+len(envelope.digest):])
	return digest.Sum(nil)[:len(envelope.digest)]
}

// Recovery is deliberately limited to the external parser. It mutates the wire
// digest, so use a copy and leave the authenticated input available to callers.
func unmarshalTrapPacket(parser *gosnmp.GoSNMP, wire []byte, useResponseSecurityParameters bool) (packet *gosnmp.SnmpPacket, err error) {
	defer func() {
		if recover() != nil {
			packet = nil
			err = errors.New("malformed SNMP notification")
		}
	}()
	packet, err = parser.UnmarshalTrap(bytes.Clone(wire), useResponseSecurityParameters)
	if err != nil {
		// Dependency errors may contain raw BER, including community credentials.
		return nil, errors.New("malformed SNMP notification")
	}
	return packet, nil
}
