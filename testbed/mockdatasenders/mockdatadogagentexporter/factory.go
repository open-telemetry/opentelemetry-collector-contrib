// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mockdatadogagentexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// This file implements factory for awsxray receiver.

// The value of "type" key in configuration.
var compType = component.MustNewType("datadog")

func NewFactory() exporter.Factory {
	return exporter.NewFactory(compType,
		createDefaultConfig,
		exporter.WithTraces(CreateTracesExporter, component.StabilityLevelAlpha))
}

// CreateDefaultConfig creates the default configuration for DDAPM Exporter
func createDefaultConfig() component.Config {
	client := confighttp.NewDefaultClientConfig()
	client.Endpoint = "localhost:8126"
	return client
}

func CreateTracesExporter(
	_ context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	c := cfg.(*Config)
	if c.Endpoint == "" {
		// TODO https://github.com/open-telemetry/opentelemetry-collector/issues/215
		return nil, errors.New("exporter config requires a non-empty 'endpoint'")
	}

	dd := createExporter(c)
	err := dd.start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTraces(
		context.Background(),
		set,
		dd.pushTraces,
		consumer.ConsumeTracesFunc(dd.pushTraces),
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
	)
}
