// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receivercreator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/antonmedv/expr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// evalBackticksInConfigValue expands any expressions within backticks inside configValue
// using variables from env.
//
// Note that when evaluating multiple expressions that the expanded result is always
// a string. For instance:
//
//   `true``false` -> "truefalse"
//
// However if there is only one expansion then the expanded result will keep the type
// of the expression. For instance:
//
//   `"secure" in pod.labels` -> true (boolean)
func evalBackticksInConfigValue(configValue string, env observer.EndpointEnv) (interface{}, error) {
	// Tracks index into configValue where an expression (backtick) begins. -1 is unset.
	exprStartIndex := -1
	// Accumulate expanded string.
	output := &strings.Builder{}
	// Accumulate results of calls to eval for use at the end to return well-typed
	// results if possible.
	var expansions []interface{}

	// Loop through configValue one rune at a time using exprStartIndex to keep track of
	// inside or outside of expressions.
	for i := 0; i < len(configValue); i++ {
		switch configValue[i] {
		case '\\':
			if i+1 == len(configValue) {
				return nil, errors.New(`encountered escape (\) without value at end of expression`)
			}
			output.WriteByte(configValue[i+1])
			i++
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

// expandMap recursively expands any expressions in backticks inside values of cfg using
// env as variables available within the expression, returning a copy of the map.
func expandMap(cfg map[string]interface{}, env observer.EndpointEnv) (map[string]interface{}, error) {
	resolved := map[string]interface{}{}
	for k, v := range cfg {
		if v == nil {
			continue
		}

		switch val := v.(type) {
		case map[string]interface{}:
			res, err := expandMap(val, env)
			if err != nil {
				return nil, err
			}
			resolved[k] = res
		case string:
			res, err := evalBackticksInConfigValue(val, env)
			if err != nil {
				return nil, fmt.Errorf("failed evaluating config expression for key %q: %v", k, err)
			}
			resolved[k] = res
		default:
			resolved[k] = v
		}
	}

	return resolved, nil
}
