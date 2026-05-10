// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestDdStatusRemapper(t *testing.T) {
	var nilString *string
	var nilByteSlice *pcommon.ByteSlice
	var nilValue *pcommon.Value
	pcommonByteSlice := pcommon.NewByteSlice()
	pcommonByteSlice.Append([]byte("notice")...)
	pcommonValue := pcommon.NewValueStr("warning")
	pcommonValuePtr := pcommon.NewValueStr("debug")

	tests := []struct {
		name  string
		value any
		want  any
	}{
		{name: "nil value", value: nil, want: nil},
		{name: "nil string pointer", value: nilString, want: nil},
		{name: "nil byte slice pointer", value: nilByteSlice, want: nil},
		{name: "nil value pointer", value: nilValue, want: nil},
		{name: "level 0", value: "0", want: "emerg"},
		{name: "level 1", value: "1", want: "alert"},
		{name: "level 2", value: "2", want: "critical"},
		{name: "level 3", value: "3", want: "error"},
		{name: "level 4", value: "4", want: "warning"},
		{name: "level 5", value: "5", want: "notice"},
		{name: "level 6", value: "6", want: "info"},
		{name: "level 7", value: "7", want: "debug"},
		{name: "level 8 (default)", value: "8", want: "info"},
		{name: "emerg prefix", value: "emergency", want: "emerg"},
		{name: "f prefix", value: "fatal", want: "emerg"},
		{name: "a prefix", value: "alert", want: "alert"},
		{name: "c prefix", value: "critical", want: "critical"},
		{name: "c prefix capitalized", value: "Critical", want: "critical"},
		{name: "e prefix", value: "error", want: "error"},
		{name: "err prefix", value: "err", want: "error"},
		{name: "w prefix", value: "warning", want: "warning"},
		{name: "warn prefix", value: "warn", want: "warning"},
		{name: "n prefix", value: "notice", want: "notice"},
		{name: "i prefix", value: "info", want: "info"},
		{name: "information prefix", value: "information", want: "info"},
		{name: "d prefix", value: "debug", want: "debug"},
		{name: "trace prefix", value: "trace", want: "debug"},
		{name: "verbose prefix", value: "verbose", want: "debug"},
		{name: "exactly ok", value: "ok", want: "ok"},
		{name: "exactly success", value: "success", want: "ok"},
		{name: "offline is not ok", value: "offline", want: "info"},
		{name: "severe is not ok", value: "severe", want: "info"},
		{name: "empty string", value: "", want: "info"},
		{name: "unknown string", value: "unknown", want: "info"},
		{name: "byte slice text prefix", value: []byte("error"), want: "error"},
		{name: "pcommon byte slice text prefix", value: pcommonByteSlice, want: "notice"},
		{name: "pcommon value text prefix", value: pcommonValue, want: "warning"},
		{name: "pcommon value pointer text prefix", value: &pcommonValuePtr, want: "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createDdStatusRemapperFunction[any](
				ottl.FunctionContext{},
				&DdStatusRemapperArguments[any]{
					Target: &ottl.StandardGetSetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.value, nil
						},
					},
				},
			)

			require.NoError(t, err)

			result, err := expressionFunc(nil, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestDdStatusRemapperRequiresTarget(t *testing.T) {
	expressionFunc, err := createDdStatusRemapperFunction[any](
		ottl.FunctionContext{},
		&DdStatusRemapperArguments[any]{},
	)

	require.Error(t, err)
	require.Nil(t, expressionFunc)
}
