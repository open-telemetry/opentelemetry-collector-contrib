// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
	"github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline"
	"github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter"
	logscompressionimpl "github.com/DataDog/datadog-agent/comp/serializer/logscompression/impl"
	"github.com/DataDog/datadog-agent/pkg/logs/sources"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"go.opentelemetry.io/collector/exporter"

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

// newLogsAgentExporter creates new instances of the logs agent and the logs agent exporter
func newLogsAgentExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *datadogconfig.Config,
	sourceProvider source.Provider,
	_ *attributes.GatewayUsage,
) (logsagentpipeline.LogsAgent, *logsagentexporter.Exporter, error) {
	logComponent := agentcomponents.NewLogComponent(params.TelemetrySettings)
	cfgComponent := agentcomponents.NewConfigComponent(
		agentcomponents.WithAPIConfig(cfg),
		agentcomponents.WithLogsEnabled(),
		agentcomponents.WithLogLevel(params.TelemetrySettings),
		agentcomponents.WithLogsConfig(cfg),
		agentcomponents.WithLogsDefaults(),
		agentcomponents.WithProxy(cfg),
	)
	logsAgentConfig := &logsagentexporter.Config{
		OtelSource:    otelSource,
		LogSourceName: logSourceName,
	}
	hostnameComponent := logs.NewHostnameService(sourceProvider)
	logsAgent := logsagentpipelineimpl.NewLogsAgent(logsagentpipelineimpl.Dependencies{
		Log:          logComponent,
		Config:       cfgComponent,
		Hostname:     hostnameComponent,
		Compression:  logscompressionimpl.NewComponent(),
		IntakeOrigin: config.OTelCollectorIntakeOrigin,
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
