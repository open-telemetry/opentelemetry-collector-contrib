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

func TestBase64Encode(t *testing.T) {
	type testCase struct {
		name          string
		value         any
		variant       string
		want          any
		expectedError string
	}
	tests := []testCase{
		{
			name:  "convert string to base64 (default variant)",
			value: "test string",
			want:  "dGVzdCBzdHJpbmc=",
		},
		{
			name:  "convert string with newline to base64 (default variant)",
			value: "test string\n",
			want:  "dGVzdCBzdHJpbmcK",
		},
		{
			name:  "convert Value to base64 (default variant)",
			value: pcommon.NewValueStr("test string"),
			want:  "dGVzdCBzdHJpbmc=",
		},
		{
			name:    "base64 variant explicit",
			value:   "test string",
			variant: "base64",
			want:    "dGVzdCBzdHJpbmc=",
		},
		{
			name:    "base64 with url-safe sensitive characters",
			value:   "data+values/items",
			variant: "base64",
			want:    "ZGF0YSt2YWx1ZXMvaXRlbXM=",
		},
		{
			name:    "base64-raw with url-safe sensitive characters",
			value:   "data+values/items",
			variant: "base64-raw",
			want:    "ZGF0YSt2YWx1ZXMvaXRlbXM",
		},
		{
			name:    "base64-url with url-safe sensitive characters",
			value:   "data+values/items",
			variant: "base64-url",
			want:    "ZGF0YSt2YWx1ZXMvaXRlbXM=",
		},
		{
			name:    "base64-raw-url with url-safe sensitive characters",
			value:   "data+values/items",
			variant: "base64-raw-url",
			want:    "ZGF0YSt2YWx1ZXMvaXRlbXM",
		},
		{
			name:          "unsupported type int",
			value:         10,
			expectedError: "expected string but got int",
		},
		{
			name:          "unsupported type []byte",
			value:         []byte{0x00, 0x01, 0x02, 0xFF},
			expectedError: "expected string but got []uint8",
		},
		{
			name:          "unsupported type valueType invalid",
			value:         pcommon.NewValueEmpty(),
			expectedError: "expected string but got Empty",
		},
		{
			name:          "unsupported variant",
			value:         "test string",
			variant:       "invalid-variant",
			expectedError: "unsupported base64 variant: invalid-variant",
		},
		{
			name:  "empty string",
			value: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &Base64EncodeArguments[any]{
				Target: &ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return tt.value, nil
					},
				},
			}

			if tt.variant != "" {
				args.Variant = ottl.NewTestingOptional[ottl.StringGetter[any]](&ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return tt.variant, nil
					},
				})
			}

			expressionFunc, err := createBase64EncodeFunction[any](ottl.FunctionContext{}, args)
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
