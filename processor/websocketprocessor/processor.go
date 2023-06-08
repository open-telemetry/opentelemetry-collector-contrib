// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package websocketprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"
)

type wsprocessor struct {
	config            *Config
	telemetrySettings component.TelemetrySettings
	obsproc           *obsreport.Processor
	logsSink          consumer.Logs
	metricsSink       consumer.Metrics
	tracesSink        consumer.Traces
	server            *http.Server
	shutdownWG        sync.WaitGroup
	cs                *channelSet
}

var processors = map[*Config]*wsprocessor{}

var logMarshaler = &plog.JSONMarshaler{}
var metricMarshaler = &pmetric.JSONMarshaler{}
var traceMarshaler = &ptrace.JSONMarshaler{}

func newProcessor(settings processor.CreateSettings, config *Config) (*wsprocessor, error) {
	if p, ok := processors[config]; ok {
		return p, nil
	}
	obsproc, err := obsreport.NewProcessor(obsreport.ProcessorSettings{
		ProcessorID:             settings.ID,
		ProcessorCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	p := &wsprocessor{
		config:            config,
		obsproc:           obsproc,
		telemetrySettings: settings.TelemetrySettings,
		cs:                newChannelSet(),
	}
	processors[config] = p

	return p, nil
}

func (w *wsprocessor) Start(_ context.Context, host component.Host) error {
	var err error
	var ln net.Listener
	ln, err = w.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", w.config.Endpoint, err)
	}
	w.server, err = w.config.HTTPServerSettings.ToServer(host, w.telemetrySettings, websocket.Handler(w.handleConn))
	if err != nil {
		return err
	}
	w.shutdownWG.Add(1)
	go func() {
		defer w.shutdownWG.Done()
		if errHTTP := w.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			host.ReportFatalError(errHTTP)
		}
	}()
	return nil
}

func (w *wsprocessor) handleConn(conn *websocket.Conn) {
	err := conn.SetDeadline(time.Time{})
	if err != nil {
		w.telemetrySettings.Logger.Debug("Error setting deadline", zap.Error(err))
		return
	}
	ch := make(chan []byte)
	idx := w.cs.add(ch)
	for bytes := range ch {
		_, err := conn.Write(bytes)
		if err != nil {
			w.telemetrySettings.Logger.Debug("websocket write error: %w", zap.Error(err))
			w.cs.closeAndRemove(idx)
			break
		}
	}
}

func (w *wsprocessor) Shutdown(ctx context.Context) error {
	if w.server != nil {
		err := w.server.Shutdown(ctx)
		return err
	}
	return nil
}

func (w *wsprocessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (w *wsprocessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	b, err := metricMarshaler.MarshalMetrics(md)
	if err != nil {
		w.telemetrySettings.Logger.Debug("Error serializing to JSON", zap.Error(err))
	} else {
		w.cs.writeBytes(b)
	}
	return w.metricsSink.ConsumeMetrics(ctx, md)
}

func (w *wsprocessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	b, err := logMarshaler.MarshalLogs(ld)
	if err != nil {
		w.telemetrySettings.Logger.Debug("Error serializing to JSON", zap.Error(err))
	} else {
		w.cs.writeBytes(b)
	}
	return w.logsSink.ConsumeLogs(ctx, ld)
}

func (w *wsprocessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	b, err := traceMarshaler.MarshalTraces(td)
	if err != nil {
		w.telemetrySettings.Logger.Debug("Error serializing to JSON", zap.Error(err))
	} else {
		w.cs.writeBytes(b)
	}
	return w.tracesSink.ConsumeTraces(ctx, td)
}
