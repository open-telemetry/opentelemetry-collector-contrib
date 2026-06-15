// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"

import "bytes"

// bytesS3ObjectReader wraps a bytes.Reader to implement S3ObjectReader.
type bytesS3ObjectReader struct {
	*bytes.Reader
}

func (bytesS3ObjectReader) Close() error { return nil }

// NewBytesS3ObjectReader returns an S3ObjectReader backed by in-memory data.
// Intended for use in tests.
func NewBytesS3ObjectReader(data []byte) S3ObjectReader {
	return &bytesS3ObjectReader{Reader: bytes.NewReader(data)}
}
