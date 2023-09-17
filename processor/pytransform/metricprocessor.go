package pytransform

import (
	"context"
	"fmt"

	"github.com/daidokoro/go-embed-python/python"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func newMetricsProcessor(ctx context.Context, logger *zap.Logger,
	cfg *Config, ep *python.EmbeddedPython, consumer consumer.Metrics) *metricProcessor {
	return &metricProcessor{
		logger:         logger,
		cfg:            cfg,
		embeddedpython: ep,
		next:           consumer,
	}
}

type metricProcessor struct {
	pmetric.JSONMarshaler
	pmetric.JSONUnmarshaler
	logger         *zap.Logger
	cfg            *Config
	embeddedpython *python.EmbeddedPython
	next           consumer.Metrics
}

func (mp *metricProcessor) Start(context.Context, component.Host) error {
	startLogServer(mp.logger)
	return nil
}

func (mp *metricProcessor) Shutdown(context.Context) error {
	return closeLogServer()
}

func (mp *metricProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	b, err := mp.MarshalMetrics(md)
	if err != nil {
		return err
	}

	output, err := runPythonCode(mp.cfg.Code, string(b), "metrics", mp.embeddedpython, mp.logger)
	if err != nil {
		return err
	}

	if len(output) == 0 {
		mp.logger.Error("python script returned empty output, returning record unchanged")
		return mp.next.ConsumeMetrics(ctx, md)
	}

	if md, err = mp.UnmarshalMetrics(output); err != nil {
		return fmt.Errorf("error unmarshalling metric data from python: %v", err)
	}

	return mp.next.ConsumeMetrics(ctx, md)
}

func (mp *metricProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
