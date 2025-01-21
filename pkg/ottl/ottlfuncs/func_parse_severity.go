// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/go-viper/mapstructure/v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	// hTTP2xx is a special key that is represents a range from 200 to 299
	hTTP2xx = "2xx"

	// hTTP3xx is a special key that is represents a range from 300 to 399
	hTTP3xx = "3xx"

	// hTTP4xx is a special key that is represents a range from 400 to 499
	hTTP4xx = "4xx"

	// hTTP5xx is a special key that is represents a range from 500 to 599
	hTTP5xx = "5xx"
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

	return parseSeverity[K](args.Target, args.Mapping), nil
}

func parseSeverity[K any](target ottl.Getter[K], mapping ottl.PMapGetter[K]) ottl.ExprFunc[K] {
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

		logLevel, err := evaluateSeverity(value, sev)
		if err != nil {
			return nil, fmt.Errorf("could not map log level: %w", err)
		}

		return logLevel, nil
	}
}

func validateSeverity(raw map[string]any) (map[string][]any, error) {
	s := map[string][]any{}
	if err := mapstructure.Decode(raw, &s); err != nil {
		return nil, fmt.Errorf("cannot decode severity mapping: %w", err)
	}

	return s, nil
}

func evaluateSeverity(value any, severities map[string][]any) (string, error) {
	for level, criteria := range severities {
		match, err := evaluateSeverityMapping(value, criteria)
		if err != nil {
			return "", fmt.Errorf("could not evaluate log level of value '%v': %w", value, err)
		}
		if match {
			return level, nil
		}
	}
	return "", fmt.Errorf("no matching log level found for value '%v'", value)
}

func evaluateSeverityMapping(value any, criteria []any) (bool, error) {
	switch v := value.(type) {
	case string:
		return evaluateSeverityStringMapping(v, criteria), nil
	case int64:
		return evaluateSeverityNumberMapping(v, criteria), nil
	default:
		return false, fmt.Errorf("log level must be either string or int64, but got %T", v)
	}
}

func evaluateSeverityNumberMapping(value int64, criteria []any) bool {
	for _, crit := range criteria {
		// if we have a numeric severity number, we need to match with number ranges
		rangeMap, ok := crit.(map[string]any)
		if !ok {
			rangeMap, ok = parseValueRangePlaceholder(crit)
			if !ok {
				continue
			}
		}
		rangeMin, gotMin := rangeMap["min"]
		rangeMax, gotMax := rangeMap["max"]
		if !gotMin || !gotMax {
			continue
		}
		rangeMinInt, ok := rangeMin.(int64)
		if !ok {
			continue
		}
		rangeMaxInt, ok := rangeMax.(int64)
		if !ok {
			continue
		}
		// TODO should we error if the range object does not contain the expected keys/types, or just proceed with checking the other criteria?
		if rangeMinInt <= value && rangeMaxInt >= value {
			return true
		}
	}
	return false
}

func parseValueRangePlaceholder(crit any) (map[string]any, bool) {
	placeholder, ok := crit.(string)
	if !ok {
		return nil, false
	}

	switch placeholder {
	case hTTP2xx:
		return map[string]any{
			"min": int64(200),
			"max": int64(299),
		}, true
	case hTTP3xx:
		return map[string]any{
			"min": int64(300),
			"max": int64(399),
		}, true
	case hTTP4xx:
		return map[string]any{
			"min": int64(400),
			"max": int64(499),
		}, true
	case hTTP5xx:
		return map[string]any{
			"min": int64(500),
			"max": int64(599),
		}, true
	default:
		return nil, false
	}
}

func evaluateSeverityStringMapping(value string, criteria []any) bool {
	for _, crit := range criteria {
		// if we have a severity string, we need to match with string mappings
		criteriaString, ok := crit.(string)
		if !ok {
			continue
		}
		if criteriaString == value {
			return true
		}
	}
	return false
}
