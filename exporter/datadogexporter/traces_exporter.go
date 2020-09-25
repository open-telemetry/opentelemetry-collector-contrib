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

package datadogexporter

import (
	"context"
	"fmt"

	apm "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/apm"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type traceExporter struct {
	logger *zap.Logger
}

func newTraceExporter(_ *Config, logger *zap.Logger) (*traceExporter, error) {
	return &traceExporter{logger}, nil
}

func (exp *traceExporter) pushTraceData(
	_ context.Context,
	td pdata.Traces,
) (int, error) {

	// TODO: improve implementation
	ddTraces := convertToDatadogTd(td)
	
	edgeConnection := apm.CreateTraceEdgeConnection("https://trace.agent.datadoghq.com", "<API_KEY>", false)

	for _, ddTrace := range ddTraces {

		err := edgeConnection.SendTraces(context.Background(), ddTrace, 3)

		if err != nil {
			fmt.Printf("Failed to send traces with error %v\n", err)
			
		}
		
		// TODO: logging for dev, remove later
		for _, trace := range ddTrace.Traces {
			for _, span := range trace.Spans {
				exp.logger.Info(fmt.Sprintf("%#v\n", span))
			}
		}
	}

	return 0, nil
}
