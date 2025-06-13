// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"

import (
	"fmt"

	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"go.uber.org/zap"
)

var _ tracelog.Logger = &Zaplogger{}

// Zaplogger implements the tracelog.Logger interface on top of a zap.Logger
type Zaplogger struct {
	// Logger is the internal zap logger
	Logger *zap.Logger
}

// Trace implements Logger.
func (z *Zaplogger) Trace(_ ...any) { /* N/A */ }

// Tracef implements Logger.
func (z *Zaplogger) Tracef(_ string, _ ...any) { /* N/A */ }

// Debug implements Logger.
func (z *Zaplogger) Debug(v ...any) {
	z.Logger.Debug(fmt.Sprint(v...))
}

// Debugf implements Logger.
func (z *Zaplogger) Debugf(format string, params ...any) {
	z.Logger.Debug(fmt.Sprintf(format, params...))
}

// Info implements Logger.
func (z *Zaplogger) Info(v ...any) {
	z.Logger.Info(fmt.Sprint(v...))
}

// Infof implements Logger.
func (z *Zaplogger) Infof(format string, params ...any) {
	z.Logger.Info(fmt.Sprintf(format, params...))
}

// Warn implements Logger.
func (z *Zaplogger) Warn(v ...any) error {
	z.Logger.Warn(fmt.Sprint(v...))
	return nil
}

// Warnf implements Logger.
func (z *Zaplogger) Warnf(format string, params ...any) error {
	z.Logger.Warn(fmt.Sprintf(format, params...))
	return nil
}

// Error implements Logger.
func (z *Zaplogger) Error(v ...any) error {
	z.Logger.Error(fmt.Sprint(v...))
	return nil
}

// Errorf implements Logger.
func (z *Zaplogger) Errorf(format string, params ...any) error {
	z.Logger.Error(fmt.Sprintf(format, params...))
	return nil
}

// Critical implements Logger.
func (z *Zaplogger) Critical(v ...any) error {
	z.Logger.Error(fmt.Sprint(v...), zap.Bool("critical", true))
	return nil
}

// Criticalf implements Logger.
func (z *Zaplogger) Criticalf(format string, params ...any) error {
	z.Logger.Error(fmt.Sprintf(format, params...), zap.Bool("critical", true))
	return nil
}

// Flush implements Logger.
func (z *Zaplogger) Flush() {
	_ = z.Logger.Sync()
}
