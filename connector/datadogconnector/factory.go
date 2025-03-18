// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"context"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient"
	"github.com/DataDog/datadog-agent/pkg/trace/timing"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

const nativeIngestFeatureGateName = "connector.datadogconnector.NativeIngest"

// NativeIngestFeatureGate is the feature gate that controls native OTel spans ingestion in Datadog APM stats
var NativeIngestFeatureGate = featuregate.GlobalRegistry().MustRegister(
	nativeIngestFeatureGateName,
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, datadogconnector uses the native OTel API to ingest OTel spans and produce APM stats."),
	featuregate.WithRegisterFromVersion("v0.104.0"),
)

// NewFactory creates a factory for tailtracer connector.
func NewFactory() connector.Factory {
	//  OTel connector factory to make a factory for connectors
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
		connector.WithTracesToTraces(createTracesToTracesConnector, metadata.TracesToTracesStability))
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
func createTracesToMetricsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (c connector.Traces, err error) {
	metricsClient := metricsclient.InitializeMetricClient(params.MeterProvider, metricsclient.ConnectorSourceTag)
	if NativeIngestFeatureGate.IsEnabled() {
		params.Logger.Info("Datadog connector using the native OTel API to ingest OTel spans and produce APM stats. To revert to the legacy processing pipeline, disable the feature gate", zap.String("feature gate", nativeIngestFeatureGateName))
		c, err = newTraceToMetricConnectorNative(params.TelemetrySettings, cfg, nextConsumer, metricsClient)
	} else {
		params.Logger.Info("Datadog connector using the old processing pipelines to ingest OTel spans and produce APM stats.")
		c, err = newTraceToMetricConnector(params.TelemetrySettings, cfg, nextConsumer, metricsClient, timing.New(metricsClient))
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func createTracesToTracesConnector(_ context.Context, params connector.Settings, _ component.Config, nextConsumer consumer.Traces) (connector.Traces, error) {
	return newTraceToTraceConnector(params.Logger, nextConsumer), nil
}
