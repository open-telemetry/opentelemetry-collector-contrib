// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func Test_LogFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[ottllog.TransformContext]()
	actual := LogFunctions([]ottl.Factory[ottllog.TransformContext]{})
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}

type TestLogFuncArguments[K any] struct {
}

func NewTestLogFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestLogFunc", &TestLogFuncArguments[K]{}, createTestLogFunc[K])
}

func createTestLogFunc[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		return nil, nil
	}, nil
}

func Test_LogFunctions_AdditionalLogFuncs(t *testing.T) {
	testLogFuncFactory := NewTestLogFuncFactory[ottllog.TransformContext]()
	expected := ottlfuncs.StandardFuncs[ottllog.TransformContext]()
	expected[testLogFuncFactory.Name()] = testLogFuncFactory

	additionalLogFuncs := []ottl.Factory[ottllog.TransformContext]{testLogFuncFactory}
	actual := LogFunctions(additionalLogFuncs)

	require.Equal(t, len(expected), len(actual))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
