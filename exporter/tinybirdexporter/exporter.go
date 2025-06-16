// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import (
	"context"
	"errors"

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
	return errors.New("this component is under development and traces are not yet supported, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40475 to track development progress")
}

func (e *tinybirdExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	return errors.New("this component is under development and metrics are not yet supported, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40475 to track development progress")
}

func (e *tinybirdExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	return errors.New("this component is under development and logs are not yet supported, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40475 to track development progress")
}
