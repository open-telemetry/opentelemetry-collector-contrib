// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type storageExporter struct {
	// TODO
}

var _ exporter.Logs = (*storageExporter)(nil)

func newStorageExporter(
	_ context.Context,
	_ *Config,
	_ *zap.Logger,
) *storageExporter {
	// TODO
	return &storageExporter{}
}

func (*storageExporter) Start(_ context.Context, _ component.Host) error {
	// TODO
	return nil
}

func (*storageExporter) Shutdown(_ context.Context) error {
	return nil
}

func (*storageExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (*storageExporter) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	// TODO
	return nil
}
