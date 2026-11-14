// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type trackedBody struct {
	io.Reader
	closed   int
	closeErr error
}

func (b *trackedBody) Close() error {
	b.closed++
	return b.closeErr
}

func TestS3StreamAcknowledgement(t *testing.T) {
	for _, mode := range []string{"sqs", "time"} {
		for _, scenario := range []string{"success", "read_error", "corrupt_gzip", "consumer_error", "close_error"} {
			t.Run(mode+"/"+scenario, func(t *testing.T) {
				failure := errors.New("stream failure")
				content := []byte(strings.Repeat("record\n", 2001))
				key := "logs.txt"
				body := &trackedBody{Reader: bytes.NewReader(content)}
				switch scenario {
				case "read_error":
					body.Reader = io.MultiReader(bytes.NewReader(content), errorReader{err: failure})
				case "corrupt_gzip":
					content = gzipCompress(content)
					content[len(content)-1] ^= 0xff
					body.Reader = bytes.NewReader(content)
					key += ".gz"
				case "close_error":
					body.closeErr = failure
				}
				var consumed, tagged int
				c, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
					require.Zero(t, body.closed)
					consumed += logs.LogRecordCount()
					if scenario == "consumer_error" {
						return failure
					}
					return nil
				})
				require.NoError(t, err)
				r := newStreamTestReceiver(t, &logsReceiver{consumer: c}, newTextExtension(t))
				client := &mockSingleObjectAPI{
					getObjectFunc: func(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
						return &s3.GetObjectOutput{Body: body}, nil
					},
					putObjectTaggingFunc: func(context.Context, *s3.PutObjectTaggingInput, ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
						require.Equal(t, 1, body.closed)
						tagged++
						return &s3.PutObjectTaggingOutput{}, nil
					},
				}
				if mode == "time" {
					reader := &s3TimeBasedReader{
						logger: zap.NewNop(), singleObjectClient: client, s3Bucket: "bucket",
						tagObjectAfterIngestion: true, S3PartitionTimeLocation: time.UTC,
						listObjectsClient: mockListObjectsAPI(func(*s3.ListObjectsV2Input) ListObjectsV2Pager {
							return &mockListObjectsV2Pager{Pages: []*s3.ListObjectsV2Output{{
								Contents: []s3types.Object{{Key: aws.String(key)}},
							}}}
						}),
					}
					err = reader.readTelemetryForTime(t.Context(), time.Time{}, "logs", r.receive)
					if scenario == "success" {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
					}
				} else {
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					queue := &mockSQSClient{}
					queue.On("ReceiveMessage", mock.Anything, mock.Anything).Return(&sqs.ReceiveMessageOutput{
						Messages: []types.Message{{ReceiptHandle: aws.String("receipt"), Body: aws.String(
							`{"Records":[{"eventSource":"aws:s3","eventName":"ObjectCreated:Put","s3":{"bucket":{"name":"bucket"},"object":{"key":"` + key + `"}}}]}`,
						)}},
					}, nil).Once()
					queue.On("ReceiveMessage", mock.Anything, mock.Anything).Return(&sqs.ReceiveMessageOutput{}, nil).Run(func(mock.Arguments) { cancel() })
					if scenario == "success" {
						queue.On("DeleteMessage", mock.Anything, mock.Anything).Return(&sqs.DeleteMessageOutput{}, nil).Run(func(mock.Arguments) {
							require.Equal(t, 1, body.closed)
							require.Equal(t, 2001, consumed)
						}).Once()
					}
					reader := &s3SQSNotificationReader{
						logger: zap.NewNop(), s3Client: client, sqsClient: queue,
						s3Bucket: "bucket", queueURL: "queue", tagObjectAfterIngestion: true,
					}
					require.ErrorIs(t, reader.readAll(ctx, "logs", r.receive), context.Canceled)
					queue.AssertExpectations(t)
					if scenario != "success" {
						queue.AssertNotCalled(t, "DeleteMessage", mock.Anything, mock.Anything)
					}
				}
				require.Equal(t, 1, body.closed)
				require.Positive(t, consumed, "the stream should deliver a batch before a later failure")
				if scenario == "success" {
					require.Equal(t, 2001, consumed)
					require.Equal(t, 1, tagged)
				} else {
					require.Zero(t, tagged)
				}
			})
		}
	}
}
