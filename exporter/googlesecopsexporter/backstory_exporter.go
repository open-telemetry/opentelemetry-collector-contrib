// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter/internal/metadata"
)

type backstoryExporter struct{}

func newBackstoryAPIExporter(_ *Config, _ exporter.Settings, _ *metadata.TelemetryBuilder) (*backstoryExporter, error) {
	return &backstoryExporter{}, nil
}

func (*backstoryExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (*backstoryExporter) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*backstoryExporter) Shutdown(_ context.Context) error {
	return nil
}

// ConsumeLogs sends logs to the Backstory API via gRPC.
func (*backstoryExporter) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
