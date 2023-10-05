// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type honeycombLogsExporter struct {
	logger *zap.Logger
}

func newLogsExporter(logger *zap.Logger, _ *Config) *honeycombLogsExporter {
	logsExp := &honeycombLogsExporter{
		logger: logger,
	}
	return logsExp
}

func (e *honeycombLogsExporter) exportLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
