// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import "github.com/klauspost/compress/zstd"

// compressFunc defines how to compress encoded telemetry data.
type compressFunc func(src []byte) []byte

var encoder, _ = zstd.NewWriter(nil)

var encoders = map[string]compressFunc{
	compressionZSTD: zstdCompress,
}

func buildCompressor(compression string) compressFunc {
	if compression == "" {
		return noneCompress
	}
	return encoders[compression]
}

// zstdCompress compress a buffer with zstd
func zstdCompress(src []byte) []byte {
	return encoder.EncodeAll(src, make([]byte, 0, len(src)))
}

// noneCompress return src
func noneCompress(src []byte) []byte {
	return src
}
