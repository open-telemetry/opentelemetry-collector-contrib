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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type Processor struct {
	contexts []consumer.Logs
	// Deprecated.  Use contexts instead
	statements []*ottl.Statement[ottllog.TransformContext]
}

func NewProcessor(statements []string, contextStatements []common.ContextStatements, settings component.TelemetrySettings) (*Processor, error) {
	if len(statements) > 0 {
		ottlp := ottllog.NewParser(LogFunctions(), settings)
		parsedStatements, err := ottlp.ParseStatements(statements)
		if err != nil {
			return nil, err
		}
		return &Processor{
			statements: parsedStatements,
		}, nil
	}

	pc, err := common.NewLogParserCollection(settings, common.WithLogParser(LogFunctions()))
	if err != nil {
		return nil, err
	}

	contexts := make([]consumer.Logs, len(contextStatements))
	for i, cs := range contextStatements {
		context, err := pc.ParseContextStatements(cs)
		if err != nil {
			return nil, err
		}
		contexts[i] = context
	}

	return &Processor{
		contexts: contexts,
	}, nil
}

func (p *Processor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	if len(p.statements) > 0 {
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			rlogs := ld.ResourceLogs().At(i)
			for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
				slogs := rlogs.ScopeLogs().At(j)
				logs := slogs.LogRecords()
				for k := 0; k < logs.Len(); k++ {
					tCtx := ottllog.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource())
					for _, statement := range p.statements {
						_, _, err := statement.Execute(ctx, tCtx)
						if err != nil {
							return ld, err
						}
					}
				}
			}
		}
	} else {
		for _, c := range p.contexts {
			err := c.ConsumeLogs(ctx, ld)
			if err != nil {
				return ld, err
			}
		}
	}
	return ld, nil
}
