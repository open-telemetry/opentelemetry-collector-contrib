// Copyright  The OpenTelemetry Authors
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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqllogs"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

type Processor struct {
	queries []tql.Query
	logger  *zap.Logger
}

func NewProcessor(statements []string, functions map[string]interface{}, settings component.ProcessorCreateSettings) (*Processor, error) {
	queries, err := tql.ParseQueries(statements, functions, tqllogs.ParsePath, tqllogs.ParseEnum)
	if err != nil {
		return nil, err
	}
	return &Processor{
		queries: queries,
		logger:  settings.Logger,
	}, nil
}

func (p *Processor) ProcessLogs(_ context.Context, td plog.Logs) (plog.Logs, error) {
	ctx := tqllogs.LogTransformContext{}
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		ctx.Resource = rlogs.Resource()
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			ctx.InstrumentationScope = slogs.Scope()
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				ctx.Log = log

				for _, statement := range p.queries {
					if statement.Condition(ctx) {
						statement.Function(ctx)
					}
				}
			}
		}
	}
	return td, nil
}
