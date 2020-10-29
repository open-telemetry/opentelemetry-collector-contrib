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
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
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
) (int, error) {
	ocmds := internaldata.MetricsToOC(md)
	droppedTimeSeries := 0
	var errs []error
	for _, ocmd := range ocmds {
		logs, dts := metricsDataToLogServiceData(s.logger, ocmd)
		if len(logs) > 0 {
			if err := s.client.SendLogs(logs); err != nil {
				errs = append(errs, err)
			}
		}
		droppedTimeSeries += dts
	}
	return droppedTimeSeries, componenterror.CombineErrors(errs)
}
