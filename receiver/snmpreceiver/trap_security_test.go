// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"bytes"
	"crypto/hmac"
	"encoding/hex"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

const testTrapEngine = "engine-000001"

func securityTestConfig(auth gosnmp.SnmpV3AuthProtocol, privacy gosnmp.SnmpV3PrivProtocol, flags gosnmp.SnmpV3MsgFlags) *TrapsConfig {
	level := "no_auth_no_priv"
	switch flags & gosnmp.AuthPriv {
	case gosnmp.AuthNoPriv:
		level = "auth_no_priv"
	case gosnmp.AuthPriv:
		level = "auth_priv"
	}
	cfg := defaultTrapsConfig()
	cfg.Versions = []string{"v1", "v2c", "v3"}
	cfg.V3 = &TrapV3Config{
		User: "trap-user", SecurityLevel: level, AuthType: auth.String(),
		AuthPassword: "auth-password", PrivacyType: privacy.String(), PrivacyPassword: "priv-password",
	}
	return cfg
}

// Generate positive fixtures with gosnmp's own key derivation, independently of
// the receiver's implementation. Only fixture generation touches its key cache.
func securityTestPacket(t *testing.T, auth gosnmp.SnmpV3AuthProtocol, privacy gosnmp.SnmpV3PrivProtocol, flags gosnmp.SnmpV3MsgFlags, engine string) *gosnmp.SnmpPacket {
	t.Helper()
	params := &gosnmp.UsmSecurityParameters{
		UserName: "trap-user", AuthoritativeEngineID: engine,
		AuthoritativeEngineBoots: 7, AuthoritativeEngineTime: 100,
		AuthenticationProtocol: gosnmp.NoAuth, PrivacyProtocol: gosnmp.NoPriv,
	}
	if flags&gosnmp.AuthNoPriv != 0 {
		params.AuthenticationProtocol, params.AuthenticationPassphrase = auth, "auth-password"
	}
	if flags&gosnmp.AuthPriv == gosnmp.AuthPriv {
		params.PrivacyProtocol, params.PrivacyPassphrase = privacy, "priv-password"
		params.PrivacyParameters = []byte{1, 2, 3, 4, 5, 6, 7, 8}
	}
	require.NoError(t, params.InitSecurityKeys())
	return &gosnmp.SnmpPacket{
		Version: gosnmp.Version3, MsgFlags: flags, SecurityModel: gosnmp.UserSecurityModel,
		SecurityParameters: params, MsgID: 1, MsgMaxSize: 65507,
		ContextEngineID: engine, PDUType: gosnmp.SNMPv2Trap, RequestID: 42,
		Variables: []gosnmp.SnmpPDU{
			{Name: trapUptimeOID, Type: gosnmp.TimeTicks, Value: uint32(100)},
			{Name: trapIdentityOID, Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.6.3.1.1.5.3"},
			{Name: "1.3.6.1.2.1.1.5.0", Type: gosnmp.OctetString, Value: []byte("device-name")},
		},
	}
}

func securityTestWire(t *testing.T, packet *gosnmp.SnmpPacket) []byte {
	t.Helper()
	wire, err := packet.MarshalMsg()
	require.NoError(t, err)
	require.NoError(t, checkTrapEnvelope(wire))
	return wire
}

// RFC 3417 allows long-form lengths with extra octets even for short values.
// Sign the newly encoded fixture directly with gosnmp's localized key, without
// using the receiver's digest offsets or authentication implementation.
func securityTestLongAuthenticationHeader(t *testing.T, packet *gosnmp.SnmpPacket, lengthOctets int) []byte {
	t.Helper()
	require.GreaterOrEqual(t, lengthOctets, 1)
	require.LessOrEqual(t, lengthOctets, 126)
	wire := securityTestWire(t, packet)
	_, message, _, _ := trapTLV(wire)
	_, version, message, _ := trapTLV(message)
	_, header, message, _ := trapTLV(message)
	_, security, scoped, _ := trapTLV(message)
	_, fields, _, _ := trapTLV(security)
	var usm []byte
	var digestAt, digestLength int
	for i := range 6 {
		before := fields
		_, value, rest, err := trapTLV(fields)
		require.NoError(t, err)
		fields = rest
		if i == 4 {
			digestLength = len(value)
			usm = append(usm, byte(gosnmp.OctetString), 0x80|byte(lengthOctets))
			usm = append(usm, make([]byte, lengthOctets-1)...)
			usm = append(usm, byte(digestLength))
			digestAt = len(usm)
			usm = append(usm, make([]byte, digestLength)...)
		} else {
			usm = append(usm, before[:len(before)-len(rest)]...)
		}
	}
	wire = securityTestEnvelope(version, header, usm, scoped)
	params := packet.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	digest := hmac.New(params.AuthenticationProtocol.HashType().New, params.SecretKey)
	_, err := digest.Write(wire)
	require.NoError(t, err)
	digestAt += len(wire) - len(scoped) - len(usm)
	copy(wire[digestAt:digestAt+digestLength], digest.Sum(nil)[:digestLength])
	require.NoError(t, checkTrapEnvelope(wire))
	return wire
}

func TestTrapUSMAuthenticationAndPrivacyProtocols(t *testing.T) {
	for _, auth := range []gosnmp.SnmpV3AuthProtocol{gosnmp.MD5, gosnmp.SHA, gosnmp.SHA224, gosnmp.SHA256, gosnmp.SHA384, gosnmp.SHA512} {
		for _, privacy := range []gosnmp.SnmpV3PrivProtocol{gosnmp.NoPriv, gosnmp.DES, gosnmp.AES, gosnmp.AES192, gosnmp.AES256, gosnmp.AES192C, gosnmp.AES256C} {
			t.Run(auth.String()+"/"+privacy.String(), func(t *testing.T) {
				flags := gosnmp.AuthPriv
				if privacy == gosnmp.NoPriv {
					flags = gosnmp.AuthNoPriv
				}
				cfg := securityTestConfig(auth, privacy, flags)
				parser, err := newTrapParser(cfg)
				require.NoError(t, err)
				source := securityTestPacket(t, auth, privacy, flags, testTrapEngine)
				wire := securityTestWire(t, source)
				original := bytes.Clone(wire)
				decoded, err := safeUnmarshalTrap(parser, wire)
				require.NoError(t, err)
				require.Equal(t, original, wire, "parsing must not erase the caller's authentication digest")
				require.Equal(t, source.Variables[2].Value, decoded.Variables[2].Value)
				require.NoError(t, admitTrap(cfg, decoded, make(map[string]trapEngineTime), time.Now()))
				require.Len(t, parser.engines, 1)
				sourceParams := source.SecurityParameters.(*gosnmp.UsmSecurityParameters)
				require.Equal(t, sourceParams.SecretKey, parser.engines[testTrapEngine].auth)
				if privacy != gosnmp.NoPriv {
					require.Equal(t, sourceParams.PrivacyKey, parser.engines[testTrapEngine].privacy)
				}
				// Cached keys authenticate and decrypt subsequent packets too.
				_, err = safeUnmarshalTrap(parser, wire)
				require.NoError(t, err)
				require.Len(t, parser.engines, 1)
				// Every digest size and privacy protocol must also work when the
				// first authenticated packet uses a long-form digest header.
				parser, err = newTrapParser(cfg)
				require.NoError(t, err)
				longWire := securityTestLongAuthenticationHeader(t, source, 1)
				original = bytes.Clone(longWire)
				decoded, err = safeUnmarshalTrap(parser, longWire)
				require.NoError(t, err)
				require.Equal(t, original, longWire)
				require.Equal(t, source.Variables[2].Value, decoded.Variables[2].Value)
				require.NoError(t, admitTrap(cfg, decoded, make(map[string]trapEngineTime), time.Now()))
				require.Len(t, parser.engines, 1)
				envelope, err := readTrapUSM(longWire)
				require.NoError(t, err)
				require.Equal(t, string(envelope.digest), decoded.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthenticationParameters)
			})
		}
	}
}

func TestTrapUSMLongAuthenticationHeaderEncodings(t *testing.T) {
	for _, flags := range []gosnmp.SnmpV3MsgFlags{gosnmp.AuthNoPriv, gosnmp.AuthPriv | gosnmp.Reportable} {
		for _, lengthOctets := range []int{1, 2, 8, 126} {
			t.Run(fmt.Sprintf("flags %d/length octets %d", flags, lengthOctets), func(t *testing.T) {
				privacy := gosnmp.NoPriv
				if flags&gosnmp.AuthPriv == gosnmp.AuthPriv {
					privacy = gosnmp.AES
				}
				cfg := securityTestConfig(gosnmp.SHA256, privacy, flags)
				parser, err := newTrapParser(cfg)
				require.NoError(t, err)
				source := securityTestPacket(t, gosnmp.SHA256, privacy, flags, testTrapEngine)
				wire := securityTestLongAuthenticationHeader(t, source, lengthOctets)
				original := bytes.Clone(wire)
				decoded, err := safeUnmarshalTrap(parser, wire)
				require.NoError(t, err)
				require.Equal(t, original, wire)
				baseline, err := safeUnmarshalTrap(parser, securityTestWire(t, source))
				require.NoError(t, err)
				require.Equal(t, baseline.Variables, decoded.Variables)
				require.Equal(t, flags, decoded.MsgFlags)
				params := decoded.SecurityParameters.(*gosnmp.UsmSecurityParameters)
				require.Equal(t, testTrapEngine, params.AuthoritativeEngineID)
				require.Equal(t, "trap-user", params.UserName)
				require.Equal(t, uint32(7), params.AuthoritativeEngineBoots)
				require.Equal(t, uint32(100), params.AuthoritativeEngineTime)
				require.NoError(t, admitTrap(cfg, decoded, make(map[string]trapEngineTime), time.Now()))
			})
		}
	}
}

func TestTrapUSMLongAuthenticationHeaderRejectsForgery(t *testing.T) {
	for _, flags := range []gosnmp.SnmpV3MsgFlags{gosnmp.AuthNoPriv, gosnmp.AuthPriv} {
		privacy := gosnmp.NoPriv
		if flags == gosnmp.AuthPriv {
			privacy = gosnmp.AES
		}
		source := securityTestPacket(t, gosnmp.SHA256, privacy, flags, testTrapEngine)
		validWire := securityTestLongAuthenticationHeader(t, source, 126)
		envelope, err := readTrapUSM(validWire)
		require.NoError(t, err)
		for _, tc := range []struct {
			name string
			at   int
		}{
			{name: "digest", at: envelope.digestAt},
			{name: "engine identity", at: bytes.Index(validWire, []byte(testTrapEngine))},
			{name: "scoped PDU", at: len(validWire) - 1},
		} {
			t.Run(fmt.Sprintf("flags %d/%s", flags, tc.name), func(t *testing.T) {
				parser, err := newTrapParser(securityTestConfig(gosnmp.SHA256, privacy, flags))
				require.NoError(t, err)
				wire := bytes.Clone(validWire)
				wire[tc.at] ^= 1
				original := bytes.Clone(wire)
				_, err = safeUnmarshalTrap(parser, wire)
				require.ErrorContains(t, err, "not authentic")
				require.Equal(t, original, wire)
				require.Empty(t, parser.engines, "forged messages must not be normalized or cached")
			})
		}
	}
}

func TestTrapReceiverLongAuthenticationHeaderPrivacyUDPDelivery(t *testing.T) {
	cfg := securityTestConfig(gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv)
	cfg.ListenAddress = "127.0.0.1:0"
	sink := &consumertest.LogsSink{}
	r := newLifecycleTrapReceiver(t, cfg, receivertest.NewNopSettings(metadata.Type), sink)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	source := securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv, testTrapEngine)
	wire := securityTestLongAuthenticationHeader(t, source, 126)
	forged := bytes.Clone(wire)
	forged[len(forged)-1] ^= 1
	sendLifecycleTrap(t, r, forged)
	sendLifecycleTrap(t, r, wire)
	require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, 3*time.Second, time.Millisecond)
	shutdownLifecycleTrap(t, r)
	require.Equal(t, 1, sink.LogRecordCount(), "only the authentic encrypted notification is delivered")
	bindings, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().Get("varbinds")
	require.True(t, ok)
	require.Equal(t, 3, bindings.Slice().Len())
}

