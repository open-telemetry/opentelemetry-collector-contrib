// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"errors"
	"fmt"

	"github.com/klauspost/compress/snappy"
)

// ErrDecodedSizeExceeded indicates that a block exceeds the configured decoded size limit.
var ErrDecodedSizeExceeded = errors.New("received message larger than max")

// DecodeSnappyBlock decompresses a raw Snappy block after checking that its
// decoded size does not exceed maxDecodedSize. It returns an error wrapping
// ErrDecodedSizeExceeded when the block is too large. Framed streams are handled
// separately by the reader factory.
func DecodeSnappyBlock(src []byte, maxDecodedSize int) ([]byte, error) {
	size, err := snappy.DecodedLen(src)
	if err != nil {
		return nil, err
	}
	if size > maxDecodedSize {
		return nil, fmt.Errorf("%w (%d vs %d)", ErrDecodedSizeExceeded, size, maxDecodedSize)
	}
	return snappy.DecodeStrict(nil, src)
}
