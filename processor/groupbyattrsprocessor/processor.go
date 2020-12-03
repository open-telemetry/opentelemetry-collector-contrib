// Copyright 2020 OpenTelemetry Authors
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

package groupbyattrsprocessor

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type groupbyattrsprocessor struct {
	logger      *zap.Logger
	groupByKeys []string
}

// ProcessTraces process traces and groups traces by attribute.
func (gap *groupbyattrsprocessor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	// TODO

	return td, nil
}

func (gap *groupbyattrsprocessor) ProcessLogs(_ context.Context, ld pdata.Logs) (pdata.Logs, error) {
	// TODO

	return ld, nil
}
