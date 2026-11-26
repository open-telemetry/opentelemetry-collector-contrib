// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import "go.opentelemetry.io/collector/config/configcompression"

// AppendFileExtension appends the file suffix for the given compression type.
func AppendFileExtension(name string, compression configcompression.Type) string {
	switch compression {
	case configcompression.TypeGzip:
		return name + ".gz"
	case configcompression.TypeZstd:
		return name + ".zst"
	case configcompression.TypeZlib:
		return name + ".zlib"
	case configcompression.TypeDeflate:
		return name + ".deflate"
	case configcompression.TypeSnappy, configcompression.TypeSnappyFramed:
		return name + ".snappy"
	case configcompression.TypeLz4:
		return name + ".lz4"
	default:
		return name
	}
}
