// Copyright 2021 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awss3exporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var logsMarshaler = plog.NewJSONMarshaler()
var traceMarshaler = ptrace.NewJSONMarshaler()

type S3Exporter struct {
	config           config.Exporter
	metricTranslator metricTranslator
	dataWriter       DataWriter
	logger           *zap.Logger
}

func NewS3Exporter(config config.Exporter,
	params component.ExporterCreateSettings) (*S3Exporter, error) {

	if config == nil {
		return nil, errors.New("s3 exporter config is nil")
	}

	logger := params.Logger
	expConfig := config.(*Config)
	expConfig.logger = logger

	expConfig.Validate()

	s3Exporter := &S3Exporter{
		config:           config,
		metricTranslator: newMetricTranslator(*expConfig),
		dataWriter:       &S3Writer{},
		logger:           logger,
	}
	return s3Exporter, nil
}

func (e *S3Exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *S3Exporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (e *S3Exporter) dumpLabels(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	labels := map[string]string{}

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		am := rm.Resource().Attributes()
		if am.Len() > 0 {
			am.Range(func(k string, v pcommon.Value) bool {
				labels[k] = v.StringVal()
				return true
			})
		}
	}

	e.logger.Info("Processing resource metrics", zap.Any("labels", labels))
}

func (e *S3Exporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	e.dumpLabels(md)
	rms := md.ResourceMetrics()

	expConfig := e.config.(*Config)
	var parquetMetrics []*ParquetMetric

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		e.metricTranslator.translateOTelToParquetMetric(&rm, &parquetMetrics, expConfig)
	}

	e.dataWriter.WriteParquet(parquetMetrics, ctx, expConfig)
	return nil
}

func (e *S3Exporter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	buf, err := logsMarshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}
	expConfig := e.config.(*Config)

	return e.dataWriter.WriteJson(buf, expConfig)
}

func (e *S3Exporter) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	buf, err := traceMarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}
	expConfig := e.config.(*Config)

	return e.dataWriter.WriteJson(buf, expConfig)
}

func (e *S3Exporter) Shutdown(context.Context) error {
	return nil
}
