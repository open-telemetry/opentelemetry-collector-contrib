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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Value != "hello" {
		t.Errorf("expected value 'hello', got %q", msg.Value)
	}
}

func TestParseProtoReader_DecompressError(t *testing.T) {
	msg := &dummyProto{}
	data := []byte("not-snappy")
	// Not snappy data
	err := parseProtoReader(bytes.NewReader(data), 10, 100, msg)
	if err == nil {
		t.Error("expected error from decompress, got nil")
	}
}

func TestParseProtoReader_UnmarshalError(t *testing.T) {
	msg := &dummyProto{}
	data := []byte("fail")
	compressed := snappy.Encode(nil, data)
	err := parseProtoReader(bytes.NewReader(compressed), len(compressed), 100, msg)
	if err == nil {
		t.Error("expected error from Unmarshal, got nil")
	}
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
	if err == nil {
		t.Error("expected error for expectedSize > maxSize")
	}
}

func TestDecompressRequest_BufferPath(t *testing.T) {
	data := snappy.Encode(nil, []byte("abc"))
	br := &bufReader{bytes.NewBuffer(data)}
	body, err := decompressRequest(br, len(data), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "abc" {
		t.Errorf("expected 'abc', got %q", string(body))
	}
}

func TestDecompressFromReader_Error(t *testing.T) {
	r := errorReader{}
	_, err := decompressFromReader(&r, 0, 100)
	if err == nil {
		t.Error("expected error from reader, got nil")
	}
}

func TestDecompressFromBuffer_SizeError(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 101))
	_, err := decompressFromBuffer(buf, 100)
	if err == nil {
		t.Error("expected error for buffer > maxSize")
	}
}

func TestTryBufferFromReader(t *testing.T) {
	br := &bufReader{bytes.NewBufferString("abc")}
	buf, ok := tryBufferFromReader(br)
	if !ok || buf.String() != "abc" {
		t.Error("expected to extract buffer from reader")
	}
	// Negative case
	_, ok = tryBufferFromReader(bytes.NewBufferString("abc"))
	if ok {
		t.Error("expected false for non-matching reader")
	}
}
