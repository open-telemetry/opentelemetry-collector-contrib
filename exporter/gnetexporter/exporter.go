// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/gnetexporter"

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/zap"
)

// Signal tags — must match gnetreceiver constants exactly.
const (
	signalTraces  byte = 0x01
	signalMetrics byte = 0x02
	signalLogs    byte = 0x03

	frameHeaderLen = 5 // 1 (tag) + 4 (length)
)

type gnetExporter struct {
	cfg      *Config
	settings exporter.Settings

	mu   sync.Mutex
	conn net.Conn

	tracesMarshaller  ptrace.ProtoMarshaler
	metricsMarshaller pmetric.ProtoMarshaler
	logsMarshaller    plog.ProtoMarshaler
}

func newExporter(params exporter.Settings, cfg *Config) (*gnetExporter, error) {
	return &gnetExporter{
		cfg:               cfg,
		settings:          params,
		tracesMarshaller:  ptrace.ProtoMarshaler{},
		metricsMarshaller: pmetric.ProtoMarshaler{},
		logsMarshaller:    plog.ProtoMarshaler{},
	}, nil
}

// ── component lifecycle ───────────────────────────────────────────────────────

func (e *gnetExporter) Start(_ context.Context, _ component.Host) error {
	return e.connect()
}

func (e *gnetExporter) Shutdown(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.conn != nil {
		err := e.conn.Close()
		e.conn = nil
		return err
	}
	return nil
}

// ── consumers ────────────────────────────────────────────────────────────────

func (e *gnetExporter) consumeTraces(_ context.Context, td ptrace.Traces) error {
	b, err := e.tracesMarshaller.MarshalTraces(td)
	if err != nil {
		return fmt.Errorf("marshal traces: %w", err)
	}
	return e.send(signalTraces, b)
}

func (e *gnetExporter) consumeMetrics(_ context.Context, md pmetric.Metrics) error {
	b, err := e.metricsMarshaller.MarshalMetrics(md)
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}
	return e.send(signalMetrics, b)
}

func (e *gnetExporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	b, err := e.logsMarshaller.MarshalLogs(ld)
	if err != nil {
		return fmt.Errorf("marshal logs: %w", err)
	}
	return e.send(signalLogs, b)
}

// ── internal ─────────────────────────────────────────────────────────────────

// send writes a single length-prefixed frame to the TCP connection.
// On write failure it attempts one reconnect and retries once.
func (e *gnetExporter) send(signal byte, payload []byte) error {
	frame := e.buildFrame(signal, payload)

	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.writeFrame(frame); err != nil {
		e.settings.Logger.Warn("gnet write failed, reconnecting", zap.Error(err))
		if reconnErr := e.connectLocked(); reconnErr != nil {
			return fmt.Errorf("reconnect: %w", reconnErr)
		}
		if err2 := e.writeFrame(frame); err2 != nil {
			return fmt.Errorf("write after reconnect: %w", err2)
		}
	}
	return nil
}

func (e *gnetExporter) buildFrame(signal byte, payload []byte) []byte {
	frame := make([]byte, frameHeaderLen+len(payload))
	frame[0] = signal
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[frameHeaderLen:], payload)
	return frame
}

func (e *gnetExporter) writeFrame(frame []byte) error {
	if err := e.conn.SetWriteDeadline(time.Now().Add(e.cfg.WriteTimeout)); err != nil {
		return err
	}
	_, err := e.conn.Write(frame)
	return err
}

func (e *gnetExporter) connect() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.connectLocked()
}

// connectLocked dials the endpoint with exponential backoff.
// Must be called with e.mu held.
func (e *gnetExporter) connectLocked() error {
	if e.conn != nil {
		_ = e.conn.Close()
		e.conn = nil
	}

	delay := e.cfg.ReconnectDelay
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		conn, err := net.DialTimeout("tcp", e.cfg.Endpoint, e.cfg.DialTimeout)
		if err == nil {
			e.conn = conn
			e.settings.Logger.Info("gnet exporter connected",
				zap.String("endpoint", e.cfg.Endpoint))
			return nil
		}
		lastErr = err
		e.settings.Logger.Warn("gnet dial failed, retrying",
			zap.String("endpoint", e.cfg.Endpoint),
			zap.Duration("delay", delay),
			zap.Error(err))
		time.Sleep(delay)
		delay = time.Duration(math.Min(
			float64(delay)*2,
			float64(e.cfg.MaxReconnectDelay),
		))
	}
	return fmt.Errorf("could not connect to %s after 10 attempts: %w", e.cfg.Endpoint, lastErr)
}
