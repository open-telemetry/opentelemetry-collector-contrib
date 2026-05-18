// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/traces"

import (
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
)

// factories returns the minimal set of component factories needed by the
// traces correctness tests. Keeping it local avoids compiling components
// the traces tests never exercise.
func factories() (otelcol.Factories, error) {
	var errs error

	receivers, err := otelcol.MakeFactoryMap[receiver.Factory](
		jaegerreceiver.NewFactory(),
		otlpreceiver.NewFactory(),
		zipkinreceiver.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	exporters, err := otelcol.MakeFactoryMap[exporter.Factory](
		otlpexporter.NewFactory(),
		zipkinexporter.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	processors, err := otelcol.MakeFactoryMap[processor.Factory](
		batchprocessor.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	return otelcol.Factories{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
		Telemetry:  otelconftelemetry.NewFactory(),
	}, errs
}
