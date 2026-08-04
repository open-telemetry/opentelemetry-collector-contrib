// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// messenger is the per-signal seam of the exporter: it knows how to marshal a
// signal's pdata and which subject to publish it to.
type messenger[T any] interface {
	// marshal marshals the signal's pdata into a byte slice. If marshaling fails,
	// the error is considered permanent and will not be retried.
	marshal(T) ([]byte, error)

	// subject returns the NATS subject to which the signal's data should be published.
	subject() string
}

// natsExporter is the generic core shared by all three signals. The messenger
// is resolved lazily in start because encoding extensions need the host.
type natsExporter[T any] struct {
	cfg     Config
	logger  *zap.Logger
	newMsgr func(component.Host) (messenger[T], error)
	msgr    messenger[T]
	client  *natsClient
}

func (e *natsExporter[T]) start(ctx context.Context, host component.Host) error {
	msgr, err := e.newMsgr(host)
	if err != nil {
		return err
	}
	client, err := newNatsClient(ctx, e.cfg, e.logger)
	if err != nil {
		return err
	}
	e.msgr = msgr
	e.client = client
	return nil
}

func (e *natsExporter[T]) export(ctx context.Context, data T) error {
	payload, err := e.msgr.marshal(data)
	if err != nil {
		// Marshaling errors are not recoverable by retrying.
		return consumererror.NewPermanent(err)
	}
	return e.client.publish(ctx, e.msgr.subject(), payload)
}

func (e *natsExporter[T]) close(ctx context.Context) error {
	if e.client == nil {
		return nil
	}
	err := e.client.close(ctx)
	e.client = nil
	return err
}

// --- traces ---

type tracesMessenger struct {
	marshaler ptrace.Marshaler
	subj      string
}

func (m tracesMessenger) marshal(td ptrace.Traces) ([]byte, error) {
	return m.marshaler.MarshalTraces(td)
}
func (m tracesMessenger) subject() string { return m.subj }

func newTracesExporter(cfg Config, set exporter.Settings) *natsExporter[ptrace.Traces] {
	return &natsExporter[ptrace.Traces]{
		cfg:    cfg,
		logger: set.Logger,
		newMsgr: func(host component.Host) (messenger[ptrace.Traces], error) {
			m, err := getTracesMarshaler(cfg.Traces.Encoding, host)
			if err != nil {
				return nil, err
			}
			return tracesMessenger{marshaler: m, subj: cfg.Traces.Subject}, nil
		},
	}
}

// --- metrics ---

type metricsMessenger struct {
	marshaler pmetric.Marshaler
	subj      string
}

func (m metricsMessenger) marshal(md pmetric.Metrics) ([]byte, error) {
	return m.marshaler.MarshalMetrics(md)
}
func (m metricsMessenger) subject() string { return m.subj }

func newMetricsExporter(cfg Config, set exporter.Settings) *natsExporter[pmetric.Metrics] {
	return &natsExporter[pmetric.Metrics]{
		cfg:    cfg,
		logger: set.Logger,
		newMsgr: func(host component.Host) (messenger[pmetric.Metrics], error) {
			m, err := getMetricsMarshaler(cfg.Metrics.Encoding, host)
			if err != nil {
				return nil, err
			}
			return metricsMessenger{marshaler: m, subj: cfg.Metrics.Subject}, nil
		},
	}
}

// --- logs ---

type logsMessenger struct {
	marshaler plog.Marshaler
	subj      string
}

func (m logsMessenger) marshal(ld plog.Logs) ([]byte, error) { return m.marshaler.MarshalLogs(ld) }
func (m logsMessenger) subject() string                      { return m.subj }

func newLogsExporter(cfg Config, set exporter.Settings) *natsExporter[plog.Logs] {
	return &natsExporter[plog.Logs]{
		cfg:    cfg,
		logger: set.Logger,
		newMsgr: func(host component.Host) (messenger[plog.Logs], error) {
			m, err := getLogsMarshaler(cfg.Logs.Encoding, host)
			if err != nil {
				return nil, err
			}
			return logsMessenger{marshaler: m, subj: cfg.Logs.Subject}, nil
		},
	}
}
