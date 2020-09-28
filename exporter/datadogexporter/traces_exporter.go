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
	"github.com/DataDog/datadog-agent/pkg/trace/obfuscate"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type traceExporter struct {
	logger *zap.Logger
	cfg    *Config
	edgeConnection apm.TraceEdgeConnection
	obfuscator     *obfuscate.Obfuscator
}

func newTraceExporter(logger *zap.Logger, cfg *Config) (*traceExporter, error) {

	obfuscator := obfuscate.NewObfuscator(&obfuscate.Config{
		ES: obfuscate.JSONSettings{
			Enabled: true,
		},
		Mongo: obfuscate.JSONSettings{
			Enabled: true,
		},
		RemoveQueryString: true,
		RemovePathDigits:  true,
		RemoveStackTraces: true,
		Redis:             true,
		Memcached:         true,
	})	
	// TODO:
	// use passed in config values for site and api key instead of hardcoded
	exporter := &traceExporter{
		logger: logger,
		cfg: cfg,
		edgeConnection: apm.CreateTraceEdgeConnection("https://trace.agent.datadoghq.com", "<API_KEY>", false),
		obfuscator: obfuscator,
		}

	return exporter, nil
}

func (exp *traceExporter) pushTraceData(
	ctx context.Context,
	td pdata.Traces,
) (int, error) {
	// TODO: 
	// improve implementation of conversion
	ddTraces := convertToDatadogTd(td, exp.cfg)

	// security/obfuscation for db, query strings, stack traces, pii, etc
	// TODO: is there any config we want here? OTEL has their own pipeline for regex obfuscation
	apm.ObfuscatePayload(exp.obfuscator, ddTraces)
	
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
