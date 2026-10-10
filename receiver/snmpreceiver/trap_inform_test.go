// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTrapInformForParsingIgnoredErrorStatus(t *testing.T) {
	requestID := int64(-12345)
	_, bindings := informTestRequest(t, requestID, 200)
	for _, tc := range []struct {
		name   string
		status []byte
	}{
		{name: "zero", status: informTestInteger(t, 0)},
		{name: "unknown status", status: informTestInteger(t, 19)},
		{name: "negative status", status: informTestInteger(t, -1)},
		{name: "above Integer32", status: informTestInteger(t, math.MaxInt32+1)},
		{name: "below Integer32", status: informTestInteger(t, math.MinInt32-1)},
		{name: "above Integer64", status: informTestTLV(0x02, []byte{0, 0x80, 0, 0, 0, 0, 0, 0, 0})},
		{name: "below Integer64", status: informTestTLV(0x02, []byte{0xff, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := informTestMessage([]byte{2, 1, 1}, 0xa6, informTestInteger(t, requestID), tc.status, informTestInteger(t, math.MaxInt32), bindings)
			original := bytes.Clone(wire)
			normalized, err := normalizeTrapInformForParsing(wire)
			require.NoError(t, err)
			require.Equal(t, original, wire, "normalization must preserve the datagram used for the acknowledgment")
			id, status, index, normalizedBindings := decodeInformTestMessage(t, normalized, 6)
			require.Equal(t, requestID, id)
			require.Zero(t, status)
			require.Equal(t, int64(math.MaxInt32), index)
			require.Equal(t, bindings, normalizedBindings)
			parser := &gosnmp.GoSNMP{Version: gosnmp.Version2c}
			packet, err := parser.UnmarshalTrap(normalized, false)
			require.NoError(t, err)
			require.Equal(t, gosnmp.InformRequest, packet.PDUType)
			require.Equal(t, uint32(requestID), packet.RequestID)
			require.Equal(t, gosnmp.NoError, packet.Error)
			// The normalized bytes must not share a receive-buffer backing array.
			normalized[len(normalized)-1] ^= 0xff
			require.Equal(t, original, wire)
		})
	}
}

func TestNormalizeTrapInformForParsingOtherPDUs(t *testing.T) {
	_, bindings := informTestRequest(t, 1, 0)
	zero := informTestInteger(t, 0)
	message := append([]byte{2, 1, 1}, informTestTLV(0x04, []byte("public"))...)
	missingPDULength := informTestTLV(0x30, append(message, 0xa7))
	for name, wire := range map[string][]byte{
		"v1":                 informTestMessage([]byte{2, 1, 0}, 0xa6, zero, zero, zero, bindings),
		"v3":                 informTestMessage([]byte{2, 1, 3}, 0xa6, zero, zero, zero, bindings),
		"v2c trap":           informTestMessage([]byte{2, 1, 1}, 0xa7, zero, zero, zero, bindings),
		"malformed v2c trap": missingPDULength,
	} {
		t.Run(name, func(t *testing.T) {
			normalized, err := normalizeTrapInformForParsing(wire)
			require.NoError(t, err)
			require.Same(t, &wire[0], &normalized[0], "other PDU types must retain their existing decoding path")
			require.Equal(t, wire, normalized)
		})
	}
}

func TestNormalizeTrapInformForParsingMalformed(t *testing.T) {
	for name, wire := range malformedInformTestMessages(t) {
		if name == "v1" || name == "v3" || name == "wrong PDU" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			original := bytes.Clone(wire)
			normalized, err := normalizeTrapInformForParsing(wire)
			require.Error(t, err)
			require.Nil(t, normalized)
			require.Equal(t, original, wire)
		})
	}
}

