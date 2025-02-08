// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"
	"math/rand"
	"text/template"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type azureBlobExporter struct {
	config           *Config
	logger           *zap.Logger
	client           *azblob.Client
	signal           pipeline.Signal
	marshaller       *marshaller
	blobNameTemplate *template.Template
}

var fileExtensionMap = map[string]string{
	formatTypeJSON:  "json",
	formatTypeProto: "pb",
}

func newAzureBlobExporter(config *Config, logger *zap.Logger, signal pipeline.Signal) *azureBlobExporter {
	azBlobExporter := &azureBlobExporter{
		config: config,
		logger: logger,
		signal: signal,
	}
	return azBlobExporter
}

func randomInRange(low, hi int) int {
	return low + rand.Intn(hi-low)
}

func (e *azureBlobExporter) start(_ context.Context, host component.Host) error {
	return nil
}

func (e *azureBlobExporter) generateBlobName(data map[string]interface{}) (string, error) {
	return "", nil
}

func (e *azureBlobExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *azureBlobExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (e *azureBlobExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}

func (e *azureBlobExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}
