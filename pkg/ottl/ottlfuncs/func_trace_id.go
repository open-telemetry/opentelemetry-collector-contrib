// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type TraceIDArguments[K any] struct {
	Target ottl.ByteSliceLikeGetter[K]
}

func NewTraceIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TraceID", &TraceIDArguments[K]{}, createTraceIDFunction[K])
}

func createTraceIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TraceIDArguments[K])

	if !ok {
		return nil, errors.New("TraceIDFactory args must be of type *TraceIDArguments[K]")
	}

	return traceID[K](args.Target), nil
}

var (
	errTraceIDInvalidLength = errors.New("invalid trace id length")
	errTraceIDHexDecode     = errors.New("invalid trace id hex")
)

func traceID[K any](target ottl.ByteSliceLikeGetter[K]) ottl.ExprFunc[K] {
	const traceIDLen = len(pcommon.TraceID{})
	const traceIDHexLen = traceIDLen * 2

	return func(ctx context.Context, tCtx K) (any, error) {
		b, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		var idArr pcommon.TraceID
		switch len(b) {
		case traceIDLen:
			// Handle 16 bytes (binary encoded trace ID)
			copy(idArr[:], b)
		case traceIDHexLen:
			// Handle 32 bytes (string hex encoded trace ID)
			_, err := hex.Decode(idArr[:], b)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errTraceIDHexDecode, err)
			}
			return idArr, nil
		default:
			return nil, fmt.Errorf("%w: expected %d or %d bytes, got %d", errTraceIDInvalidLength, traceIDLen, traceIDHexLen, len(b))
		}
		return idArr, nil
	}
}
