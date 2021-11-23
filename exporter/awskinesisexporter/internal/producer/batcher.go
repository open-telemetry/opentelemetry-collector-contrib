// Copyright  OpenTelemetry Authors
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

package producer

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/service/kinesis/kinesisiface"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
)

type batcher struct {
	stream *string

	client kinesisiface.KinesisAPI
	log    *zap.Logger
}

var _ Batcher = (*batcher)(nil)

func NewBatcher(kinesisAPI kinesisiface.KinesisAPI, stream string, opts ...BatcherOptions) (Batcher, error) {
	be := &batcher{
		stream: aws.String(stream),
		client: kinesisAPI,
		log:    zap.NewNop(),
	}
	for _, opt := range opts {
		if err := opt(be); err != nil {
			return nil, err
		}
	}
	return be, nil
}

func (b *batcher) Put(ctx context.Context, bt *batch.Batch) error {
	for _, records := range bt.Chunk() {
		out, err := b.client.PutRecordsWithContext(ctx, &kinesis.PutRecordsInput{
			StreamName: b.stream,
			Records:    records,
		})

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case kinesis.ErrCodeResourceNotFoundException, kinesis.ErrCodeInvalidArgumentException:
					err = consumererror.NewPermanent(err)
				}
			}
			fields := []zap.Field{
				zap.Error(err),
			}
			if out != nil {
				fields = append(fields, zap.Int64p("failed-records", out.FailedRecordCount))
			}
			b.log.Error("Failed to write records to kinesis", fields...)
			return err
		}

		b.log.Debug("Successfully wrote batch to kinesis", zap.Stringp("stream", b.stream))
	}
	return nil
}

func (b *batcher) Ready(ctx context.Context) error {
	_, err := b.client.DescribeStreamWithContext(ctx, &kinesis.DescribeStreamInput{
		StreamName: b.stream,
	})
	return err
}
