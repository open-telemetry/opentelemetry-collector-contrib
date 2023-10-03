package failoverconnector

import (
	"context"
	"errors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[consumer.Traces]
	logger   *zap.Logger
}

func (f *tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces will try to export to the current set priority level and in the case of a returned err
// will call failoverRouter.handlePipelineError to adjust the target priority level, this way the error is handled
// depends on if the error is due to a retry or an error returned by the stable priority level
func (f *tracesFailover) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for f.failover.pipelineIsValid() {
		tc := f.failover.getCurrentConsumer()
		err := tc.ConsumeTraces(ctx, td)
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

func (f *tracesFailover) Shutdown(ctx context.Context) error {
	if f.failover != nil {
		f.failover.handleShutdown()
	}
	return nil
}

func newTracesToTraces(set connector.CreateSettings, cfg component.Config, traces consumer.Traces) (connector.Traces, error) {
	config := cfg.(*Config)
	tr, ok := traces.(connector.TracesRouter)
	if !ok {
		return nil, errors.New("consumer is not of type TracesRouter")
	}

	failover := newFailoverRouter(tr.Consumer, config)
	err := failover.registerConsumers()
	if err != nil {
		return nil, err
	}
	return &tracesFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}
