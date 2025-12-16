// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Test_spanID(t *testing.T) {
	runIDSuccessTests(t, spanID[any], []idSuccessTestCase{
		{
			name:  "create span id from 8 bytes",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8},
			want:  pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
		},
		{
			name:  "create span id from 16 hex chars",
			value: []byte("0102030405060708"),
			want:  pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
		},
	})
}

func Test_spanID_validation(t *testing.T) {
	runIDErrorTests(t, spanID[any], spanIDFuncName, []idErrorTestCase{
		{
			name:  "byte slice less than 8 (7)",
			value: []byte{1, 2, 3, 4, 5, 6, 7},
			err:   errIDInvalidLength,
		},
		{
			name:  "byte slice longer than 8 (9)",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
			err:   errIDInvalidLength,
		},
		{
			name:  "byte slice longer than 16 (17)",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
			err:   errIDInvalidLength,
		},
		{
			name:  "invalid hex string",
			value: []byte("ZZ02030405060708"),
			err:   errIDHexDecode,
		},
	})
}
