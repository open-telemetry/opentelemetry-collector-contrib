// Copyright 2020, OpenTelemetry Authors
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

package splunkhecexporter

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr            = "splunk_hec"
	defaultMaxIdleCons = 100
	defaultHTTPTimeout = 10 * time.Second
)

// NewFactory creates a factory for Splunk HEC exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
		exporterhelper.WithMetrics(createMetricsExporter),
		exporterhelper.WithLogs(createLogsExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: defaultHTTPTimeout,
		},
		RetrySettings:        exporterhelper.DefaultRetrySettings(),
		QueueSettings:        exporterhelper.DefaultQueueSettings(),
		DisableCompression:   false,
		MaxConnections:       defaultMaxIdleCons,
		MaxContentLengthLogs: maxContentLengthLogsLimit,
	}
}

func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.TracesExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	expCfg := config.(*Config)

	exp, err := createExporter(expCfg, set.Logger, &set.BuildInfo)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		expCfg,
		set,
		exp.pushTraceData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.RetrySettings),
		exporterhelper.WithQueue(expCfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop))
}

func createMetricsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	expCfg := config.(*Config)

	exp, err := createExporter(expCfg, set.Logger, &set.BuildInfo)

	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		expCfg,
		set,
		exp.pushMetricsData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.RetrySettings),
		exporterhelper.WithQueue(expCfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop))
}

func createLogsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (exporter component.LogsExporter, err error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	expCfg := config.(*Config)

	exp, err := createExporter(expCfg, set.Logger, &set.BuildInfo)

	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		expCfg,
		set,
		exp.pushLogData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.RetrySettings),
		exporterhelper.WithQueue(expCfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop))
}