func TestTrapUSMNoAuthenticationAndReportableFlags(t *testing.T) {
	for _, flags := range []gosnmp.SnmpV3MsgFlags{
		gosnmp.NoAuthNoPriv, gosnmp.NoAuthNoPriv | gosnmp.Reportable,
		gosnmp.AuthNoPriv | gosnmp.Reportable, gosnmp.AuthPriv | gosnmp.Reportable,
	} {
		t.Run(fmt.Sprint(uint8(flags)), func(t *testing.T) {
			cfg := securityTestConfig(gosnmp.SHA256, gosnmp.AES, flags)
			parser, err := newTrapParser(cfg)
			require.NoError(t, err)
			wire := securityTestWire(t, securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, flags, testTrapEngine))
			packet, err := safeUnmarshalTrap(parser, wire)
			require.NoError(t, err)
			require.NoError(t, admitTrap(cfg, packet, make(map[string]trapEngineTime), time.Now()))
			if flags&gosnmp.AuthNoPriv == 0 {
				require.Empty(t, parser.engines, "unauthenticated engines must never be cached")
			} else {
				require.Len(t, parser.engines, 1)
			}
		})
	}
}

func TestTrapUSMRejectedEngineFloodDoesNotCache(t *testing.T) {
	cfg := securityTestConfig(gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv)
	for _, flags := range []gosnmp.SnmpV3MsgFlags{gosnmp.NoAuthNoPriv, gosnmp.AuthPriv} {
		t.Run(fmt.Sprint(uint8(flags)), func(t *testing.T) {
			parser, err := newTrapParser(cfg)
			require.NoError(t, err)
			template := securityTestWire(t, securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, flags, testTrapEngine))
			for i := range 128 {
				wire := bytes.ReplaceAll(template, []byte(testTrapEngine), fmt.Appendf(nil, "engine-%06d", i+2))
				original := bytes.Clone(wire)
				_, err = safeUnmarshalTrap(parser, wire)
				require.Error(t, err)
				require.Equal(t, original, wire, "rejected packets must not reach the mutating dependency parser")
				require.Empty(t, parser.engines)
			}
		})
	}
}

