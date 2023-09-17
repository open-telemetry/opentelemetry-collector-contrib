package pytransform

import (
	"context"
	"fmt"

	"github.com/daidokoro/go-embed-python/python"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func newTraceProcessor(ctx context.Context, logger *zap.Logger,
	cfg *Config, ep *python.EmbeddedPython, consumer consumer.Traces) *traceProcessor {
	return &traceProcessor{
		logger:         logger,
		cfg:            cfg,
		embeddedpython: ep,
		next:           consumer,
	}
}

type traceProcessor struct {
	ptrace.JSONMarshaler
	ptrace.JSONUnmarshaler
	logger         *zap.Logger
	cfg            *Config
	embeddedpython *python.EmbeddedPython
	next           consumer.Traces
}

func (tp *traceProcessor) Start(context.Context, component.Host) error {
	startLogServer(tp.logger)
	return nil
}

func (tp *traceProcessor) Shutdown(context.Context) error {
	return closeLogServer()
}

func (tp *traceProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	b, err := tp.MarshalTraces(td)
	if err != nil {
		return err
	}

	output, err := runPythonCode(tp.cfg.Code, string(b), "traces", tp.embeddedpython, tp.logger)
	if err != nil {
		return err
	}

	if len(output) == 0 {
		tp.logger.Error("python script returned empty output, returning record unchanged")
		return tp.next.ConsumeTraces(ctx, td)
	}

	if td, err = tp.UnmarshalTraces(output); err != nil {
		return fmt.Errorf("error unmarshalling metric data from python: %v", err)
	}

	return tp.next.ConsumeTraces(ctx, td)
}

func (tp *traceProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
