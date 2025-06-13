// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package agentcomponents // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"

import (
	"fmt"

	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"go.uber.org/zap"
)

var _ tracelog.Logger = &ZapLogger{}

// ZapLogger implements the tracelog.Logger interface on top of a zap.Logger
type ZapLogger struct {
	// Logger is the internal zap logger
	Logger *zap.Logger
}

// Trace implements Logger.
func (z *ZapLogger) Trace(_ ...any) { /* N/A */ }

// Tracef implements Logger.
func (z *ZapLogger) Tracef(_ string, _ ...any) { /* N/A */ }

// Debug implements Logger.
func (z *ZapLogger) Debug(v ...any) {
	z.Logger.Debug(fmt.Sprint(v...))
}

// Debugf implements Logger.
func (z *ZapLogger) Debugf(format string, params ...any) {
	z.Logger.Debug(fmt.Sprintf(format, params...))
}

// Info implements Logger.
func (z *ZapLogger) Info(v ...any) {
	z.Logger.Info(fmt.Sprint(v...))
}

// Infof implements Logger.
func (z *ZapLogger) Infof(format string, params ...any) {
	z.Logger.Info(fmt.Sprintf(format, params...))
}

// Warn implements Logger.
func (z *ZapLogger) Warn(v ...any) error {
	z.Logger.Warn(fmt.Sprint(v...))
	return nil
}

// Warnf implements Logger.
func (z *ZapLogger) Warnf(format string, params ...any) error {
	z.Logger.Warn(fmt.Sprintf(format, params...))
	return nil
}

// Error implements Logger.
func (z *ZapLogger) Error(v ...any) error {
	z.Logger.Error(fmt.Sprint(v...))
	return nil
}

// Errorf implements Logger.
func (z *ZapLogger) Errorf(format string, params ...any) error {
	z.Logger.Error(fmt.Sprintf(format, params...))
	return nil
}

// Critical implements Logger.
func (z *ZapLogger) Critical(v ...any) error {
	z.Logger.Error(fmt.Sprint(v...), zap.Bool("critical", true))
	return nil
}

// Criticalf implements Logger.
func (z *ZapLogger) Criticalf(format string, params ...any) error {
	z.Logger.Error(fmt.Sprintf(format, params...), zap.Bool("critical", true))
	return nil
}

// Flush implements Logger.
func (z *ZapLogger) Flush() {
	_ = z.Logger.Sync()
}
