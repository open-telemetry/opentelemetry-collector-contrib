// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
)

func TestExamples(t *testing.T) {
	t.Setenv("DD_API_KEY", "aaaaaaaaa")
	factories := newTestComponents(t)
	const configFile = "./examples/config.yaml"
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	// nolint:staticcheck
	_, err := otelcoltest.LoadConfigAndValidate(configFile, factories)
	require.NoError(t, err, "All yaml config must validate. Please ensure that all necessary component factories are added in newTestComponents()")
}

// newTestComponents returns the minimum amount of components necessary for
// running a collector with any of the examples/* yaml configuration files.
func newTestComponents(t *testing.T) otelcol.Factories {
	var (
		factories otelcol.Factories
		err       error
	)
	factories.Receivers, err = receiver.MakeFactoryMap(
		[]receiver.Factory{
			otlpreceiver.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Processors, err = processor.MakeFactoryMap(
		[]processor.Factory{
			batchprocessor.NewFactory(),
			tailsamplingprocessor.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Connectors, err = connector.MakeFactoryMap(
		[]connector.Factory{
			NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Exporters, err = exporter.MakeFactoryMap(
		[]exporter.Factory{
			datadogexporter.NewFactory(),
			debugexporter.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	return factories
}
