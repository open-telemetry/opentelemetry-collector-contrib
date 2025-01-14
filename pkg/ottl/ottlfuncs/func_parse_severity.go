package ottlfuncs

import (
	"context"
	"fmt"
	"github.com/go-viper/mapstructure/v2"
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
		severityMap, err := mapping.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("cannot get severity mapping: %w", err)
		}

		sev, err := validateSeverity(severityMap.AsRaw())
		if err != nil {
			return nil, fmt.Errorf("invalid severity mapping: %w", err)
		}
		return nil, nil
	}, nil
}

func validateSeverity(raw map[string]any) (map[string]any, error) {
	s := &severities{}
	if err := mapstructure.Decode(raw, s); err != nil {
		return nil, fmt.Errorf("cannot decode severity mapping: %w", err)
	}

}

type severities struct {
	Debug *severity `mapstructure:"debug,omitempty"`
	Fatal *severity `mapstructure:"fatal,omitempty"`
	Error *severity `mapstructure:"error,omitempty"`
	Warn  *severity `mapstructure:"warn,omitempty"`
	Info  *severity `mapstructure:"info,omitempty"`
}

type severity struct {
	Range   *severityRange `mapstructure:"range,omitempty"`
	MapFrom []string       `mapstructure:"mapFrom,omitempty"`
}

type severityRange struct {
	Min int `mapstructure:"min"`
	Max int `mapstructure:"max"`
}
