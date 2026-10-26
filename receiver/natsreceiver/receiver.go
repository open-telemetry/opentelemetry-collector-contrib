// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/natsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"
)

const transport = "nats"

// natsReceiver consumes telemetry from a NATS server. One instance is shared by
// the logs, metrics, and traces pipelines of a configuration, so all signals
// use a single NATS connection.
type natsReceiver struct {
	config   *Config
	settings receiver.Settings

	// Downstream consumers; nil for signals not used in any pipeline.
	nextLogs    consumer.Logs
	nextMetrics consumer.Metrics
	nextTraces  consumer.Traces

	conn *natsclient.Conn
	subs []*nats.Subscription
	// handlers counts subscriptions whose message handler may still run; each
	// is released once its subscription is drained or closed.
	handlers sync.WaitGroup

	// ctx is passed to the pipeline for every message; cancel aborts in-flight
	// consumes once shutdown gives up waiting on them.
	ctx    context.Context
	cancel context.CancelFunc

	// drainTimeout bounds Shutdown when its ctx has no deadline.
	drainTimeout time.Duration
}

// subscription pairs a signal's configuration with its message handler.
type subscription struct {
	cfg     *SignalConfig
	handler nats.MsgHandler
}

func newReceiver(set receiver.Settings, cfg *Config) *natsReceiver {
	return &natsReceiver{
		config:       cfg,
		settings:     set,
		drainTimeout: nats.DefaultDrainTimeout,
	}
}

// Start resolves the unmarshalers of the signals used in a pipeline, connects
// to NATS, and subscribes each of those signals to its subject. It runs once for
// the shared receiver.
func (r *natsReceiver) Start(ctx context.Context, host component.Host) error {
	// JetStream lands in a follow-up; reject it rather than silently consuming
	// with core NATS (at-most-once) semantics.
	if r.config.JetStream != nil {
		return errors.New("jetstream consumption is not yet implemented")
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             r.settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: r.settings,
	})
	if err != nil {
		return err
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())

	// Resolve unmarshalers before connecting so a missing encoding extension
	// does not leave a connection open: Start is not guaranteed a Shutdown.
	subs, err := r.subscriptions(host, obsrecv)
	if err != nil {
		return err
	}

	// Prefix the kind so the server's connection list tells the receiver apart
	// from a NATS exporter with the same component ID.
	name := "receiver/" + r.settings.ID.String()
	conn, err := natsclient.Connect(ctx, &r.config.ClientConfig, name, host, r.settings.Logger)
	if err != nil {
		return err
	}
	r.conn = conn
	if err := r.subscribe(ctx, subs); err != nil {
		conn.Close()
		r.conn, r.subs = nil, nil
		return err
	}
	return nil
}

// subscriptions builds a subscription for each signal used in a pipeline.
func (r *natsReceiver) subscriptions(host component.Host, obsrecv *receiverhelper.ObsReport) ([]subscription, error) {
	var subs []subscription
	if r.nextLogs != nil {
		u, err := newUnmarshaler[plog.Unmarshaler](&r.config.Logs, host, "logs", &plog.ProtoUnmarshaler{}, &plog.JSONUnmarshaler{})
		if err != nil {
			return nil, fmt.Errorf("logs: %w", err)
		}
		subs = append(subs, subscription{&r.config.Logs, r.logsHandler(u, obsrecv)})
	}
	if r.nextMetrics != nil {
		u, err := newUnmarshaler[pmetric.Unmarshaler](&r.config.Metrics, host, "metrics", &pmetric.ProtoUnmarshaler{}, &pmetric.JSONUnmarshaler{})
		if err != nil {
			return nil, fmt.Errorf("metrics: %w", err)
		}
		subs = append(subs, subscription{&r.config.Metrics, r.metricsHandler(u, obsrecv)})
	}
	if r.nextTraces != nil {
		u, err := newUnmarshaler[ptrace.Unmarshaler](&r.config.Traces, host, "traces", &ptrace.ProtoUnmarshaler{}, &ptrace.JSONUnmarshaler{})
		if err != nil {
			return nil, fmt.Errorf("traces: %w", err)
		}
		subs = append(subs, subscription{&r.config.Traces, r.tracesHandler(u, obsrecv)})
	}
	return subs, nil
}

