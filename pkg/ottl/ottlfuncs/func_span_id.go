// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SpanIDArguments[K any] struct {
	Target ottl.ByteSliceLikeGetter[K]
}

func NewSpanIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SpanID", &SpanIDArguments[K]{}, createSpanIDFunction[K])
}

func createSpanIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SpanIDArguments[K])

	if !ok {
		return nil, errors.New("SpanIDFactory args must be of type *SpanIDArguments[K]")
	}

	return spanID[K](args.Target), nil
}

func spanID[K any](target ottl.ByteSliceLikeGetter[K]) ottl.ExprFunc[K] {
	return newIDExprFunc[K, pcommon.SpanID](target)
}
