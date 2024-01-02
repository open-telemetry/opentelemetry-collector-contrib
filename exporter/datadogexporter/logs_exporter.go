// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"
	"sync"

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

// otelSource specifies a source to be added to all logs sent from the Datadog exporter
// The tag has key `otel_source` and the value specified on this constant.
const otelSource = "datadog_exporter"

type logsExporter struct {
	params           exporter.CreateSettings
	cfg              *Config
	ctx              context.Context // ctx triggers shutdown upon cancellation
	scrubber         scrub.Scrubber  // scrubber scrubs sensitive information from error messages
	translator       *logsmapping.Translator
	sender           *logs.Sender
	onceMetadata     *sync.Once
	sourceProvider   source.Provider
	metadataReporter *inframetadata.Reporter
}

// newLogsExporter creates a new instance of logsExporter
func newLogsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
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
			cfg.Metrics.TCPAddr.Endpoint,
			cfg.TimeoutSettings,
			cfg.LimitedHTTPClientSettings.TLSSetting.InsecureSkipVerify)
		go func() { errchan <- clientutil.ValidateAPIKey(ctx, string(cfg.API.Key), params.Logger, apiClient) }()
	} else {
		client := clientutil.CreateZorkianClient(string(cfg.API.Key), cfg.Metrics.TCPAddr.Endpoint)
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
	s := logs.NewSender(cfg.Logs.TCPAddr.Endpoint, params.Logger, cfg.TimeoutSettings, cfg.LimitedHTTPClientSettings.TLSSetting.InsecureSkipVerify, cfg.Logs.DumpPayloads, string(cfg.API.Key))

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
