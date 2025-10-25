// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

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
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

const nativeIngestFeatureGateName = "connector.datadogconnector.NativeIngest"

// NativeIngestFeatureGate is the feature gate that controls native OTel spans ingestion in Datadog APM stats
var NativeIngestFeatureGate = featuregate.GlobalRegistry().MustRegister(
	nativeIngestFeatureGateName,
	featuregate.StageStable,
	featuregate.WithRegisterDescription("When enabled, datadogconnector uses the native OTel API to ingest OTel spans and produce APM stats."),
	featuregate.WithRegisterFromVersion("v0.104.0"),
	featuregate.WithRegisterToVersion("v0.143.0"),
)

type factory struct {
	tagger       types.TaggerClient
	concentrator *stats.Concentrator
	hostname     option.Option[string]
}

// SourceProviderFunc is a function that returns the source of the host.
type SourceProviderFunc func(context.Context) (string, error)

// NewFactoryForAgent creates a factory for datadog connector for use in OTel agent
func NewFactoryForAgent(tagger types.TaggerClient, hostGetter SourceProviderFunc, concentrator *stats.Concentrator) connector.Factory {
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
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(f.createTracesToMetricsConnector, metadata.TracesToMetricsStability),
		connector.WithTracesToTraces(createTracesToTracesConnector, metadata.TracesToTracesStability))
}

// NewConnectorFactory creates a factory for datadog connector.
func NewFactory() connector.Factory {
	//  OTel connector factory to make a factory for connectors
	return NewFactoryForAgent(nil, nil, nil)
}

func createDefaultConfig() component.Config {
	return &Config{
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
