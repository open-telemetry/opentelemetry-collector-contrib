// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	// http2xx is a special key that is represents a range from 200 to 299
	http2xx = "2xx"

	// http3xx is a special key that is represents a range from 300 to 399
	http3xx = "3xx"

	// http4xx is a special key that is represents a range from 400 to 499
	http4xx = "4xx"

	// http5xx is a special key that is represents a range from 500 to 599
	http5xx = "5xx"

	minKey = "min"
	maxKey = "max"
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
		return nil, errors.New("ParseSeverityFactory args must be of type *ParseSeverityArguments[K")
	}

	return parseSeverity[K](args.Target, args.Mapping), nil
}

func parseSeverity[K any](target ottl.Getter[K], mapping ottl.PMapGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		severityMap, err := mapping.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("cannot get severity mapping: %w", err)
		}

		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("could not get log level: %w", err)
		}

		logLevel, err := evaluateSeverity(value, severityMap.AsRaw())
		if err != nil {
			return nil, fmt.Errorf("could not map log level: %w", err)
		}

		return logLevel, nil
	}
}

func evaluateSeverity(value any, severities map[string]any) (string, error) {
	for level, criteria := range severities {
		criteriaList, ok := criteria.([]any)
		if !ok {
			return "", errors.New("criteria for mapping log level must be []any")
		}
		match, err := evaluateSeverityMapping(value, criteriaList)
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
		return evaluateSeverityNumberMapping(v, criteria)
	default:
		return false, fmt.Errorf("log level must be either string or int64, but got %T", v)
	}
}

func evaluateSeverityNumberMapping(value int64, criteria []any) (bool, error) {
	for _, crit := range criteria {
		// if we have a numeric severity number, we need to match with number ranges
		rangeMap, ok := crit.(map[string]any)
		if !ok {
			rangeMap, ok = parseValueRangePlaceholder(crit)
			if !ok {
				continue
			}
		}
		rangeMin, gotMin := rangeMap[minKey]
		rangeMax, gotMax := rangeMap[maxKey]
		if !gotMin || !gotMax {
			continue
		}
		rangeMinInt, ok := rangeMin.(int64)
		if !ok {
			return false, fmt.Errorf("min must be int64, but got %T", rangeMin)
		}
		rangeMaxInt, ok := rangeMax.(int64)
		if !ok {
			return false, fmt.Errorf("max must be int64, but got %T", rangeMax)
		}

		if rangeMinInt <= value && rangeMaxInt >= value {
			return true, nil
		}
	}
	return false, nil
}

func parseValueRangePlaceholder(crit any) (map[string]any, bool) {
	placeholder, ok := crit.(string)
	if !ok {
		return nil, false
	}

	switch placeholder {
	case http2xx:
		return map[string]any{
			"min": int64(200),
			"max": int64(299),
		}, true
	case http3xx:
		return map[string]any{
			"min": int64(300),
			"max": int64(399),
		}, true
	case http4xx:
		return map[string]any{
			"min": int64(400),
			"max": int64(499),
		}, true
	case http5xx:
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
			return false
		}
		if criteriaString == value {
			return true
		}
	}
	return false
}
