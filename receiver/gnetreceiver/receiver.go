// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnetreceiver"

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/panjf2000/gnet/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// Wire protocol (same constants used by gnetexporter):
//
//	Every frame is:
//	  [1 byte signal tag][4 bytes big-endian payload length][N bytes OTLP proto]
//
//	Signal tags:
const (
	signalTraces  byte = 0x01
	signalMetrics byte = 0x02
	signalLogs    byte = 0x03

	frameHeaderLen = 5 // 1 (tag) + 4 (length)
)

// gnetReceiver implements gnet.EventHandler and the OTel receiver interfaces.
type gnetReceiver struct {
	gnet.BuiltinEventEngine

	cfg         *Config
	settings    receiver.Settings
	nextTraces  consumer.Traces
	nextMetrics consumer.Metrics
	nextLogs    consumer.Logs

	tracesUnmarshaller  ptrace.ProtoUnmarshaler
	metricsUnmarshaller pmetric.ProtoUnmarshaler
	logsUnmarshaller    plog.ProtoUnmarshaler

	engineDone chan error
	engineStop chan struct{}
}

func newReceiver(params receiver.Settings, cfg *Config) (*gnetReceiver, error) {
	return &gnetReceiver{
		cfg:                 cfg,
		settings:            params,
		tracesUnmarshaller:  ptrace.ProtoUnmarshaler{},
		metricsUnmarshaller: pmetric.ProtoUnmarshaler{},
		logsUnmarshaller:    plog.ProtoUnmarshaler{},
		engineDone:          make(chan error, 1),
		engineStop:          make(chan struct{}),
	}, nil
}

// ── component.Component ──────────────────────────────────────────────────────

func (r *gnetReceiver) Start(_ context.Context, _ component.Host) error {
	addr := fmt.Sprintf("tcp://%s", r.cfg.Endpoint)
	go func() {
		err := gnet.Run(
			r,
			addr,
			gnet.WithNumEventLoop(r.cfg.NumEventLoops),
			gnet.WithReadBufferCap(r.cfg.ReadBufferCap),
			gnet.WithReusePort(true),
		)
		r.engineDone <- err
	}()
	r.settings.Logger.Info("gnet receiver started", zap.String("endpoint", r.cfg.Endpoint))
	return nil
}

func (r *gnetReceiver) Shutdown(ctx context.Context) error {
	close(r.engineStop)
	select {
	case err := <-r.engineDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ── gnet.EventHandler ────────────────────────────────────────────────────────

func (r *gnetReceiver) OnTraffic(c gnet.Conn) gnet.Action {
	for {
		// Need at least a full frame header.
		if c.InboundBuffered() < frameHeaderLen {
			return gnet.None
		}

		// Peek header without consuming.
		header, err := c.Peek(frameHeaderLen)
		if err != nil {
			r.settings.Logger.Error("gnet peek error", zap.Error(err))
			return gnet.Close
		}

		signal := header[0]
		payloadLen := int(binary.BigEndian.Uint32(header[1:5]))

		if c.InboundBuffered() < frameHeaderLen+payloadLen {
			// Full payload not yet arrived; wait for more data.
			return gnet.None
		}

		// Consume header + payload.
		if _, err := c.Discard(frameHeaderLen); err != nil {
			return gnet.Close
		}
		payload := make([]byte, payloadLen)
		if _, err := c.Read(payload); err != nil {
			return gnet.Close
		}

		if err := r.dispatch(signal, payload); err != nil {
			r.settings.Logger.Warn("dispatch error", zap.Error(err))
		}
	}
}

func (r *gnetReceiver) OnClose(_ gnet.Conn, _ error) gnet.Action {
	select {
	case <-r.engineStop:
		return gnet.Shutdown
	default:
		return gnet.None
	}
}

// dispatch unmarshals the OTLP payload and hands it to the right consumer.
func (r *gnetReceiver) dispatch(signal byte, payload []byte) error {
	ctx := context.Background()
	switch signal {
	case signalTraces:
		if r.nextTraces == nil {
			return nil
		}
		td, err := r.tracesUnmarshaller.UnmarshalTraces(payload)
		if err != nil {
			return fmt.Errorf("unmarshal traces: %w", err)
		}
		return r.nextTraces.ConsumeTraces(ctx, td)

	case signalMetrics:
		if r.nextMetrics == nil {
			return nil
		}
		md, err := r.metricsUnmarshaller.UnmarshalMetrics(payload)
		if err != nil {
			return fmt.Errorf("unmarshal metrics: %w", err)
		}
		return r.nextMetrics.ConsumeMetrics(ctx, md)

	case signalLogs:
		if r.nextLogs == nil {
			return nil
		}
		ld, err := r.logsUnmarshaller.UnmarshalLogs(payload)
		if err != nil {
			return fmt.Errorf("unmarshal logs: %w", err)
		}
		return r.nextLogs.ConsumeLogs(ctx, ld)

	default:
		return fmt.Errorf("unknown signal tag 0x%02x", signal)
	}
}
