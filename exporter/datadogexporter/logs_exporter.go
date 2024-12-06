// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"
	"sync"

	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
	"github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline"
	"github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter"
	"github.com/DataDog/datadog-agent/pkg/logs/sources"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	logsmapping "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/logs"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
)

const (
	// logSourceName specifies the Datadog source tag value to be added to logs sent from the Datadog exporter.
	logSourceName = "otlp_log_ingestion"
	// otelSource specifies a source to be added to all logs sent from the Datadog exporter. The tag has key `otel_source` and the value specified on this constant.
	otelSource = "datadog_exporter"
)

type logsExporter struct {
	params           exporter.Settings
	cfg              *Config
	ctx              context.Context // ctx triggers shutdown upon cancellation
	scrubber         scrub.Scrubber  // scrubber scrubs sensitive information from error messages
	translator       *logsmapping.Translator
	sender           *logs.Sender
	onceMetadata     *sync.Once
	sourceProvider   source.Provider
	metadataReporter *inframetadata.Reporter // will be nil if host metadata is disabled
}

// newLogsExporter creates a new instance of logsExporter
func newLogsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *Config,
	onceMetadata *sync.Once,
	attributesTranslator *attributes.Translator,
	sourceProvider source.Provider,
	metadataReporter *inframetadata.Reporter,
) (*logsExporter, error) {
	// create Datadog client
	// validation endpoint is provided by Metrics
	errchan := make(chan error)
	if isMetricExportV2Enabled() {
		apiClient := clientutil.CreateAPIClient(
			params.BuildInfo,
			cfg.Metrics.TCPAddrConfig.Endpoint,
			cfg.ClientConfig)
		go func() { errchan <- clientutil.ValidateAPIKey(ctx, string(cfg.API.Key), params.Logger, apiClient) }()
	} else {
		client := clientutil.CreateZorkianClient(string(cfg.API.Key), cfg.Metrics.TCPAddrConfig.Endpoint)
		go func() { errchan <- clientutil.ValidateAPIKeyZorkian(params.Logger, client) }()
	}
	// validate the apiKey
	if cfg.API.FailOnInvalidKey {
		if err := <-errchan; err != nil {
			return nil, err
		}
	}

	translator, err := logsmapping.NewTranslator(params.TelemetrySettings, attributesTranslator, otelSource)
	if err != nil {
		return nil, fmt.Errorf("failed to create logs translator: %w", err)
	}
	s := logs.NewSender(cfg.Logs.TCPAddrConfig.Endpoint, params.Logger, cfg.ClientConfig, cfg.Logs.DumpPayloads, string(cfg.API.Key))

	return &logsExporter{
		params:           params,
		cfg:              cfg,
		ctx:              ctx,
		translator:       translator,
		sender:           s,
		onceMetadata:     onceMetadata,
		scrubber:         scrub.NewScrubber(),
		sourceProvider:   sourceProvider,
		metadataReporter: metadataReporter,
	}, nil
}

var _ consumer.ConsumeLogsFunc = (*logsExporter)(nil).consumeLogs

// consumeLogs is implementation of cosumer.ConsumeLogsFunc
func (exp *logsExporter) consumeLogs(ctx context.Context, ld plog.Logs) (err error) {
	defer func() { err = exp.scrubber.Scrub(err) }()
	if exp.cfg.HostMetadata.Enabled {
		// start host metadata with resource attributes from
		// the first payload.
		exp.onceMetadata.Do(func() {
			attrs := pcommon.NewMap()
			if ld.ResourceLogs().Len() > 0 {
				attrs = ld.ResourceLogs().At(0).Resource().Attributes()
			}
			go hostmetadata.RunPusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), exp.sourceProvider, attrs, exp.metadataReporter)
		})

		// Consume resources for host metadata
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			res := ld.ResourceLogs().At(i).Resource()
			consumeResource(exp.metadataReporter, res, exp.params.Logger)
		}
	}

	payloads := exp.translator.MapLogs(ctx, ld)
	return exp.sender.SubmitLogs(exp.ctx, payloads)
}

// newLogsAgentExporter creates new instances of the logs agent and the logs agent exporter
func newLogsAgentExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *Config,
	sourceProvider source.Provider,
) (logsagentpipeline.LogsAgent, *logsagentexporter.Exporter, error) {
	logComponent := newLogComponent(params.TelemetrySettings)
	cfgComponent := newConfigComponent(params.TelemetrySettings, cfg)
	logsAgentConfig := &logsagentexporter.Config{
		OtelSource:    otelSource,
		LogSourceName: logSourceName,
	}
	hostnameComponent := logs.NewHostnameService(sourceProvider)
	logsAgent := logsagentpipelineimpl.NewLogsAgent(logsagentpipelineimpl.Dependencies{
		Log:      logComponent,
		Config:   cfgComponent,
		Hostname: hostnameComponent,
	})
	err := logsAgent.Start(ctx)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, fmt.Errorf("failed to create logs agent: %w", err)
	}

	pipelineChan := logsAgent.GetPipelineProvider().NextPipelineChan()
	logSource := sources.NewLogSource(logsAgentConfig.LogSourceName, &config.LogsConfig{})
	attributesTranslator, err := attributes.NewTranslator(params.TelemetrySettings)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, fmt.Errorf("failed to create attribute translator: %w", err)
	}

	logsAgentExporter, err := logsagentexporter.NewExporter(params.TelemetrySettings, logsAgentConfig, logSource, pipelineChan, attributesTranslator)
	if err != nil {
		return nil, &logsagentexporter.Exporter{}, fmt.Errorf("failed to create logs agent exporter: %w", err)
	}
	return logsAgent, logsAgentExporter, nil
}
