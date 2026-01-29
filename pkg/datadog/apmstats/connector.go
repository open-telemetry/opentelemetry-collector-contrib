// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apmstats // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apmstats"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/comp/core/tagger/types"
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-agent/pkg/trace/stats"
	"github.com/DataDog/datadog-agent/pkg/util/option"
	"github.com/DataDog/datadog-go/v5/statsd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/featuregates"
)

// traceToMetricConnector is the schema for connector
type traceToMetricConnector struct {
	metricsConsumer consumer.Metrics // the next component in the pipeline to ingest metrics after connector
	logger          *zap.Logger

	wg sync.WaitGroup

	// concentrator ingests spans and produces APM stats
	concentrator *stats.Concentrator

	// tcfg is the trace agent config
	tcfg *config.AgentConfig

	// ctagKeys are container tag keys
	ctagKeys []string

	// peerTagKeys are peer tag keys to group APM stats
	peerTagKeys []string

	// translator specifies the translator used to transform APM Stats Payloads
	// from the agent to OTLP Metrics.
	// We use the deprecated Translator type because it's the only one that provides
	// the StatsToMetrics method needed for APM stats conversion.
	//nolint:staticcheck // SA1019: Using deprecated Translator type for StatsToMetrics functionality
	translator *metrics.Translator

	// statsout specifies the channel through which the agent will output Stats Payloads
	// resulting from ingested traces.
	statsout chan *pb.StatsPayload

	// obfuscator is used to obfuscate sensitive data from various span
	// tags based on their type.
	obfuscator *obfuscate.Obfuscator

	// exit specifies the exit channel, which will be closed upon shutdown.
	exit chan struct{}

	// isStarted tracks whether Start() has been called.
	isStarted bool
}

// newTraceToMetricConnector creates a new connector with native OTel span ingestion
func newTraceToMetricConnector(set component.TelemetrySettings, cfg component.Config, metricsConsumer consumer.Metrics, metricsClient statsd.ClientInterface, concentrator *stats.Concentrator, tagger types.TaggerClient, hostnameOpt option.Option[string]) (*traceToMetricConnector, error) {
	set.Logger.Info("Building datadog connector for traces to metrics")
	statsout := make(chan *pb.StatsPayload, 100)
	set.MeterProvider = noop.NewMeterProvider() // disable metrics for the connector
	attributesTranslator, err := attributes.NewTranslator(set)
	if err != nil {
		return nil, fmt.Errorf("failed to create attributes translator: %w", err)
	}
	// We use the deprecated NewTranslator because the new NewDefaultTranslator
	// doesn't provide the StatsToMetrics method which is required for APM stats conversion.
	//nolint:staticcheck // SA1019: Using deprecated NewTranslator for StatsToMetrics functionality
	trans, err := metrics.NewTranslator(set, attributesTranslator)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics translator: %w", err)
	}

	tcfg := getTraceAgentCfg(set.Logger, cfg.(*datadogconfig.ConnectorComponentConfig).Traces, attributesTranslator, tagger, hostnameOpt)
	oconf := tcfg.Obfuscation.Export(tcfg)
	oconf.Statsd = metricsClient
	oconf.Redis.Enabled = true

	if concentrator == nil {
		statsWriter := &otelStatsWriter{statsout}
		concentrator = stats.NewConcentrator(tcfg, statsWriter, time.Now(), metricsClient)
	}

	return &traceToMetricConnector{
		logger:          set.Logger,
		translator:      trans,
		tcfg:            tcfg,
		ctagKeys:        cfg.(*datadogconfig.ConnectorComponentConfig).Traces.ResourceAttributesAsContainerTags,
		peerTagKeys:     tcfg.ConfiguredPeerTags(),
		concentrator:    concentrator,
		statsout:        statsout,
		metricsConsumer: metricsConsumer,
		obfuscator:      obfuscate.NewObfuscator(oconf),
		exit:            make(chan struct{}),
	}, nil
}

