// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type TestSpanFuncArguments[K any] struct{}

func NewTestSpanFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestSpanFunc", &TestSpanFuncArguments[K]{}, createTestSpanFunc[K])
}

func createTestSpanFunc[K any](_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(_ context.Context, _ K) (any, error) {
		return nil, nil
	}, nil
}

type TestSpanEventFuncArguments[K any] struct{}

func NewTestSpanEventFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestSpanEventFunc", &TestSpanEventFuncArguments[K]{}, createTestSpanEventFunc[K])
}

func createTestSpanEventFunc[K any](_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(_ context.Context, _ K) (any, error) {
		return nil, nil
	}, nil
}

func Test_SpanFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[ottlspan.TransformContext]()
	isRootSpanFactory := ottlfuncs.NewIsRootSpanFactory()
	expected[isRootSpanFactory.Name()] = isRootSpanFactory
	actual := SpanFunctions()
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}

func Test_SpanFunctions_AdditionalSpanFuncs(t *testing.T) {
	testSpanFuncFactory := NewTestSpanFuncFactory[ottlspan.TransformContext]()

	expected := ottlfuncs.StandardFuncs[ottlspan.TransformContext]()
	expected[testSpanFuncFactory.Name()] = testSpanFuncFactory
	isRootSpanFactory := ottlfuncs.NewIsRootSpanFactory()
	expected[isRootSpanFactory.Name()] = isRootSpanFactory

	additionalSpanFuncs := []ottl.Factory[ottlspan.TransformContext]{testSpanFuncFactory}
	actual := SpanFunctions(additionalSpanFuncs...)

	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}

func Test_SpanEventFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[ottlspanevent.TransformContext]()
	actual := SpanEventFunctions()
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}

func Test_SpanEventFunctions_AdditionalSpanEventFuncs(t *testing.T) {
	testSpanEventFuncFactory := NewTestSpanEventFuncFactory[ottlspanevent.TransformContext]()

	expected := ottlfuncs.StandardFuncs[ottlspanevent.TransformContext]()
	expected[testSpanEventFuncFactory.Name()] = testSpanEventFuncFactory

	additionalSpanEventFuncs := []ottl.Factory[ottlspanevent.TransformContext]{testSpanEventFuncFactory}
	actual := SpanEventFunctions(additionalSpanEventFuncs...)
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
