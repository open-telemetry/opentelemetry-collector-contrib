// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