func getTraceAgentCfg(logger *zap.Logger, cfg datadogconfig.TracesConnectorConfig, attributesTranslator *attributes.Translator, tagger types.TaggerClient, hostnameOpt option.Option[string]) *config.AgentConfig {
	acfg := config.New()
	acfg.OTLPReceiver.AttributesTranslator = attributesTranslator
	acfg.OTLPReceiver.SpanNameRemappings = cfg.SpanNameRemappings
	acfg.OTLPReceiver.SpanNameAsResourceName = cfg.SpanNameAsResourceName
	acfg.OTLPReceiver.IgnoreMissingDatadogFields = cfg.IgnoreMissingDatadogFields
	acfg.Ignore["resource"] = cfg.IgnoreResources
	acfg.ComputeStatsBySpanKind = cfg.ComputeStatsBySpanKind
	acfg.PeerTagsAggregation = cfg.PeerTagsAggregation
	acfg.PeerTags = cfg.PeerTags

	// ==== START for DDOT
	if hostname, found := hostnameOpt.Get(); found {
		acfg.Hostname = hostname
	}
	if tagger != nil {
		acfg.ContainerTags = func(cid string) ([]string, error) {
			return tagger.Tag(types.NewEntityID(types.ContainerID, cid), types.HighCardinality)
		}
	}
	if len(cfg.ResourceAttributesAsContainerTags) > 0 {
		acfg.Features["enable_cid_stats"] = struct{}{}
		delete(acfg.Features, "disable_cid_stats")
	}
	// ==== END for DDOT

	if v := cfg.TraceBuffer; v > 0 {
		acfg.TraceBuffer = v
	}
	if cfg.ComputeTopLevelBySpanKind {
		logger.Info("traces::compute_top_level_by_span_kind needs to be enabled in both the Datadog connector and Datadog exporter configs if both components are being used")
		acfg.Features["enable_otlp_compute_top_level_by_span_kind"] = struct{}{}
	}
	if !featuregates.ReceiveResourceSpansV2FeatureGate.IsEnabled() {
		acfg.Features["disable_receive_resource_spans_v2"] = struct{}{}
	}
	if !featuregates.OperationAndResourceNameV2FeatureGate.IsEnabled() {
		acfg.Features["disable_operation_and_resource_name_logic_v2"] = struct{}{}
	} else {
		logger.Info("Please enable feature gate datadog.EnableOperationAndResourceNameV2 for improved operation and resource name logic. The v1 logic will be deprecated in the future - if you have Datadog monitors or alerts set on operation/resource names, you may need to migrate them to the new convention. See the migration guide at https://docs.datadoghq.com/opentelemetry/guide/migrate/migrate_operation_names/")
	}
	if v := cfg.BucketInterval; v > 0 {
		acfg.BucketInterval = v
	}
	return acfg
}

var _ component.Component = (*traceToMetricConnector)(nil) // testing that the connectorImp properly implements the type Component interface

// Start implements the component.Component interface.
func (c *traceToMetricConnector) Start(_ context.Context, _ component.Host) error {
	c.logger.Info("Starting datadogconnector")
	c.concentrator.Start()
	c.wg.Add(1)
	go c.run()
	c.isStarted = true
	return nil
}

// Shutdown implements the component.Component interface.
func (c *traceToMetricConnector) Shutdown(context.Context) error {
	if !c.isStarted {
		// Note: it is not necessary to manually close c.exit, c.in and c.concentrator.exit channels as these are unused.
		c.logger.Info("Requested shutdown, but not started, ignoring.")
		return nil
	}
	c.logger.Info("Shutting down datadog connector")
	c.logger.Info("Stopping obfuscator and concentrator")
	// stop the obfuscator and concentrator and wait for the run loop to exit
	c.obfuscator.Stop()
	c.concentrator.Stop()
	close(c.exit)
	c.wg.Wait() // wait for close
	return nil
}

// Capabilities implements the consumer interface.
// tells use whether the component(connector) will mutate the data passed into it. if set to true the connector does modify the data
func (*traceToMetricConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *traceToMetricConnector) ConsumeTraces(_ context.Context, traces ptrace.Traces) error {
	inputs := stats.OTLPTracesToConcentratorInputsWithObfuscation(traces, c.tcfg, c.ctagKeys, c.peerTagKeys, c.obfuscator)
	for _, input := range inputs {
		c.concentrator.Add(input)
	}
	return nil
}

// run awaits incoming stats resulting from the agent's ingestion, converts them
// to metrics and flushes them using the configured metrics exporter.
func (c *traceToMetricConnector) run() {
	defer c.wg.Done()
	for {
		select {
		case stats := <-c.statsout:
			if len(stats.Stats) == 0 {
				continue
			}
			var mx pmetric.Metrics
			var err error

			c.logger.Debug("Received stats payload", zap.Any("stats", stats))

			mx, err = c.translator.StatsToMetrics(stats)
			if err != nil {
				c.logger.Error("Failed to convert stats to metrics", zap.Error(err))
				continue
			}
			// APM stats as metrics
			ctx := context.TODO()

			// send metrics to the consumer or next component in pipeline
			if err := c.metricsConsumer.ConsumeMetrics(ctx, mx); err != nil {
				c.logger.Error("Failed ConsumeMetrics", zap.Error(err))
				continue
			}
		case <-c.exit:
			return
		}
	}
}

var _ stats.Writer = (*otelStatsWriter)(nil)

type otelStatsWriter struct {
	out chan *pb.StatsPayload
}

// Write this payload to the `out` channel
func (a *otelStatsWriter) Write(payload *pb.StatsPayload) {
	a.out <- payload
}
