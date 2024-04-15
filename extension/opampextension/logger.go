// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"context"

	"github.com/open-telemetry/opamp-go/client/types"
	"go.uber.org/zap"
)

var _ types.Logger = &opAMPLogger{}

type opAMPLogger struct {
	l *zap.SugaredLogger
}

// Debugf implements types.Logger.
func (o *opAMPLogger) Debugf(_ context.Context, format string, v ...any) {
	o.l.Debugf(format, v...)
}

// Errorf implements types.Logger.
func (o *opAMPLogger) Errorf(_ context.Context, format string, v ...any) {
	o.l.Errorf(format, v...)
}

func newLoggerFromZap(l *zap.Logger) types.Logger {
	return &opAMPLogger{
		l: l.Sugar(),
	}
}
