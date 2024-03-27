// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type rabbitmqExporter struct {
	config   *Config
	settings component.TelemetrySettings
}

func newRabbitmqExporter(cfg *Config, set component.TelemetrySettings) *rabbitmqExporter {
	return &rabbitmqExporter{
		config:   cfg,
		settings: set,
	}
}

func (s *rabbitmqExporter) start(_ context.Context, _ component.Host) error {

	// To Be Implemented
	return nil
}

func (s *rabbitmqExporter) pushTraces(_ context.Context, _ ptrace.Traces) error {

	// To Be Implemented
	return nil
}

func (s *rabbitmqExporter) pushMetrics(_ context.Context, _ pmetric.Metrics) error {

	// To Be Implemented
	return nil
}

func (s *rabbitmqExporter) pushLogs(_ context.Context, _ plog.Logs) error {

	// To Be Implemented
	return nil
}

func (s *rabbitmqExporter) shutdown(_ context.Context) error {
	// To Be Implemented
	return nil
}
