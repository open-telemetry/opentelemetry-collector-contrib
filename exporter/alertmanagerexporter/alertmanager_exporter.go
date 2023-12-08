// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type alertmanagerExporter struct {
	config            *Config
	tracesMarshaler   ptrace.Marshaler
	settings          component.TelemetrySettings
	endpoint          string
	generatorURL      string
	defaultSeverity   string
	severityAttribute string
}

func (s *alertmanagerExporter) pushTraces(_ context.Context, _ ptrace.Traces) error {

	// To Be Implemented
	return nil
}

func (s *alertmanagerExporter) start(_ context.Context, _ component.Host) error {

	// To Be Implemented
	return nil
}

func (s *alertmanagerExporter) shutdown(context.Context) error {
	// To Be Implemented
	return nil
}

func newAlertManagerExporter(cfg *Config, set component.TelemetrySettings) *alertmanagerExporter {

	return &alertmanagerExporter{
		config:            cfg,
		settings:          set,
		tracesMarshaler:   &ptrace.JSONMarshaler{},
		endpoint:          cfg.HTTPClientSettings.Endpoint,
		generatorURL:      cfg.GeneratorURL,
		defaultSeverity:   cfg.DefaultSeverity,
		severityAttribute: cfg.SeverityAttribute,
	}
}

func newTracesExporter(ctx context.Context, cfg component.Config, set exporter.CreateSettings) (exporter.Traces, error) {
	config := cfg.(*Config)

	s := newAlertManagerExporter(config, set.TelemetrySettings)

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		s.pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithStart(s.start),
		exporterhelper.WithTimeout(config.TimeoutSettings),
		exporterhelper.WithRetry(config.RetrySettings),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithShutdown(s.shutdown),
	)
}
