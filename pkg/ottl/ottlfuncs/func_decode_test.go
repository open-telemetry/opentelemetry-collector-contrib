// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestDecode(t *testing.T) {
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
			name:     "decode us-ascii encoded string",
			value:    "test string",
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
			name:          "decode GB2312 encoded string; no decoder available",
			value:         "test string",
			encoding:      "GB2312",
			want:          nil,
			expectedError: "no decoder available for encoding: GB2312",
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

			require.Nil(t, err)

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
