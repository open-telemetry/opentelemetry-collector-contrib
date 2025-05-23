// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	exp "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.uber.org/zap"
)

func newLogsExporter(cfg component.Config, set exp.Settings) (*logsExporter, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config exporter, expect type: %T, got: %T", &Config{}, cfg)
	}

	signalExporter, err := newSignalExporter(oCfg, set, oCfg.Logs.Endpoint, oCfg.Logs.Headers)
	if err != nil {
		return nil, err
	}

	return &logsExporter{
		signalExporter: signalExporter,
	}, nil
}

type logsExporter struct {
	logExporter plogotlp.GRPCClient
	*signalExporter
}

func (e *logsExporter) start(ctx context.Context, host component.Host) (err error) {
	wrapper := &signalConfigWrapper{config: &e.config.Logs}
	if err := e.startSignalExporter(ctx, host, wrapper); err != nil {
		return err
	}
	e.logExporter = plogotlp.NewGRPCClient(e.clientConn)
	return nil
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	if !e.canSend() {
		return e.rateError.GetError()
	}

	rss := ld.ResourceLogs()
	for i := 0; i < rss.Len(); i++ {
		resourceLog := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceLog.Resource())
		resourceLog.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceLog.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	resp, err := e.logExporter.Export(e.enhanceContext(ctx), plogotlp.NewExportRequestFromLogs(ld), e.callOptions...)
	if err != nil {
		return e.processError(err)
	}

	partialSuccess := resp.PartialSuccess()
	if partialSuccess.ErrorMessage() != "" || partialSuccess.RejectedLogRecords() != 0 {
		e.settings.Logger.Error("Partial success response from Coralogix",
			zap.String("message", partialSuccess.ErrorMessage()),
			zap.Int64("rejected_log_records", partialSuccess.RejectedLogRecords()),
		)
	}

	return nil
}

func (e *logsExporter) enhanceContext(ctx context.Context) context.Context {
	return e.signalExporter.enhanceContext(ctx)
}
