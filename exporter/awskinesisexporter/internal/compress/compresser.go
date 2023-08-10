// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compress // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"sync"

	"go.uber.org/zap"
)

type bufferedResetWriter interface {
	Write(p []byte) (int, error)
	Flush() error
	Reset(newWriter io.Writer)
}

type Compressor interface {
	Do(in []byte) (out []byte, err error)
}

var _ Compressor = (*compressor)(nil)

type compressor struct {
	compressionPool sync.Pool
}

func NewCompressor(format string, log *zap.Logger) (Compressor, error) {
	var c Compressor
	switch format {
	case "flate":
		c = &compressor{
			compressionPool: sync.Pool{
				New: func() any {
					w, err := flate.NewWriter(nil, flate.BestSpeed)
					if err != nil {
						errMsg := fmt.Sprintf("Unable to instantiate Flate compressor: %v", err)
						log.Error(errMsg)
						return nil
					}
					return w
				},
			},
		}
	case "gzip":

		c = &compressor{
			compressionPool: sync.Pool{
				New: func() any {
					w, err := gzip.NewWriterLevel(nil, gzip.BestSpeed)
					if err != nil {
						errMsg := fmt.Sprintf("Unable to instantiate Gzip compressor: %v", err)
						log.Error(errMsg)
						return nil
					}
					return w
				},
			},
		}
	case "zlib":
		c = &compressor{
			compressionPool: sync.Pool{
				New: func() any {
					w, err := zlib.NewWriterLevel(nil, zlib.BestSpeed)
					if err != nil {
						errMsg := fmt.Sprintf("Unable to instantiate Zlib compressor: %v", err)
						log.Error(errMsg)
						return nil
					}
					return w
				},
			},
		}
	case "noop", "none":
		c = &compressor{
			compressionPool: sync.Pool{
				New: func() any {
					return &noop{}
				},
			},
		}
	default:
		return nil, fmt.Errorf("unknown compression format: %s", format)
	}

	return c, nil
}

func (c *compressor) Do(in []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	comp := c.compressionPool.Get().(bufferedResetWriter)
	if comp == nil {
		return nil, fmt.Errorf("compressor is nil and did not get instantiated correctly")
	}
	defer c.compressionPool.Put(comp)

	comp.Reset(buf)

	if _, err := comp.Write(in); err != nil {
		return nil, err
	}

	if err := comp.Flush(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
