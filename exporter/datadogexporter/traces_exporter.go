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
	edgeConnection apm.TraceEdgeConnection
}

func newTraceExporter(_ *Config, logger *zap.Logger) (*traceExporter, error) {
	
	exporter := &traceExporter{
		logger: logger,
		edgeConnection: apm.CreateTraceEdgeConnection("https://trace.agent.datadoghq.com", "<API_KEY>", false),
		}

	return exporter, nil
}

func (exp *traceExporter) pushTraceData(
	_ context.Context,
	td pdata.Traces,
) (int, error) {

	// TODO: improve implementation
	ddTraces := convertToDatadogTd(td)
	
	// edgeConnection := 

	for _, ddTracePayload := range ddTraces {

		err := exp.edgeConnection.SendTraces(context.Background(), ddTracePayload, 3)

		if err != nil {
			fmt.Printf("Failed to send traces with error %v\n", err)
		}

		stats := apm.ComputeAPMStats(ddTracePayload)
		err_stats := exp.edgeConnection.SendStats(context.Background(), stats, 3)		

		if err_stats != nil {
			fmt.Printf("Failed to send trace stats with error %v\n", err_stats)
		}
		
		// TODO: logging for dev, remove later
		for _, trace := range ddTracePayload.Traces {
			for _, span := range trace.Spans {
				exp.logger.Info(fmt.Sprintf("%#v\n", span))
			}
		}
	}

	return 0, nil
}
