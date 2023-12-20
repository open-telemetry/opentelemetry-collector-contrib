// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
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
	server            *http.Server
	shutdownWG        sync.WaitGroup
	cs                *channelSet
}

var logMarshaler = &plog.JSONMarshaler{}
var metricMarshaler = &pmetric.JSONMarshaler{}
var traceMarshaler = &ptrace.JSONMarshaler{}

func newProcessor(settings processor.CreateSettings, config *Config) *wsprocessor {
	return &wsprocessor{
		config:            config,
		telemetrySettings: settings.TelemetrySettings,
		cs:                newChannelSet(),
	}
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

func (w *wsprocessor) ConsumeMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	b, err := metricMarshaler.MarshalMetrics(md)
	if err != nil {
		w.telemetrySettings.Logger.Debug("Error serializing to JSON", zap.Error(err))
	} else {
		w.cs.writeBytes(b)
	}
	return md, nil
}

func (w *wsprocessor) ConsumeLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	b, err := logMarshaler.MarshalLogs(ld)
	if err != nil {
		w.telemetrySettings.Logger.Debug("Error serializing to JSON", zap.Error(err))
	} else {
		w.cs.writeBytes(b)
	}
	return ld, nil
}

func (w *wsprocessor) ConsumeTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	b, err := traceMarshaler.MarshalTraces(td)
	if err != nil {
		w.telemetrySettings.Logger.Debug("Error serializing to JSON", zap.Error(err))
	} else {
		w.cs.writeBytes(b)
	}
	return td, nil
}