func TestTrapUSMRejectedMalformedAndForgedPackets(t *testing.T) {
	cfg := securityTestConfig(gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv)
	for _, tc := range []struct {
		name   string
		mutate func(*gosnmp.SnmpPacket)
	}{
		{"wrong user", func(p *gosnmp.SnmpPacket) {
			p.SecurityParameters.(*gosnmp.UsmSecurityParameters).UserName = "another-user"
		}},
		{"short engine", func(p *gosnmp.SnmpPacket) {
			p.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthoritativeEngineID = "tiny"
		}},
		{"long engine", func(p *gosnmp.SnmpPacket) {
			p.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthoritativeEngineID = string(bytes.Repeat([]byte{'x'}, 33))
		}},
		{"exhausted boots", func(p *gosnmp.SnmpPacket) {
			p.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthoritativeEngineBoots = math.MaxInt32
		}},
		{"boots out of range", func(p *gosnmp.SnmpPacket) {
			p.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthoritativeEngineBoots = math.MaxUint32
		}},
		{"time out of range", func(p *gosnmp.SnmpPacket) {
			p.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthoritativeEngineTime = math.MaxUint32
		}},
		{"short privacy parameters", func(p *gosnmp.SnmpPacket) {
			p.SecurityParameters.(*gosnmp.UsmSecurityParameters).PrivacyParameters = []byte{1}
		}},
		{"wrong security model", func(p *gosnmp.SnmpPacket) { p.SecurityModel = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parser, err := newTrapParser(cfg)
			require.NoError(t, err)
			packet := securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv, testTrapEngine)
			tc.mutate(packet)
			wire := securityTestWire(t, packet)
			_, err = safeUnmarshalTrap(parser, wire)
			require.Error(t, err)
			require.Empty(t, parser.engines)
		})
	}
	t.Run("tampered authenticated packet", func(t *testing.T) {
		parser, err := newTrapParser(cfg)
		require.NoError(t, err)
		wire := securityTestWire(t, securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv, testTrapEngine))
		wire[len(wire)-1] ^= 1
		_, err = safeUnmarshalTrap(parser, wire)
		require.ErrorContains(t, err, "not authentic")
		require.Empty(t, parser.engines)
	})
	t.Run("invalid minimum security", func(t *testing.T) {
		parser, err := newTrapParser(cfg)
		require.NoError(t, err)
		wire := securityTestWire(t, securityTestPacket(t, gosnmp.SHA256, gosnmp.NoPriv, gosnmp.AuthNoPriv, testTrapEngine))
		_, err = safeUnmarshalTrap(parser, wire)
		require.ErrorContains(t, err, "security level")
		require.Empty(t, parser.engines)
	})
}

