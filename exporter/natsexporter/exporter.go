// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/grouper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/marshaler"
)

// natsExporter publishes telemetry to a NATS server. A single value serves any
// of the three signals; the factory wires the matching push method per pipeline.
type natsExporter struct {
	config   *Config
	settings exporter.Settings

	publisher publisher

	logsGrouper    grouper.Grouper[plog.Logs]
	metricsGrouper grouper.Grouper[pmetric.Metrics]
	tracesGrouper  grouper.Grouper[ptrace.Traces]

	logsMarshaler    marshaler.Marshaler[plog.Logs]
	metricsMarshaler marshaler.Marshaler[pmetric.Metrics]
	tracesMarshaler  marshaler.Marshaler[ptrace.Traces]
}

func newExporter(set exporter.Settings, cfg *Config) *natsExporter {
	return &natsExporter{
		config:   cfg,
		settings: set,
	}
}

// start builds the per-signal groupers and marshalers from the configured
// subjects, resolves the marshalers against the host (for encoding extensions),
// and opens the NATS connection.
func (e *natsExporter) start(ctx context.Context, host component.Host) error {
	logsGrouper, err := grouper.NewLogsGrouper(e.config.Logs.Subject, e.settings.TelemetrySettings)
	if err != nil {
		return err
	}
	metricsGrouper, err := grouper.NewMetricsGrouper(e.config.Metrics.Subject, e.settings.TelemetrySettings)
	if err != nil {
		return err
	}
	tracesGrouper, err := grouper.NewTracesGrouper(e.config.Traces.Subject, e.settings.TelemetrySettings)
	if err != nil {
		return err
	}

	logsResolver, err := createResolver(&e.config.Logs)
	if err != nil {
		return err
	}
	metricsResolver, err := createResolver(&e.config.Metrics)
	if err != nil {
		return err
	}
	tracesResolver, err := createResolver(&e.config.Traces)
	if err != nil {
		return err
	}

	e.logsGrouper = logsGrouper
	e.metricsGrouper = metricsGrouper
	e.tracesGrouper = tracesGrouper
	e.logsMarshaler = marshaler.NewMarshaler(logsResolver, marshaler.PickMarshalLogs)
	e.metricsMarshaler = marshaler.NewMarshaler(metricsResolver, marshaler.PickMarshalMetrics)
	e.tracesMarshaler = marshaler.NewMarshaler(tracesResolver, marshaler.PickMarshalTraces)

	var errs error
	errs = multierr.Append(errs, e.logsMarshaler.Resolve(host))
	errs = multierr.Append(errs, e.metricsMarshaler.Resolve(host))
	errs = multierr.Append(errs, e.tracesMarshaler.Resolve(host))
	// Don't open a connection if the marshalers didn't resolve: Start is not
	// guaranteed a matching Shutdown, so a connection opened here could leak.
	if errs != nil {
		return errs
	}

	pub, err := newPublisher(ctx, e.config)
	if err != nil {
		return err
	}
	e.publisher = pub

	return nil
}

func (e *natsExporter) shutdown(_ context.Context) error {
	if e.publisher != nil {
		e.publisher.close()
	}
	return nil
}

func (e *natsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	return publishSignal(ctx, e.publisher, e.logsGrouper, e.logsMarshaler, ld)
}

func (e *natsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	return publishSignal(ctx, e.publisher, e.metricsGrouper, e.metricsMarshaler, md)
}

func (e *natsExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	return publishSignal(ctx, e.publisher, e.tracesGrouper, e.tracesMarshaler, td)
}

// publishSignal groups a signal by subject, marshals each group, and publishes
// it. It is shared by the three push methods.
//
// Subject evaluation and marshaling failures are deterministic — they would
// fail identically on retry — so they are reported as permanent errors and
// dropped. Publish failures are transient and left retryable.
func publishSignal[T any](ctx context.Context, pub publisher, g grouper.Grouper[T], m marshaler.Marshaler[T], data T) error {
	var permanent, transient error

	groups, err := g.Group(ctx, data)
	permanent = multierr.Append(permanent, err)

	for _, group := range groups {
		bytes, err := m.Marshal(group.Data)
		if err != nil {
			permanent = multierr.Append(permanent, err)
			continue
		}

		if err := pub.publish(ctx, group.Subject, bytes); err != nil {
			transient = multierr.Append(transient, err)
		}
	}

	// A transient publish failure retries the whole batch; when only
	// deterministic failures remain, mark them permanent so they are dropped
	// rather than retried forever.
	if transient != nil {
		return multierr.Append(transient, permanent)
	}
	if permanent != nil {
		return consumererror.NewPermanent(permanent)
	}
	return nil
}

// publisher abstracts how marshaled payloads are written to NATS, so the push
// path does not care whether it targets core NATS or (in a follow-up) JetStream.
type publisher interface {
	publish(ctx context.Context, subject string, data []byte) error
	close()
}

// corePublisher publishes with core NATS (fire-and-forget, no delivery guarantee).
type corePublisher struct {
	conn *nats.Conn
}

func (p *corePublisher) publish(_ context.Context, subject string, data []byte) error {
	return p.conn.Publish(subject, data)
}

func (p *corePublisher) close() {
	p.conn.Close()
}

// newPublisher connects to NATS and returns a core-NATS publisher. JetStream
// publishing lands in a follow-up PR; until then a configured jetstream block is
// rejected rather than silently downgraded to fire-and-forget delivery.
func newPublisher(ctx context.Context, cfg *Config) (publisher, error) {
	if cfg.JetStream != nil {
		return nil, errors.New("jetstream publishing is not yet implemented")
	}
	conn, err := connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &corePublisher{conn: conn}, nil
}

func createResolver(cfg *SignalConfig) (marshaler.Resolver, error) {
	if cfg.EncodingExtension != "" {
		return marshaler.NewEncodingExtensionResolver(cfg.EncodingExtension)
	}
	// Built-in marshaler; an empty name defaults to otlp_proto.
	name := marshaler.BuiltinMarshalerName(cfg.Marshaler)
	if name == "" {
		name = marshaler.OtlpProtoBuiltinMarshalerName
	}
	return marshaler.NewBuiltinMarshalerResolver(name)
}
