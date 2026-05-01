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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter/internal/validation"
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
		signalExporter: signalExporter,
	}, nil
}

type tracesExporter struct {
	grpcTracesExporter ptraceotlp.GRPCClient
	httpTracesExporter httpTracesExporter
	*signalExporter
}

func (e *tracesExporter) start(ctx context.Context, host component.Host) (err error) {
	wrapper := &signalConfigWrapper{config: &e.config.Traces}
	if err := e.startSignalExporter(ctx, host, wrapper); err != nil {
		return err
	}
	if e.config.Protocol == httpProtocol {
		e.httpTracesExporter = newHTTPTracesExporter(e.clientHTTP, e.config)
	} else {
		e.grpcTracesExporter = ptraceotlp.NewGRPCClient(e.clientConn)
	}
	return nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	if !e.canSend() {
		return e.rateError.GetError()
	}

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		resourceSpan := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceSpan.Resource())
		resourceSpan.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceSpan.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	er := ptraceotlp.NewExportRequestFromTraces(td)
	var resp ptraceotlp.ExportResponse
	var err error

	if e.config.Protocol == httpProtocol {
		resp, err = e.httpTracesExporter.Export(ctx, er)
	} else {
		resp, err = e.grpcTracesExporter.Export(e.enhanceContext(ctx), er, e.callOptions...)
	}
	if err != nil {
		return e.processError(err)
	}

	partialSuccess := resp.PartialSuccess()
	if partialSuccess.ErrorMessage() != "" || partialSuccess.RejectedSpans() != 0 {
		logFields := []zap.Field{
			zap.String("message", partialSuccess.ErrorMessage()),
			zap.Int64("rejected_spans", partialSuccess.RejectedSpans()),
		}

		if e.settings.Logger.Level() == zap.DebugLevel {
			logFields = append(logFields, validation.BuildPartialSuccessLogFieldsForTraces(
				partialSuccess.ErrorMessage(),
				td,
				cxAppNameAttrName,
				cxSubsystemNameAttrName,
			)...)
		}

		e.settings.Logger.Error("Partial success response from Coralogix", logFields...)
	}

	e.rateError.errorCount.Store(0)
	return nil
}

func (e *tracesExporter) enhanceContext(ctx context.Context) context.Context {
	return e.signalExporter.enhanceContext(ctx)
}
