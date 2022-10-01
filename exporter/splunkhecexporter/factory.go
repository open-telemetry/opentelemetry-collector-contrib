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

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
)

const (
	// The value of "type" key in configuration.
	typeStr = "splunk_hec"
	// The stability level of the exporter.
	stability          = component.StabilityLevelBeta
	defaultMaxIdleCons = 100
	defaultHTTPTimeout = 10 * time.Second
)

// TODO: Find a place for this to be shared.
type baseMetricsExporter struct {
	component.Component
	consumer.Metrics
}

// TODO: Find a place for this to be shared.
type baseLogsExporter struct {
	component.Component
	consumer.Logs
}

// NewFactory creates a factory for Splunk HEC exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTracesExporter, stability),
		component.WithMetricsExporter(createMetricsExporter, stability),
		component.WithLogsExporter(createLogsExporter, stability))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		LogDataEnabled:       true,
		ProfilingDataEnabled: true,
		ExporterSettings:     config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: defaultHTTPTimeout,
		},
		RetrySettings:           exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:           exporterhelper.NewDefaultQueueSettings(),
		DisableCompression:      false,
		MaxConnections:          defaultMaxIdleCons,
		MaxContentLengthLogs:    defaultContentLengthLogsLimit,
		MaxContentLengthMetrics: defaultContentLengthMetricsLimit,
		MaxContentLengthTraces:  defaultContentLengthTracesLimit,
		HecToOtelAttrs: splunk.HecToOtelAttrs{
			Source:     splunk.DefaultSourceLabel,
			SourceType: splunk.DefaultSourceTypeLabel,
			Index:      splunk.DefaultIndexLabel,
			Host:       conventions.AttributeHostName,
		},
		HecFields: OtelToHecFields{
			SeverityText:   splunk.DefaultSeverityTextLabel,
			SeverityNumber: splunk.DefaultSeverityNumberLabel,
			Name:           splunk.DefaultNameLabel,
		},
	}
}

func createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.TracesExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	cfg := config.(*Config)

	exp, err := createExporter(cfg, set.Logger, &set.BuildInfo)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exp.pushTraceData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop))
}

func createMetricsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	cfg := config.(*Config)

	exp, err := createExporter(cfg, set.Logger, &set.BuildInfo)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exp.pushMetricsData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop))
	if err != nil {
		return nil, err
	}

	wrapped := &baseMetricsExporter{
		Component: exporter,
		Metrics:   batchperresourceattr.NewBatchPerResourceMetrics(splunk.HecTokenLabel, exporter),
	}

	return wrapped, nil
}

func createLogsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (exporter component.LogsExporter, err error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	cfg := config.(*Config)

	exp, err := createExporter(cfg, set.Logger, &set.BuildInfo)

	if err != nil {
		return nil, err
	}

	logsExporter, err := exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exp.pushLogData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop))

	if err != nil {
		return nil, err
	}

	wrapped := &baseLogsExporter{
		Component: logsExporter,
		Logs:      batchperresourceattr.NewBatchPerResourceLogs(splunk.HecTokenLabel, logsExporter),
	}

	return wrapped, nil
}
