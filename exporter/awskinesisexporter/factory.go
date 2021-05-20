// Copyright 2019 OpenTelemetry Authors
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

package awskinesisexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/producer"
)

const (
	// The value of "type" key in configuration.
	typeStr = "awskinesis"
)

// NewFactory creates a factory for Kinesis exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(NewTracesExporter),
		exporterhelper.WithMetrics(NewMetricsExporter),
		exporterhelper.WithLogs(NewLogsExporter),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		AWS: AWSConfig{
			Region: "us-west-2",
		},
		MaxRecordsPerBatch: batch.MaxBatchedRecords,
		MaxRecordSize:      batch.MaxRecordSize,
	}
}

func NewTracesExporter(ctx context.Context, params component.ExporterCreateSettings, conf config.Exporter) (component.TracesExporter, error) {
	return createExporter(ctx, params, conf)
}

func NewMetricsExporter(ctx context.Context, params component.ExporterCreateSettings, conf config.Exporter) (component.MetricsExporter, error) {
	return createExporter(ctx, params, conf)
}

func NewLogsExporter(ctx context.Context, params component.ExporterCreateSettings, conf config.Exporter) (component.LogsExporter, error) {
	return createExporter(ctx, params, conf)
}

func createExporter(_ context.Context, params component.ExporterCreateSettings, conf config.Exporter) (*Exporter, error) {
	c, ok := conf.(*Config)

	if !ok {
		return nil, errors.New("unable to cast provided config")
	}

	sess, err := session.NewSession(aws.NewConfig().WithRegion(c.AWS.Region))
	if err != nil {
		return nil, err
	}

	var cfgs []*aws.Config
	if c.AWS.Role != "" {
		cfgs = append(cfgs, &aws.Config{Credentials: stscreds.NewCredentials(sess, c.AWS.Role)})
	}
	if c.AWS.KinesisEndpoint != "" {
		cfgs = append(cfgs, &aws.Config{Endpoint: aws.String(c.AWS.KinesisEndpoint)})
	}

	producer, err := producer.NewBatcher(kinesis.New(sess, cfgs...), c.AWS.StreamName,
		producer.WithLogger(params.Logger),
	)

	return &Exporter{
		producer: producer,
		batcher:  batch.NewJaeger(c.MaxRecordsPerBatch, c.MaxRecordSize),
	}, err
}
