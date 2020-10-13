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

package kinesisexporter

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	producer "github.com/signalfx/omnition-kinesis-producer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr      = "kinesis"
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
		ExportFormat: "",
	}
}

func createTraceExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter,
) (component.TraceExporter, error) {
	c := config.(*Config)
	awsConfig := aws.NewConfig().WithRegion(c.AWS.Region).WithEndpoint(c.AWS.KinesisEndpoint)
	client := kinesis.New(session.Must(session.NewSession(awsConfig)))

	producer := producer.New(&producer.Config{
		StreamName: c.AWS.StreamName,
		// KPL parameters
		FlushInterval:       time.Duration(c.KPL.FlushIntervalSeconds) * time.Second,
		BatchCount:          c.KPL.BatchCount,
		BatchSize:           c.KPL.BatchSize,
		AggregateBatchCount: c.KPL.AggregateBatchCount,
		AggregateBatchSize:  c.KPL.AggregateBatchSize,
		BacklogCount:        c.KPL.BacklogCount,
		MaxConnections:      c.KPL.MaxConnections,
		MaxRetries:          c.KPL.MaxRetries,
		MaxBackoffTime:      time.Duration(c.KPL.MaxBackoffSeconds) * time.Second,

		Logger: nil,
		Client: client,
	}, nil)

	return Exporter{producer, params.Logger}, nil
}
