// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
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
	failure := errors.New("stream failure")
	content := []byte(strings.Repeat("record\n", 2001))
	corruptGzip := gzipCompress(content)
	corruptGzip[len(corruptGzip)-1] ^= 0xff
	tests := []struct {
		name                string
		key                 string
		content             []byte
		readErr             error
		consumerErr         error
		closeErr            error
		wantErr             error
		wantRecords         int
		wantTagged          int
		wantDeleted         int
		maxUnmarshalSize    int64
		wantPartialDelivery bool
	}{
		{
			name:        "success",
			key:         "logs.txt",
			content:     content,
			wantRecords: 2001,
			wantTagged:  1,
			wantDeleted: 1,
		},
		{
			name:                "read_error",
			wantPartialDelivery: true,
			key:                 "logs.txt",
			content:             content,
			readErr:             failure,
			wantErr:             failure,
		},
		{
			name:                "corrupt_gzip",
			wantPartialDelivery: true,
			key:                 "logs.txt.gz",
			content:             corruptGzip,
			wantErr:             gzip.ErrChecksum,
		},
		{
			name:                "consumer_error",
			wantPartialDelivery: true,
			key:                 "logs.txt",
			content:             content,
			consumerErr:         failure,
			wantErr:             failure,
		},
		{
			name:                "close_error",
			wantPartialDelivery: true,
			key:                 "logs.txt",
			content:             content,
			closeErr:            failure,
			wantErr:             failure,
		},
		{
			name:             "buffer_limit",
			key:              "logs.json.gz",
			content:          gzipCompress([]byte(strings.Repeat(" ", 4096) + "{}")),
			maxUnmarshalSize: 16,
			wantErr:          errUnmarshalSizeLimit,
		},
	}
	for _, mode := range []string{"sqs", "time"} {
		for _, tt := range tests {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				body := &trackedBody{
					Reader:   bytes.NewReader(tt.content),
					closeErr: tt.closeErr,
				}
				if tt.readErr != nil {
					body.Reader = io.MultiReader(body.Reader, iotest.ErrReader(tt.readErr))
				}
				var consumed, tagged int
				c, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
					require.Zero(t, body.closed)
					consumed += logs.LogRecordCount()
					return tt.consumerErr
				})
				require.NoError(t, err)
				r := newStreamTestReceiver(t, &logsReceiver{consumer: c}, newTextExtension(t))
				r.maxUnmarshalSize = tt.maxUnmarshalSize
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
						logger:                  zap.NewNop(),
						singleObjectClient:      client,
						s3Bucket:                "bucket",
						tagObjectAfterIngestion: true,
						S3PartitionTimeLocation: time.UTC,
						listObjectsClient: mockListObjectsAPI(func(*s3.ListObjectsV2Input) ListObjectsV2Pager {
							return &mockListObjectsV2Pager{Pages: []*s3.ListObjectsV2Output{{
								Contents: []s3types.Object{{Key: aws.String(tt.key)}},
							}}}
						}),
					}
					err = reader.readTelemetryForTime(t.Context(), time.Time{}, "logs", r.receive)
					if tt.wantErr != nil {
						require.ErrorIs(t, err, tt.wantErr)
					} else {
						require.NoError(t, err)
					}
				} else {
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					queue := &mockSQSClient{}
					queue.On("ReceiveMessage", mock.Anything, mock.Anything).Return(&sqs.ReceiveMessageOutput{
						Messages: []types.Message{{ReceiptHandle: aws.String("receipt"), Body: aws.String(
							`{"Records":[{"eventSource":"aws:s3","eventName":"ObjectCreated:Put","s3":{"bucket":{"name":"bucket"},"object":{"key":"` + tt.key + `"}}}]}`,
						)}},
					}, nil).Once()
					queue.On("ReceiveMessage", mock.Anything, mock.Anything).Return(&sqs.ReceiveMessageOutput{}, nil).Run(func(mock.Arguments) { cancel() })
					if tt.wantDeleted > 0 {
						queue.On("DeleteMessage", mock.Anything, mock.Anything).Return(&sqs.DeleteMessageOutput{}, nil).Run(func(mock.Arguments) {
							require.Equal(t, 1, body.closed)
							require.Equal(t, tt.wantRecords, consumed)
						}).Times(tt.wantDeleted)
					}
					reader := &s3SQSNotificationReader{
						logger:                  zap.NewNop(),
						s3Client:                client,
						sqsClient:               queue,
						s3Bucket:                "bucket",
						queueURL:                "queue",
						tagObjectAfterIngestion: true,
					}
					require.ErrorIs(t, reader.readAll(ctx, "logs", r.receive), context.Canceled)
					queue.AssertExpectations(t)
					queue.AssertNumberOfCalls(t, "DeleteMessage", tt.wantDeleted)
				}
				require.Equal(t, 1, body.closed)
				if tt.wantPartialDelivery {
					require.Positive(t, consumed, "the stream should deliver a batch before a later failure")
				} else {
					require.Equal(t, tt.wantRecords, consumed)
				}
				require.Equal(t, tt.wantTagged, tagged)
			})
		}
	}
}

func TestProcessS3ObjectErrors(t *testing.T) {
	processErr := errors.New("processing failed")
	closeErr := errors.New("closing failed")
	tests := []struct {
		name       string
		processErr error
		closeErr   error
	}{
		{
			name: "success",
		},
		{
			name:       "processing",
			processErr: processErr,
		},
		{
			name:     "closing",
			closeErr: closeErr,
		},
		{
			name:       "both",
			processErr: processErr,
			closeErr:   closeErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedBody{
				Reader:   strings.NewReader("data"),
				closeErr: tt.closeErr,
			}
			err := processS3Object(t.Context(), "key", body, func(context.Context, string, io.Reader) error {
				require.Zero(t, body.closed)
				return tt.processErr
			})
			require.Equal(t, 1, body.closed)
			if tt.processErr == nil && tt.closeErr == nil {
				require.NoError(t, err)
			}
			if tt.processErr != nil {
				require.ErrorIs(t, err, tt.processErr)
			}
			if tt.closeErr != nil {
				require.ErrorIs(t, err, tt.closeErr)
			}
		})
	}
}
