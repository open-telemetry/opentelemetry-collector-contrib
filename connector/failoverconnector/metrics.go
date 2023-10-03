package failoverconnector

import (
	"context"
	"errors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[consumer.Metrics]
	logger   *zap.Logger
}

func (f *metricsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics will try to export to the current set priority level and in the case of a returned err
// will call failoverRouter.handlePipelineError to adjust the target priority level, this way the error is handled
// depends on if the error is due to a retry or an error returned by the stable priority level
func (f *metricsFailover) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for f.failover.pipelineIsValid() {
		tc := f.failover.getCurrentConsumer()
		err := tc.ConsumeMetrics(ctx, md)
		if err != nil {
			ctx = context.Background()
			f.failover.handlePipelineError()
			continue
		}
		f.failover.reportStable()
		return nil
	}
	f.logger.Error("All provided pipelines return errors, dropping data")
	return errNoValidPipeline
}

func (f *metricsFailover) Shutdown(ctx context.Context) error {
	if f.failover != nil {
		f.failover.handleShutdown()
	}
	return nil
}

func newMetricsToMetrics(set connector.CreateSettings, cfg component.Config, metrics consumer.Metrics) (connector.Metrics, error) {
	config := cfg.(*Config)
	mr, ok := metrics.(connector.MetricsRouter)
	if !ok {
		return nil, errors.New("consumer is not of type MetricsRouter")
	}

	failover := newFailoverRouter(mr.Consumer, config)
	err := failover.registerConsumers()
	if err != nil {
		return nil, err
	}
	return &metricsFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}
