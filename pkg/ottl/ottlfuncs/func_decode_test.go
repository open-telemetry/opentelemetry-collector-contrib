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

func TestDecode(t *testing.T) {
	testByteSlice := pcommon.NewByteSlice()
	testByteSlice.FromRaw([]byte("test string"))
	testByteSliceB64 := pcommon.NewByteSlice()
	testByteSliceB64.FromRaw([]byte("aGVsbG8gd29ybGQ="))

	testValue := pcommon.NewValueEmpty()
	_ = testValue.FromRaw("test string")
	testValueB64 := pcommon.NewValueEmpty()
	_ = testValueB64.FromRaw("aGVsbG8gd29ybGQ=")

	type testCase struct {
		name          string
		value         any
		encoding      string
		want          any
		expectedError string
	}
	tests := []testCase{
		{
			name:     "convert base64 byte array",
			value:    []byte("dGVzdAo="),
			encoding: "base64",
			want:     "test\n",
		},
		{
			name:     "convert base64 string",
			value:    "aGVsbG8gd29ybGQ=",
			encoding: "base64",
			want:     "hello world",
		},
		{
			name:     "convert base64 ByteSlice",
			value:    testByteSliceB64,
			encoding: "base64",
			want:     "hello world",
		},
		{
			name:     "convert base64 Value",
			value:    testValueB64,
			encoding: "base64",
			want:     "hello world",
		},
		{
			name:     "convert base64 ByteSlice pointer",
			value:    &testByteSliceB64,
			encoding: "base64",
			want:     "hello world",
		},
		{
			name:     "convert base64 Value pointer",
			value:    &testValueB64,
			encoding: "base64",
			want:     "hello world",
		},
		{
			name:     "decode us-ascii encoded string",
			value:    "test string",
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "decode us-ascii encoded byte array",
			value:    []byte("test string"),
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "decode us-ascii encoded byte slice",
			value:    testByteSlice,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "decode us-ascii encoded Value",
			value:    testValue,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "decode us-ascii encoded byte slice pointer",
			value:    &testByteSlice,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "decode us-ascii encoded Value pointer",
			value:    &testValue,
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "decode ISO-8859-1 encoded string",
			value:    "test string",
			encoding: "ISO-8859-1",
			want:     "test string",
		},
		{
			name:     "decode WINDOWS-1251 encoded string",
			value:    "test string",
			encoding: "WINDOWS-1251",
			want:     "test string",
		},
		{
			name:     "decode WINDOWS-1252 encoded string",
			value:    "test string",
			encoding: "WINDOWS-1252",
			want:     "test string",
		},
		{
			name:     "decode UTF-8 encoded string",
			value:    "test string",
			encoding: "UTF-8",
			want:     "test string",
		},
		{
			name:     "decode UTF-16 encoded string 1",
			value:    []byte{116, 0, 101, 0, 115, 0, 116, 0, 32, 0, 115, 0, 116, 0, 114, 0, 105, 0, 110, 0, 103, 0},
			encoding: "UTF-16",
			want:     "test string",
		},
		{
			name:     "decode UTF-16 encoded string 2",
			value:    []byte{116, 0, 101, 0, 115, 0, 116, 0, 32, 0, 115, 0, 116, 0, 114, 0, 105, 0, 110, 0, 103, 0},
			encoding: "UTF16",
			want:     "test string",
		},
		{
			name:          "decode GB2312 encoded string; no decoder available",
			value:         "test string",
			encoding:      "GB2312",
			want:          nil,
			expectedError: "no charmap defined for encoding 'GB2312'",
		},
		{
			name:          "non-string",
			value:         10,
			encoding:      "base64",
			expectedError: "unsupported type provided to Decode function: int",
		},
		{
			name:          "nil",
			value:         nil,
			encoding:      "base64",
			expectedError: "unsupported type provided to Decode function: <nil>",
		},
		{
			name:          "not-base64-string",
			value:         "!@#$%^&*()_+",
			encoding:      "base64",
			expectedError: "illegal base64 data at input byte",
		},
		{
			name:          "missing-base64-padding",
			value:         "cmVtb3ZlZCBwYWRkaW5nCg",
			encoding:      "base64",
			expectedError: "illegal base64 data at input byte",
		},
		{
			name:     "base64 with url-safe sensitive characters",
			value:    "R28/L1p+eA==",
			encoding: "base64",
			want:     "Go?/Z~x",
		},
		{
			name:     "base64-raw with url-safe sensitive characters",
			value:    "R28/L1p+eA",
			encoding: "base64-raw",
			want:     "Go?/Z~x",
		},
		{
			name:     "base64-url with url-safe sensitive characters",
			value:    "R28_L1p-eA==",
			encoding: "base64-url",
			want:     "Go?/Z~x",
		},
		{
			name:     "base64-raw-url with url-safe sensitive characters",
			value:    "R28_L1p-eA",
			encoding: "base64-raw-url",
			want:     "Go?/Z~x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createDecodeFunction[any](ottl.FunctionContext{}, &DecodeArguments[any]{
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
