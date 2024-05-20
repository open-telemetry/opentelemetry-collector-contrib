package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type YearArguments[K any] struct {
	Time ottl.TimeGetter[K]
}

func NewYearFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Year", &YearArguments[K]{}, createYearFunction[K])
}

func createYearFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*YearArguments[K])

	if !ok {
		return nil, fmt.Errorf("YearFactory args must be of type *YearArguments[K]")
	}

	return Year(args.Time)
}

func Year[K any](duration ottl.TimeGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return int64(d.Year()), nil
	}, nil
}
