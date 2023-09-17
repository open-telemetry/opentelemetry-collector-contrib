package pytransform

import (
	"context"
	"fmt"

	"github.com/daidokoro/go-embed-python/python"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func newLogsProcessor(ctx context.Context, logger *zap.Logger,
	cfg *Config, ep *python.EmbeddedPython, consumer consumer.Logs) *logsProcessor {
	return &logsProcessor{
		logger:         logger,
		cfg:            cfg,
		embeddedpython: ep,
		next:           consumer,
	}
}

type logsProcessor struct {
	plog.JSONMarshaler
	plog.JSONUnmarshaler
	logger         *zap.Logger
	cfg            *Config
	embeddedpython *python.EmbeddedPython
	next           consumer.Logs
}

func (lp *logsProcessor) Start(context.Context, component.Host) error {
	startLogServer(lp.logger)
	return nil
}

func (lp *logsProcessor) Shutdown(context.Context) error {
	return closeLogServer()
}

func (lp *logsProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	b, err := lp.MarshalLogs(ld)
	if err != nil {
		return err
	}

	output, err := runPythonCode(lp.cfg.Code, string(b), "logs", lp.embeddedpython, lp.logger)
	if err != nil {
		return err
	}

	if len(output) == 0 {
		lp.logger.Error("python script returned empty output, returning record unchanged")
		return lp.next.ConsumeLogs(ctx, ld)
	}

	if ld, err = lp.UnmarshalLogs(output); err != nil {
		return fmt.Errorf("error unmarshalling logs data from python: %w", err)
	}

	return lp.next.ConsumeLogs(ctx, ld)
}

func (lp *logsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
