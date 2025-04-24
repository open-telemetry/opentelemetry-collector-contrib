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

func Test_SpanFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[ottlspan.TransformContext]()
	isRootSpanFactory := ottlfuncs.NewIsRootSpanFactory()
	expected[isRootSpanFactory.Name()] = isRootSpanFactory
	actual := SpanFunctions([]ottl.Factory[ottlspan.TransformContext]{})
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}

type TestSpanFuncArguments[K any] struct {
}

func NewTestSpanFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ExtractPatternsRubyRegex", &TestSpanFuncArguments[K]{}, createTestSpanFunc[K])
}

func createTestSpanFunc[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		return nil, nil
	}, nil
}

func Test_SpanFunctions_AdditionalSpanFuncs(t *testing.T) {
	testSpanFuncFactory := NewTestSpanFuncFactory[ottlspan.TransformContext]()

	expected := ottlfuncs.StandardFuncs[ottlspan.TransformContext]()
	expected[testSpanFuncFactory.Name()] = testSpanFuncFactory
	isRootSpanFactory := ottlfuncs.NewIsRootSpanFactory()
	expected[isRootSpanFactory.Name()] = isRootSpanFactory

	additionalSpanFuncs := []ottl.Factory[ottlspan.TransformContext]{testSpanFuncFactory}
	actual := SpanFunctions(additionalSpanFuncs)

	require.Equal(t, len(expected), len(actual))
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
