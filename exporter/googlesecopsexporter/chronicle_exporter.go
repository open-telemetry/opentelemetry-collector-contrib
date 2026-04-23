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

type chronicleAPIExporter struct{}

func newChronicleAPIExporter(_ *Config, _ exporter.Settings, _ *metadata.TelemetryBuilder) (*chronicleAPIExporter, error) {
	return &chronicleAPIExporter{}, nil
}

func (*chronicleAPIExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (*chronicleAPIExporter) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*chronicleAPIExporter) Shutdown(_ context.Context) error {
	return nil
}

// ConsumeLogs sends logs to Chronicle via HTTP.
func (*chronicleAPIExporter) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
