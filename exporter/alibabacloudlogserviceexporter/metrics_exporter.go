// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alibabacloudlogserviceexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

// newMetricsExporter return a new LogSerice metrics exporter.
func newMetricsExporter(logger *zap.Logger, cfg configmodels.Exporter) (component.MetricsExporter, error) {

	l := &logServiceMetricsSender{
		logger: logger,
	}

	var err error
	if l.client, err = NewLogServiceClient(cfg.(*Config), logger); err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		cfg,
		logger,
		l.pushMetricsData)
}

type logServiceMetricsSender struct {
	logger *zap.Logger
	client LogServiceClient
}

func (s *logServiceMetricsSender) pushMetricsData(
	_ context.Context,
	md pdata.Metrics,
) (droppedTimeSeries int, err error) {
	logs, dts := metricsDataToLogServiceData(s.logger, md)
	if len(logs) > 0 {
		err = s.client.SendLogs(logs)
	}
	return dts, err
}
