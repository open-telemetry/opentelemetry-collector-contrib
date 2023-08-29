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

func (e parquetExporter) start(_ context.Context, _ component.Host) error {
	return nil
}

func (e parquetExporter) shutdown(_ context.Context) error {
	return nil
}

func (e parquetExporter) consumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	return nil
}

func (e parquetExporter) consumeTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
}

func (e parquetExporter) consumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
