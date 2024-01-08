// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"context"
	"fmt"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog"
)

// connectorImp is the schema for connector
type connectorImp struct {
	metricsConsumer consumer.Metrics // the next component in the pipeline to ingest metrics after connector
	tracesConsumer  consumer.Traces  // the next component in the pipeline to ingest traces after connector
	logger          *zap.Logger

	// agent specifies the agent used to ingest traces and output APM Stats.
	// It is implemented by the traceagent structure; replaced in tests.
	agent datadog.Ingester

	// translator specifies the translator used to transform APM Stats Payloads
	// from the agent to OTLP Metrics.
	translator *metrics.Translator

	// in specifies the channel through which the agent will output Stats Payloads
	// resulting from ingested traces.
	in chan *pb.StatsPayload

	// exit specifies the exit channel, which will be closed upon shutdown.
	exit chan struct{}
}

var _ component.Component = (*connectorImp)(nil) // testing that the connectorImp properly implements the type Component interface

// function to create a new connector
func newConnector(set component.TelemetrySettings, _ component.Config, metricsConsumer consumer.Metrics, tracesConsumer consumer.Traces) (*connectorImp, error) {
	set.Logger.Info("Building datadog connector")

	in := make(chan *pb.StatsPayload, 100)
	set.MeterProvider = noop.NewMeterProvider() // disable metrics for the connector
	attributesTranslator, err := attributes.NewTranslator(set)
	if err != nil {
		return nil, fmt.Errorf("failed to create attributes translator: %w", err)
	}
	trans, err := metrics.NewTranslator(set, attributesTranslator)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics translator: %w", err)
	}

	ctx := context.Background()
	return &connectorImp{
		logger:          set.Logger,
		agent:           datadog.NewAgent(ctx, in),
		translator:      trans,
		in:              in,
		metricsConsumer: metricsConsumer,
		tracesConsumer:  tracesConsumer,
		exit:            make(chan struct{}),
	}, nil
}

// Start implements the component.Component interface.
func (c *connectorImp) Start(_ context.Context, _ component.Host) error {
	c.logger.Info("Starting datadogconnector")
	c.agent.Start()
	if c.metricsConsumer != nil {
		go c.run()
	}
	return nil
}

// Shutdown implements the component.Component interface.
func (c *connectorImp) Shutdown(context.Context) error {
	c.logger.Info("Shutting down datadog connector")
	c.agent.Stop()
	c.exit <- struct{}{} // signal exit
	<-c.exit             // wait for close
	return nil
}

// Capabilities implements the consumer interface.
// tells use whether the component(connector) will mutate the data passed into it. if set to true the connector does modify the data
func (c *connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true} // ConsumeTraces puts a new attribute _dd.stats_computed
}

func (c *connectorImp) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	c.agent.Ingest(ctx, traces)
	if c.tracesConsumer != nil {
		return c.tracesConsumer.ConsumeTraces(ctx, traces)
	}
	return nil
}

// run awaits incoming stats resulting from the agent's ingestion, converts them
// to metrics and flushes them using the configured metrics exporter.
func (c *connectorImp) run() {
	defer close(c.exit)
	for {
		select {
		case stats := <-c.in:
			if len(stats.Stats) == 0 {
				continue
			}
			var mx pmetric.Metrics
			var err error
			if datadog.ConnectorPerformanceFeatureGate.IsEnabled() {
				c.logger.Debug("Received stats payload", zap.Any("stats", stats))
				mx, err = c.translator.StatsToMetrics(stats)
				if err != nil {
					c.logger.Error("Failed to convert stats to metrics", zap.Error(err))
					continue
				}
			} else {
				mx = c.translator.StatsPayloadToMetrics(stats)
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
