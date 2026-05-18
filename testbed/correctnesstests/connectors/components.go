// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connectors

import (
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"
)

// factories returns the minimal set of component factories needed by the
// connectors correctness tests. Keeping it local avoids compiling
// components the connectors tests never exercise.
func factories() (otelcol.Factories, error) {
	var errs error

	receivers, err := otelcol.MakeFactoryMap[receiver.Factory](
		otlpreceiver.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	exporters, err := otelcol.MakeFactoryMap[exporter.Factory](
		otlpexporter.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	processors, err := otelcol.MakeFactoryMap[processor.Factory](
		batchprocessor.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	connectors, err := otelcol.MakeFactoryMap[connector.Factory](
		routingconnector.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	return otelcol.Factories{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
		Connectors: connectors,
		Telemetry:  otelconftelemetry.NewFactory(),
	}, errs
}
