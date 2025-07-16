// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/pmetric"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// Compile-time interface implementation check
var _ consumer.Metrics = (*routingMetricsExporter)(nil)

// routingMetricsExporter wraps the standard Datadog metrics exporter with routing capabilities
type routingMetricsExporter struct {
	datadogPushFunc consumer.ConsumeMetricsFunc
	otlpExporter    exporter.Metrics
	routingConfig   datadogconfig.MetricsRoutingConfig
}

// newRoutingMetricsExporter creates a new routing metrics exporter
func newRoutingMetricsExporter(
	datadogPushFunc consumer.ConsumeMetricsFunc,
	otlpExporter exporter.Metrics,
	routingConfig datadogconfig.MetricsRoutingConfig,
) *routingMetricsExporter {
	return &routingMetricsExporter{
		datadogPushFunc: datadogPushFunc,
		otlpExporter:    otlpExporter,
		routingConfig:   routingConfig,
	}
}

// ConsumeMetrics routes metrics based on the configured routing rules
func (r *routingMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if !r.routingConfig.Enabled || r.otlpExporter == nil {
		return r.datadogPushFunc(ctx, md)
	}

	// Check if any of the data should be routed to OTLP
	shouldRouteToOTLP := false
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			target := r.routingConfig.DetermineMetricRoute(resourceMetric, scopeMetric)
			if target == datadogconfig.TargetOTLP {
				shouldRouteToOTLP = true
				break
			}
		}
		if shouldRouteToOTLP {
			break
		}
	}

	// Route to appropriate exporter
	if shouldRouteToOTLP {
		// Route to OTLP endpoint
		if otlpConsumer, ok := r.otlpExporter.(consumer.Metrics); ok {
			return otlpConsumer.ConsumeMetrics(ctx, md)
		}
	}

	// Default to Datadog
	return r.datadogPushFunc(ctx, md)
}

// Capabilities returns the consumer capabilities
func (r *routingMetricsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// createOTLPMetricsExporter creates an OTLP HTTP exporter for metrics routing
func createOTLPMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	routingConfig datadogconfig.MetricsRoutingConfig,
	apiKey string,
) (exporter.Metrics, error) {
	if !routingConfig.Enabled || routingConfig.OTLPEndpoint == "" {
		return nil, nil
	}

	factory := otlphttpexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlphttpexporter.Config)

	// Configure endpoint
	cfg.ClientConfig.Endpoint = routingConfig.OTLPEndpoint

	// Configure headers
	cfg.ClientConfig.Headers = make(map[string]configopaque.String)

	// Add configured headers
	for key, value := range routingConfig.OTLPHeaders {
		cfg.ClientConfig.Headers[key] = value
	}

	// Add API key header dynamically
	if apiKey != "" {
		cfg.ClientConfig.Headers["Dd-Api-Key"] = configopaque.String(apiKey)
	}

	// Disable retries and queuing for routing to avoid double buffering
	cfg.RetryConfig.Enabled = false
	cfg.QueueConfig.Enabled = false

	return factory.CreateMetrics(ctx, set, cfg)
}
