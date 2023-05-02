// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jqprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/jqprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type jqProcessor struct {
	logger      *zap.Logger
	jqStatement string `mapstructure:"jq_statement"`
}

func (jq *jqProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {

	if ld.ResourceLogs().Len() > 0 {
		fmt.Printf("do some jq magic with %s", jq.jqStatement)

		return ld, nil
	}
	return ld, nil
}
