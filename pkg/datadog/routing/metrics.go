// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routing // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/routing"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// MetricsRouter provides metrics routing capabilities between Datadog and OTLP endpoints
type MetricsRouter interface {
	consumer.Metrics
	component.Component
}

// MetricsRouterSettings contains configuration for creating a metrics router
type MetricsRouterSettings struct {
	// DatadogPushFunc is the function to push metrics to Datadog
	DatadogPushFunc consumer.ConsumeMetricsFunc
	// RoutingConfig contains the routing configuration
	RoutingConfig config.MetricsRoutingConfig
	// APIKey is the Datadog API key for OTLP routing
	APIKey string
	// Site is the Datadog site for OTLP routing
	Site string
	// Logger for the router
	Logger *zap.Logger
	// ExporterSettings for creating OTLP exporter
	ExporterSettings exporter.Settings
}

// metricsRouter implements MetricsRouter interface
type metricsRouter struct {
	datadogPushFunc consumer.ConsumeMetricsFunc
	otlpExporter    exporter.Metrics
	routingConfig   config.MetricsRoutingConfig
	logger          *zap.Logger
}

// NewMetricsRouter creates a new metrics router with the provided settings
func NewMetricsRouter(ctx context.Context, settings MetricsRouterSettings) (MetricsRouter, error) {
	var otlpExporter exporter.Metrics
	var err error

	// Create OTLP exporter if routing is enabled
	if settings.RoutingConfig.Enabled {
		otlpExporter, err = createOTLPMetricsExporter(
			ctx,
			settings.ExporterSettings,
			settings.RoutingConfig,
			settings.APIKey,
			settings.Site,
		)
		if err != nil {
			settings.Logger.Error("Failed to create OTLP metrics exporter for routing", zap.Error(err))
			return nil, err
		}
	}

	router := &metricsRouter{
		datadogPushFunc: settings.DatadogPushFunc,
		otlpExporter:    otlpExporter,
		routingConfig:   settings.RoutingConfig,
		logger:          settings.Logger,
	}

	settings.Logger.Info("Created metrics router",
		zap.Bool("routing_enabled", settings.RoutingConfig.Enabled),
		zap.String("otlp_endpoint", settings.RoutingConfig.OTLPEndpoint),
		zap.Bool("has_otlp_exporter", otlpExporter != nil))

	return router, nil
}

// ConsumeMetrics routes metrics based on the configured routing rules
func (r *metricsRouter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if !r.routingConfig.Enabled || r.otlpExporter == nil {
		r.logger.Debug("Routing disabled or no OTLP exporter, using standard Datadog exporter",
			zap.Bool("routing_enabled", r.routingConfig.Enabled),
			zap.Bool("has_otlp_exporter", r.otlpExporter != nil))
		return r.datadogPushFunc(ctx, md)
	}

	// Check if any of the data should be routed to OTLP
	shouldRouteToOTLP := false
	resourceCount := md.ResourceMetrics().Len()
	totalScopeMetrics := 0

	for i := 0; i < resourceCount; i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		scopeMetricsCount := resourceMetric.ScopeMetrics().Len()
		totalScopeMetrics += scopeMetricsCount

		for j := 0; j < scopeMetricsCount; j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			target := r.routingConfig.DetermineMetricRoute(resourceMetric, scopeMetric)

			r.logger.Debug("Evaluated metric routing",
				zap.Int("resource_index", i),
				zap.Int("scope_index", j),
				zap.String("target", string(target)),
				zap.Int("metrics_count", scopeMetric.Metrics().Len()))

			if target == config.TargetOTLP {
				shouldRouteToOTLP = true
				r.logger.Debug("Found metrics that should be routed to OTLP")
				break
			}
		}
		if shouldRouteToOTLP {
			break
		}
	}

	// Route to appropriate exporter
	if shouldRouteToOTLP {
		r.logger.Info("Routing metrics to OTLP endpoint",
			zap.Int("resource_metrics_count", resourceCount),
			zap.Int("total_scope_metrics", totalScopeMetrics),
			zap.String("otlp_endpoint", r.routingConfig.OTLPEndpoint))

		// Route to OTLP endpoint
		if otlpConsumer, ok := r.otlpExporter.(consumer.Metrics); ok {
			err := otlpConsumer.ConsumeMetrics(ctx, md)
			if err != nil {
				r.logger.Error("Failed to route metrics to OTLP endpoint",
					zap.Error(err),
					zap.String("otlp_endpoint", r.routingConfig.OTLPEndpoint))
				return err
			}
			r.logger.Debug("Successfully routed metrics to OTLP endpoint")
			return nil
		} else {
			r.logger.Warn("OTLP exporter does not implement consumer.Metrics interface, falling back to Datadog")
		}
	} else {
		r.logger.Debug("No metrics matched OTLP routing rules, using standard Datadog exporter",
			zap.Int("resource_metrics_count", resourceCount),
			zap.Int("total_scope_metrics", totalScopeMetrics))
	}

	// Default to Datadog
	r.logger.Debug("Routing metrics to Datadog")
	return r.datadogPushFunc(ctx, md)
}

