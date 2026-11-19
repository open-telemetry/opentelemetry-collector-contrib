// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func visibilityMessage(key string) types.Message {
	return types.Message{
		ReceiptHandle: aws.String(key),
		Body:          aws.String(`{"Records":[{"eventSource":"aws:s3","eventName":"ObjectCreated:Put","s3":{"bucket":{"name":"bucket"},"object":{"key":"` + key + `"}}}]}`),
	}
}

func TestVisibilityRenewal(t *testing.T) {
	for _, scenario := range []string{"disabled", "complete_before_renewal", "renew", "renewal_error", "request_timeout", "cancel", "maximum_duration"} {
		t.Run(scenario, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				queue := &mockSQSClient{}
				r := &s3SQSNotificationReader{logger: zap.NewNop(), sqsClient: queue, queueURL: "queue", visibilityTimeoutSeconds: 6}
				started := time.Now()
				failure := errors.New("renewal failed")
				var calls int
				switch scenario {
				case "disabled":
					r.visibilityTimeoutSeconds = 0
				case "renewal_error":
					queue.On("ChangeMessageVisibility", mock.Anything, mock.Anything).Return(nil, failure).Once()
				case "request_timeout":
					queue.On("ChangeMessageVisibility", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
						<-args.Get(0).(context.Context).Done()
					}).Return(nil, context.DeadlineExceeded).Once()
				case "maximum_duration":
					r.visibilityTimeoutSeconds = 300
					started = started.Add(-12*time.Hour + 160*time.Second)
					queue.On("ChangeMessageVisibility", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
						input := args.Get(1).(*sqs.ChangeMessageVisibilityInput)
						calls++
						assert.Positive(t, input.VisibilityTimeout)
						assert.Less(t, input.VisibilityTimeout, int32(300))
						assert.True(t, time.Now().Add(time.Duration(input.VisibilityTimeout)*time.Second).Before(started.Add(12*time.Hour)))
					}).Return(nil, nil)
				default:
					queue.On("ChangeMessageVisibility", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
						input := args.Get(1).(*sqs.ChangeMessageVisibilityInput)
						assert.Equal(t, "queue", aws.ToString(input.QueueUrl))
						assert.Equal(t, "first", aws.ToString(input.ReceiptHandle))
						assert.Equal(t, int32(6), input.VisibilityTimeout)
						calls++
					}).Return(nil, nil).Maybe()
				}
				processingCtx, finish := r.startVisibilityRenewal(ctx, visibilityMessage("first"), started)
				defer func() { _ = finish() }()
				synctest.Wait()
				switch scenario {
				case "disabled", "complete_before_renewal":
					time.Sleep(time.Second)
					require.NoError(t, finish())
					require.Zero(t, calls)
				case "renew":
					time.Sleep(10 * time.Second)
					synctest.Wait()
					require.NoError(t, processingCtx.Err())
					require.Equal(t, 3, calls)
					require.NoError(t, finish())
				case "renewal_error":
					<-processingCtx.Done()
					require.ErrorIs(t, finish(), failure)
				case "request_timeout":
					<-processingCtx.Done()
					require.ErrorIs(t, finish(), context.DeadlineExceeded)
				case "cancel":
					cancel()
					require.ErrorIs(t, finish(), context.Canceled)
				case "maximum_duration":
					<-processingCtx.Done()
					require.ErrorIs(t, finish(), errVisibilityExpired)
					require.Positive(t, calls)
				}
				before := calls
				time.Sleep(time.Minute)
				synctest.Wait()
				require.Equal(t, before, calls, "renewal must stop when processing finishes")
				queue.AssertExpectations(t)
			})
		})
	}
}

func TestSQSVisibilityProtectsWaitingMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := &mockSQSClient{}
		objects := &mockS3ClientSQS{}
		r := &s3SQSNotificationReader{
			logger: zap.NewNop(), sqsClient: queue, s3Client: objects, s3Bucket: "bucket",
			queueURL: "queue", visibilityTimeoutSeconds: 6,
		}
		counts := map[string]int{}
		var mu sync.Mutex
		queue.On("ChangeMessageVisibility", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			mu.Lock()
			counts[aws.ToString(args.Get(1).(*sqs.ChangeMessageVisibilityInput).ReceiptHandle)]++
			mu.Unlock()
		}).Return(nil, nil)
		for _, key := range []string{"first", "second"} {
			objects.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
				return aws.ToString(input.Key) == key
			})).Return([]byte(key), nil).Once()
			queue.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
				return aws.ToString(input.ReceiptHandle) == key
			})).Return(&sqs.DeleteMessageOutput{}, nil).Once()
		}
		var processed []string
		r.processMessages(t.Context(), []types.Message{visibilityMessage("first"), visibilityMessage("second")}, time.Now(), func(_ context.Context, key string, content io.Reader) error {
			data, err := io.ReadAll(content)
			require.NoError(t, err)
			require.Equal(t, key, string(data))
			processed = append(processed, key)
			if key == "first" {
				time.Sleep(14 * time.Second)
				synctest.Wait()
				mu.Lock()
				assert.GreaterOrEqual(t, counts["first"], 4)
				assert.GreaterOrEqual(t, counts["second"], 4)
				mu.Unlock()
			}
			return nil
		})
		require.Equal(t, []string{"first", "second"}, processed)
		queue.AssertExpectations(t)
		objects.AssertExpectations(t)
		queue.AssertNumberOfCalls(t, "DeleteMessage", 2)
		before := len(queue.Calls)
		time.Sleep(time.Minute)
		synctest.Wait()
		require.Len(t, queue.Calls, before)
	})
}

func TestSQSVisibilityFailureDoesNotAcknowledge(t *testing.T) {
	for _, failingKey := range []string{"first", "second"} {
		t.Run(failingKey, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				queue := &mockSQSClient{}
				objects := &mockS3ClientSQS{}
				r := &s3SQSNotificationReader{logger: zap.NewNop(), sqsClient: queue, s3Client: objects, s3Bucket: "bucket", visibilityTimeoutSeconds: 6}
				for _, key := range []string{"first", "second"} {
					var err error
					if key == failingKey {
						err = errors.New("visibility renewal failed")
					} else {
						queue.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
							return aws.ToString(input.ReceiptHandle) == key
						})).Return(&sqs.DeleteMessageOutput{}, nil).Once()
					}
					queue.On("ChangeMessageVisibility", mock.Anything, mock.MatchedBy(func(input *sqs.ChangeMessageVisibilityInput) bool {
						return aws.ToString(input.ReceiptHandle) == key
					})).Return(nil, err).Maybe()
				}
				objects.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
					return failingKey == "first" || aws.ToString(input.Key) == "first"
				})).Return([]byte("data"), nil)
				var processed []string
				r.processMessages(t.Context(), []types.Message{visibilityMessage("first"), visibilityMessage("second")}, time.Now(), func(ctx context.Context, key string, _ io.Reader) error {
					processed = append(processed, key)
					if key == "first" {
						time.Sleep(4 * time.Second)
						synctest.Wait()
					}
					return ctx.Err()
				})
				if failingKey == "second" {
					require.Equal(t, []string{"first"}, processed)
				} else {
					require.Equal(t, []string{"first", "second"}, processed)
				}
				queue.AssertExpectations(t)
				queue.AssertNumberOfCalls(t, "DeleteMessage", 1)
			})
		})
	}
}

func TestSQSReceiveSetsVisibilityTimeout(t *testing.T) {
	queue := &mockSQSClient{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	queue.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
		return input.VisibilityTimeout == 300
	})).Return(&sqs.ReceiveMessageOutput{}, nil).Run(func(mock.Arguments) { cancel() }).Once()
	r := &s3SQSNotificationReader{logger: zap.NewNop(), sqsClient: queue, visibilityTimeoutSeconds: 300}
	require.ErrorIs(t, r.readAll(ctx, "logs", func(context.Context, string, io.Reader) error { return nil }), context.Canceled)
	queue.AssertExpectations(t)
}
