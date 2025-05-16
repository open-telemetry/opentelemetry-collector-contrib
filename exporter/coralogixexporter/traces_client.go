// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
)

func newTracesExporter(cfg component.Config, set exporter.Settings) (*tracesExporter, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config exporter, expect type: %T, got: %T", &Config{}, cfg)
	}

	signalExporter, err := newSignalExporter(oCfg, set, oCfg.Traces.Endpoint, oCfg.Traces.Headers)
	if err != nil {
		return nil, err
	}

	return &tracesExporter{
		signalExporter: *signalExporter,
	}, nil
}

type tracesExporter struct {
	traceExporter ptraceotlp.GRPCClient
	signalExporter
}

func (e *tracesExporter) start(ctx context.Context, host component.Host) (err error) {
	wrapper := &signalConfigWrapper{config: &e.config.Traces}
	if err := e.startSignalExporter(ctx, host, wrapper); err != nil {
		return err
	}
	e.traceExporter = ptraceotlp.NewGRPCClient(e.clientConn)
	return nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		resourceSpan := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceSpan.Resource())
		resourceSpan.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceSpan.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	resp, err := e.traceExporter.Export(e.enhanceContext(ctx), ptraceotlp.NewExportRequestFromTraces(td), e.callOptions...)
	if err != nil {
		return processError(err)
	}

	partialSuccess := resp.PartialSuccess()
	if partialSuccess.ErrorMessage() != "" || partialSuccess.RejectedSpans() != 0 {
		e.settings.Logger.Error("Partial success response from Coralogix",
			zap.String("message", partialSuccess.ErrorMessage()),
			zap.Int64("rejected_spans", partialSuccess.RejectedSpans()),
		)
	}

	return nil
}

func (e *tracesExporter) enhanceContext(ctx context.Context) context.Context {
	return e.signalExporter.enhanceContext(ctx)
}
