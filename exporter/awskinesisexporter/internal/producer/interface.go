// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/producer"

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
)

// Batcher abstracts the raw kinesis client to reduce complexity with delivering dynamic encoded data.
type Batcher interface {
	// Put is a blocking operation that will attempt to write the data at most once to kinesis.
	// Any unrecoverable errors such as misconfigured client or hard limits being exceeded
	// will result in consumeerr.Permanent being returned to allow for existing retry patterns within
	// the project to be used.
	Put(ctx context.Context, b *batch.Batch) error

	// Ready ensures that the configuration is valid and can write the configured stream.
	Ready(ctx context.Context) error
}

// Kinesis is the interface used to interact with the V2 API for the aws SDK since the *iface packages have been deprecated
type Kinesis interface {
	DescribeStream(ctx context.Context, params *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error)
	PutRecords(ctx context.Context, params *kinesis.PutRecordsInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordsOutput, error)
}

var (
	_ Kinesis = (*kinesis.Client)(nil)
)
