// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"sync"

	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4"
	"go.opentelemetry.io/collector/config/configcompression"
)

// resettableWriter is used to create partially configured writer
// that is allowed to reset any internal state with a new output destination.
type resettableWriter interface {
	io.WriteCloser

	Reset(dst io.Writer)
}

// Writer encapsulates the steps
// to convert a raw input stream into a compressed output stream.
type Writer interface {
	Compress(dst io.Writer, src io.Reader) (int64, error)
	CompressBytes(dst io.Writer, b []byte) (int64, error)
}

var (
	_ Writer = (*compressWriterPool[resettableWriter])(nil)

	_ resettableWriter = (*gzip.Writer)(nil)
	_ resettableWriter = (*zlib.Writer)(nil)
	_ resettableWriter = (*flate.Writer)(nil)
	_ resettableWriter = (*lz4.Writer)(nil)
	_ resettableWriter = (*zstd.Encoder)(nil)
	_ resettableWriter = (*snappy.Writer)(nil)
)

// compressWriterPool reuses resettable compression writers across calls.
// Its zero value is not ready for use; construct it with
// newcompressWriterPool. The pool may be used concurrently. Like sync.Pool,
// it may discard cached writers at any time.
type compressWriterPool[RW resettableWriter] struct {
	buffers sync.Pool
	writers sync.Pool
}

// newcompressWriterPool creates a pool whose supplier creates writers when
// the pool has no reusable writer. The supplier must return a usable writer.
func newCompressWriterPool[RW resettableWriter](supplier func() RW) *compressWriterPool[RW] {
	return &compressWriterPool[RW]{
		buffers: sync.Pool{
			New: func() any {
				return bytes.NewReader(nil)
			},
		},
		writers: sync.Pool{
			New: func() any {
				return supplier()
			},
		},
	}
}

func newGZIPCompressWriterPool(params configcompression.CompressionParams) (Writer, error) {
	if _, err := gzip.NewWriterLevel(io.Discard, int(params.Level)); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *gzip.Writer {
		w, _ := gzip.NewWriterLevel(io.Discard, int(params.Level))
		return w
	}), nil
}

func newZLibCompressWriterPool(params configcompression.CompressionParams) (Writer, error) {
	if _, err := zlib.NewWriterLevel(io.Discard, int(params.Level)); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *zlib.Writer {
		w, _ := zlib.NewWriterLevel(io.Discard, int(params.Level))
		return w
	}), nil
}

func newFlateCompressWriterPool(params configcompression.CompressionParams) (Writer, error) {
	if _, err := flate.NewWriter(io.Discard, int(params.Level)); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *flate.Writer {
		w, _ := flate.NewWriter(io.Discard, int(params.Level))
		return w
	}), nil
}

func newLZ4CompressWriterPool(params configcompression.CompressionParams) (Writer, error) {
	return newCompressWriterPool(func() *lz4.Writer {
		w := lz4.NewWriter(io.Discard)
		w.CompressionLevel = int(params.Level)
		return w
	}), nil
}

// newZSTDcompressWriterPool creates a pool of zstd encoders at the configured level.
func newZSTDCompressWriterPool(params configcompression.CompressionParams) (Writer, error) {
	level := zstd.EncoderLevelFromZstd(int(params.Level))
	if _, err := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(level)); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *zstd.Encoder {
		w, _ := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(level))
		return w
	}), nil
}

func newSnappyCompressWriterPool(_ configcompression.CompressionParams) (Writer, error) {
	return newCompressWriterPool(func() *snappy.Writer {
		return snappy.NewBufferedWriter(io.Discard)
	}), nil
}

// Compress writes the compressed contents of src to dst and returns the number
// of uncompressed bytes read from src. It closes the compression stream before
// returning so the output is complete. A source, write, or close error is
// returned; errors from copying and closing are joined when both occur.
func (pw *compressWriterPool[RW]) Compress(dst io.Writer, src io.Reader) (int64, error) {
	w := pw.writers.Get().(RW)
	defer pw.writers.Put(w)

	w.Reset(dst)

	n, err := io.Copy(w, src)
	err = errors.Join(err, w.Close())
	return n, err
}

func (pw *compressWriterPool[RW]) CompressBytes(dst io.Writer, b []byte) (int64, error) {
	r := pw.buffers.Get().(*bytes.Reader)
	defer pw.buffers.Put(r)

	r.Reset(b)

	return pw.Compress(dst, r)
}
