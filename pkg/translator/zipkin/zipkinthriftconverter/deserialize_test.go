// Copyright The OpenTelemetry Authors
// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2018 Uber Technologies, Inc.
// SPDX-License-Identifier: Apache-2.0

package zipkin

import (
	"encoding/binary"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"github.com/stretchr/testify/require"
)

const oversizedCollectionSize = 1_000_000

func TestDeserializeWithBadListStart(t *testing.T) {
	ctx := t.Context()
	spanBytes, err := SerializeThrift(ctx, []*zipkincore.Span{{}})
	require.NoError(t, err)
	_, err = DeserializeThrift(ctx, append([]byte{0, 255, 255}, spanBytes...))
	require.Error(t, err)
}

func TestDeserializeWithCorruptedList(t *testing.T) {
	ctx := t.Context()
	spanBytes, err := SerializeThrift(ctx, []*zipkincore.Span{{}})
	require.NoError(t, err)
	spanBytes[2] = 255
	_, err = DeserializeThrift(ctx, spanBytes)
	require.Error(t, err)
}

func TestDeserialize(t *testing.T) {
	ctx := t.Context()
	spanBytes, err := SerializeThrift(ctx, []*zipkincore.Span{{}})
	require.NoError(t, err)
	_, err = DeserializeThrift(ctx, spanBytes)
	require.NoError(t, err)
}

func TestDeserializeRejectsOversizedCollectionHeaders(t *testing.T) {
	tests := map[string][]byte{
		"top level list":          thriftListBegin(thrift.STRUCT, oversizedCollectionSize),
		"span annotations list":   thriftSpanWithField(thrift.LIST, 6, thriftListBegin(thrift.STRUCT, oversizedCollectionSize)),
		"unknown map field skip":  thriftSpanWithField(thrift.MAP, 99, thriftMapBegin(thrift.BOOL, thrift.BOOL, oversizedCollectionSize)),
		"unknown set field skip":  thriftSpanWithField(thrift.SET, 99, thriftSetBegin(thrift.BOOL, oversizedCollectionSize)),
		"binary annotations list": thriftSpanWithField(thrift.LIST, 8, thriftListBegin(thrift.STRUCT, oversizedCollectionSize)),
	}

	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := DeserializeThrift(t.Context(), payload)
			require.Error(t, err)
			require.ErrorContains(t, err, "payload bytes remain")
		})
	}
}

func TestDeserializeRejectsCollectionHeadersOverLimit(t *testing.T) {
	_, err := DeserializeThrift(t.Context(), thriftListBegin(thrift.STRUCT, maxThriftCollectionElements+1))
	require.Error(t, err)
	require.ErrorContains(t, err, "limit is 1048576")
}

func thriftSpanWithField(fieldType thrift.TType, fieldID int16, fieldPayload []byte) []byte {
	payload := thriftListBegin(thrift.STRUCT, 1)
	payload = append(payload, byte(fieldType))
	payload = binary.BigEndian.AppendUint16(payload, uint16(fieldID))
	payload = append(payload, fieldPayload...)
	return payload
}

func thriftListBegin(elemType thrift.TType, size int32) []byte {
	payload := []byte{byte(elemType)}
	return binary.BigEndian.AppendUint32(payload, uint32(size))
}

func thriftMapBegin(keyType, valueType thrift.TType, size int32) []byte {
	payload := []byte{byte(keyType), byte(valueType)}
	return binary.BigEndian.AppendUint32(payload, uint32(size))
}

func thriftSetBegin(elemType thrift.TType, size int32) []byte {
	return thriftListBegin(elemType, size)
}
