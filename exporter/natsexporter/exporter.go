// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// natsExporter publishes telemetry to a NATS server.
//
// NOTE: this is a skeleton. Connection management and the publish paths are
// intentionally unimplemented and land in follow-up PRs (see the component
// donation issue). The push methods are currently no-ops.
type natsExporter struct {
	config   *Config
	settings exporter.Settings
}

func newExporter(set exporter.Settings, cfg *Config) *natsExporter {
	return &natsExporter{
		config:   cfg,
		settings: set,
	}
}

func (e *natsExporter) start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *natsExporter) shutdown(_ context.Context) error {
	return nil
}

func (e *natsExporter) pushLogs(_ context.Context, _ plog.Logs) error {
	return nil
}

func (e *natsExporter) pushMetrics(_ context.Context, _ pmetric.Metrics) error {
	return nil
}

func (e *natsExporter) pushTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
}
