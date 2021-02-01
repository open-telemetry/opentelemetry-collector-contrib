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

	awskinesis "github.com/signalfx/opencensus-go-exporter-kinesis"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr      = "awskinesis"
	exportFormat = "jaeger-proto"
)

// NewFactory creates a factory for Kinesis exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		AWS: AWSConfig{
			Region: "us-west-2",
		},
		KPL: KPLConfig{
			BatchSize:            5242880,
			BatchCount:           1000,
			BacklogCount:         2000,
			FlushIntervalSeconds: 5,
			MaxConnections:       24,
		},

		QueueSize:            100000,
		NumWorkers:           8,
		FlushIntervalSeconds: 5,
		MaxBytesPerBatch:     100000,
		MaxBytesPerSpan:      900000,
	}
}

func createTraceExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter,
) (component.TracesExporter, error) {
	c := config.(*Config)
	k, err := awskinesis.NewExporter(&awskinesis.Options{
		Name:               c.Name(),
		StreamName:         c.AWS.StreamName,
		AWSRegion:          c.AWS.Region,
		AWSRole:            c.AWS.Role,
		AWSKinesisEndpoint: c.AWS.KinesisEndpoint,

		KPLAggregateBatchSize:   c.KPL.AggregateBatchSize,
		KPLAggregateBatchCount:  c.KPL.AggregateBatchCount,
		KPLBatchSize:            c.KPL.BatchSize,
		KPLBatchCount:           c.KPL.BatchCount,
		KPLBacklogCount:         c.KPL.BacklogCount,
		KPLFlushIntervalSeconds: c.KPL.FlushIntervalSeconds,
		KPLMaxConnections:       c.KPL.MaxConnections,
		KPLMaxRetries:           c.KPL.MaxRetries,
		KPLMaxBackoffSeconds:    c.KPL.MaxBackoffSeconds,

		QueueSize:             c.QueueSize,
		NumWorkers:            c.NumWorkers,
		MaxAllowedSizePerSpan: c.MaxBytesPerSpan,
		MaxListSize:           c.MaxBytesPerBatch,
		ListFlushInterval:     c.FlushIntervalSeconds,
		Encoding:              exportFormat,
	}, params.Logger)
	if err != nil {
		return nil, err
	}

	return Exporter{k, params.Logger}, nil
}
