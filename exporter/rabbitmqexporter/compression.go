// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"

import (
	"bytes"
	"compress/gzip"
	"fmt"

	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/collector/config/configcompression"
)

// compressBody compresses body with the configured algorithm and returns the
// AMQP content-encoding to set on the message, or "" when uncompressed.
func compressBody(compression configcompression.Type, body []byte) ([]byte, string, error) {
	if !compression.IsCompressed() {
		return body, "", nil
	}
	switch compression {
	case configcompression.TypeGzip:
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(body); err != nil {
			return nil, "", err
		}
		if err := w.Close(); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), string(configcompression.TypeGzip), nil
	case configcompression.TypeZstd:
		enc, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, "", err
		}
		defer enc.Close()
		return enc.EncodeAll(body, nil), string(configcompression.TypeZstd), nil
	default:
		return nil, "", fmt.Errorf("unsupported compression %q", compression)
	}
}