// Capabilities returns the consumer capabilities
func (r *metricsRouter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start starts the metrics router and any associated exporters
func (r *metricsRouter) Start(ctx context.Context, host component.Host) error {
	if r.otlpExporter != nil {
		return r.otlpExporter.Start(ctx, host)
	}
	return nil
}

// Shutdown shuts down the metrics router and any associated exporters
func (r *metricsRouter) Shutdown(ctx context.Context) error {
	if r.otlpExporter != nil {
		return r.otlpExporter.Shutdown(ctx)
	}
	return nil
}

// createOTLPMetricsExporter creates an OTLP HTTP exporter for metrics routing
func createOTLPMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	routingConfig config.MetricsRoutingConfig,
	apiKey string,
	site string,
) (exporter.Metrics, error) {
	if !routingConfig.Enabled {
		set.Logger.Debug("OTLP metrics exporter not created",
			zap.Bool("routing_enabled", routingConfig.Enabled),
			zap.String("otlp_endpoint", routingConfig.OTLPEndpoint))
		return nil, nil
	}

	// Auto-generate endpoint if not provided
	endpoint := routingConfig.OTLPEndpoint
	if endpoint == "" {
		if site == "" {
			site = "datadoghq.com" // Use default site if not specified
		}
		endpoint = fmt.Sprintf("https://trace.agent.%s/api/v0.2/stats", site)
		set.Logger.Info("Auto-generated OTLP endpoint for metrics routing",
			zap.String("site", site),
			zap.String("generated_endpoint", endpoint))
	}

	set.Logger.Info("Creating OTLP metrics exporter for routing",
		zap.String("endpoint", endpoint),
		zap.Int("header_count", len(routingConfig.OTLPHeaders)),
		zap.Bool("has_api_key", apiKey != ""))

	factory := otlphttpexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlphttpexporter.Config)

	// Configure metrics endpoint
	cfg.MetricsEndpoint = endpoint

	// Configure headers
	cfg.ClientConfig.Headers = make(map[string]configopaque.String)

	// Add configured headers
	for key, value := range routingConfig.OTLPHeaders {
		cfg.ClientConfig.Headers[key] = value
		set.Logger.Debug("Adding OTLP header", zap.String("key", key))
	}

	// Add API key header dynamically
	if apiKey != "" {
		cfg.ClientConfig.Headers["Dd-Api-Key"] = configopaque.String(apiKey)
		set.Logger.Debug("Added Dd-Api-Key header to OTLP exporter")
	} else {
		set.Logger.Warn("No API key provided for OTLP exporter")
	}

	// Disable retries and queuing for routing to avoid double buffering
	cfg.RetryConfig.Enabled = false
	cfg.QueueConfig.Enabled = false
	set.Logger.Debug("Disabled retries and queuing for OTLP routing exporter to avoid double buffering")

	// Create settings for OTLP exporter with correct component type
	otlpSettings := exporter.Settings{
		ID:                set.ID,
		TelemetrySettings: set.TelemetrySettings,
		BuildInfo:         set.BuildInfo,
	}
	// Set the correct component type for OTLP exporter
	otlpSettings.ID = component.NewIDWithName(component.MustNewType("otlphttp"), set.ID.Name())

	exporter, err := factory.CreateMetrics(ctx, otlpSettings, cfg)
	if err != nil {
		set.Logger.Error("Failed to create OTLP metrics exporter", zap.Error(err))
		return nil, err
	}

	set.Logger.Info("Successfully created OTLP metrics exporter for routing")
	return exporter, nil
}
