package failoverconnector

import (
	"context"
	"errors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[consumer.Logs]
	logger   *zap.Logger
}

func (f *logsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs will try to export to the current set priority level and in the case of a returned err
// will call failoverRouter.handlePipelineError to adjust the target priority level, this way the error is handled
// depends on if the error is due to a retry or an error returned by the stable priority level
func (f *logsFailover) ConsumeLogs(ctx context.Context, md plog.Logs) error {
	for f.failover.pipelineIsValid() {
		tc := f.failover.getCurrentConsumer()
		err := tc.ConsumeLogs(ctx, md)
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

func (f *logsFailover) Shutdown(ctx context.Context) error {
	if f.failover != nil {
		f.failover.handleShutdown()
	}
	return nil
}

func newLogsToLogs(set connector.CreateSettings, cfg component.Config, logs consumer.Logs) (connector.Logs, error) {
	config := cfg.(*Config)
	lr, ok := logs.(connector.LogsRouter)
	if !ok {
		return nil, errors.New("consumer is not of type LogsRouter")
	}

	failover := newFailoverRouter(lr.Consumer, config)
	err := failover.registerConsumers()
	if err != nil {
		return nil, err
	}
	return &logsFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}