func TestTrapUSMEngineKeyLimit(t *testing.T) {
	cfg := securityTestConfig(gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv)
	parser, err := newTrapParser(cfg)
	require.NoError(t, err)
	wire := securityTestWire(t, securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv, testTrapEngine))
	_, err = safeUnmarshalTrap(parser, wire)
	require.NoError(t, err)
	for i := range maxTrapEngines - 1 {
		parser.engines[fmt.Sprintf("filled-engine-%04d", i)] = trapUSMKeys{}
	}
	require.Len(t, parser.engines, maxTrapEngines)
	_, err = safeUnmarshalTrap(parser, wire)
	require.NoError(t, err, "already authenticated engines remain usable at capacity")
	newWire := securityTestWire(t, securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv, "new-engine-01"))
	_, err = safeUnmarshalTrap(parser, newWire)
	require.ErrorContains(t, err, "engine limit")
	require.Len(t, parser.engines, maxTrapEngines)
	require.NotContains(t, parser.engines, "new-engine-01")
}

func TestTrapUSMMalformedEnvelopesAndFields(t *testing.T) {
	cfg := securityTestConfig(gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv)
	wire := securityTestWire(t, securityTestPacket(t, gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv, testTrapEngine))
	parser, err := newTrapParser(cfg)
	require.NoError(t, err)
	for length := range len(wire) {
		_, err = safeUnmarshalTrap(parser, wire[:length])
		require.Error(t, err, "truncated length %d", length)
		require.Empty(t, parser.engines)
	}
	_, message, _, _ := trapTLV(wire)
	_, version, message, _ := trapTLV(message)
	_, header, message, _ := trapTLV(message)
	_, security, scoped, _ := trapTLV(message)
	_, fields, _, _ := trapTLV(security)
	var encoded [6][]byte
	for i := range encoded {
		before := fields
		_, _, fields, _ = trapTLV(fields)
		encoded[i] = before[:len(before)-len(fields)]
	}
	for _, index := range []int{0, 1, 2, 3, 4, 5} {
		t.Run(fmt.Sprintf("wrong USM tag %d", index), func(t *testing.T) {
			var modified []byte
			for i, field := range encoded {
				field = bytes.Clone(field)
				if i == index {
					field[0] = byte(gosnmp.Null)
				}
				modified = append(modified, field...)
			}
			candidate := securityTestEnvelope(version, header, modified, scoped)
			require.NoError(t, checkTrapEnvelope(candidate))
			_, err := safeUnmarshalTrap(parser, candidate)
			require.Error(t, err)
			require.Empty(t, parser.engines)
		})
	}
	t.Run("unsigned long authentication TLV header", func(t *testing.T) {
		var modified []byte
		for i, field := range encoded {
			if i == 4 {
				modified = append(modified, field[0], 0x81)
				modified = append(modified, field[1:]...)
			} else {
				modified = append(modified, field...)
			}
		}
		_, err := safeUnmarshalTrap(parser, securityTestEnvelope(version, header, modified, scoped))
		require.ErrorContains(t, err, "not authentic")
		require.Empty(t, parser.engines)
	})
	t.Run("privacy without authentication", func(t *testing.T) {
		modified := bytes.Replace(header, []byte{byte(gosnmp.OctetString), 1, byte(gosnmp.AuthPriv)}, []byte{byte(gosnmp.OctetString), 1, 2}, 1)
		_, err := safeUnmarshalTrap(parser, securityTestEnvelope(version, modified, bytes.Join(encoded[:], nil), scoped))
		require.ErrorContains(t, err, "security level")
		require.Empty(t, parser.engines)
	})
	t.Run("plaintext with privacy flag", func(t *testing.T) {
		candidate := bytes.Clone(wire)
		candidate[len(wire)-len(scoped)] = byte(gosnmp.Sequence)
		_, err := safeUnmarshalTrap(parser, candidate)
		require.ErrorContains(t, err, "privacy flag")
		require.Empty(t, parser.engines)
	})
}

