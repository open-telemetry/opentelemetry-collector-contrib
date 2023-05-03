// Copyright The OpenTelemetry Authors
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

package awskinesisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter"

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/producer"
)

// Exporter implements an OpenTelemetry trace exporter that exports all spans to AWS Kinesis
type Exporter struct {
	producer producer.Batcher
	batcher  batch.Encoder
}

// options is used to override the default shipped behavior
// to allow for testing correct setup of components
type options struct {
	NewKinesisClient func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client
}

func createExporter(ctx context.Context, c component.Config, log *zap.Logger, opts ...func(opt *options)) (*Exporter, error) {
	options := &options{
		NewKinesisClient: kinesis.NewFromConfig,
	}

	for _, opt := range opts {
		opt(options)
	}

	conf, ok := c.(*Config)
	if !ok || conf == nil {
		return nil, errors.New("incorrect config provided")
	}

	var configOpts []func(*awsconfig.LoadOptions) error
	if conf.AWS.Region != "" {
		configOpts = append(configOpts, func(lo *awsconfig.LoadOptions) error {
			lo.Region = conf.AWS.Region
			return nil
		})
	}

	awsconf, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, err
	}

	var kinesisOpts []func(*kinesis.Options)
	if conf.AWS.Role != "" {
		kinesisOpts = append(kinesisOpts, func(o *kinesis.Options) {
			o.Credentials = stscreds.NewAssumeRoleProvider(
				sts.NewFromConfig(awsconf),
				conf.AWS.Role,
			)
		})
	}

	if conf.AWS.KinesisEndpoint != "" {
		kinesisOpts = append(kinesisOpts,
			kinesis.WithEndpointResolver(
				kinesis.EndpointResolverFromURL(conf.AWS.KinesisEndpoint),
			),
		)
	}

	producer, err := producer.NewBatcher(
		options.NewKinesisClient(awsconf, kinesisOpts...),
		conf.AWS.StreamName,
		producer.WithLogger(log),
	)
	if err != nil {
		return nil, err
	}

	compressor, err := compress.NewCompressor(conf.Encoding.Compression)
	if err != nil {
		return nil, err
	}

	encoder, err := batch.NewEncoder(
		conf.Encoding.Name,
		batch.WithMaxRecordSize(conf.MaxRecordSize),
		batch.WithMaxRecordsPerBatch(conf.MaxRecordsPerBatch),
		batch.WithCompression(compressor),
	)

	if err != nil {
		return nil, err
	}

	if conf.Encoding.Name == "otlp_json" {
		log.Info("otlp_json is considered experimental and should not be used in a production environment")
	}

	return &Exporter{
		producer: producer,
		batcher:  encoder,
	}, nil
}

// start validates that the Kinesis stream is available.
func (e Exporter) start(ctx context.Context, _ component.Host) error {
	return e.producer.Ready(ctx)
}

// ConsumeTraces receives a span batch and exports it to AWS Kinesis
func (e Exporter) consumeTraces(ctx context.Context, td ptrace.Traces) error {
	bt, err := e.batcher.Traces(td)
	if err != nil {
		return err
	}
	return e.producer.Put(ctx, bt)
}

func (e Exporter) consumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	bt, err := e.batcher.Metrics(md)
	if err != nil {
		return err
	}
	return e.producer.Put(ctx, bt)
}

func (e Exporter) consumeLogs(ctx context.Context, ld plog.Logs) error {
	bt, err := e.batcher.Logs(ld)
	if err != nil {
		return err
	}
	return e.producer.Put(ctx, bt)
}
