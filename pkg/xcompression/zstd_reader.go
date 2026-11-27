// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"io"

	"github.com/klauspost/compress/zstd"
)

// NewZstdReader opens a zstd stream with the supplied decoder memory limit in bytes.
// A zero limit is invalid. For streaming reads, maxMemory bounds the decoder
// window size. Callers that need a total output limit must enforce it separately.
// If the decoder uses its in-memory path, maxMemory limits the decoded size.
// The caller must close the reader, including when it stops reading before EOF.
func NewZstdReader(src io.Reader, maxMemory uint64) (io.ReadCloser, error) {
	decoder, err := zstd.NewReader(src, zstd.WithDecoderMaxMemory(maxMemory))
	if err != nil {
		return nil, err
	}
	return decoder.IOReadCloser(), nil
}
