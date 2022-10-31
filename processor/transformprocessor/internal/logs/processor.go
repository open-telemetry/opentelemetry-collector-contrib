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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
)

type Processor struct {
	contexts []common.LogsContext
}

func NewProcessor(statements []common.ContextStatements, functions map[string]interface{}, settings component.TelemetrySettings) (*Processor, error) {
	pc := common.NewLogParserCollection(Functions(), settings)
	contexts, err := pc.ParseContextStatements(statements)
	if err != nil {
		return nil, err
	}
	return &Processor{
		contexts: contexts,
	}, nil
}

func (p *Processor) ProcessLogs(_ context.Context, td plog.Logs) (plog.Logs, error) {
	for _, contexts := range p.contexts {
		contexts.ProcessLogs(td)
	}
	return td, nil
}
