// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package supervisor

import (
	"flag"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

type windowsService struct {
	sup *Supervisor
}

func NewSvcHandler() svc.Handler {
	return &windowsService{}
}

func (ws *windowsService) Execute(args []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	// The first argument supplied to service.Execute is the service name. If this is
	// not provided for some reason, raise a relevant error to the system event log
	if len(args) == 0 {
		return false, uint32(windows.ERROR_INVALID_SERVICENAME)
	}

	elog, err := openEventLog(args[0])
	if err != nil {
		return false, uint32(windows.ERROR_EVENTLOG_CANT_START)
	}

	changes <- svc.Status{State: svc.StartPending}
	if err = ws.start(elog); err != nil {
		_ = elog.Error(3, fmt.Sprintf("failed to start service: %v", err))
		return false, uint32(windows.ERROR_EXCEPTION_IN_SERVICE)
	}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for req := range requests {
		switch req.Cmd {
		case svc.Interrogate:
			changes <- req.CurrentStatus

		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			ws.stop()
			changes <- svc.Status{State: svc.Stopped}
			return false, 0

		default:
			_ = elog.Error(3, fmt.Sprintf("unexpected service control request #%d", req.Cmd))
			return false, uint32(windows.ERROR_INVALID_SERVICE_CONTROL)
		}
	}

	return false, 0
}

func (ws *windowsService) start(elog *eventlog.Log) error {
	configFlag := flag.String("config", "", "Path to a supervisor configuration file")
	flag.Parse()

	logger, _ := zap.NewDevelopment(zap.WrapCore(withWindowsCore(elog)))

	cfg, err := config.Load(*configFlag)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	sup, err := NewSupervisor(logger, cfg)
	if err != nil {
		return fmt.Errorf("new supervisor: %w", err)
	}
	ws.sup = sup

	return ws.sup.Start()
}

func (ws *windowsService) stop() {
	ws.sup.Shutdown()
}

func openEventLog(serviceName string) (*eventlog.Log, error) {
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return nil, fmt.Errorf("service failed to open event log: %w", err)
	}

	return elog, nil
}

// Logger wrappings
var _ zapcore.Core = (*windowsEventLogCore)(nil)

type windowsEventLogCore struct {
	core    zapcore.Core
	elog    *eventlog.Log
	encoder zapcore.Encoder
}

func (w windowsEventLogCore) Enabled(level zapcore.Level) bool {
	return w.core.Enabled(level)
}

func (w windowsEventLogCore) With(fields []zapcore.Field) zapcore.Core {
	enc := w.encoder.Clone()
	for _, field := range fields {
		field.AddTo(enc)
	}
	return windowsEventLogCore{
		core:    w.core,
		elog:    w.elog,
		encoder: enc,
	}
}

func (w windowsEventLogCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if w.Enabled(ent.Level) {
		return ce.AddCore(ent, w)
	}
	return ce
}

func (w windowsEventLogCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	buf, err := w.encoder.EncodeEntry(ent, fields)
	if err != nil {
		_ = w.elog.Warning(2, fmt.Sprintf("failed encoding log entry %v\r\n", err))
		return err
	}
	msg := buf.String()
	buf.Free()

	switch ent.Level {
	case zapcore.FatalLevel, zapcore.PanicLevel, zapcore.DPanicLevel:
		// golang.org/x/sys/windows/svc/eventlog does not support Critical level event logs
		return w.elog.Error(3, msg)
	case zapcore.ErrorLevel:
		return w.elog.Error(3, msg)
	case zapcore.WarnLevel:
		return w.elog.Warning(2, msg)
	case zapcore.InfoLevel:
		return w.elog.Info(1, msg)
	}
	// We would not be here if debug were disabled so log as info to not drop.
	return w.elog.Info(1, msg)
}

func (w windowsEventLogCore) Sync() error {
	return w.core.Sync()
}

// TODO: If supervisor logging becomes configurable, update this function to respect that config
func withWindowsCore(elog *eventlog.Log) func(zapcore.Core) zapcore.Core {
	return func(core zapcore.Core) zapcore.Core {
		// Use the Windows Event Log
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.LineEnding = "\r\n"
		return windowsEventLogCore{core, elog, zapcore.NewConsoleEncoder(encoderConfig)}
	}
}
