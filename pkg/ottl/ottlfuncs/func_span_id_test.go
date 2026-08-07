// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
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

func Test_SpanIDFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewSpanIDFactory[any]()
		assert.Equal(t, "SpanID", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewSpanIDFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &SpanIDArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewSpanIDFactory[any]()
		args := factory.CreateDefaultArguments()
		spanIDArgs, ok := args.(*SpanIDArguments[any])
		require.True(t, ok)
		spanIDArgs.Target = &ottl.StandardByteSliceLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return []byte{1, 2, 3, 4, 5, 6, 7, 8}, nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createSpanIDFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "SpanIDFactory args must be of type *SpanIDArguments[K]")
	})
}