func securityTestTLV(tag byte, value []byte) []byte {
	field := []byte{tag}
	switch {
	case len(value) < 128:
		field = append(field, byte(len(value)))
	case len(value) < 256:
		field = append(field, 0x81, byte(len(value)))
	default:
		field = append(field, 0x82, byte(len(value)>>8), byte(len(value)))
	}
	return append(field, value...)
}

func TestTrapBERDefiniteLengthHeaders(t *testing.T) {
	for _, count := range []int{1, 2, 8, 126} {
		t.Run(fmt.Sprintf("length octets %d", count), func(t *testing.T) {
			wire := []byte{byte(gosnmp.OctetString), 0x80 | byte(count)}
			wire = append(wire, make([]byte, count-1)...)
			wire = append(wire, 1, 0x42)
			tag, value, rest, err := trapTLV(wire)
			require.NoError(t, err)
			require.Equal(t, byte(gosnmp.OctetString), tag)
			require.Equal(t, []byte{0x42}, value)
			require.Empty(t, rest)
		})
	}
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{name: "indefinite", wire: []byte{0x04, 0x80, 0x00}},
		{name: "reserved length count", wire: append([]byte{0x04, 0xff}, make([]byte, 127)...)},
		{name: "truncated length", wire: []byte{0x04, 0x82, 0x00}},
		{name: "length overflow", wire: append(append([]byte{0x04, 0x88}, bytes.Repeat([]byte{0xff}, 8)...), 0x42)},
		{name: "long length overflow", wire: append(append([]byte{0x04, 0xfe, 0x01}, make([]byte, 125)...), 0x42)},
		{name: "missing value", wire: append(append([]byte{0x04, 0xfe}, make([]byte, 125)...), 0x02, 0x42)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := trapTLV(tc.wire)
			require.Error(t, err)
		})
	}
}

