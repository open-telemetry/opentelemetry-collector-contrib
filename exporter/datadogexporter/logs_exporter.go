// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !solaris && !freebsd

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
	"github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline"
	pipelineimpl "github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/impl"
	"github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter"
	logscompressionimpl "github.com/DataDog/datadog-agent/comp/serializer/logscompression/impl"
	"github.com/DataDog/datadog-agent/pkg/logs/sources"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

const (
	// logSourceName specifies the Datadog source tag value to be added to logs sent from the Datadog exporter.
	logSourceName = "otlp_log_ingestion"
	// otelSource specifies a source to be added to all logs sent from the Datadog exporter. The tag has key `otel_source` and the value specified on this constant.
	otelSource = "datadog_exporter"
)

const (
	// logsSendersPerPipeline and defaultLogsBatchSize mirror the defaults of the logs agent pipelines
	// that a sync sender replaces: the number of payloads that the HTTP sender of each pipeline sends
	// at the same time, and the maximum number of log records in an intake payload.
	logsSendersPerPipeline = 10
	defaultLogsBatchSize   = 1000
)

// logsSyncSettings sizes the sending queue of an exporter that uses a sync sender.
type logsSyncSettings struct {
	// concurrency is the number of payloads that the asynchronous logs pipelines send at the same time.
	concurrency int
	// batchSize is the maximum number of log records in an intake payload.
	batchSize int
}

// intConfig is the part of the logs agent configuration that sizes the settings of a sync sender.
type intConfig interface {
	GetInt(key string) int
}

// logsSenderConcurrency returns the number of payloads that the asynchronous logs pipelines send at
// the same time.
func logsSenderConcurrency(cfg intConfig) int {
	perPipeline := logsSendersPerPipeline
	if configured := cfg.GetInt("logs_config.batch_max_concurrent_send"); configured > 0 {
		perPipeline = configured
	}
	return max(1, cfg.GetInt("logs_config.pipelines")) * perPipeline
}

// logsBatchSize returns the maximum number of log records in an intake payload.
func logsBatchSize(cfg intConfig) int {
	if size := cfg.GetInt("logs_config.batch_max_size"); size > 0 {
		return size
	}
	return defaultLogsBatchSize
}

// logsExporterHelperSettings returns the timeout and the sending queue of the logs exporter.
//
// A sync sender sends logs in ConsumeLogs: exporterhelper bounds each call by the client timeout,
// and the consumers and batches of the sending queue stand in for the concurrency and the batching
// of the asynchronous logs pipelines.
func logsExporterHelperSettings(cfg *datadogconfig.Config, syncSettings *logsSyncSettings) (exporterhelper.TimeoutConfig, configoptional.Optional[exporterhelper.QueueBatchConfig]) {
	if syncSettings == nil {
		// explicitly disable since we rely on http.Client timeout logic.
		return exporterhelper.TimeoutConfig{Timeout: 0}, cfg.QueueSettings
	}
	return exporterhelper.TimeoutConfig{Timeout: cfg.ClientConfig.Timeout},
		logsagentexporter.SyncSenderQueueConfig(cfg.QueueSettings, syncSettings.concurrency, syncSettings.batchSize, cfg.IsQueueNumConsumersSet())
}

// newLogsAgentExporter creates new instances of the logs agent and the logs agent exporter.
//
// When the logsagentexporter UseSyncSender feature gate is enabled, the exporter sends logs with a sync
// sender instead: no logs agent is started, so the returned logs agent is nil, and the returned settings
// size the sending queue. Otherwise, the returned settings are nil.
func newLogsAgentExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *datadogconfig.Config,
	sourceProvider source.Provider,
	_ *attributes.GatewayUsage,
) (logsagentpipeline.LogsAgent, *logsagentexporter.Exporter, *logsSyncSettings, error) {
	logComponent := agentcomponents.NewLogComponent(params.TelemetrySettings)
	cfgComponent := agentcomponents.NewConfigComponent(
		agentcomponents.WithAPIConfig(cfg),
		agentcomponents.WithLogsEnabled(),
		agentcomponents.WithLogLevel(params.TelemetrySettings),
		agentcomponents.WithLogsConfig(cfg),
		agentcomponents.WithLogsDefaults(),
		agentcomponents.WithProxy(cfg),
		agentcomponents.WithTLSSetting(cfg),
	)
	hostnameComponent := logs.NewHostnameService(sourceProvider)
	logsAgentConfig := &logsagentexporter.Config{
		OtelSource:    otelSource,
		LogSourceName: logSourceName,
		OrchestratorConfig: logsagentexporter.OrchestratorConfig{
			Hostname: hostnameComponent,
			Key:      string(cfg.API.Key),
			Site:     cfg.API.Site,
			Endpoint: cfg.OrchestratorExplorer.Endpoint,
			Enabled:  cfg.OrchestratorExplorer.Enabled,
		},
	}
	deps := logsagentpipelineimpl.Dependencies{
		Log:          logComponent,
		Config:       cfgComponent,
		Hostname:     hostnameComponent,
		Compression:  logscompressionimpl.NewComponent(),
		IntakeOrigin: config.OTelCollectorIntakeOrigin,
	}
	logSource := sources.NewLogSource(logsAgentConfig.LogSourceName, &config.LogsConfig{})
	attributesTranslator, err := attributes.NewTranslator(params.TelemetrySettings)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, nil, fmt.Errorf("failed to create attribute translator: %w", err)
	}

	if logsagentexporter.IsSyncSenderEnabled() {
		return newSyncLogsAgentExporter(params, logsAgentConfig, logSource, attributesTranslator, deps, cfgComponent)
	}

	logsAgent := logsagentpipelineimpl.NewLogsAgent(deps)
	err = logsAgent.Start(ctx)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, nil, fmt.Errorf("failed to create logs agent: %w", err)
	}

	pipelineChan := logsAgent.GetPipelineProvider().NextPipelineChan()
	logsAgentExporter, err := logsagentexporter.NewExporter(params.TelemetrySettings, logsAgentConfig, logSource, pipelineChan, attributesTranslator)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, nil, fmt.Errorf("failed to create logs agent exporter: %w", err)
	}
	return logsAgent, logsAgentExporter, nil, nil
}

// newSyncLogsAgentExporter creates an exporter that sends logs with a sync sender, which reports delivery
// errors to exporterhelper, so the asynchronous logs agent is not started. sizing is the configuration
// of the logs agent that deps was built from.
func newSyncLogsAgentExporter(
	params exporter.Settings,
	logsAgentConfig *logsagentexporter.Config,
	logSource *sources.LogSource,
	attributesTranslator *attributes.Translator,
	deps pipelineimpl.Dependencies,
	sizing intConfig,
) (logsagentpipeline.LogsAgent, *logsagentexporter.Exporter, *logsSyncSettings, error) {
	syncSender, err := pipelineimpl.NewSyncSender(deps)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, nil, fmt.Errorf("failed to create logs sync sender: %w", err)
	}
	logsAgentExporter, err := logsagentexporter.NewExporterWithSyncSender(params.TelemetrySettings, logsAgentConfig, logSource, syncSender, attributesTranslator)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, nil, fmt.Errorf("failed to create logs agent exporter: %w", err)
	}
	return nil, logsAgentExporter, &logsSyncSettings{
		concurrency: logsSenderConcurrency(sizing),
		batchSize:   logsBatchSize(sizing),
	}, nil
}
