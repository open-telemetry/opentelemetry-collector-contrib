// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
)

type compressor struct {
	format CompressEncodingType
	writer encoder
	buf    bytes.Buffer
}

type encoder interface {
	io.WriteCloser
	Reset(dst io.Writer)
}

// newCompressor takes encoding format and returns compressor struct and error eventually
func newCompressor(format CompressEncodingType) (compressor, error) {
	var (
		writer encoder
		err    error
	)

	switch format {
	case GZIPCompression:
		writer = gzip.NewWriter(io.Discard)
	case DeflateCompression:
		writer, err = flate.NewWriter(io.Discard, flate.BestSpeed)
		if err != nil {
			return compressor{}, err
		}
	case NoCompression:
		writer = nil
	default:
		return compressor{}, fmt.Errorf("invalid format: %s", format)
	}

	return compressor{
		format: format,
		writer: writer,
	}, nil
}

// compress takes a reader with uncompressed data and returns
// a reader with the same data compressed using c.writer
func (c *compressor) compress(data io.Reader) (io.Reader, error) {
	if c.writer == nil {
		return data, nil
	}

	var dataBytes bytes.Buffer
	if _, err := dataBytes.ReadFrom(data); err != nil {
		return nil, err
	}

	// Reset c.buf to start with empty message
	c.buf.Reset()
	c.writer.Reset(&c.buf)

	if _, err := c.writer.Write(dataBytes.Bytes()); err != nil {
		return nil, err
	}

	if err := c.writer.Close(); err != nil {
		return nil, err
	}

	return bytes.NewReader(c.buf.Bytes()), nil
}
