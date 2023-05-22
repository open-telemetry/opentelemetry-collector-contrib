// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"

import (
	"github.com/signalfx/signalfx-agent/pkg/apm/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapShim struct {
	log *zap.Logger
}

func newZapShim(log *zap.Logger) zapShim {
	// Add caller skip so that the shim isn't logged as the caller.
	return zapShim{log.WithOptions(zap.AddCallerSkip(1))}
}

func (z zapShim) Debug(msg string) {
	z.log.Debug(msg)
}

func (z zapShim) Warn(msg string) {
	z.log.Warn(msg)
}

func (z zapShim) Error(msg string) {
	z.log.Error(msg)
}

func (z zapShim) Info(msg string) {
	z.log.Info(msg)
}

func (z zapShim) Panic(msg string) {
	z.log.Panic(msg)
}

func (z zapShim) WithFields(fields log.Fields) log.Logger {
	f := make([]zapcore.Field, 0, len(fields))
	for k, v := range fields {
		f = append(f, zap.Any(k, v))
	}
	return zapShim{z.log.With(f...)}
}

func (z zapShim) WithError(err error) log.Logger {
	return zapShim{z.log.With(zap.Error(err))}
}

var _ log.Logger = (*zapShim)(nil)
