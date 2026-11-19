// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"errors"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
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

// Writer encapsulates the steps
// to convert a raw input stream into a compressed output stream.
type Writer interface {
	Compress(dst io.Writer, src io.Reader) (int64, error)
	CompressBytes(dst io.Writer, b []byte) (int64, error)
}

// WriteLeaser provides access to a pooled [ResettableWriter] within a callback.
// The writer is valid only until the callback returns. Storing it for later use
// or allowing a goroutine to use it after the callback returns can cause data
// races and corrupt data when another operation reuses the writer.
type WriteLeaser interface {
	// LeaseWriter calls leaser synchronously with an exclusively leased writer.
	// The callback is responsible for resetting the writer and closing it on
	// every exit path, including errors and panics. Closing finishes the compressed
	// stream. LeaseWriter returns the callback's error unchanged and returns the
	// writer to the pool even if the callback returns an error or panics.
	LeaseWriter(leaser func(rw ResettableWriter) error) error
}

var (
	_ Writer      = (*compressWriterPool[ResettableWriter])(nil)
	_ WriteLeaser = (*compressWriterPool[ResettableWriter])(nil)

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

// LeaseWriter calls leaser with a writer that is valid only during the callback.
// See [WriteLeaser] for the callback's responsibilities and lifetime restrictions.
func (pw *compressWriterPool[RW]) LeaseWriter(leaser func(rw ResettableWriter) error) error {
	rw := pw.writers.Get().(RW)
	defer pw.writers.Put(rw)

	return leaser(rw)
}
