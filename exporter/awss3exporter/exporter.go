// Copyright 2022 OpenTelemetry Authors
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

func (e *S3Exporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (e *S3Exporter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	return nil
}

func (e *S3Exporter) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	return nil
}

func (e *S3Exporter) Shutdown(context.Context) error {
	return nil
}
