// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xutilcompress // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/xutilcompress"

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

// ResettableWriter is used to create partially configured writer
// that is allowed to reset any internal state with a new output destination.
type ResettableWriter interface {
	io.WriteCloser

	Reset(dst io.Writer)
}

// CompressWriter is used to encapsulate the required steps
// to convert a raw input stream into a compressed output stream.
type CompressWriter interface {
	Compress(dst io.Writer, src io.Reader) (int64, error)
	CompressBytes(dst io.Writer, b []byte) (int64, error)
}

// NopCompressWriter is used where no/none compression is provided
// but it allows for the same workflow to be followed,
// reducing required branching or required checking
type NopCompressWriter struct{}

func (NopCompressWriter) Compress(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

func (NopCompressWriter) CompressBytes(dst io.Writer, b []byte) (int64, error) {
	// Write directly to avoid allocating an intermediate reader to the heap.
	if len(b) == 0 {
		return 0, nil
	}
	n, err := dst.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return int64(n), err
}

var (
	_ CompressWriter = (*compressWriterPool[ResettableWriter])(nil)
	_ CompressWriter = (*NopCompressWriter)(nil)

	_ ResettableWriter = (*gzip.Writer)(nil)
	_ ResettableWriter = (*zlib.Writer)(nil)
	_ ResettableWriter = (*flate.Writer)(nil)
	_ ResettableWriter = (*lz4.Writer)(nil)
	_ ResettableWriter = (*zstd.Encoder)(nil)
	_ ResettableWriter = (*snappy.Writer)(nil)
)

// compressWriterPool reuses resettable compression writers across calls.
// Its zero value is not ready for use; construct it with
// newcompressWriterPool. The pool may be used concurrently. Like sync.Pool,
// it may discard cached writers at any time.
type compressWriterPool[RW ResettableWriter] struct {
	buffers sync.Pool
	writers sync.Pool
}

// newcompressWriterPool creates a pool whose supplier creates writers when
// the pool has no reusable writer. The supplier must return a usable writer.
func newCompressWriterPool[RW ResettableWriter](supplier func() RW) *compressWriterPool[RW] {
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

func newGZIPCompressWriterPool(params configcompression.CompressionParams) (CompressWriter, error) {
	if _, err := gzip.NewWriterLevel(io.Discard, int(params.Level)); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *gzip.Writer {
		w, _ := gzip.NewWriterLevel(io.Discard, int(params.Level))
		return w
	}), nil
}

func newZLibCompressWriterPool(params configcompression.CompressionParams) (CompressWriter, error) {
	if _, err := zlib.NewWriterLevel(io.Discard, int(params.Level)); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *zlib.Writer {
		w, _ := zlib.NewWriterLevel(io.Discard, int(params.Level))
		return w
	}), nil
}

func newFlateCompressWriterPool(params configcompression.CompressionParams) (CompressWriter, error) {
	if _, err := flate.NewWriter(io.Discard, int(params.Level)); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *flate.Writer {
		w, _ := flate.NewWriter(io.Discard, int(params.Level))
		return w
	}), nil
}

func newLZ4CompressWriterPool(params configcompression.CompressionParams) (CompressWriter, error) {
	return newCompressWriterPool(func() *lz4.Writer {
		w := lz4.NewWriter(io.Discard)
		w.CompressionLevel = int(params.Level)
		return w
	}), nil
}

// newZSTDcompressWriterPool creates a pool of zstd encoders at the configured level.
func newZSTDCompressWriterPool(params configcompression.CompressionParams) (CompressWriter, error) {
	if _, err := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(zstd.EncoderLevel(params.Level))); err != nil {
		return nil, err
	}
	return newCompressWriterPool(func() *zstd.Encoder {
		w, _ := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(zstd.EncoderLevel(params.Level)))
		return w
	}), nil
}

func newSnappyCompressWriterPool(_ configcompression.CompressionParams) (CompressWriter, error) {
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
