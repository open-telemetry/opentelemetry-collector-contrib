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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllogs"
)

type LogsContext interface {
	ProcessLogs(td plog.Logs) error
}

var _ Context = &logStatements{}
var _ LogsContext = &logStatements{}

type logStatements []*ottl.Statement[ottllogs.TransformContext]

func (l logStatements) isContext() {}

func (l logStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				ctx := ottllogs.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource())
				for _, statement := range l {
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
