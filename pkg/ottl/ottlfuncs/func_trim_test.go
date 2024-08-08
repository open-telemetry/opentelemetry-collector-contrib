// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_trim(t *testing.T) {
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(_ context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Str(), nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
	}

	tests := []struct {
		name          string
		scenarioValue string
		target        ottl.GetSetter[pcommon.Value]
		cutset        ottl.Optional[string]
		wanted        func(pcommon.Value)
	}{
		{
			name:          "empty string",
			target:        target,
			scenarioValue: "",
			cutset:        ottl.Optional[string]{},
			wanted: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("")
			},
		},
		{
			name:          "empty string with symbols",
			target:        target,
			scenarioValue: "",
			cutset:        ottl.NewTestingOptional[string]("tg"),
			wanted: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("")
			},
		},
		{
			name:          "no trim string",
			target:        target,
			scenarioValue: "trim string",
			cutset:        ottl.Optional[string]{},
			wanted: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("trim string")
			},
		},
		{
			name:          "trim string with spaces",
			target:        target,
			scenarioValue: "   trim string   ",
			cutset:        ottl.Optional[string]{},
			wanted: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("trim string")
			},
		},
		{
			name:          "trim string with unicode",
			target:        target,
			scenarioValue: "\u0009trim string\u0020\u2005",
			cutset:        ottl.Optional[string]{},
			wanted: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("trim string")
			},
		},
		{
			name:          "trim string with symbols",
			target:        target,
			scenarioValue: "trim string",
			cutset:        ottl.NewTestingOptional[string]("tg"),
			wanted: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("rim strin")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := trim(tt.target, tt.cutset)
			val := pcommon.NewValueStr(tt.scenarioValue)
			result, err := exprFunc(nil, val)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr("")
			tt.wanted(expected)

			assert.Equal(t, expected, val)
		})
	}
}

func Test_trim_nil_NoError(t *testing.T) {
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(_ context.Context, _ pcommon.Value) (any, error) {
			return nil, nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
	}

	val := pcommon.NewValueEmpty()
	exprFunc := trim(target, ottl.Optional[string]{})
	_, err := exprFunc(nil, val)
	assert.NoError(t, err)
}

func Test_trim_invalid_type_NoError(t *testing.T) {
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(_ context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Int(), nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
	}

	val := pcommon.NewValueInt(3)
	exprFunc := trim(target, ottl.Optional[string]{})
	_, err := exprFunc(nil, val)
	assert.NoError(t, err)
}

func Test_trim_invalid_type_with_arg_NoError(t *testing.T) {
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(_ context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Int(), nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
	}

	val := pcommon.NewValueInt(3)
	exprFunc := trim(target, ottl.NewTestingOptional[string](","))
	_, err := exprFunc(nil, val)
	assert.NoError(t, err)
}
