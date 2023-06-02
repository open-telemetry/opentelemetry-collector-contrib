// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// newLogsExporter return a new LogService logs exporter.
func newLogsExporter(set exporter.CreateSettings, cfg component.Config) (exporter.Logs, error) {
	l := &logServiceLogsSender{
		logger: set.Logger,
	}

	l.client = newLogServiceClient(cfg.(*Config), set.Logger)

	return exporterhelper.NewLogsExporter(
		context.TODO(),
		set,
		cfg,
		l.pushLogsData)
}

type logServiceLogsSender struct {
	logger *zap.Logger
	client logServiceClient
}

func (s *logServiceLogsSender) pushLogsData(
	_ context.Context,
	md plog.Logs) error {
	var err error
	clsLogs := convertLogs(md)
	if len(clsLogs) > 0 {
		err = s.client.sendLogs(clsLogs)
	}
	return err
}
