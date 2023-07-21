// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"context"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// schema for connector
type connectorImp struct {
	config          Config
	metricsConsumer consumer.Metrics // the next component in the pipeline to ingest data after connector
	count           int
	logger          *zap.Logger
	started         bool

	// agent specifies the agent used to ingest traces and output APM Stats.
	// It is implemented by the traceagent structure; replaced in tests.
	agent ingester

	// translator specifies the translator used to transform APM Stats Payloads
	// from the agent to OTLP Metrics.
	translator *metrics.Translator

	// in specifies the channel through which the agent will output Stats Payloads
	// resulting from ingested traces.
	in chan pb.StatsPayload

	// exit specifies the exit channel, which will be closed upon shutdown.
	exit chan struct{}
}

// function to create a new connector
func newConnector(logger *zap.Logger, config component.Config) (*connectorImp, error) {
	logger.Info("Building simpleconnector connector")
	cfg := config.(*Config)
	in := make(chan pb.StatsPayload, 100)
	trans, err := metrics.NewTranslator(logger)

	ctx := context.Background()
	if err != nil {
		return nil, err
	}
	return &connectorImp{
		logger:     logger,
		agent:      newAgent(ctx, in),
		translator: trans,
		in:         in,
		config:     *cfg,
		exit:       make(chan struct{}),
	}, nil
}

// Start implements the component.Component interface.
func (c *connectorImp) Start(ctx context.Context, _ component.Host) error {
	c.logger.Info("Starting datadogconnector")
	c.started = true
	c.agent.Start()
	go c.run()
	return nil
}

// Shutdown implements the component.Component interface.
func (c *connectorImp) Shutdown(context.Context) error {
	c.logger.Info("Shutting down spanmetrics connector")
	if !c.started {
		return nil
	}
	c.started = false
	c.agent.Stop()
	c.exit <- struct{}{} // signal exit
	<-c.exit             // wait for close
	return nil
}

// Capabilities implements the consumer interface.
// tells use whether the component(connector) will mutate the data passed into it. if set to true the processor does modify the data
func (c *connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *connectorImp) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	c.logger.Debug("Received traces.", zap.Int("spans", traces.SpanCount()))
	c.agent.Ingest(ctx, traces)
	return nil
	// return p.nextConsumer.ConsumeTraces(ctx, traces)
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
			// APM stats as metrics
			mx := c.translator.StatsPayloadToMetrics(stats)
			ctx := context.TODO()
			c.logger.Debug("Exporting APM Stats metrics.", zap.Int("count", mx.MetricCount()))

			// send metrics to the consumer or next compoonet in pipeline
			if err := c.metricsConsumer.ConsumeMetrics(ctx, mx); err != nil {
				c.logger.Error("Failed ConsumeMetrics", zap.Error(err))
				return
			}
		case <-c.exit:
			return
		}
	}
}
