// Copyright The OpenTelemetry Authors
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
	ctx context.Context,
	md plog.Logs) error {
	var err error
	clsLogs := convertLogs(md)
	if len(clsLogs) > 0 {
		err = s.client.sendLogs(clsLogs)
	}
	return err
}
