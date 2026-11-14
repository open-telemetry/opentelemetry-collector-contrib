// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"bytes"
	"crypto/rand"
	"errors"
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

// transientErrorWriteCloser fails writes when failNext is true, then succeeds.
// If partialNext > 0, it writes partialNext bytes before returning error.
type transientErrorWriteCloser struct {
	failNext    bool
	partialNext int
	buf         bytes.Buffer
	rotated     bool
	files       []*bytes.Buffer
}

func (w *transientErrorWriteCloser) Write(p []byte) (int, error) {
	if w.failNext {
		w.failNext = false
		if w.partialNext > 0 && len(p) > w.partialNext {
			w.buf.Write(p[:w.partialNext])
			return w.partialNext, errors.New("transient write error")
		}
		return 0, errors.New("transient write error")
	}
	return w.buf.Write(p)
}

func (w *transientErrorWriteCloser) Rotate() error {
	w.rotated = true
	old := w.buf
	w.files = append(w.files, &old)
	w.buf = bytes.Buffer{}
	return nil
}

func (*transientErrorWriteCloser) Close() error { return nil }

// nonRotatingTransientErrorWriteCloser does not implement Rotate().
type nonRotatingTransientErrorWriteCloser struct {
	failNext    bool
	partialNext int
	buf         bytes.Buffer
}

func (w *nonRotatingTransientErrorWriteCloser) Write(p []byte) (int, error) {
	if w.failNext {
		w.failNext = false
		if w.partialNext > 0 && len(p) > w.partialNext {
			w.buf.Write(p[:w.partialNext])
			return w.partialNext, errors.New("transient write error")
		}
		return 0, errors.New("transient write error")
	}
	return w.buf.Write(p)
}

func (*nonRotatingTransientErrorWriteCloser) Close() error { return nil }

func TestCompressingWriter_RotationRecoversAfterTransientError(t *testing.T) {
	base := &transientErrorWriteCloser{failNext: true}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, &Rotation{MaxMegabytes: 1})
	require.NoError(t, err)

	// First write fails due to the transient error.
	_, err = cw.Write([]byte("failed message\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient write error")

	// Next write should succeed and not be permanently rejected by a cached sticky error.
	msg := "recovered message\n"
	n, err := cw.Write([]byte(msg))
	require.NoError(t, err)
	require.Equal(t, len(msg), n)

	require.NoError(t, cw.Close())

	// Verify the recovered message was written as a valid, decodable zstd frame.
	decoder, err := zstd.NewReader(&base.buf)
	require.NoError(t, err)
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	require.NoError(t, err)
	require.Equal(t, msg, string(decompressed))
}

func TestCompressingWriter_RotationChunkedRecoversAfterTransientError(t *testing.T) {
	const maxBytes = 1 << 20
	base := &transientErrorWriteCloser{failNext: true}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, &Rotation{MaxMegabytes: 1})
	require.NoError(t, err)

	oversized := make([]byte, 2*maxBytes)
	_, err = rand.Read(oversized)
	require.NoError(t, err)

	// First chunked write fails on initial frame.
	_, err = cw.Write(oversized)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient write error")

	// Subsequent write succeeds.
	msg := "normal record after chunk failure\n"
	_, err = cw.Write([]byte(msg))
	require.NoError(t, err)
	require.NoError(t, cw.Close())

	decoder, err := zstd.NewReader(&base.buf)
	require.NoError(t, err)
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	require.NoError(t, err)
	require.Equal(t, msg, string(decompressed))
}

func TestCompressingWriter_NonRotationStickyError(t *testing.T) {
	base := &transientErrorWriteCloser{failNext: true}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, nil)
	require.NoError(t, err)

	// Write enough random data to force zstd.Writer to flush compressed blocks to base.
	data := make([]byte, 1<<20)
	_, err = rand.Read(data)
	require.NoError(t, err)

	// First write fails when zstd flushes to base.
	_, err = cw.Write(data)
	require.Error(t, err)

	// In non-rotation streaming mode, errors are sticky because the zstd stream is corrupted.
	_, err = cw.Write([]byte("message 2\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient write error")
}

func TestCompressingWriter_RotationPartialWriteRotates(t *testing.T) {
	// A partial write writes part of a frame before failing, creating a torn frame.
	base := &transientErrorWriteCloser{failNext: true, partialNext: 5}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, &Rotation{MaxMegabytes: 1})
	require.NoError(t, err)

	// First write fails partway through.
	_, err = cw.Write([]byte("partial failed message\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient write error")

	// Because base implements Rotate(), it should have rotated to isolate the torn frame.
	require.True(t, base.rotated)
	require.Len(t, base.files, 1)

	// Next write goes to the fresh rotated file and succeeds.
	msg := "recovered message in new rotated file\n"
	n, err := cw.Write([]byte(msg))
	require.NoError(t, err)
	require.Equal(t, len(msg), n)

	require.NoError(t, cw.Close())

	// Verify the recovered file decodes cleanly.
	decoder, err := zstd.NewReader(&base.buf)
	require.NoError(t, err)
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	require.NoError(t, err)
	require.Equal(t, msg, string(decompressed))
}

func TestCompressingWriter_RotationPartialWriteStickyWithoutRotate(t *testing.T) {
	// Base does not implement Rotate(). A partial write leaves a torn frame in the file.
	base := &nonRotatingTransientErrorWriteCloser{failNext: true, partialNext: 5}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, &Rotation{MaxMegabytes: 1})
	require.NoError(t, err)

	_, err = cw.Write([]byte("partial failed message\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient write error")

	// Since base cannot rotate away from the torn frame, the error must be sticky.
	_, err = cw.Write([]byte("next message\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient write error")
}
