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

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter/internal/metadata"
)

const CfgTypeStr = "dataset"

// NewFactory created new factory with DataSet exporters.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		CfgTypeStr,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		MaxDelayMs:      maxDelayMs,
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}
}

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Logs, error) {
	cfg := config.(*Config)
	e, err := getDatasetExporter("logs", cfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot get DataSetExpoter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		config,
		e.consumeLogs,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
}

func createTracesExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Traces, error) {
	cfg := config.(*Config)
	e, err := getDatasetExporter("traces", cfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot get DataSetExpoter: %w", err)
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		config,
		e.consumeTraces,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
}
