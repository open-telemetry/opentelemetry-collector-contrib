// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package processor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/processor"

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
)

var _ LogsContext = &LogStatements{}

type LogsContext interface {
	ProcessLogs(td plog.Logs) error
}

type LogStatements struct {
	statements []*ottl.Statement[ottllogs.TransformContext]
}

func (l *LogStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				ctx := ottllogs.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource())
				for _, statement := range l.statements {
					_, _, err := statement.Execute(ctx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

type LogParserCollection struct {
	parserCollection
	logParser ottl.Parser[ottllogs.TransformContext]
}

func NewLogParserCollection(functions map[string]interface{}, settings component.TelemetrySettings) LogParserCollection {
	return LogParserCollection{
		parserCollection: parserCollection{
			resourceParser: ottlresource.NewParser(common.ResourceFunctions(), settings),
			scopeParser:    ottlscope.NewParser(common.ScopeFunctions(), settings),
		},
		logParser: ottllogs.NewParser(functions, settings),
	}
}

func (pc LogParserCollection) ParseContextStatements(contextStatements []ContextStatements) ([]LogsContext, error) {
	contexts := make([]LogsContext, len(contextStatements))
	var errors error

	for i, s := range contextStatements {
		switch s.Context {
		case Resource:
			statements, err := pc.resourceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &ResourceStatements{
				Statements: statements,
			}
		case Scope:
			statements, err := pc.scopeParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &ScopeStatements{
				Statements: statements,
			}
		case Log:
			statements, err := pc.logParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &LogStatements{
				statements: statements,
			}
		default:
			errors = multierr.Append(errors, fmt.Errorf("context, %v, is not a valid context", s.Context))
		}
	}

	if errors != nil {
		return nil, errors
	}
	return contexts, nil
}
