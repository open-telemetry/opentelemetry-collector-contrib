// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type azureBlobExporter struct {
	config *Config
	logger *zap.Logger
	signal pipeline.Signal
}

func newAzureBlobExporter(config *Config, logger *zap.Logger, signal pipeline.Signal) *azureBlobExporter {
	azBlobExporter := &azureBlobExporter{
		config: config,
		logger: logger,
		signal: signal,
	}
	return azBlobExporter
}

func (e *azureBlobExporter) start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *azureBlobExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *azureBlobExporter) ConsumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	return nil
}

func (e *azureBlobExporter) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}

func (e *azureBlobExporter) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
}
