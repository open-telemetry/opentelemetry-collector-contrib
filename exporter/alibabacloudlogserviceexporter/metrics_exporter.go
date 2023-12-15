// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// newMetricsExporter return a new LogSerice metrics exporter.
func newMetricsExporter(set exporter.CreateSettings, cfg component.Config) (exporter.Metrics, error) {

	l := &logServiceMetricsSender{
		logger: set.Logger,
	}

	var err error
	if l.client, err = newLogServiceClient(cfg.(*Config), set.Logger); err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		context.TODO(),
		set,
		cfg,
		l.pushMetricsData)
}

type logServiceMetricsSender struct {
	logger *zap.Logger
	client logServiceClient
}

func (s *logServiceMetricsSender) pushMetricsData(
	_ context.Context,
	md pmetric.Metrics,
) error {
	var err error
	logs := metricsDataToLogServiceData(s.logger, md)
	if len(logs) > 0 {
		err = s.client.sendLogs(logs)
	}
	return err
}
