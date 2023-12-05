// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

const (
	// The value of "type" key in configuration.
	defaultLogsIndex   = "logs-generic-default"
	defaultTracesIndex = "traces-generic-default"
)

// NewFactory creates a factory for Elastic exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false
	return &Config{
		QueueSettings: qs,
		HTTPClientSettings: HTTPClientSettings{
			Timeout: 90 * time.Second,
		},
		Index:       "",
		LogsIndex:   defaultLogsIndex,
		TracesIndex: defaultTracesIndex,
		Retry: RetrySettings{
			Enabled:         true,
			MaxRequests:     3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Minute,
		},
		Mapping: MappingsSettings{
			Mode:  "ecs",
			Dedup: true,
			Dedot: true,
		},
		LogstashFormat: LogstashFormatSettings{
			Enabled:         false,
			PrefixSeparator: "-",
			DateFormat:      "%Y.%m.%d",
		},
	}
}

// createLogsExporter creates a new exporter for logs.
//
// Logs are directly indexed into Elasticsearch.
func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	cf := cfg.(*Config)
	if cf.Index != "" {
		set.Logger.Warn("index option are deprecated and replaced with logs_index and traces_index.")
	}

	logsExporter, err := newLogsExporter(set.Logger, cf)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch logsExporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		logsExporter.pushLogsData,
		exporterhelper.WithShutdown(logsExporter.Shutdown),
		exporterhelper.WithQueue(cf.QueueSettings),
	)
}

func createTracesExporter(ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Traces, error) {

	cf := cfg.(*Config)
	tracesExporter, err := newTracesExporter(set.Logger, cf)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch tracesExporter: %w", err)
	}
	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		tracesExporter.pushTraceData,
		exporterhelper.WithShutdown(tracesExporter.Shutdown),
		exporterhelper.WithQueue(cf.QueueSettings))
}
