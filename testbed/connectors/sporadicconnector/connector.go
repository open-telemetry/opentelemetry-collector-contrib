package spoardicconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type baseConnector struct {
	component.StartFunc
	component.ShutdownFunc
}

// Capabilities implements the consumer interface.
func (c *baseConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

type metricsConnector struct {
	baseConnector

	config          Config
	metricsConsumer consumer.Metrics
}

func newMetricsConnector(logger *zap.Logger, config component.Config) *metricsConnector {
	return &metricsConnector{
		config:        config.(*Config),
		baseConnector: baseConnector{},
	}
}

func (mc *metricsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	switch mc.config.decision {
	case 1:
		if err := randomNonPermanentError(); err != nil {
			return err
		}
	case 2:
		if err := randomPermanentError(); err != nil {
			return err
		}
	case 3:
		// TODO - make connector unavailable
	}
	return mc.metricsConsumer.ConsumeMetrics(ctx, md)
}

type logsConnector struct {
	baseConnector

	config       Config
	logsConsumer consumer.Logs
}

func newlogsConnector(logger *zap.Logger, config component.Config) *logsConnector {
	return &logsConnector{
		config:        config.(*Config),
		baseConnector: baseConnector{},
	}
}

func (lc *logsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	switch lc.config.decision {
	case 1:
		if err := randomNonPermanentError(); err != nil {
			return err
		}
	case 2:
		if err := randomPermanentError(); err != nil {
			return err
		}
	case 3:
		// TODO - make connector unavailable
	}
	return lc.logsConsumer.ConsumeLogs(ctx, ld)
}

type traceConnector struct {
	baseConnector

	config         Config
	tracesConsumer consumer.Traces
}

func newTraceConnector(logger *zap.Logger, config component.Config) *traceConnector {
	return &traceConnector{
		config:        config.(*Config),
		baseConnector: baseConnector{},
	}
}

func (tc *traceConnector) ConsumeTraces(ctx context.Context, tr ptrace.Traces) error {
	switch tc.config.decision {
	case 1:
		if err := randomNonPermanentError(); err != nil {
			return err
		}
	case 2:
		if err := randomPermanentError(); err != nil {
			return err
		}
	case 3:
		// TODO - make connector unavailable
	}
	return tc.tracesConsumer.ConsumeTraces(ctx, tr)
}
