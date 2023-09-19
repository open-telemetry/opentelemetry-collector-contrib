// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

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

	var err error
	if l.client, err = newLogServiceClient(cfg.(*Config), set.Logger); err != nil {
		return nil, err
	}

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
	slsLogs := logDataToLogService(md)
	if len(slsLogs) > 0 {
		err = s.client.sendLogs(slsLogs)
	}
	return err
}