func TestNormalizeTrapInformForParsingBERLongLengths(t *testing.T) {
	_, bindings := informTestRequest(t, -1, 0)
	_, values, _, err := trapTLV(bindings)
	require.NoError(t, err)
	var longBindings []byte
	for len(values) > 0 {
		_, binding, rest, parseErr := trapTLV(values)
		require.NoError(t, parseErr)
		longBindings = append(longBindings, informTestBERLongTLV(0x30, binding)...)
		values = rest
	}
	pdu := informTestBERLongTLV(0x02, []byte{0xff})
	pdu = append(pdu, informTestBERLongTLV(0x02, []byte{0, 0x80, 0, 0, 0, 0, 0, 0, 0})...)
	pdu = append(pdu, informTestBERLongTLV(0x02, []byte{0})...)
	pdu = append(pdu, informTestBERLongTLV(0x30, longBindings)...)
	message := informTestBERLongTLV(0x02, []byte{1})
	message = append(message, informTestBERLongTLV(0x04, []byte("public"))...)
	message = append(message, informTestBERLongTLV(0xa6, pdu)...)
	wire := informTestBERLongTLV(0x30, message)
	original := bytes.Clone(wire)
	normalized, err := normalizeTrapInformForParsing(wire)
	require.NoError(t, err, "RFC 3417 permits definite long-form lengths with extra zero length octets")
	require.Equal(t, original, wire)
	parser := &gosnmp.GoSNMP{Version: gosnmp.Version2c}
	packet, err := parser.UnmarshalTrap(normalized, false)
	require.NoError(t, err)
	require.Equal(t, gosnmp.InformRequest, packet.PDUType)
	require.Equal(t, uint32(math.MaxUint32), packet.RequestID)
	require.Equal(t, gosnmp.NoError, packet.Error)
	require.Len(t, packet.Variables, 2)
	response, err := buildTrapInformResponse(wire, 65507)
	require.NoError(t, err)
	packet, err = parser.UnmarshalTrap(response, false)
	require.NoError(t, err)
	require.Equal(t, gosnmp.GetResponse, packet.PDUType)
	require.Equal(t, uint32(math.MaxUint32), packet.RequestID)
	require.Zero(t, packet.Error)
	require.Zero(t, packet.ErrorIndex)
}

func informTestBERLongTLV(tag byte, value []byte) []byte {
	return append([]byte{tag, 0x89, 0, 0, 0, 0, 0, 0, 0, byte(len(value) >> 8), byte(len(value))}, value...)
}

func TestAcknowledgeTrapInformSignedRequestID(t *testing.T) {
	for _, id := range []int64{math.MinInt32, -1, 0, 1, math.MaxInt32} {
		t.Run(strconv.FormatInt(id, 10), func(t *testing.T) {
			wire, bindings := informTestRequest(t, id, 0)
			original := bytes.Clone(wire)
			conn := &informTestPacketConn{}
			sender := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 1620}
			require.NoError(t, acknowledgeTrapInform(conn, wire, sender))
			require.Equal(t, original, wire, "the receive buffer must not be modified")
			require.Equal(t, sender, conn.destination)
			require.False(t, conn.writeDeadline.IsZero())
			responseID, status, index, responseBindings := decodeInformTestResponse(t, conn.wire)
			require.Equal(t, id, responseID)
			require.Zero(t, status)
			require.Zero(t, index)
			require.Equal(t, bindings, responseBindings)
		})
	}
}

func TestTrapInformResponsePreservesFields(t *testing.T) {
	wire, bindings := informTestRequest(t, 12345, 200)
	response, err := buildTrapInformResponse(wire, 65507)
	require.NoError(t, err)
	// For a canonical request with zero error fields, only the PDU tag changes.
	expected := bytes.Clone(wire)
	var message asn1.RawValue
	_, err = asn1.Unmarshal(wire, &message)
	require.NoError(t, err)
	var field asn1.RawValue
	rest, err := asn1.Unmarshal(message.Bytes, &field)
	require.NoError(t, err)
	rest, err = asn1.Unmarshal(rest, &field)
	require.NoError(t, err)
	expected[len(wire)-len(rest)] = 0xa2
	require.Equal(t, expected, response)
	id, status, index, responseBindings := decodeInformTestResponse(t, response)
	require.Equal(t, int64(12345), id)
	require.Zero(t, status)
	require.Zero(t, index)
	require.Equal(t, bindings, responseBindings)
}

func TestTrapInformResponseClearsErrorFields(t *testing.T) {
	_, bindings := informTestRequest(t, -12345, 0)
	wire := informTestMessage([]byte{2, 1, 1}, 0xa6, informTestInteger(t, -12345), informTestInteger(t, 18), informTestInteger(t, math.MaxInt32), bindings)
	response, err := buildTrapInformResponse(wire, 65507)
	require.NoError(t, err)
	id, status, index, responseBindings := decodeInformTestResponse(t, response)
	require.Equal(t, int64(-12345), id)
	require.Zero(t, status)
	require.Zero(t, index)
	require.Equal(t, bindings, responseBindings)
}