func securityTestEnvelope(version, header, usm, scoped []byte) []byte {
	message := securityTestTLV(byte(gosnmp.Integer), version)
	message = append(message, securityTestTLV(byte(gosnmp.Sequence), header)...)
	message = append(message, securityTestTLV(byte(gosnmp.OctetString), securityTestTLV(byte(gosnmp.Sequence), usm))...)
	message = append(message, scoped...)
	return securityTestTLV(byte(gosnmp.Sequence), message)
}

func TestTrapDependencyErrorDoesNotExposeCommunity(t *testing.T) {
	parser, err := newTrapParser(defaultTrapsConfig())
	require.NoError(t, err)
	message := []byte{byte(gosnmp.Integer), 1, byte(gosnmp.Version2c)}
	message = append(message, securityTestTLV(byte(gosnmp.OctetString), []byte("confidential-community"))...)
	message = append(message, byte(gosnmp.SNMPv2Trap)) // Missing the PDU length and content.
	_, err = safeUnmarshalTrap(parser, securityTestTLV(byte(gosnmp.Sequence), message))
	require.EqualError(t, err, "malformed SNMP notification")
	require.NotContains(t, err.Error(), "confidential-community")
}

func TestTrapParserMixedVersionsWithV3Credentials(t *testing.T) {
	cfg := securityTestConfig(gosnmp.SHA256, gosnmp.AES, gosnmp.AuthPriv)
	cfg.Communities = []configopaque.String{"public"}
	parser, err := newTrapParser(cfg)
	require.NoError(t, err)
	for _, version := range []gosnmp.SnmpVersion{gosnmp.Version1, gosnmp.Version2c} {
		t.Run(version.String(), func(t *testing.T) {
			packet := &gosnmp.SnmpPacket{Version: version, Community: "public", PDUType: gosnmp.SNMPv2Trap, RequestID: 1}
			if version == gosnmp.Version1 {
				packet.PDUType = gosnmp.Trap
				packet.SnmpTrap = gosnmp.SnmpTrap{Enterprise: "1.3.6.1.4.1.9", AgentAddress: "192.0.2.1", GenericTrap: 1, Timestamp: 100}
			}
			decoded, err := safeUnmarshalTrap(parser, securityTestWire(t, packet))
			require.NoError(t, err)
			require.NoError(t, admitTrap(cfg, decoded, make(map[string]trapEngineTime), time.Now()))
			require.Empty(t, parser.engines)
			decoded.Community = "wrong"
			require.ErrorContains(t, admitTrap(cfg, decoded, make(map[string]trapEngineTime), time.Now()), "community")
		})
	}
}

