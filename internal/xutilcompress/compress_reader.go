// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xutilcompress // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/xutilcompress"

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

type resettableReader interface {
	io.ReadCloser

	Reset(r io.Reader) error
}

var (
	_ resettableReader = (*gzip.Reader)(nil)
)

type Reader interface {
	Decompress(dst io.Writer, src io.Reader) (int64, error)
	DecompressBytes(dst io.Writer, b []byte) (int64, error)
}

type compressReaderPool[RR resettableReader] struct {
	buffers sync.Pool
	readers sync.Pool
}

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
		zl, _ := zlib.NewReader(bytes.NewBuffer(nil))
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
		return newReadAdapter(
			new(zstd.Decoder),
			withReadAdapterClose(func(r *zstd.Decoder) error {
				r.Close()
				return nil
			}),
			withReadAdapterReset(func(zr *zstd.Decoder, r io.Reader) error {
				return zr.Reset(r)
			}),
		)
	})
}

func newSnappyCompressReaderPool(_ configcompression.CompressionParams) Reader {
	return newCompressReaderPool(func() *readAdapter[*snappy.Reader] {
		return newReadAdapter(
			new(snappy.Reader),
			withReadAdapterReset(func(sp *snappy.Reader, r io.Reader) error {
				sp.Reset(r)
				return nil
			}),
		)
	})
}

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

func (crp *compressReaderPool[RR]) DecompressBytes(dst io.Writer, b []byte) (int64, error) {
	buf := crp.buffers.Get().(*bytes.Reader)
	defer crp.buffers.Put(buf)

	buf.Reset(b)
	return crp.Decompress(dst, buf)
}