func TestAcknowledgeTrapInformIgnoresErrorStatus(t *testing.T) {
	_, bindings := informTestRequest(t, -12345, 0)
	for _, tc := range []struct {
		name   string
		status []byte
	}{
		{name: "unknown status", status: informTestInteger(t, 19)},
		{name: "negative status", status: informTestInteger(t, -1)},
		{name: "above Integer32", status: informTestInteger(t, math.MaxInt32+1)},
		{name: "below Integer32", status: informTestInteger(t, math.MinInt32-1)},
		{name: "above Integer64", status: informTestTLV(0x02, []byte{0, 0x80, 0, 0, 0, 0, 0, 0, 0})},
		{name: "below Integer64", status: informTestTLV(0x02, []byte{0xff, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := informTestMessage([]byte{2, 1, 1}, 0xa6, informTestInteger(t, -12345), tc.status, informTestInteger(t, math.MaxInt32), bindings)
			original := bytes.Clone(wire)
			conn := &informTestPacketConn{}
			require.NoError(t, acknowledgeTrapInform(conn, wire, &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 1620}))
			require.Equal(t, original, wire)
			id, status, index, responseBindings := decodeInformTestResponse(t, conn.wire)
			require.Equal(t, int64(-12345), id)
			require.Zero(t, status)
			require.Zero(t, index)
			require.Equal(t, bindings, responseBindings)
		})
	}
}

func TestTrapInformResponseSizeLimit(t *testing.T) {
	wire, _ := informTestRequest(t, -1, 200)
	response, err := buildTrapInformResponse(wire, 100)
	require.NoError(t, err)
	require.LessOrEqual(t, len(response), 100)
	id, status, index, bindings := decodeInformTestResponse(t, response)
	require.Equal(t, int64(-1), id)
	require.Equal(t, int64(1), status)
	require.Zero(t, index)
	require.Equal(t, []byte{0x30, 0}, bindings)
	_, err = buildTrapInformResponse(wire, len(response)-1)
	require.Error(t, err)
	_, err = buildTrapInformResponse(wire, 0)
	require.Error(t, err)
	_, err = buildTrapInformResponse(make([]byte, 65536), 65507)
	require.Error(t, err)
}

func TestTrapInformResponseMalformed(t *testing.T) {
	for name, malformed := range malformedInformTestMessages(t) {
		t.Run(name, func(t *testing.T) {
			conn := &informTestPacketConn{}
			require.Error(t, acknowledgeTrapInform(conn, malformed, &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1)}))
			require.Nil(t, conn.wire, "malformed messages must not be acknowledged")
		})
	}
}

