// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
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

func (e *honeycombLogsExporter) start(_ context.Context, _ component.Host) error {

	return nil
}

func (e *honeycombLogsExporter) shutdown(_ context.Context) error {
	return nil
}

func (e *honeycombLogsExporter) pushLogsData(_ context.Context, _ plog.Logs) error {
	return nil
}
