// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"
)

func newMetricsExporter(cfg component.Config, set exporter.Settings) (*metricsExporter, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config exporter, expect type: %T, got: %T", &Config{}, cfg)
	}

	signalExporter, err := newSignalExporter(oCfg, set, oCfg.Metrics.Endpoint, oCfg.Metrics.Headers)
	if err != nil {
		return nil, err
	}

	return &metricsExporter{
		signalExporter: signalExporter,
	}, nil
}

type metricsExporter struct {
	metricExporter pmetricotlp.GRPCClient
	*signalExporter
}

func (e *metricsExporter) start(ctx context.Context, host component.Host) (err error) {
	wrapper := &signalConfigWrapper{config: &e.config.Metrics}
	if err := e.startSignalExporter(ctx, host, wrapper); err != nil {
		return err
	}
	e.metricExporter = pmetricotlp.NewGRPCClient(e.clientConn)
	return nil
}

func (e *metricsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	if !e.canSend() {
		return e.rateError.GetError()
	}

	rss := md.ResourceMetrics()
	for i := 0; i < rss.Len(); i++ {
		resourceMetric := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceMetric.Resource())
		resourceMetric.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceMetric.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	resp, err := e.metricExporter.Export(e.enhanceContext(ctx), pmetricotlp.NewExportRequestFromMetrics(md), e.callOptions...)
	if err != nil {
		return e.processError(err)
	}

	partialSuccess := resp.PartialSuccess()
	if partialSuccess.ErrorMessage() != "" || partialSuccess.RejectedDataPoints() != 0 {
		e.settings.Logger.Error("Partial success response from Coralogix",
			zap.String("message", partialSuccess.ErrorMessage()),
			zap.Int64("rejected_data_points", partialSuccess.RejectedDataPoints()),
		)
	}

	return nil
}

func (e *metricsExporter) enhanceContext(ctx context.Context) context.Context {
	return e.signalExporter.enhanceContext(ctx)
}
