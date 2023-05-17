// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/parquetexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type parquetExporter struct {
	path string
}

func (e parquetExporter) start(ctx context.Context, host component.Host) error {
	return nil
}

func (e parquetExporter) shutdown(ctx context.Context) error {
	return nil
}

func (e parquetExporter) consumeMetrics(ctx context.Context, ld pmetric.Metrics) error {
	return nil
}

func (e parquetExporter) consumeTraces(ctx context.Context, ld ptrace.Traces) error {
	return nil
}

func (e parquetExporter) consumeLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}
