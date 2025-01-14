package ottlfuncs

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ParseSeverityArguments[K any] struct {
	Target  ottl.StringGetter[K]
	Mapping ottl.PMapGetter[K]
}

func NewParseSeverityFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseSeverity", &ParseSeverityArguments[K]{}, createParseSeverityFunction[K])
}

func createParseSeverityFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseSeverityArguments[K])

	if !ok {
		return nil, fmt.Errorf("ParseSeverityFactory args must be of type *ParseSeverityArguments[K")
	}

	return parseSeverity[K](args.Target, args.Mapping)
}

func parseSeverity[K any](target ottl.StringGetter[K], mapping ottl.PMapGetter[K]) (ottl.ExprFunc[K], error) {

	return func(ctx context.Context, tCtx K) (any, error) {
		return nil, nil
	}, nil
}
