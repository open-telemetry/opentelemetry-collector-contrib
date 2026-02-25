// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apmstats // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apmstats"

import (
	"context"
	"time"

	"github.com/DataDog/datadog-agent/comp/core/tagger/types"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient"
	"github.com/DataDog/datadog-agent/pkg/trace/stats"
	"github.com/DataDog/datadog-agent/pkg/util/option"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

type factory struct {
	tagger       types.TaggerClient
	concentrator *stats.Concentrator
	hostname     option.Option[string]
}

// SourceProviderFunc is a function that returns the source of the host.
type SourceProviderFunc func(context.Context) (string, error)

// NewConnectorFactory creates a factory for datadog connector for use in OTel agent
func NewConnectorFactory(componentType component.Type, metricsStability, traceStability component.StabilityLevel, tagger types.TaggerClient, hostGetter SourceProviderFunc, concentrator *stats.Concentrator) connector.Factory {
	f := &factory{
		tagger:       tagger,
		concentrator: concentrator,
	}

	if hostGetter != nil {
		if hostname, err := hostGetter(context.Background()); err == nil {
			f.hostname = option.New(hostname)
		}
	}

	//  OTel connector factory to make a factory for connectors
	return connector.NewFactory(
		componentType,
		createDefaultConfig,
		connector.WithTracesToMetrics(f.createTracesToMetricsConnector, metricsStability),
		connector.WithTracesToTraces(createTracesToTracesConnector, traceStability))
}

func createDefaultConfig() component.Config {
	return &datadogconfig.ConnectorComponentConfig{
		Traces: datadogconfig.TracesConnectorConfig{
			TracesConfig: datadogconfig.TracesConfig{
				IgnoreResources:        []string{},
				PeerServiceAggregation: true,
				PeerTagsAggregation:    true,
				ComputeStatsBySpanKind: true,
			},

			TraceBuffer:    1000,
			BucketInterval: 10 * time.Second,
		},
	}
}

// defines the consumer type of the connector
// we want to consume traces and export metrics therefore define nextConsumer as metrics, consumer is the next component in the pipeline
func (f *factory) createTracesToMetricsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (c connector.Traces, err error) {
	metricsClient, err := metricsclient.InitializeMetricClient(params.MeterProvider, metricsclient.ConnectorSourceTag)
	if err != nil {
		return nil, err
	}
	c, err = newTraceToMetricConnector(params.TelemetrySettings, cfg, nextConsumer, metricsClient, f.concentrator, f.tagger, f.hostname)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func createTracesToTracesConnector(_ context.Context, params connector.Settings, _ component.Config, nextConsumer consumer.Traces) (connector.Traces, error) {
	return newTraceToTraceConnector(params.Logger, nextConsumer), nil
}
