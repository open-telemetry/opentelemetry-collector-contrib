// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// newTracesExporter return a new LogSerice trace exporter.
func newTracesExporter(set exporter.Settings, cfg component.Config) (exporter.Traces, error) {

	l := &logServiceTraceSender{
		logger: set.Logger,
	}

	var err error
	if l.client, err = newLogServiceClient(cfg.(*Config), set.Logger); err != nil {
		return nil, err
	}

	return exporterhelper.NewTraces(
		context.TODO(),
		set,
		cfg,
		l.pushTraceData)
}

type logServiceTraceSender struct {
	logger *zap.Logger
	client logServiceClient
}

func (s *logServiceTraceSender) pushTraceData(
	_ context.Context,
	td ptrace.Traces,
) error {
	var err error
	slsLogs := traceDataToLogServiceData(td)
	if len(slsLogs) > 0 {
		err = s.client.sendLogs(slsLogs)
	}
	return err
}
