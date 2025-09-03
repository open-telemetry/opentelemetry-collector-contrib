// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestEncode(t *testing.T) {
	testByteSlice := pcommon.NewByteSlice()
	testByteSlice.FromRaw([]byte("test string"))
	testByteSliceB64 := pcommon.NewByteSlice()
	testByteSliceB64.FromRaw([]byte("hello world"))

	testValue := pcommon.NewValueEmpty()
	_ = testValue.FromRaw("test string")
	testValueB64 := pcommon.NewValueEmpty()
	_ = testValueB64.FromRaw("hello world")

	type testCase struct {
		name          string
		value         any
		encoding      string
		want          any
		expectedError string
	}
	tests := []testCase{
		{
			name:     "convert byte array to base64",
			value:    []byte("test\n"),
			encoding: "base64",
			want:     "dGVzdAo=",
		},
		{
			name:     "convert string to base64",
			value:    "hello world",
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "convert ByteSlice to base64",
			value:    testByteSliceB64,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "convert Value to base64",
			value:    testValueB64,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "convert ByteSlice pointer to base64",
			value:    &testByteSliceB64,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "convert Value pointer to base64",
			value:    &testValueB64,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "encode string to us-ascii",
			value:    "test string",
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "encode byte array to us-ascii",
			value:    []byte("test string"),
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "encode byte slice to us-ascii",
			value:    testByteSlice,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "encode Value to us-ascii",
			value:    testValue,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "encode byte slice pointer to us-ascii",
			value:    &testByteSlice,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "encode Value pointer to us-ascii",
			value:    &testValue,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "encode string to ISO-8859-1",
			value:    "test string",
			encoding: "ISO-8859-1",
			want:     "test string",
		},
		{
			name:     "encode string to WINDOWS-1251",
			value:    "test string",
			encoding: "WINDOWS-1251",
			want:     "test string",
		},
		{
			name:     "encode string to WINDOWS-1252",
			value:    "test string",
			encoding: "WINDOWS-1252",
			want:     "test string",
		},
		{
			name:     "encode string to UTF-8",
			value:    "test string",
			encoding: "UTF-8",
			want:     "test string",
		},
		{
			name:     "encode string to UTF-16 1",
			value:    "test string",
			encoding: "UTF-16",
			want:     "t\x00e\x00s\x00t\x00 \x00s\x00t\x00r\x00i\x00n\x00g\x00",
		},
		{
			name:     "encode string to UTF-16 2",
			value:    "test string",
			encoding: "UTF16",
			want:     "t\x00e\x00s\x00t\x00 \x00s\x00t\x00r\x00i\x00n\x00g\x00",
		},
		{
			name:          "encode string to GB2312; no encoder available",
			value:         "test string",
			encoding:      "GB2312",
			want:          nil,
			expectedError: "no charmap defined for encoding 'GB2312'",
		},
		{
			name:          "non-string",
			value:         10,
			encoding:      "base64",
			expectedError: "unsupported type provided to Encode function: int",
		},
		{
			name:          "nil",
			value:         nil,
			encoding:      "base64",
			expectedError: "unsupported type provided to Encode function: <nil>",
		},
		{
			name:     "base64 with url-safe sensitive characters",
			value:    "Go?/Z~x",
			encoding: "base64",
			want:     "R28/L1p+eA==",
		},
		{
			name:     "base64-raw with url-safe sensitive characters",
			value:    "Go?/Z~x",
			encoding: "base64-raw",
			want:     "R28/L1p+eA",
		},
		{
			name:     "base64-url with url-safe sensitive characters",
			value:    "Go?/Z~x",
			encoding: "base64-url",
			want:     "R28_L1p-eA==",
		},
		{
			name:     "base64-raw-url with url-safe sensitive characters",
			value:    "Go?/Z~x",
			encoding: "base64-raw-url",
			want:     "R28_L1p-eA",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createEncodeFunction[any](ottl.FunctionContext{}, &EncodeArguments[any]{
				Target: &ottl.StandardGetSetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return tt.value, nil
					},
				},
				Encoding: tt.encoding,
			})

			require.NoError(t, err)

			result, err := expressionFunc(nil, nil)
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}
