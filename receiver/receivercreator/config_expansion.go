// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"errors"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// evalBackticksInConfigValue expands any expressions within backticks inside configValue
// using variables from env.
//
// Note that when evaluating multiple expressions that the expanded result is always
// a string. For instance:
//
//	`true``false` -> "truefalse"
//
// However if there is only one expansion then the expanded result will keep the type
// of the expression. For instance:
//
//	`"secure" in pod.labels` -> true (boolean)
func evalBackticksInConfigValue(configValue string, env observer.EndpointEnv) (any, error) {
	// Tracks index into configValue where an expression (backtick) begins. -1 is unset.
	exprStartIndex := -1
	// Accumulate expanded string.
	output := &strings.Builder{}
	// Accumulate results of calls to eval for use at the end to return well-typed
	// results if possible.
	var expansions []any

	// Loop through configValue one rune at a time using exprStartIndex to keep track of
	// inside or outside of expressions.
	for i := 0; i < len(configValue); i++ {
		switch configValue[i] {
		case '\\':
			if i+1 == len(configValue) {
				return nil, errors.New(`encountered escape (\) without value at end of expression`)
			}
			if configValue[i+1] != '`' {
				if exprStartIndex == -1 {
					output.WriteByte(configValue[i])
				}
			} else {
				output.WriteByte(configValue[i+1])
				i++
			}
		case '`':
			if exprStartIndex == -1 {
				// Opening backtick encountered, expression starts one after current index.
				exprStartIndex = i + 1
			} else {
				// Closing backtick encountered, evaluate the expression.
				exprText := configValue[exprStartIndex:i]

				// If expression has no text inside it return an error.
				if strings.TrimSpace(exprText) == "" {
					return nil, errors.New("expression is empty")
				}
				res, err := expr.Eval(exprText, env)
				if err != nil {
					return nil, err
				}
				expansions = append(expansions, res)
				_, _ = fmt.Fprintf(output, "%v", res)

				// Reset start index since this expression just closed.
				exprStartIndex = -1
			}
		default:
			if exprStartIndex == -1 {
				output.WriteByte(configValue[i])
			}
		}
	}

	// Should always be a closing backtick if it's balanced.
	if exprStartIndex != -1 {
		return nil, fmt.Errorf("expression was unbalanced starting at character %d", exprStartIndex)
	}

	// If there was only one expansion and it is equal to the full output string return the expansion
	// itself so that it retains the type returned by the expression. Might be a bool, int, etc.
	// instead of a string.
	if len(expansions) == 1 && output.String() == fmt.Sprintf("%v", expansions[0]) {
		return expansions[0], nil
	}

	return output.String(), nil
}

// expandConfig will walk the provided user config and expand any `backticked` content
// with associated observer.EndpointEnv values.
func expandConfig(cfg userConfigMap, env observer.EndpointEnv) (userConfigMap, error) {
	expanded, err := expandAny(map[string]any(cfg), env)
	if err != nil {
		return nil, err
	}
	return expanded.(map[string]any), nil
}

// expandAny recursively expands any expressions in backticks inside values of input using
// env as variables available within the expression, returning a copy of input
func expandAny(input any, env observer.EndpointEnv) (any, error) {
	switch v := input.(type) {
	case string:
		res, err := evalBackticksInConfigValue(v, env)
		if err != nil {
			return nil, fmt.Errorf("failed evaluating config expression for %v: %w", v, err)
		}
		return res, nil
	case []string, []any:
		var vSlice []any
		if vss, ok := v.([]string); ok {
			// expanded strings aren't guaranteed to remain them, so we
			// coerce to any for shared []any expansion path
			for _, vs := range vss {
				vSlice = append(vSlice, vs)
			}
		} else {
			vSlice = v.([]any)
		}
		expandedSlice := make([]any, 0, len(vSlice))
		for _, val := range vSlice {
			expanded, err := expandAny(val, env)
			if err != nil {
				return nil, fmt.Errorf("failed evaluating config expression for %v: %w", val, err)
			}
			expandedSlice = append(expandedSlice, expanded)
		}
		return expandedSlice, nil
	case map[string]any:
		expandedMap := map[string]any{}
		for key, val := range v {
			expandedVal, err := expandAny(val, env)
			if err != nil {
				return nil, fmt.Errorf("failed evaluating config expression for {%q: %v}: %w", key, val, err)
			}
			expandedMap[key] = expandedVal
		}
		return expandedMap, nil
	default:
		return v, nil
	}
}
