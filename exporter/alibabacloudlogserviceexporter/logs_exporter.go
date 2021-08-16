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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

// newLogsExporter return a new LogService logs exporter.
func newLogsExporter(set component.ExporterCreateSettings, cfg config.Exporter) (component.LogsExporter, error) {

	l := &logServiceLogsSender{
		logger: set.Logger,
	}

	var err error
	if l.client, err = NewLogServiceClient(cfg.(*Config), set.Logger); err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		cfg,
		set,
		l.pushLogsData)
}

type logServiceLogsSender struct {
	logger *zap.Logger
	client LogServiceClient
}

func (s *logServiceLogsSender) pushLogsData(
	ctx context.Context,
	md pdata.Logs) error {
	var err error
	slsLogs := logDataToLogService(md)
	if len(slsLogs) > 0 {
		err = s.client.SendLogs(slsLogs)
	}
	return err
}