func malformedInformTestMessages(t *testing.T) map[string][]byte {
	t.Helper()
	wire, bindings := informTestRequest(t, 1, 0)
	valid := []byte{2, 1, 0}
	oid := []byte{6, 1, 43}
	return map[string][]byte{
		"empty":                       nil,
		"truncated":                   wire[:len(wire)-1],
		"trailing message":            append(bytes.Clone(wire), 0),
		"wrong envelope":              {0x04, 0},
		"indefinite length":           {0x30, 0x80, 0, 0},
		"oversized length":            {0x30, 0x88, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		"v1":                          informTestMessage([]byte{2, 1, 0}, 0xa6, valid, valid, valid, bindings),
		"v3":                          informTestMessage([]byte{2, 1, 3}, 0xa6, valid, valid, valid, bindings),
		"wrong version tag":           informTestMessage([]byte{4, 1, 1}, 0xa6, valid, valid, valid, bindings),
		"wrong PDU":                   informTestMessage([]byte{2, 1, 1}, 0xa0, valid, valid, valid, bindings),
		"empty ID":                    informTestMessage([]byte{2, 1, 1}, 0xa6, []byte{2, 0}, valid, valid, bindings),
		"ID below Integer32":          informTestMessage([]byte{2, 1, 1}, 0xa6, informTestInteger(t, math.MinInt32-1), valid, valid, bindings),
		"ID above Integer32":          informTestMessage([]byte{2, 1, 1}, 0xa6, informTestInteger(t, math.MaxInt32+1), valid, valid, bindings),
		"nonminimal positive ID":      informTestMessage([]byte{2, 1, 1}, 0xa6, []byte{2, 2, 0, 1}, valid, valid, bindings),
		"nonminimal negative ID":      informTestMessage([]byte{2, 1, 1}, 0xa6, []byte{2, 2, 0xff, 0xff}, valid, valid, bindings),
		"wrong ID tag":                informTestMessage([]byte{2, 1, 1}, 0xa6, []byte{4, 1, 1}, valid, valid, bindings),
		"empty error status":          informTestMessage([]byte{2, 1, 1}, 0xa6, valid, []byte{2, 0}, valid, bindings),
		"wrong error status tag":      informTestMessage([]byte{2, 1, 1}, 0xa6, valid, []byte{4, 1, 0}, valid, bindings),
		"nonminimal positive status":  informTestMessage([]byte{2, 1, 1}, 0xa6, valid, []byte{2, 2, 0, 1}, valid, bindings),
		"nonminimal negative status":  informTestMessage([]byte{2, 1, 1}, 0xa6, valid, []byte{2, 2, 0xff, 0xff}, valid, bindings),
		"negative error index":        informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, informTestInteger(t, -1), bindings),
		"error index above Integer32": informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, informTestInteger(t, math.MaxInt32+1), bindings),
		"empty error index":           informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, []byte{2, 0}, bindings),
		"wrong error index tag":       informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, []byte{4, 1, 0}, bindings),
		"nonminimal error index":      informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, []byte{2, 2, 0, 1}, bindings),
		"wrong bindings tag":          informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, []byte{4, 0}),
		"wrong binding tag":           informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestTLV(0x30, []byte{4, 0})),
		"wrong binding OID tag":       informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings([]byte{4, 3, '1', '.', '3'}, []byte{5, 0})),
		"empty binding OID":           informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings([]byte{6, 0}, []byte{5, 0})),
		"unterminated binding OID":    informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings([]byte{6, 1, 0x81}, []byte{5, 0})),
		"nonminimal binding OID":      informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings([]byte{6, 2, 0x80, 43}, []byte{5, 0})),
		"missing binding value":       informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, nil)),
		"trailing binding value":      informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, []byte{5, 0, 0})),
		"truncated binding value":     informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, []byte{4, 2, 'x'})),
		"nonempty null":               informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, []byte{5, 1, 0})),
		"empty binding integer":       informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, []byte{2, 0})),
		"nonminimal binding integer":  informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, []byte{2, 2, 0, 1})),
		"negative binding counter":    informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, []byte{0x41, 1, 0xff})),
		"empty OID value":             informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, informTestBindings(oid, []byte{6, 0})),
		"trailing PDU":                informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid, bindings, []byte{0}),
		"missing bindings":            informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid, valid),
		"missing error index":         informTestMessage([]byte{2, 1, 1}, 0xa6, valid, valid),
		"missing error status":        informTestMessage([]byte{2, 1, 1}, 0xa6, valid),
		"missing request ID":          informTestMessage([]byte{2, 1, 1}, 0xa6),
		"unterminated length":         {0x30, 0x82, 0x01},
	}
}

func informTestBindings(oid, value []byte) []byte {
	return informTestTLV(0x30, informTestTLV(0x30, append(bytes.Clone(oid), value...)))
}

func TestAcknowledgeTrapInformWriteErrors(t *testing.T) {
	wire, _ := informTestRequest(t, -1, 0)
	sender := &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 1620}
	failure := errors.New("socket failure")
	t.Run("write", func(t *testing.T) {
		conn := &informTestPacketConn{writeErr: failure}
		require.ErrorIs(t, acknowledgeTrapInform(conn, wire, sender), failure)
	})
	t.Run("deadline", func(t *testing.T) {
		conn := &informTestPacketConn{deadlineErr: failure}
		require.ErrorIs(t, acknowledgeTrapInform(conn, wire, sender), failure)
		require.Nil(t, conn.wire)
	})
	t.Run("short write", func(t *testing.T) {
		conn := &informTestPacketConn{shortWrite: true}
		require.ErrorIs(t, acknowledgeTrapInform(conn, wire, sender), io.ErrShortWrite)
	})
	t.Run("missing sender", func(t *testing.T) {
		require.Error(t, acknowledgeTrapInform(&informTestPacketConn{}, wire, nil))
	})
}

func informTestRequest(t *testing.T, id int64, payloadSize int) (wire, bindings []byte) {
	t.Helper()
	uptimeOID, err := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 3, 0})
	require.NoError(t, err)
	trapOID, err := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 6, 1, 6, 3, 1, 1, 4, 1, 0})
	require.NoError(t, err)
	coldStartOID, err := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 6, 1, 6, 3, 1, 1, 5, 1})
	require.NoError(t, err)
	values := informTestTLV(0x30, append(uptimeOID, 0x43, 1, 42))
	values = append(values, informTestTLV(0x30, append(trapOID, coldStartOID...))...)
	if payloadSize > 0 {
		value := informTestTLV(0x04, bytes.Repeat([]byte{0x7f}, payloadSize))
		values = append(values, informTestTLV(0x30, append(bytes.Clone(uptimeOID), value...))...)
	}
	bindings = informTestTLV(0x30, values)
	wire = informTestMessage([]byte{2, 1, 1}, 0xa6, informTestInteger(t, id), []byte{2, 1, 0}, []byte{2, 1, 0}, bindings)
	return wire, bindings
}

