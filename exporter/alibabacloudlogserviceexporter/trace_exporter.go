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

// newTraceExporter return a new LogSerice trace exporter.
func newTraceExporter(logger *zap.Logger, cfg configmodels.Exporter) (component.TraceExporter, error) {

	l := &logServiceTraceSender{
		logger: logger,
	}

	var err error
	if l.client, err = NewLogServiceClient(cfg.(*Config), logger); err != nil {
		return nil, err
	}

	return exporterhelper.NewTraceExporter(
		cfg,
		l.pushTraceData)
}

type logServiceTraceSender struct {
	logger *zap.Logger
	client LogServiceClient
}

func (s *logServiceTraceSender) pushTraceData(
	_ context.Context,
	td pdata.Traces,
) (int, error) {
	octds := internaldata.TraceDataToOC(td)
	var errs []error
	for _, octd := range octds {
		logs := traceDataToLogServiceData(octd)
		if len(logs) > 0 {
			if err := s.client.SendLogs(logs); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return 0, componenterror.CombineErrors(errs)

}
