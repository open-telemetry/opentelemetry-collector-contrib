// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func TestUTF8Encoding(t *testing.T) {
	tests := []struct {
		name         string
		encoding     encoding.Encoding
		encodingName string
	}{
		{
			name:         "UTF8 encoding",
			encoding:     unicode.UTF8,
			encodingName: "utf8",
		},
		{
			name:         "UTF8-raw encoding",
			encoding:     UTF8Raw,
			encodingName: "utf8-raw",
		},
		{
			name:         "GBK encoding",
			encoding:     simplifiedchinese.GBK,
			encodingName: "gbk",
		},
		{
			name:         "SHIFT_JIS encoding",
			encoding:     japanese.ShiftJIS,
			encodingName: "shift_jis",
		},
		{
			name:         "EUC-KR encoding",
			encoding:     korean.EUCKR,
			encodingName: "euc-kr",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enc, err := LookupEncoding(test.encodingName)
			assert.NoError(t, err)
			assert.Equal(t, test.encoding, enc)
		})
	}
}

func TestDecodeAsString(t *testing.T) {
	tests := []struct {
		name     string
		decoder  *encoding.Decoder
		input    []byte
		expected string
	}{
		{
			name:     "nil",
			decoder:  &encoding.Decoder{Transformer: transform.Nop},
			input:    nil,
			expected: "",
		},
		{
			name:     "empty",
			decoder:  &encoding.Decoder{Transformer: transform.Nop},
			input:    []byte{},
			expected: "",
		},
		{
			name:     "empty",
			decoder:  &encoding.Decoder{Transformer: transform.Nop},
			input:    []byte("test"),
			expected: "test",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enc, err := DecodeAsString(test.decoder, test.input)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, enc)
		})
	}
}
