// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const traceIDFuncName = "TraceID"

type TraceIDArguments[K any] struct {
	Target ottl.ByteSliceLikeGetter[K]
}

func NewTraceIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(traceIDFuncName, &TraceIDArguments[K]{}, createTraceIDFunction[K])
}

func createTraceIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TraceIDArguments[K])

	if !ok {
		return nil, errors.New("TraceIDFactory args must be of type *TraceIDArguments[K]")
	}

	return traceID[K](args.Target), nil
}

func traceID[K any](target ottl.ByteSliceLikeGetter[K]) ottl.ExprFunc[K] {
	return newIDExprFunc[K, pcommon.TraceID](traceIDFuncName, target)
}
