// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/metrics"

import (
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"
)

// factories returns the minimal set of component factories needed by the
// metrics correctness tests. Keeping it local avoids compiling components
// the metrics tests never exercise.
func factories() (otelcol.Factories, error) {
	var errs error

	receivers, err := otelcol.MakeFactoryMap[receiver.Factory](
		otlpreceiver.NewFactory(),
		stefreceiver.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	exporters, err := otelcol.MakeFactoryMap[exporter.Factory](
		otlpexporter.NewFactory(),
		stefexporter.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	return otelcol.Factories{
		Receivers: receivers,
		Exporters: exporters,
		Telemetry: otelconftelemetry.NewFactory(),
	}, errs
}
