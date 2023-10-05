// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"

	"github.com/DataDog/datadog-agent/pkg/conf"
	"github.com/DataDog/datadog-agent/pkg/logs/message"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.opentelemetry.io/collector/exporter"

	pkgLogsAgent "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs/agent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
)

// otelTag specifies a tag to be added to all logs sent from the Datadog exporter
const otelTag = "otel_source:datadog_exporter"

type logsExporter struct {
	params           exporter.CreateSettings
	cfg              *Config
	ctx              context.Context // ctx triggers shutdown upon cancellation
	scrubber         scrub.Scrubber  // scrubber scrubs sensitive information from error messages
	onceMetadata     *sync.Once
	sourceProvider   source.Provider
	metadataReporter *inframetadata.Reporter
	logsAgent        *pkgLogsAgent.Agent
	pipelineChan     chan *message.Message
}

// newLogsExporter creates a new instance of logsExporter
func newLogsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg *Config,
	onceMetadata *sync.Once,
	sourceProvider source.Provider,
	metadataReporter *inframetadata.Reporter,
) (*logsExporter, error) {
	c := conf.NewConfig("test", "DD", strings.NewReplacer(".", "_"))
	c.Set("api_key", os.Getenv("DD_API_KEY"))
	c.Set("site", "datadoghq.com")
	logsAgent := pkgLogsAgent.NewLogsAgent(params.Logger, c)
	err := logsAgent.Start(ctx)
	if err != nil {
		params.Logger.Error("Failed to create logs agent", zap.Error(err))
		return nil, err
	}
	pipelineChan := logsAgent.GetPipelineProvider().NextPipelineChan()

	return &logsExporter{
		params:           params,
		cfg:              cfg,
		ctx:              ctx,
		onceMetadata:     onceMetadata,
		scrubber:         scrub.NewScrubber(),
		sourceProvider:   sourceProvider,
		metadataReporter: metadataReporter,
		logsAgent:        logsAgent,
		pipelineChan:     pipelineChan,
	}, nil
}
