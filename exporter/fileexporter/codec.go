// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import "github.com/klauspost/compress/zstd"

// compressFunc defines how to compress encoded telemetry data.
type compressFunc func(src []byte) []byte

// zstdLevel maps compression_params.level to a zstd encoder level. 0 (unset) uses the zstd default.
func zstdLevel(level int) zstd.EncoderLevel {
	if level == 0 {
		return zstd.SpeedDefault
	}
	return zstd.EncoderLevelFromZstd(level)
}

// buildCompressor returns a message-level compressor for the given compression and level.
func buildCompressor(compression string, level int) (compressFunc, error) {
	if compression == "" {
		return noneCompress, nil
	}
	// Only zstd passes config validation. EncodeAll is safe for concurrent use.
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstdLevel(level)))
	if err != nil {
		return nil, err
	}
	return func(src []byte) []byte {
		return encoder.EncodeAll(src, make([]byte, 0, len(src)))
	}, nil
}

// noneCompress return src
func noneCompress(src []byte) []byte {
	return src
}
