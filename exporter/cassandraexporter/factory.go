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

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter/internal/metadata"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		DSN:        "127.0.0.1",
		Keyspace:   "otel",
		TraceTable: "otel_spans",
		LogsTable:  "otel_logs",
		Replication: Replication{
			Class:             "SimpleStrategy",
			ReplicationFactor: 1,
		},
		Compression: Compression{
			Algorithm: "LZ4Compressor",
		},
	}
}

func createTracesExporter(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	exporter, err := newTracesExporter(set.Logger, c)

	if err != nil {
		return nil, fmt.Errorf("cannot configure cassandra traces exporter: %w", err)
	}

	return exporterhelper.NewTracesExporter(ctx, set, cfg, exporter.pushTraceData, exporterhelper.WithShutdown(exporter.Shutdown), exporterhelper.WithStart(exporter.Start))
}

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (exporter.Logs, error) {
	c := cfg.(*Config)
	exporter, err := newLogsExporter(set.Logger, c)

	if err != nil {
		return nil, fmt.Errorf("cannot configure cassandra traces exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(ctx, set, cfg, exporter.pushLogsData, exporterhelper.WithShutdown(exporter.Shutdown), exporterhelper.WithStart(exporter.Start))
}
