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

package googlecloudexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter"

import (
	"context"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/featuregate"
)

const (
	// The value of "type" key in configuration.
	typeStr = "googlecloud"
	// The stability level of the exporter.
	stability      = component.StabilityLevelBeta
	defaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

var _ = featuregate.GlobalRegistry().MustRegister(
	"exporter.googlecloud.OTLPDirect",
	featuregate.StageStable,
	featuregate.WithRegisterDescription("When enabled, the googlecloud exporter translates pdata directly to google cloud monitoring's types, rather than first translating to opencensus."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/7132"),
	featuregate.WithRegisterToVersion("v0.69.0"),
)

// NewFactory creates a factory for the googlecloud exporter
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithMetrics(createMetricsExporter, stability),
		exporter.WithLogs(createLogsExporter, stability),
	)
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	retrySettings := exporterhelper.NewDefaultRetrySettings()
	retrySettings.Enabled = false
	return &Config{
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
		RetrySettings:   retrySettings,
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		Config:          collector.DefaultConfig(),
	}
}

func createLogsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg component.Config) (exporter.Logs, error) {
	eCfg := cfg.(*Config)
	logsExporter, err := collector.NewGoogleCloudLogsExporter(ctx, eCfg.Config, params.TelemetrySettings.Logger)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(
		ctx,
		params,
		cfg,
		logsExporter.PushLogs,
		exporterhelper.WithShutdown(logsExporter.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithRetry(eCfg.RetrySettings))
}

// createTracesExporter creates a trace exporter based on this config.
func createTracesExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg component.Config) (exporter.Traces, error) {
	eCfg := cfg.(*Config)
	tExp, err := collector.NewGoogleCloudTracesExporter(ctx, eCfg.Config, params.BuildInfo.Version, eCfg.Timeout)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTracesExporter(
		ctx,
		params,
		cfg,
		tExp.PushTraces,
		exporterhelper.WithShutdown(tExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithRetry(eCfg.RetrySettings))
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg component.Config) (exporter.Metrics, error) {
	eCfg := cfg.(*Config)
	mExp, err := collector.NewGoogleCloudMetricsExporter(ctx, eCfg.Config, params.TelemetrySettings.Logger, params.BuildInfo.Version, eCfg.Timeout)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		params,
		cfg,
		mExp.PushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithRetry(eCfg.RetrySettings))
}
