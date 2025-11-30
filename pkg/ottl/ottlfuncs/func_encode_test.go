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
	testByteSlice.FromRaw([]byte("hello world"))

	testValue := pcommon.NewValueEmpty()
	_ = testValue.FromRaw("hello world")

	type testCase struct {
		name          string
		value         any
		encoding      string
		want          any
		expectedError string
	}
	tests := []testCase{
		{
			name:     "encode string to base64",
			value:    "hello world",
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "encode byte array to base64",
			value:    []byte("test\n"),
			encoding: "base64",
			want:     "dGVzdAo=",
		},
		{
			name:     "encode ByteSlice to base64",
			value:    testByteSlice,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "encode Value to base64",
			value:    testValue,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "encode ByteSlice pointer to base64",
			value:    &testByteSlice,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "encode Value pointer to base64",
			value:    &testValue,
			encoding: "base64",
			want:     "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "encode empty string to base64",
			value:    "",
			encoding: "base64",
			want:     "",
		},
		{
			name:     "encode string with url-safe sensitive characters to base64",
			value:    "Go?/Z~x",
			encoding: "base64",
			want:     "R28/L1p+eA==",
		},
		{
			name:     "encode string with url-safe sensitive characters to base64-raw",
			value:    "Go?/Z~x",
			encoding: "base64-raw",
			want:     "R28/L1p+eA",
		},
		{
			name:     "encode string with url-safe sensitive characters to base64-url",
			value:    "Go?/Z~x",
			encoding: "base64-url",
			want:     "R28_L1p-eA==",
		},
		{
			name:     "encode string with url-safe sensitive characters to base64-raw-url",
			value:    "Go?/Z~x",
			encoding: "base64-raw-url",
			want:     "R28_L1p-eA",
		},
		{
			name:     "encode us-ascii string",
			value:    "test string",
			encoding: "us-ascii",
			want:     "test string",
		},
		{
			name:     "encode UTF-8 string",
			value:    "test string",
			encoding: "UTF-8",
			want:     "test string",
		},
		{
			name:     "encode ISO-8859-1 string",
			value:    "test string",
			encoding: "ISO-8859-1",
			want:     "test string",
		},
		{
			name:          "non-string input",
			value:         10,
			encoding:      "base64",
			expectedError: "unsupported type provided to Encode function: int",
		},
		{
			name:          "nil input",
			value:         nil,
			encoding:      "base64",
			expectedError: "unsupported type provided to Encode function: <nil>",
		},
		{
			name:          "unsupported encoding",
			value:         "test",
			encoding:      "invalid-encoding",
			expectedError: "unsupported encoding",
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
				Encoding: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return tt.encoding, nil
					},
				},
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
