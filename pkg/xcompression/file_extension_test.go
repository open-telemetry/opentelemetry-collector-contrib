// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcompression"
)

func ExampleAppendFileExtension() {
	compressed := AppendFileExtension("data.json", configcompression.TypeGzip)
	uncompressed := AppendFileExtension("data.json", "")

	fmt.Println(compressed)
	fmt.Println(uncompressed)
	// Output:
	// data.json.gz
	// data.json
}

func TestAppendFileExtension(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		compression configcompression.Type
		want        string
	}{
		{name: "gzip", compression: configcompression.TypeGzip, want: "data.json.gz"},
		{name: "zstd", compression: configcompression.TypeZstd, want: "data.json.zst"},
		{name: "zlib", compression: configcompression.TypeZlib, want: "data.json.zlib"},
		{name: "deflate", compression: configcompression.TypeDeflate, want: "data.json.deflate"},
		{name: "snappy", compression: configcompression.TypeSnappy, want: "data.json.snappy"},
		{name: "framed snappy", compression: configcompression.TypeSnappyFramed, want: "data.json.snappy"},
		{name: "lz4", compression: configcompression.TypeLz4, want: "data.json.lz4"},
		{name: "unset", want: "data.json"},
		{name: "none", compression: "none", want: "data.json"},
		{name: "unknown", compression: "unknown", want: "data.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, AppendFileExtension("data.json", tc.compression))
		})
	}
}
