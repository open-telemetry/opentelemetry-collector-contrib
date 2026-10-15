// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"io"
	"net"
	"time"
)

// acknowledgeTrapInform preserves the original BER request ID and variable
// bindings. gosnmp represents request IDs as uint32, which cannot be marshaled
// back to the same signed Integer32 value for negative request IDs.
func acknowledgeTrapInform(conn net.PacketConn, wire []byte, sender *net.UDPAddr) error {
	if sender == nil {
		return errors.New("SNMP inform sender is missing")
	}
	// UDP payload limits exclude the IP and UDP headers.
	maxResponseSize := 65527
	if sender.IP.To4() != nil {
		maxResponseSize = 65507
	}
	response, err := buildTrapInformResponse(wire, maxResponseSize)
	if err != nil {
		return err
	}
	if deadlineErr := conn.SetWriteDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
		return deadlineErr
	}
	n, err := conn.WriteTo(response, sender)
	if err == nil && n != len(response) {
		return io.ErrShortWrite
	}
	return err
}

// normalizeTrapInformForParsing replaces the ignored error-status before gosnmp
// converts it to a fixed-width integer. The original datagram remains available
// for an acknowledgment that preserves the signed request ID and bindings.
func normalizeTrapInformForParsing(wire []byte) ([]byte, error) {
	fields, inform, err := readTrapInformFields(wire)
	if err != nil {
		return nil, err
	}
	if !inform {
		return wire, nil
	}
	pdu := append([]byte(nil), fields.requestID...)
	pdu = append(pdu, 0x02, 0x01, 0x00)
	pdu = append(pdu, fields.errorIndex...)
	pdu = append(pdu, fields.bindings...)
	message := append([]byte(nil), fields.version...)
	message = append(message, fields.community...)
	message = appendTrapTLV(message, 0xa6, pdu)
	return appendTrapTLV(nil, 0x30, message), nil
}

func buildTrapInformResponse(wire []byte, maxResponseSize int) ([]byte, error) {
	if maxResponseSize <= 0 {
		return nil, errors.New("SNMP inform exceeds the message size limit")
	}
	fields, inform, err := readTrapInformFields(wire)
	if err != nil {
		return nil, err
	}
	if !inform {
		return nil, errors.New("SNMP inform response requires a v2c InformRequest PDU")
	}

	response := marshalTrapInformResponse(fields.version, fields.community, fields.requestID, fields.bindings, 0)
	if len(response) <= maxResponseSize {
		return response, nil
	}
	// RFC 3416 section 4.2.7 specifies a tooBig response with empty bindings
	// when the ordinary response cannot fit the local message size constraint.
	response = marshalTrapInformResponse(fields.version, fields.community, fields.requestID, []byte{0x30, 0}, 1)
	if len(response) > maxResponseSize {
		return nil, errors.New("SNMP inform response exceeds the message size limit")
	}
	return response, nil
}

type trapInformFields struct {
	version, community, requestID, errorIndex, bindings []byte
}

// readTrapInformFields validates the fields needed for an acknowledgment before
// admission. Other versions and PDU types keep their existing decoding path.
func readTrapInformFields(wire []byte) (fields trapInformFields, inform bool, err error) {
	if len(wire) > 65535 {
		return fields, false, errors.New("SNMP inform exceeds the message size limit")
	}
	_, message, rest, err := readTrapInformField(wire, 0x30)
	if err != nil || len(rest) != 0 {
		return fields, false, errors.New("malformed SNMP inform message")
	}
	version, value, rest, err := readTrapInformField(message, 0x02)
	if err != nil {
		return fields, false, err
	}
	v, err := readTrapInformInteger(value)
	if err != nil {
		return fields, false, err
	}
	if v != 1 {
		return fields, false, nil
	}
	community, _, rest, err := readTrapInformField(rest, 0x04)
	if err != nil {
		return fields, false, err
	}
	if len(rest) > 0 && rest[0] != 0xa6 {
		return fields, false, nil
	}
	_, pdu, rest, err := trapTLV(rest)
	if err != nil || len(rest) != 0 {
		return fields, false, errors.New("malformed SNMP inform PDU")
	}
	requestID, value, rest, err := readTrapInformField(pdu, 0x02)
	if err != nil {
		return fields, false, err
	}
	if _, err = readTrapInformInteger(value); err != nil {
		return fields, false, err
	}
	_, value, rest, err = readTrapInformField(rest, 0x02)
	if err != nil {
		return fields, false, err
	}
	// RFC 3416 sections 4.2 and 4.2.7 ignore the incoming error-status.
	// Its named INTEGER values do not constrain its range. Check the BER
	// encoding without converting or restricting the ignored value.
	if checkTrapInformInteger(value) != nil {
		return fields, false, errors.New("malformed SNMP inform error status")
	}
	encodedErrorIndex, value, rest, err := readTrapInformField(rest, 0x02)
	if err != nil {
		return fields, false, err
	}
	errorIndex, err := readTrapInformInteger(value)
	if err != nil || errorIndex < 0 {
		return fields, false, errors.New("malformed SNMP inform error index")
	}
	bindings, value, rest, err := readTrapInformField(rest, 0x30)
	if err != nil || len(rest) != 0 {
		return fields, false, errors.New("malformed SNMP inform variable bindings")
	}
	if err := checkTrapInformBindings(value); err != nil {
		return fields, false, err
	}
	return trapInformFields{version: version, community: community, requestID: requestID, errorIndex: encodedErrorIndex, bindings: bindings}, true, nil
}

