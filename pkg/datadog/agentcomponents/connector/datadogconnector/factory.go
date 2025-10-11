// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package agentcomponents

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/comp/core/tagger/types"
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/util/option"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient"
	"github.com/DataDog/datadog-agent/pkg/trace/stats"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/otel/metric/noop"
)

type factory struct {
	tagger       types.TaggerClient
	concentrator *stats.Concentrator
	hostname     option.Option[string]
}

// SourceProviderFunc is a function that returns the source of the host.
type SourceProviderFunc func(context.Context) (string, error)

// NewConnectorFactoryForAgent creates a factory for datadog connector for use in OTel agent
func NewConnectorFactoryForAgent(tagger types.TaggerClient, hostGetter SourceProviderFunc, concentrator *stats.Concentrator) connector.Factory {
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
		Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(f.createTracesToMetricsConnector, TracesToMetricsStability),
		connector.WithTracesToTraces(f.createTracesToTracesConnector, TracesToTracesStability))
}

// NewConnectorFactory creates a factory for datadog connector.
func NewConnectorFactory() connector.Factory {
	//  OTel connector factory to make a factory for connectors
	return NewConnectorFactoryForAgent(nil, nil, nil)
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

	params.Logger.Info("Building datadog connector for traces to metrics")
	statsout := make(chan *pb.StatsPayload, 100)
	params.MeterProvider = noop.NewMeterProvider() // disable metrics for the connector
	attributesTranslator, err := attributes.NewTranslator(params.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create attributes translator: %w", err)
	}
	trans, err := metrics.NewTranslator(params.TelemetrySettings, attributesTranslator)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics translator: %w", err)
	}

	tcfg := getTraceAgentCfg(params.Logger, cfg.(*Config).Traces, attributesTranslator, f.tagger, f.hostname)
	oconf := tcfg.Obfuscation.Export(tcfg)
	oconf.Statsd = metricsClient
	oconf.Redis.Enabled = true

	return &traceToMetricConnector{
		logger:          params.Logger,
		translator:      trans,
		tcfg:            tcfg,
		ctagKeys:        cfg.(*Config).Traces.ResourceAttributesAsContainerTags,
		peerTagKeys:     tcfg.ConfiguredPeerTags(),
		concentrator:    f.concentrator,
		statsout:        statsout,
		metricsConsumer: nextConsumer,
		obfuscator:      obfuscate.NewObfuscator(oconf),
		exit:            make(chan struct{}),
	}, nil
}

func (f *factory) createTracesToTracesConnector(_ context.Context, params connector.Settings, _ component.Config, nextConsumer consumer.Traces) (connector.Traces, error) {
	return newTraceToTraceConnector(params.Logger, nextConsumer), nil
}
