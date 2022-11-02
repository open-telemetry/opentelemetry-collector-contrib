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

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type Processor struct {
	contexts []common.Context
	// Deprecated.  Use contexts instead
	statements []*ottl.Statement[ottllogs.TransformContext]
}

func NewProcessor(statements []string, contextStatements []common.ContextStatements, settings component.TelemetrySettings) (*Processor, error) {
	if len(statements) > 0 {
		ottlp := ottllogs.NewParser(Functions(), settings)
		parsedStatements, err := ottlp.ParseStatements(statements)
		if err != nil {
			return nil, err
		}
		return &Processor{
			statements: parsedStatements,
		}, nil
	}
	pc := common.NewParserCollection(settings, common.WithLogParser(Functions()))
	contexts, err := pc.ParseContextStatements(contextStatements)
	if err != nil {
		return nil, err
	}
	return &Processor{
		contexts: contexts,
	}, nil
}

func (p *Processor) ProcessLogs(_ context.Context, td plog.Logs) (plog.Logs, error) {
	if len(p.statements) > 0 {
		for i := 0; i < td.ResourceLogs().Len(); i++ {
			rlogs := td.ResourceLogs().At(i)
			for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
				slogs := rlogs.ScopeLogs().At(j)
				logs := slogs.LogRecords()
				for k := 0; k < logs.Len(); k++ {
					ctx := ottllogs.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource())
					for _, statement := range p.statements {
						statement.Execute(ctx)
					}
				}
			}
		}
	} else {
		for _, contexts := range p.contexts {
			contexts.(common.LogsContext).ProcessLogs(td)
		}
	}
	return td, nil
}
