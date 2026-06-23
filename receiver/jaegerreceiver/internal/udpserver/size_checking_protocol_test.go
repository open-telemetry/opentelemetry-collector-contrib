// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udpserver

import (
	"context"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSizeCheckingProtocol_ReadListBegin_Ok(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	protocol := thrift.NewTBinaryProtocolTransport(trans)
	wrapped := WrapProtocol(protocol)

	// Write a valid list header: type=STRUCT(12), size=5
	ctx := context.Background()
	err := protocol.WriteListBegin(ctx, thrift.STRUCT, 5)
	require.NoError(t, err)
	// Write 5 bytes of data so the remaining bytes checks pass (each struct is at least 1 byte stop field)
	_, err = trans.Write([]byte{0, 0, 0, 0, 0})
	require.NoError(t, err)

	elemType, size, err := wrapped.ReadListBegin(ctx)
	require.NoError(t, err)
	assert.Equal(t, thrift.TType(thrift.STRUCT), elemType)
	assert.Equal(t, 5, size)
}

func TestSizeCheckingProtocol_ReadListBegin_NegativeSize(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	protocol := thrift.NewTBinaryProtocolTransport(trans)
	wrapped := WrapProtocol(protocol)

	// Write negative size.
	// In TBinaryProtocol, list is serialized as 1 byte type + 4 bytes size.
	// Type STRUCT (12)
	trans.Write([]byte{12})
	// Size -5 (0xFFFFFFFB)
	trans.Write([]byte{0xFF, 0xFF, 0xFF, 0xFB})

	ctx := context.Background()
	_, _, err := wrapped.ReadListBegin(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestSizeCheckingProtocol_ReadListBegin_ExcessiveSize(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	protocol := thrift.NewTBinaryProtocolTransport(trans)
	wrapped := WrapProtocol(protocol)

	// Write a list with type=STRUCT(12), size=100
	// But there are 0 remaining bytes in the buffer after reading the header.
	trans.Write([]byte{12})
	trans.Write([]byte{0x00, 0x00, 0x00, 0x64}) // 100

	ctx := context.Background()
	_, _, err := wrapped.ReadListBegin(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds remaining bytes")
}

func TestSizeCheckingProtocol_ReadMapBegin_ExcessiveSize(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	protocol := thrift.NewTBinaryProtocolTransport(trans)
	wrapped := WrapProtocol(protocol)

	// In TBinaryProtocol, map is keyType (1 byte) + valType (1 byte) + size (4 bytes).
	trans.Write([]byte{12, 12})                 // key=STRUCT, val=STRUCT
	trans.Write([]byte{0x00, 0x00, 0x00, 0x64}) // 100

	ctx := context.Background()
	_, _, _, err := wrapped.ReadMapBegin(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds remaining bytes")
}

func TestSizeCheckingProtocol_ReadSetBegin_ExcessiveSize(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	protocol := thrift.NewTBinaryProtocolTransport(trans)
	wrapped := WrapProtocol(protocol)

	// Set is like List in binary protocol: type (1 byte) + size (4 bytes).
	trans.Write([]byte{12})
	trans.Write([]byte{0x00, 0x00, 0x00, 0x64}) // 100

	ctx := context.Background()
	_, _, err := wrapped.ReadSetBegin(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds remaining bytes")
}
