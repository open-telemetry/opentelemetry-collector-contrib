// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type tinybirdExporter struct{}

func newExporter(cfg component.Config, set exporter.Settings) (*tinybirdExporter, error) {
	return &tinybirdExporter{}, nil
}

func (e *tinybirdExporter) start(ctx context.Context, host component.Host) error {
	return nil
}

func (e *tinybirdExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}

func (e *tinybirdExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (e *tinybirdExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}
