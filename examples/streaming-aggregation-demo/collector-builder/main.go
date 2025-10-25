package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/forwardconnector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"
)

func main() {
	info := component.BuildInfo{
		Command:     "otelcol-streaming",
		Description: "OpenTelemetry Collector with Streaming Aggregation Processor",
		Version:     "0.96.0",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: func() (otelcol.Factories, error) {
			var err error
			factories := otelcol.Factories{}

			// Extensions
			factories.Extensions, err = extension.MakeFactoryMap(
				zpagesextension.NewFactory(),
			)
			if err != nil {
				return otelcol.Factories{}, err
			}

			// Receivers
			factories.Receivers, err = receiver.MakeFactoryMap(
				otlpreceiver.NewFactory(),
			)
			if err != nil {
				return otelcol.Factories{}, err
			}

			// Processors
			factories.Processors, err = processor.MakeFactoryMap(
				streamingaggregationprocessor.NewFactory(),
				deltatocumulativeprocessor.NewFactory(),
			)
			if err != nil {
				return otelcol.Factories{}, err
			}

			// Exporters
			factories.Exporters, err = exporter.MakeFactoryMap(
				debugexporter.NewFactory(),
				prometheusexporter.NewFactory(),
				prometheusremotewriteexporter.NewFactory(),
			)
			if err != nil {
				return otelcol.Factories{}, err
			}

			// Connectors
			factories.Connectors, err = connector.MakeFactoryMap(
				forwardconnector.NewFactory(),
			)
			if err != nil {
				return otelcol.Factories{}, err
			}

			return factories, nil
		},
	}

	if err := run(set); err != nil {
		log.Fatal(err)
	}
}

func run(settings otelcol.CollectorSettings) error {
	cmd := otelcol.NewCommand(settings)
	return cmd.Execute()
}
