// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type honeycombLogsExporter struct {
	logger  *zap.Logger
	markers []Marker
}

func newLogsExporter(logger *zap.Logger, config *Config) *honeycombLogsExporter {
	logsExp := &honeycombLogsExporter{
		logger:  logger,
		markers: config.Markers,
	}
	return logsExp
}

func (e *honeycombLogsExporter) exportLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
