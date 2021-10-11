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

package filterexpr

import (
	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"go.opentelemetry.io/collector/model/pdata"
)

type LogMatcher struct {
	program *vm.Program
	v       vm.VM
}

// logEnv is a structure of variables and functions for expr language
type logEnv struct {
	Body           string
	Name           string
	SeverityNumber int32
	SeverityText   string
}

// NewLogMatcher creates new log matcher
func NewLogMatcher(expression string) (*LogMatcher, error) {
	program, err := expr.Compile(expression)
	if err != nil {
		return nil, err
	}
	return &LogMatcher{program: program, v: vm.VM{}}, nil
}

// MatchLog returns true if log matches the matcher
func (m *LogMatcher) MatchLog(log pdata.LogRecord) (bool, error) {
	matched, err := m.match(createLogEnv(log))
	if err != nil {
		return false, err
	}

	if matched {
		return true, nil
	}

	return false, nil
}

// createLogEnv converts pdata.LogRecord to logEnv
func createLogEnv(log pdata.LogRecord) logEnv {
	return logEnv{
		Name:           log.Name(),
		Body:           log.Body().AsString(),
		SeverityNumber: int32(log.SeverityNumber()),
		SeverityText:   log.SeverityText(),
	}
}

func (m *LogMatcher) match(env logEnv) (bool, error) {
	result, err := m.v.Run(m.program, env)
	if err != nil {
		return false, err
	}
	return result.(bool), nil
}
