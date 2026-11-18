// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4"
	"go.opentelemetry.io/collector/config/configcompression"
)

// resettableReader is an io.ReadCloser that can be reset to read from a new source.
type resettableReader interface {
	io.ReadCloser

	Reset(r io.Reader) error
}

var _ resettableReader = (*gzip.Reader)(nil)

// Reader decompresses input data into a destination.
type Reader interface {
	// Decompress reads compressed data from src, writes the uncompressed data to dst,
	// and returns the number of uncompressed bytes written.
	Decompress(dst io.Writer, src io.Reader) (int64, error)
	// DecompressBytes writes the uncompressed form of b to dst and returns the number
	// of bytes written.
	DecompressBytes(dst io.Writer, b []byte) (int64, error)
}

// compressReaderPool reuses resettable decompression readers across calls.
// Its zero value is not ready for use; construct it with newCompressReaderPool.
// The pool may be used concurrently and may discard cached readers at any time.
type compressReaderPool[RR resettableReader] struct {
	buffers sync.Pool
	readers sync.Pool
}

// newCompressReaderPool creates a pool whose supplier creates readers when the
// pool has no reusable reader. The supplier must return a usable reader.
func newCompressReaderPool[RR resettableReader](supplier func() RR) *compressReaderPool[RR] {
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
