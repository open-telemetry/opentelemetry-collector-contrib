// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4"
	"go.opentelemetry.io/collector/config/configcompression"
)

// ResettableReader is an io.ReadCloser that can be reset to read from a new source.
type ResettableReader interface {
	io.ReadCloser

	Reset(r io.Reader) error
}

var _ ResettableReader = (*gzip.Reader)(nil)

// Reader decompresses input data into a destination.
type Reader interface {
	// Decompress reads compressed data from src, writes the uncompressed data to dst,
	// and returns the number of uncompressed bytes written.
	Decompress(dst io.Writer, src io.Reader) (int64, error)
	// DecompressBytes writes the uncompressed form of b to dst and returns the number
	// of bytes written.
	DecompressBytes(dst io.Writer, b []byte) (int64, error)
}

// ReadLeaser provides access to a pooled [ResettableReader] within a callback.
// The reader is valid only until the callback returns. Storing it for later use
// or allowing a goroutine to use it after the callback returns can cause data
// races and corrupt data when another operation reuses the reader.
type ReadLeaser interface {
	// LeaseReader calls leaser synchronously with an exclusively leased reader.
	// The callback is responsible for resetting the reader and closing it on
	// every exit path, including errors and panics. LeaseReader returns the
	// callback's error unchanged and returns the reader to the pool even if the
	// callback returns an error or panics.
	LeaseReader(leaser func(r ResettableReader) error) error
}

// compressReaderPool reuses resettable decompression readers across calls.
// Its zero value is not ready for use; construct it with newCompressReaderPool.
// The pool may be used concurrently and may discard cached readers at any time.
type compressReaderPool[RR ResettableReader] struct {
	buffers sync.Pool
	readers sync.Pool
}

var (
	_ Reader     = (*compressReaderPool[ResettableReader])(nil)
	_ ReadLeaser = (*compressReaderPool[ResettableReader])(nil)
)

// newCompressReaderPool creates a pool whose supplier creates readers when the
// pool has no reusable reader. The supplier must return a usable reader.
func newCompressReaderPool[RR ResettableReader](supplier func() RR) *compressReaderPool[RR] {
	return &compressReaderPool[RR]{
		buffers: sync.Pool{
			New: func() any {
				return bytes.NewReader(nil)
			},
		},
		readers: sync.Pool{
			New: func() any {
				return supplier()
			},
		},
	}
}

func newGZIPCompressReaderPool(_ configcompression.CompressionParams) Reader {
	return newCompressReaderPool(func() *gzip.Reader {
		return new(gzip.Reader)
	})
}

func newZLibCompressReaderPool(_ configcompression.CompressionParams) Reader {
	return newCompressReaderPool(func() *readAdapter[io.ReadCloser] {
		// A valid zlib stream for empty input, used only to initialize the reader.
		zl, _ := zlib.NewReader(bytes.NewReader([]byte{
			0x78, 0x9c, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01,
		}))
		return newReadAdapter(
			zl,
			withReadAdapterClose(func(rc io.ReadCloser) error {
				return rc.Close()
			}),
			withReadAdapterReset(func(rc io.ReadCloser, r io.Reader) error {
				resetter, ok := rc.(zlib.Resetter)
				if !ok {
					return fmt.Errorf("unable to convert %T to resetter", rc)
				}
				return resetter.Reset(r, nil)
			}),
		)
	})
}

func newFlateCompressReaderPool(_ configcompression.CompressionParams) Reader {
	return newCompressReaderPool(func() *readAdapter[io.ReadCloser] {
		return newReadAdapter(
			flate.NewReader(bytes.NewBuffer(nil)),
			withReadAdapterClose(func(rc io.ReadCloser) error {
				return rc.Close()
			}),
			withReadAdapterReset(func(rc io.ReadCloser, r io.Reader) error {
				resetter, ok := rc.(flate.Resetter)
				if !ok {
					return fmt.Errorf("unable to convert %T to resetter", rc)
				}
				return resetter.Reset(r, nil)
			}),
		)
	})
}

func newLZ4CompressReaderPool(_ configcompression.CompressionParams) Reader {
	return newCompressReaderPool(func() *readAdapter[*lz4.Reader] {
		return newReadAdapter(
			new(lz4.Reader),
			withReadAdapterReset(func(lzz *lz4.Reader, r io.Reader) error {
				lzz.Reset(r)
				return nil
			}),
		)
	})
}

func newZSTDCompressReaderPool(_ configcompression.CompressionParams) Reader {
	return newCompressReaderPool(func() *readAdapter[*zstd.Decoder] {
		r, _ := zstd.NewReader(bytes.NewBuffer(nil))
		return newReadAdapter(
			r,
			withReadAdapterReset(func(zr *zstd.Decoder, r io.Reader) error {
				return zr.Reset(r)
			}),
		)
	})
}

func newSnappyCompressReaderPool(_ configcompression.CompressionParams) Reader {
	return newCompressReaderPool(func() *readAdapter[*snappy.Reader] {
		return newReadAdapter(
			snappy.NewReader(bytes.NewBuffer(nil)),
			withReadAdapterReset(func(sp *snappy.Reader, r io.Reader) error {
				sp.Reset(r)
				return nil
			}),
		)
	})
}

// Decompress reads compressed data, writes its uncompressed contents to dst,
// and returns the number of uncompressed bytes written. Reset, read, and close
// errors are returned; reset errors are wrapped with the "reset" operation.
func (crp *compressReaderPool[RR]) Decompress(dst io.Writer, compressed io.Reader) (int64, error) {
	src := crp.readers.Get().(RR)
	defer crp.readers.Put(src)

	if err := src.Reset(compressed); err != nil {
		return 0, fmt.Errorf("reset: %w", err)
	}

	n, err := io.Copy(dst, src)
	err = errors.Join(err, src.Close())

	return n, err
}

// DecompressBytes decompresses b into dst and returns the number of
// uncompressed bytes written.
func (crp *compressReaderPool[RR]) DecompressBytes(dst io.Writer, b []byte) (int64, error) {
	buf := crp.buffers.Get().(*bytes.Reader)
	defer crp.buffers.Put(buf)

	buf.Reset(b)
	return crp.Decompress(dst, buf)
}

// LeaseReader calls leaser with a reader that is valid only during the callback.
// See [ReadLeaser] for the callback's responsibilities and lifetime restrictions.
func (crp *compressReaderPool[RR]) LeaseReader(leaser func(r ResettableReader) error) error {
	r := crp.readers.Get().(RR)
	defer crp.readers.Put(r)

	return leaser(r)
}
