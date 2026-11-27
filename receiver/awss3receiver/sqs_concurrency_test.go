// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

type concurrentSQSClient struct {
	receive func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error)
	delete  func(context.Context, *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error)
}

func (c concurrentSQSClient) ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	return c.receive(ctx, input)
}

func (c concurrentSQSClient) DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	return c.delete(ctx, input)
}

func (concurrentSQSClient) ChangeMessageVisibility(context.Context, *sqs.ChangeMessageVisibilityInput, ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	return nil, errors.New("unexpected ChangeMessageVisibility call")
}

func TestSQSClientConcurrency(t *testing.T) {
	for _, concurrency := range []int{1, 3} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var received, active atomic.Int32
			started := make(chan string, 4)
			release := make(chan struct{})
			deleted := make(chan string, 4)
			failed := make(chan struct{})
			queue := concurrentSQSClient{
				receive: func(ctx context.Context, _ *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
					n := received.Add(1)
					if n > 4 {
						<-ctx.Done()
						return nil, ctx.Err()
					}
					key := fmt.Sprint(n)
					return &sqs.ReceiveMessageOutput{Messages: []types.Message{{
						ReceiptHandle: aws.String(key),
						Body:          aws.String(`{"Records":[{"eventSource":"aws:s3","eventName":"ObjectCreated:Put","s3":{"bucket":{"name":"bucket"},"object":{"key":"` + key + `"}}}]}`),
					}}}, nil
				},
				delete: func(_ context.Context, input *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
					deleted <- aws.ToString(input.ReceiptHandle)
					return &sqs.DeleteMessageOutput{}, nil
				},
			}
			r := &s3SQSNotificationReader{
				logger: zap.NewNop(), sqsClient: queue, s3Bucket: "bucket", clientConcurrency: concurrency,
				s3Client: &mockSingleObjectAPI{getObjectFunc: func(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(aws.ToString(input.Key)))}, nil
				}},
			}
			done := make(chan error, 1)
			go func() {
				done <- r.readAll(ctx, "logs", func(ctx context.Context, key string, _ io.Reader) error {
					active.Add(1)
					defer active.Add(-1)
					started <- key
					select {
					case <-release:
						if key == "4" {
							close(failed)
							return errors.New("consumer rejected object")
						}
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}()
			for range concurrency {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("configured workers did not start processing concurrently")
				}
			}
			require.EqualValues(t, concurrency, active.Load())
			require.EqualValues(t, concurrency, received.Load(), "busy workers must not prefetch another poll")
			// Completing one object should free a worker while the others remain busy.
			release <- struct{}{}
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("an idle worker did not resume polling")
			}
			require.EqualValues(t, concurrency, active.Load())
			close(release)
			var acknowledgements []string
			for range 3 {
				select {
				case key := <-deleted:
					acknowledgements = append(acknowledgements, key)
				case <-ctx.Done():
					t.Fatal("successful messages were not acknowledged")
				}
			}
			select {
			case <-failed:
			case <-ctx.Done():
				t.Fatal("failed object was not processed")
			}
			cancel()
			require.ErrorIs(t, <-done, context.Canceled)
			require.Zero(t, active.Load())
			require.ElementsMatch(t, []string{"1", "2", "3"}, acknowledgements)
			require.Empty(t, deleted, "failed messages must not be acknowledged")
		})
	}
}

func TestSQSWorkersCancelDuringPollingBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var polls atomic.Int32
		r := &s3SQSNotificationReader{
			logger: zap.NewNop(), clientConcurrency: 3,
			sqsClient: concurrentSQSClient{receive: func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
				polls.Add(1)
				return nil, errors.New("queue unavailable")
			}},
		}
		done := make(chan error, 1)
		go func() { done <- r.readAll(ctx, "logs", nil) }()
		synctest.Wait()
		require.EqualValues(t, 3, polls.Load())
		started := time.Now()
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
		require.Equal(t, started, time.Now(), "shutdown must interrupt polling backoff")
	})
}

func TestSQSConcurrencyConfiguration(t *testing.T) {
	for _, value := range []int{-1, 0, 1, 4} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cm := confmap.NewFromStringMap(map[string]any{
				"s3downloader": map[string]any{"s3_bucket": "bucket"},
				"sqs":          map[string]any{"queue_url": "queue", "region": "us-east-1", "client_concurrency": value},
			})
			require.NoError(t, cm.Unmarshal(cfg))
			if value < 1 {
				require.ErrorContains(t, cfg.Validate(), "sqs.client_concurrency must be positive")
			} else {
				require.NoError(t, cfg.Validate())
				r, err := newS3SQSReader(t.Context(), zap.NewNop(), cfg)
				require.NoError(t, err)
				require.Equal(t, value, r.clientConcurrency)
			}
		})
	}
}