func informTestInteger(t *testing.T, value int64) []byte {
	t.Helper()
	wire, err := asn1.Marshal(value)
	require.NoError(t, err)
	return wire
}

func informTestMessage(version []byte, pduTag byte, fields ...[]byte) []byte {
	var pdu []byte
	for _, field := range fields {
		pdu = append(pdu, field...)
	}
	message := append(bytes.Clone(version), informTestTLV(0x04, []byte("public"))...)
	message = append(message, informTestTLV(pduTag, pdu)...)
	return informTestTLV(0x30, message)
}

func informTestTLV(tag byte, value []byte) []byte {
	wire := []byte{tag}
	switch {
	case len(value) < 128:
		wire = append(wire, byte(len(value)))
	case len(value) < 256:
		wire = append(wire, 0x81, byte(len(value)))
	default:
		wire = append(wire, 0x82, byte(len(value)>>8), byte(len(value)))
	}
	return append(wire, value...)
}

// Decode independently with encoding/asn1. Decoding the response with gosnmp
// would convert the signed request ID to uint32 again and hide the regression.
func decodeInformTestResponse(t *testing.T, wire []byte) (id, status, index int64, bindings []byte) {
	t.Helper()
	return decodeInformTestMessage(t, wire, 2)
}

func decodeInformTestMessage(t *testing.T, wire []byte, pduTag int) (id, status, index int64, bindings []byte) {
	t.Helper()
	var message, community, pdu, variables asn1.RawValue
	rest, err := asn1.Unmarshal(wire, &message)
	require.NoError(t, err)
	require.Empty(t, rest)
	require.Equal(t, asn1.ClassUniversal, message.Class)
	require.Equal(t, asn1.TagSequence, message.Tag)
	var version int
	rest, err = asn1.Unmarshal(message.Bytes, &version)
	require.NoError(t, err)
	require.Equal(t, 1, version)
	rest, err = asn1.Unmarshal(rest, &community)
	require.NoError(t, err)
	require.Equal(t, []byte("public"), community.Bytes)
	rest, err = asn1.Unmarshal(rest, &pdu)
	require.NoError(t, err)
	require.Empty(t, rest)
	require.Equal(t, asn1.ClassContextSpecific, pdu.Class)
	require.Equal(t, pduTag, pdu.Tag)
	require.True(t, pdu.IsCompound)
	rest, err = asn1.Unmarshal(pdu.Bytes, &id)
	require.NoError(t, err)
	rest, err = asn1.Unmarshal(rest, &status)
	require.NoError(t, err)
	rest, err = asn1.Unmarshal(rest, &index)
	require.NoError(t, err)
	rest, err = asn1.Unmarshal(rest, &variables)
	require.NoError(t, err)
	require.Empty(t, rest)
	require.Equal(t, asn1.TagSequence, variables.Tag)
	return id, status, index, variables.FullBytes
}

type informTestPacketConn struct {
	wire          []byte
	destination   net.Addr
	writeDeadline time.Time
	writeErr      error
	deadlineErr   error
	shortWrite    bool
}

func (*informTestPacketConn) ReadFrom([]byte) (int, net.Addr, error) { return 0, nil, io.EOF }
func (c *informTestPacketConn) WriteTo(wire []byte, destination net.Addr) (int, error) {
	c.wire, c.destination = bytes.Clone(wire), destination
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	if c.shortWrite {
		return len(wire) - 1, nil
	}
	return len(wire), nil
}
func (*informTestPacketConn) Close() error                    { return nil }
func (*informTestPacketConn) LocalAddr() net.Addr             { return &net.UDPAddr{} }
func (*informTestPacketConn) SetDeadline(time.Time) error     { return nil }
func (*informTestPacketConn) SetReadDeadline(time.Time) error { return nil }
func (c *informTestPacketConn) SetWriteDeadline(deadline time.Time) error {
	c.writeDeadline = deadline
	return c.deadlineErr
}