// Each binding contains exactly an OBJECT IDENTIFIER and one complete value.
// Check BER syntax that the dependency can otherwise accept or repair. Value
// types and decoded ranges remain subject to the existing notification decoder.
func checkTrapInformBindings(bindings []byte) error {
	for len(bindings) > 0 {
		_, binding, rest, err := readTrapInformField(bindings, 0x30)
		if err != nil {
			return errors.New("malformed SNMP inform variable binding")
		}
		_, oid, binding, err := readTrapInformField(binding, 0x06)
		if err != nil || checkTrapInformOID(oid) != nil {
			return errors.New("malformed SNMP inform variable binding OID")
		}
		tag, value, trailing, err := trapTLV(binding)
		if err != nil || len(trailing) != 0 {
			return errors.New("malformed SNMP inform variable binding value")
		}
		switch tag {
		case 0x02, 0x41, 0x42, 0x43, 0x46, 0x47:
			if checkTrapInformInteger(value) != nil || (tag != 0x02 && value[0]&0x80 != 0) {
				return errors.New("malformed SNMP inform variable binding integer")
			}
		case 0x05, 0x80, 0x81, 0x82:
			if len(value) != 0 {
				return errors.New("malformed SNMP inform variable binding null")
			}
		case 0x06:
			if err := checkTrapInformOID(value); err != nil {
				return err
			}
		}
		bindings = rest
	}
	return nil
}

func checkTrapInformOID(value []byte) error {
	if len(value) == 0 || value[len(value)-1]&0x80 != 0 {
		return errors.New("malformed SNMP inform object identifier")
	}
	for i, b := range value {
		if b == 0x80 && (i == 0 || value[i-1]&0x80 == 0) {
			return errors.New("malformed SNMP inform object identifier")
		}
	}
	return nil
}

func readTrapInformField(wire []byte, expectedTag byte) (encoded, value, rest []byte, err error) {
	tag, value, rest, err := trapTLV(wire)
	if err != nil || tag != expectedTag {
		return nil, nil, nil, errors.New("malformed SNMP inform field")
	}
	return wire[:len(wire)-len(rest)], value, rest, nil
}

// Decode request IDs as Integer32 and error-index within 0..MaxInt32.
// Error-status has no such constraint and is not decoded here.
func readTrapInformInteger(value []byte) (int64, error) {
	if len(value) > 4 || checkTrapInformInteger(value) != nil {
		return 0, errors.New("malformed SNMP inform integer")
	}
	var result int64
	for _, b := range value {
		result = result<<8 | int64(b)
	}
	if value[0]&0x80 != 0 {
		result -= int64(1) << (8 * len(value))
	}
	return result, nil
}

// BER INTEGER contents are nonempty and use the shortest two's-complement
// representation. No numeric conversion is needed for an ignored INTEGER.
func checkTrapInformInteger(value []byte) error {
	if len(value) == 0 || (len(value) > 1 && ((value[0] == 0 && value[1]&0x80 == 0) || (value[0] == 0xff && value[1]&0x80 != 0))) {
		return errors.New("malformed SNMP inform integer")
	}
	return nil
}

func marshalTrapInformResponse(version, community, requestID, bindings []byte, status byte) []byte {
	pdu := append([]byte(nil), requestID...)
	pdu = append(pdu, 0x02, 0x01, status, 0x02, 0x01, 0x00)
	pdu = append(pdu, bindings...)
	message := append([]byte(nil), version...)
	message = append(message, community...)
	message = appendTrapTLV(message, 0xa2, pdu)
	return appendTrapTLV(nil, 0x30, message)
}

// Callers bound the message to a UDP datagram, so at most three length octets
// are needed, including the length-of-length octet.
func appendTrapTLV(dst []byte, tag byte, value []byte) []byte {
	dst = append(dst, tag)
	switch {
	case len(value) < 128:
		dst = append(dst, byte(len(value)))
	case len(value) < 256:
		dst = append(dst, 0x81, byte(len(value)))
	default:
		dst = append(dst, 0x82, byte(len(value)>>8), byte(len(value)))
	}
	return append(dst, value...)
}
