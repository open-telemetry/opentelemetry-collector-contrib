// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"
import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		func() component.Config {
			return nil
		},
		connector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
		connector.WithTracesToTraces(createTracesToTracesConnector, metadata.TracesToTracesStability),
	)
}

func createTracesToTracesConnector(context.Context, connector.Settings, component.Config, consumer.Traces) (connector.Traces, error) {
	return nil, errors.New("datadogconnector is not supported on aix")
}

func createTracesToMetricsConnector(context.Context, connector.Settings, component.Config, consumer.Metrics) (connector.Traces, error) {
	return nil, errors.New("datadogconnector is not supported on aix")
}
