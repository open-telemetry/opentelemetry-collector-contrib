// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"

	"go.uber.org/zap"
)

type promLogger struct {
	realLog *zap.Logger
}

func newPromLogger(zapLog *zap.Logger) *promLogger {
	return &promLogger{
		realLog: zapLog,
	}
}

func (l *promLogger) Println(v ...interface{}) {
	l.realLog.Error(fmt.Sprintln(v...))
}
