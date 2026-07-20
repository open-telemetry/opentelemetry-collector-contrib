// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

// rotatingWriteCloser models timberjack: it never splits a single Write across
// files and rotates before a write that would exceed max.
type rotatingWriteCloser struct {
	max   int
	files []*bytes.Buffer
}

func (r *rotatingWriteCloser) cur() *bytes.Buffer {
	if len(r.files) == 0 {
		r.files = append(r.files, &bytes.Buffer{})
	}
	return r.files[len(r.files)-1]
}

func (r *rotatingWriteCloser) Write(p []byte) (int, error) {
	if len(p) > r.max {
		return 0, fmt.Errorf("write length %d exceeds maximum file size %d", len(p), r.max)
	}
	if r.cur().Len()+len(p) > r.max {
		r.files = append(r.files, &bytes.Buffer{})
	}
	return r.cur().Write(p)
}

func (*rotatingWriteCloser) Close() error { return nil }

// TestCompressingWriter_RotationFrameIntegrity: with max sized so a rotation
// falls mid-record, every rotated file must still decode independently.
func TestCompressingWriter_RotationFrameIntegrity(t *testing.T) {
	base := &rotatingWriteCloser{max: 70}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, &Rotation{MaxMegabytes: 1})
	require.NoError(t, err)

	var records []string
	for i := range 8 {
		records = append(records, fmt.Sprintf("record-%03d-payload\n", i))
		_, werr := cw.Write([]byte(records[i]))
		require.NoError(t, werr)
	}
	require.NoError(t, cw.Close())

	require.Greater(t, len(base.files), 1, "test must actually rotate to be meaningful")

	var reassembled bytes.Buffer
	for i, f := range base.files {
		dec, derr := zstd.NewReader(bytes.NewReader(f.Bytes()))
		require.NoError(t, derr)
		out, rerr := io.ReadAll(dec)
		dec.Close()
		require.NoErrorf(t, rerr, "file %d is not independently decodable: a zstd frame was split across rotation", i)
		reassembled.Write(out)
	}

	var want bytes.Buffer
	for _, r := range records {
		want.WriteString(r)
	}
	require.Equal(t, want.String(), reassembled.String())
}

// TestCompressingWriter_RotationOversizedRecord: a record whose frame would
// exceed max is split into in-bounds frames instead of being rejected, and
// still round-trips.
func TestCompressingWriter_RotationOversizedRecord(t *testing.T) {
	const maxBytes = 1 << 20 // 1 MiB, matches Rotation{MaxMegabytes: 1}
	base := &rotatingWriteCloser{max: maxBytes}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, &Rotation{MaxMegabytes: 1})
	require.NoError(t, err)

	// Incompressible payload several times larger than the limit.
	record := make([]byte, 3*maxBytes)
	_, err = rand.Read(record)
	require.NoError(t, err)

	_, err = cw.Write(record)
	require.NoError(t, err)
	require.NoError(t, cw.Close())

	require.Greater(t, len(base.files), 1, "oversized record must span multiple files")

	var reassembled bytes.Buffer
	for i, f := range base.files {
		require.LessOrEqualf(t, f.Len(), maxBytes, "file %d exceeds the rotation size limit", i)
		dec, derr := zstd.NewReader(bytes.NewReader(f.Bytes()))
		require.NoError(t, derr)
		out, rerr := io.ReadAll(dec)
		dec.Close()
		require.NoErrorf(t, rerr, "file %d is not independently decodable", i)
		reassembled.Write(out)
	}

	require.Equal(t, record, reassembled.Bytes())
}

func TestCompressingWriter_Zstd(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, nil)
	require.NoError(t, err)

	testData := []byte("hello world from zstd compression")
	n, err := cw.Write(testData)
	require.NoError(t, err)
	require.Equal(t, len(testData), n)

	err = cw.Close()
	require.NoError(t, err)

	require.Positive(t, buf.Len())

	// Decompress with standard zstd decoder
	decoder, err := zstd.NewReader(&buf)
	require.NoError(t, err)
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	require.NoError(t, err)
	require.Equal(t, testData, decompressed)
}

func TestCompressingWriter_MultipleWrites_Zstd(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	cw, err := newCompressingWriter(base, compressionZSTD, 0, nil)
	require.NoError(t, err)

	messages := []string{
		"first message\n",
		"second message\n",
		"third message\n",
	}
	for _, msg := range messages {
		_, writeErr := cw.Write([]byte(msg))
		require.NoError(t, writeErr)
	}

	err = cw.Close()
	require.NoError(t, err)

	// Decompress and verify all messages are present
	decoder, err := zstd.NewReader(&buf)
	require.NoError(t, err)
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	require.NoError(t, err)

	expected := "first message\nsecond message\nthird message\n"
	require.Equal(t, expected, string(decompressed))
}

func TestCompressingWriter_UnsupportedCompression(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	_, err := newCompressingWriter(base, "snappy", 0, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported compression")
}

func TestCompressingWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	cw, err := newCompressingWriter(base, compressionZSTD, 0, nil)
	require.NoError(t, err)

	testData := []byte("data to flush")
	_, err = cw.Write(testData)
	require.NoError(t, err)

	// flush() should not error
	err = cw.flush()
	require.NoError(t, err)

	err = cw.Close()
	require.NoError(t, err)
}

func TestZstdEncoderLevelFromZstd(t *testing.T) {
	tests := []struct {
		level    int
		expected zstd.EncoderLevel
	}{
		{1, zstd.SpeedFastest},
		{3, zstd.SpeedDefault},
		{6, zstd.SpeedBetterCompression},
		{11, zstd.SpeedBestCompression},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, zstd.EncoderLevelFromZstd(tt.level), "level %d", tt.level)
	}
}
