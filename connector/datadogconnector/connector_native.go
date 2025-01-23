// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/statsprocessor"
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-agent/pkg/trace/stats"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
)

// traceToMetricConnectorNative is the schema for connector
type traceToMetricConnectorNative struct {
	metricsConsumer consumer.Metrics // the next component in the pipeline to ingest metrics after connector
	logger          *zap.Logger

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

var _ component.Component = (*traceToMetricConnectorNative)(nil) // testing that the connectorImp properly implements the type Component interface

// newTraceToMetricConnectorNative creates a new connector with native OTel span ingestion
func newTraceToMetricConnectorNative(set component.TelemetrySettings, cfg component.Config, metricsConsumer consumer.Metrics, metricsClient statsd.ClientInterface) (*traceToMetricConnectorNative, error) {
	set.Logger.Info("Building datadog connector for traces to metrics")
	statsout := make(chan *pb.StatsPayload, 100)
	statsWriter := statsprocessor.NewOtelStatsWriter(statsout)
	set.MeterProvider = noop.NewMeterProvider() // disable metrics for the connector
	attributesTranslator, err := attributes.NewTranslator(set)
	if err != nil {
		return nil, fmt.Errorf("failed to create attributes translator: %w", err)
	}
	trans, err := metrics.NewTranslator(set, attributesTranslator)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics translator: %w", err)
	}

	tcfg := getTraceAgentCfg(set.Logger, cfg.(*Config).Traces, attributesTranslator)
	oconf := tcfg.Obfuscation.Export(tcfg)
	oconf.Statsd = metricsClient
	oconf.Redis.Enabled = true

	return &traceToMetricConnectorNative{
		logger:          set.Logger,
		translator:      trans,
		tcfg:            tcfg,
		ctagKeys:        cfg.(*Config).Traces.ResourceAttributesAsContainerTags,
		peerTagKeys:     tcfg.ConfiguredPeerTags(),
		concentrator:    stats.NewConcentrator(tcfg, statsWriter, time.Now(), metricsClient),
		statsout:        statsout,
		metricsConsumer: metricsConsumer,
		obfuscator:      obfuscate.NewObfuscator(oconf),
		exit:            make(chan struct{}),
	}, nil
}

// Start implements the component.Component interface.
func (c *traceToMetricConnectorNative) Start(_ context.Context, _ component.Host) error {
	c.logger.Info("Starting datadogconnector")
	c.concentrator.Start()
	go c.run()
	c.isStarted = true
	return nil
}

// Shutdown implements the component.Component interface.
func (c *traceToMetricConnectorNative) Shutdown(context.Context) error {
	if !c.isStarted {
		// Note: it is not necessary to manually close c.exit, c.in and c.concentrator.exit channels as these are unused.
		c.logger.Info("Requested shutdown, but not started, ignoring.")
		return nil
	}
	c.logger.Info("Shutting down datadog connector")
	c.logger.Info("Stopping obfuscator and concentrator")
	// stop the obfuscator and concentrator and wait for the run loop to exit
	c.obfuscator.Stop()
	c.logger.Info("Stopping concentrator")
	// stop the concentrator and wait for the run loop to exit
	c.concentrator.Stop()
	c.exit <- struct{}{} // signal exit
	<-c.exit             // wait for close
	return nil
}

// Capabilities implements the consumer interface.
// tells use whether the component(connector) will mutate the data passed into it. if set to true the connector does modify the data
func (c *traceToMetricConnectorNative) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *traceToMetricConnectorNative) ConsumeTraces(_ context.Context, traces ptrace.Traces) error {
	inputs := stats.OTLPTracesToConcentratorInputsWithObfuscation(traces, c.tcfg, c.ctagKeys, c.peerTagKeys, c.obfuscator)
	for _, input := range inputs {
		c.concentrator.Add(input)
	}
	return nil
}

// run awaits incoming stats resulting from the agent's ingestion, converts them
// to metrics and flushes them using the configured metrics exporter.
func (c *traceToMetricConnectorNative) run() {
	defer close(c.exit)
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
				return
			}
		case <-c.exit:
			return
		}
	}
}
