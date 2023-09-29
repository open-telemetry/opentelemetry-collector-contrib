// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicexporter

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
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

// newCompressor takes encoding format and returns the compressor and an error.
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

	// Reset c.buf to start with empty message
	c.buf.Reset()
	c.writer.Reset(&c.buf)

	// use io.Copy here to do the smart thing depending on what the destination and source implement
	// in most cases this results in no buffer being needed
	// the above is definitely the case for strings.NewReader and bytes.NewReader which we use in the sender
	if _, err := io.Copy(c.writer, data); err != nil {
		return nil, err
	}

	if err := c.writer.Close(); err != nil {
		return nil, err
	}

	return bytes.NewReader(c.buf.Bytes()), nil
}
