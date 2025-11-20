// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

func BenchmarkHandleS3Notification(b *testing.B) {
	bucket := "test-bucket"
	object := "test-file.txt"

	event := events.S3Event{
		Records: []events.S3EventRecord{
			{
				EventSource: "aws:s3",
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: bucket, Arn: "arn:aws:s3:::test-bucket"},
					Object: events.S3Object{Key: object},
				},
			},
		},
	}
	data, err := json.Marshal(event)
	require.NoError(b, err)

	service := internal.NewMockS3Service(gomock.NewController(b))
	service.EXPECT().ReadObject(gomock.Any(), bucket, object).Return([]byte(mockContent), nil).AnyTimes()

	consumer := noOpLogsConsumer{}
	// Wrap the consumer to match the new s3EventConsumerFunc signature
	logsConsumer := func(ctx context.Context, event events.S3EventRecord, logs plog.Logs) error {
		setObservedTimestampForAllLogs(logs, event.EventTime)
		return consumer.ConsumeLogs(ctx, logs)
	}
	handler := newS3Handler(service, zap.NewNop(), mockS3LogUnmarshaler{}.UnmarshalLogs, logsConsumer)

	b.Run("HandleS3Event", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			err = handler.handle(b.Context(), data)
			require.NoError(b, err)
		}
	})
}
