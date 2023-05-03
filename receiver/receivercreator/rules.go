// Copyright The OpenTelemetry Authors
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

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// rule wraps expr rule for later evaluation.
type rule struct {
	program *vm.Program
}

// ruleRe is used to verify the rule starts type check.
var ruleRe = regexp.MustCompile(
	fmt.Sprintf(`^type\s*==\s*(%q|%q|%q|%q|%q)`, observer.PodType, observer.PortType, observer.HostPortType, observer.ContainerType, observer.K8sNodeType),
)

// newRule creates a new rule instance.
func newRule(ruleStr string) (rule, error) {
	if ruleStr == "" {
		return rule{}, errors.New("rule cannot be empty")
	}
	if !ruleRe.MatchString(ruleStr) {
		// TODO: Try validating against bytecode instead.
		return rule{}, errors.New("rule must specify type")
	}

	// TODO: Maybe use https://godoc.org/github.com/antonmedv/expr#Env in type checking
	// depending on type == specified.
	v, err := expr.Compile(ruleStr)
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
