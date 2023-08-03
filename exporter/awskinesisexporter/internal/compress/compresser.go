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
)

type bufferedResetWriter interface {
	Write(p []byte) (int, error)
	Flush() error
	Reset(new io.Writer)
}

type Compressor interface {
	Do(in []byte) (out []byte, err error)
}

var _ Compressor = (*compressor)(nil)

type compressor struct {
	compression bufferedResetWriter
}

func NewCompressor(format string) (Compressor, error) {
	c := &compressor{
		compression: &noop{},
	}
	switch format {
	case "flate":
		w, err := flate.NewWriter(nil, flate.BestSpeed)
		if err != nil {
			return nil, err
		}
		c.compression = w
	case "gzip":
		w, err := gzip.NewWriterLevel(nil, gzip.BestSpeed)
		if err != nil {
			return nil, err
		}
		c.compression = w

	case "zlib":
		w, err := zlib.NewWriterLevel(nil, zlib.BestSpeed)
		if err != nil {
			return nil, err
		}
		c.compression = w
	case "noop", "none":
		// Already the default case
	default:
		return nil, fmt.Errorf("unknown compression format: %s", format)
	}

	return c, nil
}

func (c *compressor) Do(in []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	c.compression.Reset(buf)

	if _, err := c.compression.Write(in); err != nil {
		return nil, err
	}

	if err := c.compression.Flush(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
