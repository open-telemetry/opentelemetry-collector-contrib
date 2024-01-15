package elasticconnector

import (
	"context"
	"strings"

	"github.com/tommyers-elastic/opentelemetry-collector-contrib/connector/elasticconnector/internal/datastream"
	"github.com/tommyers-elastic/opentelemetry-collector-contrib/connector/elasticconnector/internal/hostmetrics"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.uber.org/zap"
)

type ElasticConnector struct {
	config          Config
	tracesConsumer  consumer.Traces
	metricsConsumer consumer.Metrics
	logsConsumer    consumer.Logs
	logger          *zap.Logger

	component.StartFunc
	component.ShutdownFunc
}

// newConnector is a function to create a new connector
func newConnector(logger *zap.Logger, config component.Config) (*ElasticConnector, error) {
	logger.Info("Building elastic connector")
	cfg := config.(*Config)

	return &ElasticConnector{
		config: *cfg,
		logger: logger,
	}, nil
}

// Capabilities implements the consumer interface.
func (c *ElasticConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeTraces method is called for each instance of a trace sent to the connector
func (c *ElasticConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return c.tracesConsumer.ConsumeTraces(ctx, td)
}

// ConsumeMetrics method is called for each instance of a metric sent to the connector
func (c *ElasticConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			scope := scopeMetric.Scope()

			if err := datastream.AddDataStreamFields(datastream.Metrics, scope); err != nil {
				c.logger.Error("error adding Elastic data stream fields", zap.Error(err))
			}

			if strings.HasPrefix(scope.Name(), "otelcol/hostmetricsreceiver") {
				if err := hostmetrics.TransformHostMetricsForElasticCompatibilty(scopeMetric); err != nil {
					c.logger.Error("error adding hostmetrics data", zap.Error(err))
				}
			}

		}
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, md)
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (c *ElasticConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return c.logsConsumer.ConsumeLogs(ctx, ld)
}