// subscribe creates the core NATS subscriptions and flushes, so that a
// subscription the server rejects fails Start instead of receiving nothing.
func (r *natsReceiver) subscribe(ctx context.Context, subs []subscription) error {
	for _, s := range subs {
		// An empty queue group is a plain subscription.
		sub, err := r.conn.QueueSubscribe(s.cfg.Subject, s.cfg.QueueGroup, s.handler)
		if err != nil {
			return fmt.Errorf("failed to subscribe to %q: %w", s.cfg.Subject, err)
		}
		// The closed handler runs after the last message handler returns.
		r.handlers.Add(1)
		sub.SetClosedHandler(func(string) { r.handlers.Done() })
		r.subs = append(r.subs, sub)
	}

	// FlushWithContext requires a deadline; otherwise use the bounded Flush.
	var err error
	if _, ok := ctx.Deadline(); ok {
		err = r.conn.FlushWithContext(ctx)
	} else {
		err = r.conn.Flush()
	}
	if err != nil {
		// An invalid subject makes the server close the connection; its reason
		// is more useful than the resulting "connection closed".
		if lastErr := r.conn.LastError(); errors.Is(err, nats.ErrConnectionClosed) && lastErr != nil {
			err = lastErr
		}
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	// A permission violation keeps the connection open, so the flush succeeds;
	// the server's error arrives before the flush reply and is recorded here.
	if lastErr := r.conn.LastError(); errors.Is(lastErr, nats.ErrPermissionViolation) {
		return fmt.Errorf("failed to subscribe: %w", lastErr)
	}
	return nil
}

// Shutdown drains the subscriptions, which stops them and waits for messages
// already received to be handed to the pipeline, bounded by ctx (or by
// drainTimeout if ctx has no deadline). It drains subscriptions rather than the
// connection because closing a connection while it drains makes nats.go close
// it a second time.
func (r *natsReceiver) Shutdown(ctx context.Context) error {
	if r.cancel != nil {
		defer r.cancel()
	}
	if r.conn == nil {
		return nil
	}
	// Close also stops any subscription the drain did not finish.
	defer r.conn.Close()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.drainTimeout)
		defer cancel()
	}

	for _, sub := range r.subs {
		if err := sub.Drain(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
			r.settings.Logger.Warn("failed to drain subscription", zap.String("subject", sub.Subject), zap.Error(err))
		}
	}

	drained := make(chan struct{})
	go func() {
		r.handlers.Wait()
		close(drained)
	}()
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *natsReceiver) logsHandler(u plog.Unmarshaler, obsrecv *receiverhelper.ObsReport) nats.MsgHandler {
	encoding := encodingName(&r.config.Logs)
	return func(msg *nats.Msg) {
		ctx := obsrecv.StartLogsOp(r.ctx)
		ld, err := u.UnmarshalLogs(msg.Data)
		if err != nil {
			r.logUnmarshalError(msg, err)
			obsrecv.EndLogsOp(ctx, encoding, 0, err)
			return
		}
		// Count first: the pipeline may take ownership of ld and move it.
		n := ld.LogRecordCount()
		err = r.nextLogs.ConsumeLogs(ctx, ld)
		obsrecv.EndLogsOp(ctx, encoding, n, err)
		if err != nil {
			r.logConsumeError(msg, err)
		}
	}
}

func (r *natsReceiver) metricsHandler(u pmetric.Unmarshaler, obsrecv *receiverhelper.ObsReport) nats.MsgHandler {
	encoding := encodingName(&r.config.Metrics)
	return func(msg *nats.Msg) {
		ctx := obsrecv.StartMetricsOp(r.ctx)
		md, err := u.UnmarshalMetrics(msg.Data)
		if err != nil {
			r.logUnmarshalError(msg, err)
			obsrecv.EndMetricsOp(ctx, encoding, 0, err)
			return
		}
		// Count first: the pipeline may take ownership of md and move it.
		n := md.DataPointCount()
		err = r.nextMetrics.ConsumeMetrics(ctx, md)
		obsrecv.EndMetricsOp(ctx, encoding, n, err)
		if err != nil {
			r.logConsumeError(msg, err)
		}
	}
}

func (r *natsReceiver) tracesHandler(u ptrace.Unmarshaler, obsrecv *receiverhelper.ObsReport) nats.MsgHandler {
	encoding := encodingName(&r.config.Traces)
	return func(msg *nats.Msg) {
		ctx := obsrecv.StartTracesOp(r.ctx)
		td, err := u.UnmarshalTraces(msg.Data)
		if err != nil {
			r.logUnmarshalError(msg, err)
			obsrecv.EndTracesOp(ctx, encoding, 0, err)
			return
		}
		// Count first: the pipeline may take ownership of td and move it.
		n := td.SpanCount()
		err = r.nextTraces.ConsumeTraces(ctx, td)
		obsrecv.EndTracesOp(ctx, encoding, n, err)
		if err != nil {
			r.logConsumeError(msg, err)
		}
	}
}

// Core NATS has no redelivery, so a message that fails either step is dropped.

func (r *natsReceiver) logUnmarshalError(msg *nats.Msg, err error) {
	r.settings.Logger.Error("failed to unmarshal message; dropping it",
		zap.String("subject", msg.Subject), zap.Error(err))
}

func (r *natsReceiver) logConsumeError(msg *nats.Msg, err error) {
	if r.ctx.Err() != nil {
		// Shutdown gave up waiting and aborted the consume; not a pipeline error.
		r.settings.Logger.Debug("shutdown aborted message; dropping it",
			zap.String("subject", msg.Subject), zap.Error(err))
		return
	}
	r.settings.Logger.Error("pipeline rejected message; dropping it",
		zap.String("subject", msg.Subject), zap.Error(err))
}
