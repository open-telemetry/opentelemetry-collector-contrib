// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type azureBlobExporter struct {
	config *Config
	logger *zap.Logger
}

func newAzureBlobExporter(config *Config,
	params exporter.Settings) *azureBlobExporter {

	azBlobExporter := &azureBlobExporter{}
	return azBlobExporter
}

func (e *azureBlobExporter) start(_ context.Context, host component.Host) error {
	// TODO to be implemented
	return nil
}

func (e *azureBlobExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *azureBlobExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// TODO to be implemented
	return nil
}

func (e *azureBlobExporter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	// TODO to be implemented
	return nil
}

func (e *azureBlobExporter) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	// TODO to be implemented
	return nil
}
