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

func newExporter(_ component.Config, _ exporter.Settings) (*tinybirdExporter, error) {
	return &tinybirdExporter{}, nil
}

func (e *tinybirdExporter) start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *tinybirdExporter) pushTraces(_ context.Context, _ ptrace.Traces) error {
	return errors.New("this component is under development and traces are not yet supported, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40475 to track development progress")
}

func (e *tinybirdExporter) pushMetrics(_ context.Context, _ pmetric.Metrics) error {
	return errors.New("this component is under development and metrics are not yet supported, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40475 to track development progress")
}

func (e *tinybirdExporter) pushLogs(_ context.Context, _ plog.Logs) error {
	return errors.New("this component is under development and logs are not yet supported, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40475 to track development progress")
}