func TestTrapTimeWindowAndEngineBoots(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, tc := range []struct {
		name       string
		boots      uint32
		engineTime uint32
		delay      time.Duration
		wantError  bool
	}{
		{"initial duplicate", 7, 100, 0, false},
		{"window boundary", 7, 100, 150 * time.Second, false},
		{"expired replay", 7, 100, 150*time.Second + time.Nanosecond, true},
		{"older boots", 6, 100, time.Second, true},
		{"new boots", 8, 0, 200 * time.Second, false},
		{"new authenticated time resynchronizes", 7, 101, 200 * time.Second, false},
		{"exhausted boots", math.MaxInt32, 100, 0, true},
		{"time exceeds range", 7, math.MaxUint32, 0, true},
		{"maximum valid time", 7, math.MaxInt32, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := map[string]trapEngineTime{testTrapEngine: {boots: 7, latest: 100, received: now}}
			before := state[testTrapEngine]
			params := &gosnmp.UsmSecurityParameters{AuthoritativeEngineID: testTrapEngine, AuthoritativeEngineBoots: tc.boots, AuthoritativeEngineTime: tc.engineTime}
			err := checkTrapTime(state, params, now.Add(tc.delay))
			if tc.wantError {
				require.Error(t, err)
				require.Equal(t, before, state[testTrapEngine], "rejected messages must not reset synchronization")
			} else {
				require.NoError(t, err)
			}
		})
	}
	t.Run("new engine cap", func(t *testing.T) {
		state := make(map[string]trapEngineTime, maxTrapEngines)
		for i := range maxTrapEngines {
			state[fmt.Sprint(i)] = trapEngineTime{}
		}
		params := &gosnmp.UsmSecurityParameters{AuthoritativeEngineID: testTrapEngine, AuthoritativeEngineBoots: 1, AuthoritativeEngineTime: 1}
		require.ErrorContains(t, checkTrapTime(state, params, now), "engine limit")
		require.Len(t, state, maxTrapEngines)
	})
}

func TestTrapPasswordLocalizationRFC3414Vectors(t *testing.T) {
	engine, err := hex.DecodeString("000000000000000000000002")
	require.NoError(t, err)
	for _, tc := range []struct {
		protocol gosnmp.SnmpV3AuthProtocol
		key      string
	}{
		{gosnmp.MD5, "526f5eed9fcce26f8964c2930787d82b"},
		{gosnmp.SHA, "6695febc9288e36282235fc7151f128497b38f3f"},
	} {
		t.Run(tc.protocol.String(), func(t *testing.T) {
			master := trapPasswordKey(tc.protocol.HashType(), []byte("maplesyrup"))
			localized := trapLocalizeKey(tc.protocol.HashType(), master, string(engine))
			require.Equal(t, tc.key, hex.EncodeToString(localized))
		})
	}
}
