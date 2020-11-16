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
	"go.uber.org/zap"
)

// newLogsExporter return a new LogService logs exporter.
func newLogsExporter(logger *zap.Logger, cfg configmodels.Exporter) (component.LogsExporter, error) {

	l := &logServiceLogsSender{
		logger: logger,
	}

	var err error
	if l.client, err = NewLogServiceClient(cfg.(*Config), logger); err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		cfg,
		logger,
		l.pushLogsData)
}

type logServiceLogsSender struct {
	logger *zap.Logger
	client LogServiceClient
}

func (s *logServiceLogsSender) pushLogsData(
	ctx context.Context,
	md pdata.Logs) (droppedTimeSeries int, err error) {
	slsLogs, dropped := logDataToLogService(s.logger, md)
	var errs []error
	if len(slsLogs) > 0 {
		if err := s.client.SendLogs(slsLogs); err != nil {
			errs = append(errs, err)
		}
	}
	return dropped, componenterror.CombineErrors(errs)
}
