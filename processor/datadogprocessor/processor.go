// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor"

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type datadogProcessor struct {
	logger       *zap.Logger
	nextConsumer consumer.Traces
	cfg          *Config
	started      bool

	// metricsExporter specifies the metrics exporter used to exporter APM Stats
	// as metrics through
	metricsExporter exporter.Metrics

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

func newProcessor(ctx context.Context, logger *zap.Logger, config component.Config, nextConsumer consumer.Traces) (*datadogProcessor, error) {
	cfg := config.(*Config)
	in := make(chan pb.StatsPayload, 100)
	trans, err := metrics.NewTranslator(logger)
	if err != nil {
		return nil, err
	}
	return &datadogProcessor{
		logger:       logger,
		nextConsumer: nextConsumer,
		agent:        newAgent(ctx, in),
		translator:   trans,
		in:           in,
		cfg:          cfg,
		exit:         make(chan struct{}),
	}, nil
}

// Start implements the component.Component interface.
func (p *datadogProcessor) Start(_ context.Context, host component.Host) error {
	var datadogs []exporter.Metrics
loop:
	for k, exp := range host.GetExporters()[component.DataTypeMetrics] { //nolint:staticcheck
		mexp, ok := exp.(exporter.Metrics)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.String())
		}
		switch p.cfg.MetricsExporter {
		case k:
			// we found exactly the configured metrics exporter
			p.metricsExporter = mexp
			break loop
		case datadogComponent:
			// we are looking for the default "datadog" component
			if k.Type() == datadogComponent.Type() {
				// and k has the type, but not the name, so it's not an exact match. Store
				// it for later; if we discover that k was the only Datadog component, we will
				// will conclude that it is safe to use. Otherwise, we will fail and force the
				// user to choose.
				datadogs = append(datadogs, mexp)
			}
		}
	}
	if p.metricsExporter == nil {
		// the exact component was not found
		switch len(datadogs) {
		case 0:
			// no valid defaults to fall back to
			return fmt.Errorf("failed to find metrics exporter %q; please specify a valid processor::datadog::metrics_exporter", p.cfg.MetricsExporter)
		case 1:
			// exactly one valid default to fall back to; use it
			p.metricsExporter = datadogs[0]
		default:
			// too many defaults to fall back to; ambiguous situation; force the user to choose:
			return fmt.Errorf("too many exporters of type %q; please choose one using processor::datadog::metrics_exporter", p.cfg.MetricsExporter)
		}
	}
	p.started = true
	p.agent.Start()
	go p.run()
	p.logger.Debug("Started datadogprocessor", zap.Stringer("metrics_exporter", p.cfg.MetricsExporter))
	return nil
}

// Shutdown implements the component.Component interface.
func (p *datadogProcessor) Shutdown(context.Context) error {
	if !p.started {
		return nil
	}
	p.started = false
	p.agent.Stop()
	p.exit <- struct{}{} // signal exit
	<-p.exit             // wait for close
	return nil
}

// Capabilities implements the consumer interface.
func (p *datadogProcessor) Capabilities() consumer.Capabilities {
	// A resource attribute is added to traces to specify that stats have already
	// been computed for them; thus, we end up mutating the data:
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeTraces implements consumer.Traces.
func (p *datadogProcessor) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	p.logger.Debug("Received traces.", zap.Int("spans", traces.SpanCount()))
	p.agent.Ingest(ctx, traces)
	return p.nextConsumer.ConsumeTraces(ctx, traces)
}

// run awaits incoming stats resulting from the agent's ingestion, converts them
// to metrics and flushes them using the configured metrics exporter.
func (p *datadogProcessor) run() {
	defer close(p.exit)
	for {
		select {
		case stats := <-p.in:
			if len(stats.Stats) == 0 {
				continue
			}
			mx := p.translator.StatsPayloadToMetrics(stats)
			ctx := context.TODO()
			p.logger.Debug("Exporting APM Stats metrics.", zap.Int("count", mx.MetricCount()))
			if err := p.metricsExporter.ConsumeMetrics(ctx, mx); err != nil {
				p.logger.Error("Error exporting metrics.", zap.Error(err))
			}
		case <-p.exit:
			return
		}
	}
}
