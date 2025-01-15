package ottlfuncs

import (
	"context"
	"fmt"
	"github.com/go-viper/mapstructure/v2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ParseSeverityArguments[K any] struct {
	Target  ottl.Getter[K]
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

func parseSeverity[K any](target ottl.Getter[K], mapping ottl.PMapGetter[K]) (ottl.ExprFunc[K], error) {

	return func(ctx context.Context, tCtx K) (any, error) {
		severityMap, err := mapping.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("cannot get severity mapping: %w", err)
		}

		sev, err := validateSeverity(severityMap.AsRaw())
		if err != nil {
			return nil, fmt.Errorf("invalid severity mapping: %w", err)
		}

		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("could not get log level: %w", err)
		}

		logLevel, err := sev.evaluate(value)
		if err != nil {
			return nil, fmt.Errorf("could not map log level: %w", err)
		}

		return logLevel, nil
	}, nil
}

func validateSeverity(raw map[string]any) (*severities, error) {
	s := &severities{}
	if err := mapstructure.Decode(raw, s); err != nil {
		return nil, fmt.Errorf("cannot decode severity mapping: %w", err)
	}

	return s, nil

}

type severities struct {
	Debug *severity `mapstructure:"debug,omitempty"`
	Fatal *severity `mapstructure:"fatal,omitempty"`
	Error *severity `mapstructure:"error,omitempty"`
	Warn  *severity `mapstructure:"warn,omitempty"`
	Info  *severity `mapstructure:"info,omitempty"`
}

func (s severities) evaluate(value any) (string, error) {
	if s.Debug != nil {
		match, err := s.Debug.matches(value)
		if err != nil {
			return "", fmt.Errorf("could not evaluate log level of value '%v': %w", value, err)
		}
		if match {
			return "debug", nil
		}
	}
	if s.Fatal != nil {
		match, err := s.Fatal.matches(value)
		if err != nil {
			return "", fmt.Errorf("could not evaluate log level of value '%v': %w", value, err)
		}
		if match {
			return "fatal", nil
		}
	}
	if s.Error != nil {
		match, err := s.Error.matches(value)
		if err != nil {
			return "", fmt.Errorf("could not evaluate log level of value '%v': %w", value, err)
		}
		if match {
			return "error", nil
		}
	}
	if s.Warn != nil {
		match, err := s.Warn.matches(value)
		if err != nil {
			return "", fmt.Errorf("could not evaluate log level of value '%v': %w", value, err)
		}
		if match {
			return "warn", nil
		}
	}
	if s.Info != nil {
		match, err := s.Info.matches(value)
		if err != nil {
			return "", fmt.Errorf("could not evaluate log level of value '%v': %w", value, err)
		}
		if match {
			return "info", nil
		}
	}
	return "", fmt.Errorf("no matching log level found for value '%v'", value)
}

type severity struct {
	Range   *severityRange `mapstructure:"range,omitempty"`
	MapFrom []string       `mapstructure:"mapFrom,omitempty"`
}

func (s severity) matches(value any) (bool, error) {
	switch v := value.(type) {
	case string:
		for _, mappedLevel := range s.MapFrom {
			if mappedLevel == v {
				return true, nil
			}
		}
	case int64:
		return s.Range.Min <= v && s.Range.Max >= v, nil
	default:
		return false, fmt.Errorf("log level must be either string or int")
	}
	return false, nil
}

type severityRange struct {
	Min int64 `mapstructure:"min"`
	Max int64 `mapstructure:"max"`
}
