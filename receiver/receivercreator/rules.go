// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"errors"
	"regexp"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/vm"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// rule wraps expr rule for later evaluation.
type rule struct {
	program *vm.Program
}

// ruleRe is used to verify the rule starts type check. It doesn't validate a fixed list of known
// `observer.EndpointType` because hard coding the valid values here prevents new types of
// `observer.EndpointDetails`, which carry the value pair with "type", from being created in
// downstream distributions.
var ruleRe = regexp.MustCompile(`^type\s*==\s*"([a-z0-9]+((\.|_)[a-z0-9]+)*)"`)

// newRule creates a new rule instance.
func newRule(ruleStr string) (rule, error) {
	if ruleStr == "" {
		return rule{}, errors.New("rule cannot be empty")
	}
	if !ruleRe.MatchString(ruleStr) {
		// TODO: Try validating against bytecode instead.
		return rule{}, errors.New("rule must specify type")
	}

	// TODO: Maybe use https://godoc.org/github.com/expr-lang/expr#Env in type checking
	// depending on type == specified.
	v, err := expr.Compile(
		ruleStr,
		// expr v1.14.1 introduced a `type` builtin whose implementation we relocate to `typeOf`
		// to avoid collision
		expr.DisableBuiltin("type"),
		expr.Function("typeOf", func(params ...any) (any, error) {
			return builtin.Type(params[0]), nil
		}, new(func(any) string)),
	)
	if err != nil {
		return rule{}, err
	}
	return rule{v}, nil
}

// eval the rule against the given endpoint.
func (r *rule) eval(env observer.EndpointEnv) (bool, error) {
	res, err := expr.Run(r.program, env)
	if err != nil {
		return false, err
	}
	if ret, ok := res.(bool); ok {
		return ret, nil
	}
	return false, errors.New("rule did not return a boolean")
}
