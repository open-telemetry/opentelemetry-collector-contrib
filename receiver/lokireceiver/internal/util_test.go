// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"errors"
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/require"
)

type dummyProto struct {
	Value string
}

func (d *dummyProto) Reset()         { *d = dummyProto{} }
func (d *dummyProto) String() string { return d.Value }
func (*dummyProto) ProtoMessage()    {}
func (d *dummyProto) Unmarshal(b []byte) error {
	if string(b) == "fail" {
		return errors.New("unmarshal error")
	}
	d.Value = string(b)
	return nil
}

type bareProto struct {
	Value string
}

func (b *bareProto) Reset()         { *b = bareProto{} }
func (b *bareProto) String() string { return b.Value }
func (*bareProto) ProtoMessage()    {}

type bufReader struct{ *bytes.Buffer }

func (b *bufReader) BytesBuffer() *bytes.Buffer { return b.Buffer }

type errorReader struct{}

func (*errorReader) Read(_ []byte) (int, error) { return 0, errors.New("read error") }

func TestParseProtoReader_Success(t *testing.T) {
	msg := &dummyProto{}
	data := []byte("hello")
	compressed := snappy.Encode(nil, data)
	err := parseProtoReader(bytes.NewReader(compressed), len(compressed), 100, msg)

	require.NoError(t, err, "unexpected error while parsing proto reader")
	require.Equal(t, "hello", msg.Value, "unexpected value in parsed proto message")
}

func TestParseProtoReader_DecompressError(t *testing.T) {
	msg := &dummyProto{}
	data := []byte("not-snappy")
	// Not snappy data
	err := parseProtoReader(bytes.NewReader(data), 10, 100, msg)
	require.Error(t, err, "expected error from decompress, got nil")
}

func TestParseProtoReader_UnmarshalError(t *testing.T) {
	msg := &dummyProto{}
	data := []byte("fail")
	compressed := snappy.Encode(nil, data)
	err := parseProtoReader(bytes.NewReader(compressed), len(compressed), 100, msg)
	require.Error(t, err, "expected error from Unmarshal, got nil")
}

func TestParseProtoReader_UsesProtoNewBuffer(t *testing.T) {
	msg := &bareProto{}
	data := snappy.Encode(nil, []byte("not-a-real-proto"))

	require.Panics(t, func() {
		_ = parseProtoReader(bytes.NewReader(data), len(data), 100, msg)
	}, "expected panic from proto.NewBuffer.Unmarshal")
}

func TestDecompressRequest_SizeChecks(t *testing.T) {
	data := snappy.Encode(nil, []byte("abc"))
	_, err := decompressRequest(bytes.NewReader(data), 200, 100)
	require.Error(t, err, "expected error for expectedSize > maxSize")
}

func TestDecompressRequest_BufferPath(t *testing.T) {
	data := snappy.Encode(nil, []byte("abc"))
	br := &bufReader{bytes.NewBuffer(data)}
	body, err := decompressRequest(br, len(data), 100)

	require.NoError(t, err, "unexpected error during decompression: %v", err)
	require.Equal(t, "abc", string(body), "expected decompressed content to be 'abc', got %q", string(body))
}

func TestDecompressFromReader_Error(t *testing.T) {
	r := errorReader{}
	_, err := decompressFromReader(&r, 0, 100)
	require.Error(t, err, "expected error from reader, got nil")
}

func TestDecompressFromBuffer_SizeError(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 101))
	_, err := decompressFromBuffer(buf, 100)
	require.Error(t, err, "expected error for buffer > maxSize")
}

func TestTryBufferFromReader(t *testing.T) {
	br := &bufReader{bytes.NewBufferString("abc")}
	buf, ok := tryBufferFromReader(br)

	require.True(t, ok, "expected to extract buffer from reader of type *bufReader")
	require.Equal(t, "abc", buf.String(), "expected buffer to contain 'abc', got %q", buf.String())
	// Negative case
	_, ok = tryBufferFromReader(bytes.NewBufferString("abc"))
	require.False(t, ok, "expected false for non-*bufReader type")
}
