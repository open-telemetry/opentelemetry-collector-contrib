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
	if l.client, err = NewLogServiceClient(cfg.(*Config), set.Logger); err != nil {
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
	client LogServiceClient
}

func (s *logServiceMetricsSender) pushMetricsData(
	_ context.Context,
	md pmetric.Metrics,
) error {
	var err error
	logs := metricsDataToLogServiceData(s.logger, md)
	if len(logs) > 0 {
		err = s.client.SendLogs(logs)
	}
	return err
}
