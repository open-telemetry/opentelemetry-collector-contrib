// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver

import (
	"context"
	"encoding/json"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockS3ClientSQS struct {
	mock.Mock
}

func (m *mockS3ClientSQS) GetObject(ctx context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	// If test wants to return content, wrap it in a ReadCloser
	content, ok := args.Get(0).([]byte)
	if !ok {
		return nil, errors.New("unexpected type for mock GetObject content")
	}

	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(string(content))),
	}, args.Error(1)
}

func (m *mockS3ClientSQS) GetObjectTagging(ctx context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.GetObjectTaggingOutput), args.Error(1)
}

func (m *mockS3ClientSQS) PutObjectTagging(ctx context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.PutObjectTaggingOutput), args.Error(1)
}

type mockSQSClient struct {
	mock.Mock
}

func (m *mockSQSClient) ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.ReceiveMessageOutput), args.Error(1)
}

func (m *mockSQSClient) DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.DeleteMessageOutput), args.Error(1)
}

func TestNewS3SQSReader(t *testing.T) {
	logger := zap.NewNop()

	t.Run("fails with nil SQS config", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket: "test-bucket",
				Region:   "us-east-1",
			},
		}

		reader, err := newS3SQSReader(t.Context(), logger, cfg)
		assert.Error(t, err)
		assert.Nil(t, reader)
	})

	t.Run("succeeds with valid SQS config", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket: "test-bucket",
				Region:   "us-east-1",
			},
			SQS: &SQSConfig{
				QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
				Region:   "us-east-1",
			},
		}

		r, err := newS3SQSReader(t.Context(), logger, cfg)
		assert.NotNil(t, r)
		assert.NoError(t, err)

		// check all defaults are set
		assert.Equal(t, int32(10), r.maxNumberOfMessages)
		assert.Equal(t, int32(20), r.waitTimeSeconds)
	})

	t.Run("override non-default config", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket: "test-bucket",
				Region:   "us-east-1",
			},
			SQS: &SQSConfig{
				QueueURL:            "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
				Region:              "us-east-1",
				MaxNumberOfMessages: aws.Int64(5),
				WaitTimeSeconds:     aws.Int64(10),
			},
		}

		r, err := newS3SQSReader(t.Context(), logger, cfg)
		assert.NotNil(t, r)
		assert.NoError(t, err)
		assert.Equal(t, int32(5), r.maxNumberOfMessages)
		assert.Equal(t, int32(10), r.waitTimeSeconds)
	})
}

func TestS3SQSReader_ReadAll(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{
		S3Downloader: S3DownloaderConfig{
			S3Bucket: "test-bucket",
			Region:   "us-east-1",
		},
		SQS: &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			Region:   "us-east-1",
		},
	}

	mockS3 := new(mockS3ClientSQS)
	mockSQS := new(mockSQSClient)

	reader := &s3SQSNotificationReader{
		logger:              logger,
		s3Client:            mockS3,
		sqsClient:           mockSQS,
		queueURL:            cfg.SQS.QueueURL,
		s3Bucket:            cfg.S3Downloader.S3Bucket,
		s3Prefix:            cfg.S3Downloader.S3Prefix,
		maxNumberOfMessages: 10,
		waitTimeSeconds:     20,
	}

	s3Event := s3EventNotification{
		Records: []s3EventRecord{
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "test-key",
					},
				},
			},
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "url-encoding%3dtest-key",
					},
				},
			},
		},
	}

	eventJSON, err := json.Marshal(s3Event)
	require.NoError(t, err)

	snsNotification := snsMessage{
		Type:    "Notification",
		Message: string(eventJSON),
	}

	snsJSON, err := json.Marshal(snsNotification)
	require.NoError(t, err)

	mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			input.MaxNumberOfMessages == 10 &&
			input.WaitTimeSeconds == 20
	})).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{
					Body:          aws.String(string(snsJSON)),
					ReceiptHandle: aws.String("test-receipt-handle"),
				},
			},
		},
		nil,
	).Once()

	// After processing one message, return empty results to exit the loop
	mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{},
		},
		nil,
	)

	mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key"),
	}).Return(
		[]byte("test-content"),
		nil,
	)

	mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("url-encoding=test-key"),
	}).Return(
		[]byte("url-encoded-content"),
		nil,
	)

	mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			*input.ReceiptHandle == "test-receipt-handle"
	})).Return(
		&sqs.DeleteMessageOutput{},
		nil,
	)

	// Run test with callback
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	var callbackCallCount int
	var receivedKeys []string
	var receivedContents [][]byte

	err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, content []byte) error {
		callbackCallCount++
		receivedKeys = append(receivedKeys, key)
		receivedContents = append(receivedContents, content)
		return nil
	})

	// Context cancellation is expected
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify callback was called with correct data
	assert.Equal(t, 2, callbackCallCount)
	assert.Equal(t, "test-key", receivedKeys[0])
	assert.Equal(t, []byte("test-content"), receivedContents[0])
	assert.Equal(t, "url-encoding=test-key", receivedKeys[1])
	assert.Equal(t, []byte("url-encoded-content"), receivedContents[1])

	// Verify all expectations
	mockS3.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

// TestS3SQSReader_ReadAllDirectS3EventNotification tests processing S3 event notifications received directly in SQS
// without being wrapped in an SNS notification
func TestS3SQSReader_ReadAllDirectS3EventNotification(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{
		S3Downloader: S3DownloaderConfig{
			S3Bucket: "test-bucket", // Match the bucket in the notification
			Region:   "us-east-1",
		},
		SQS: &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			Region:   "us-east-1",
		},
	}

	mockS3 := new(mockS3ClientSQS)
	mockSQS := new(mockSQSClient)

	// Create reader with mocks
	reader := &s3SQSNotificationReader{
		logger:              logger,
		s3Client:            mockS3,
		sqsClient:           mockSQS,
		queueURL:            cfg.SQS.QueueURL,
		s3Bucket:            cfg.S3Downloader.S3Bucket,
		s3Prefix:            cfg.S3Downloader.S3Prefix,
		maxNumberOfMessages: 10,
		waitTimeSeconds:     20,
	}

	// Create S3 event notification
	s3Event := s3EventNotification{
		Records: []s3EventRecord{
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "test-key",
					},
				},
			},
		},
	}

	directS3Notification, err := json.Marshal(s3Event)
	require.NoError(t, err)

	// Mock SQS message reception with direct S3 notification
	mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL
	})).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{
					Body:          aws.String(string(directS3Notification)),
					ReceiptHandle: aws.String("direct-s3-receipt-handle"),
				},
			},
		},
		nil,
	).Once() // Only return the message once

	// After processing one message, return empty results to exit test
	mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{},
		},
		nil,
	)

	// Mock S3 object retrieval
	mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key"),
	}).Return(
		[]byte("test-trace-data"),
		nil,
	)

	// Mock message deletion
	mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			*input.ReceiptHandle == "direct-s3-receipt-handle"
	})).Return(
		&sqs.DeleteMessageOutput{},
		nil,
	)

	// Run test with callback
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	var callbackCalled bool
	var receivedKey string
	var receivedContent []byte

	err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, content []byte) error {
		callbackCalled = true
		receivedKey = key
		receivedContent = content
		return nil
	})

	// Context cancellation is expected
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify callback was called with correct data
	assert.True(t, callbackCalled)
	assert.Equal(t, "test-key", receivedKey)
	assert.Equal(t, []byte("test-trace-data"), receivedContent)

	// Verify all expectations
	mockS3.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

func TestS3SQSReader_ReadAllErrorHandling(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{
		S3Downloader: S3DownloaderConfig{
			S3Bucket: "test-bucket",
			Region:   "us-east-1",
		},
		SQS: &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			Region:   "us-east-1",
		},
	}

	t.Run("handles context cancellation", func(t *testing.T) {
		mockS3 := new(mockS3ClientSQS)
		mockSQS := new(mockSQSClient)

		// Create reader with mocks
		reader := &s3SQSNotificationReader{
			logger:                  logger,
			s3Client:                mockS3,
			sqsClient:               mockSQS,
			queueURL:                cfg.SQS.QueueURL,
			s3Bucket:                cfg.S3Downloader.S3Bucket,
			s3Prefix:                cfg.S3Downloader.S3Prefix,
			maxNumberOfMessages:     10,
			waitTimeSeconds:         20,
			tagObjectAfterIngestion: true,
		}

		// Mock error during receive messages
		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			nil,
			errors.New("context canceled"),
		)

		ctx, cancel := context.WithCancel(t.Context())
		cancel() // Cancel context immediately to trigger error case

		err := reader.readAll(ctx, "test-telemetry", nil)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("does not delete message or tag object on S3 retrieval error", func(t *testing.T) {
		mockS3 := new(mockS3ClientSQS)
		mockSQS := new(mockSQSClient)

		// Create reader with mocks
		reader := &s3SQSNotificationReader{
			logger:                  logger,
			s3Client:                mockS3,
			sqsClient:               mockSQS,
			queueURL:                cfg.SQS.QueueURL,
			s3Bucket:                cfg.S3Downloader.S3Bucket,
			s3Prefix:                cfg.S3Downloader.S3Prefix,
			maxNumberOfMessages:     10,
			waitTimeSeconds:         20,
			tagObjectAfterIngestion: true,
		}

		// Create S3 event notification
		s3Event := s3EventNotification{
			Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{
							Name: "test-bucket",
						},
						Object: s3ObjectData{
							Key: "test-key",
						},
					},
				},
			},
		}

		eventJSON, err := json.Marshal(s3Event)
		require.NoError(t, err)

		snsNotification := snsMessage{
			Type:    "Notification",
			Message: string(eventJSON),
		}

		snsJSON, err := json.Marshal(snsNotification)
		require.NoError(t, err)

		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{
						Body:          aws.String(string(snsJSON)),
						ReceiptHandle: aws.String("test-receipt-handle"),
					},
				},
			},
			nil,
		).Once()

		// After processing one message, return empty results to exit the loop
		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{},
			},
			nil,
		)

		// Mock S3 object retrieval with error
		mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("test-key"),
		}).Return(
			[]byte{},
			errors.New("object retrieval failed"),
		)

		// NOTE: DeleteMessage should NOT be called when S3 retrieval fails
		// The message should remain in the queue for retry

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		err = reader.readAll(ctx, "test-telemetry", func(context.Context, string, []byte) error {
			t.Fatal("Callback should not be called when S3 retrieval fails")
			return nil
		})
		assert.Error(t, err)
		mockS3.AssertExpectations(t)
		mockSQS.AssertExpectations(t)
		mockSQS.AssertNotCalled(t, "DeleteMessage", mock.Anything, mock.Anything)
		mockS3.AssertNotCalled(t, "PutObjectTagging", mock.Anything, mock.Anything)
	})

	t.Run("does not delete message or tag object on partial failure", func(t *testing.T) {
		mockS3 := new(mockS3ClientSQS)
		mockSQS := new(mockSQSClient)

		reader := &s3SQSNotificationReader{
			logger:                  logger,
			s3Client:                mockS3,
			sqsClient:               mockSQS,
			queueURL:                cfg.SQS.QueueURL,
			s3Bucket:                cfg.S3Downloader.S3Bucket,
			s3Prefix:                cfg.S3Downloader.S3Prefix,
			maxNumberOfMessages:     10,
			waitTimeSeconds:         20,
			tagObjectAfterIngestion: true,
		}

		// Create S3 event notification with THREE objects:
		// - First will succeed
		// - Second will fail S3 retrieval
		// - Third will fail callback processing
		s3Event := s3EventNotification{
			Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "success-key"},
					},
				},
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "s3-failure-key"},
					},
				},
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "callback-failure-key"},
					},
				},
			},
		}

		eventJSON, err := json.Marshal(s3Event)
		require.NoError(t, err)

		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{
						Body:          aws.String(string(eventJSON)),
						ReceiptHandle: aws.String("test-receipt-handle"),
					},
				},
			},
			nil,
		).Once()

		// After processing one message, return empty results to exit the loop.
		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{},
			},
			nil,
		)

		// First S3 object succeeds.
		mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("success-key"),
		}).Return([]byte("success-content"), nil)
		mockS3.On("PutObjectTagging", mock.Anything, mock.MatchedBy(func(input *s3.PutObjectTaggingInput) bool {
			return *input.Bucket == "test-bucket" && *input.Key == "success-key"
		})).Return(&s3.PutObjectTaggingOutput{}, nil)

		// Second S3 object FAILS to retrieve.
		mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("s3-failure-key"),
		}).Return([]byte{}, errors.New("S3 GetObject failed"))

		// Third S3 object retrieves successfully.
		mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("callback-failure-key"),
		}).Return([]byte("callback-content"), nil)

		// NOTE: DeleteMessage should NOT be called when any record fails.
		// The message should remain in the queue for retry.

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		successfulCallbacks := 0
		failedCallbacks := 0

		err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, _ []byte) error {
			if key == "callback-failure-key" {
				failedCallbacks++
				return errors.New("callback processing failed")
			}
			successfulCallbacks++
			return nil
		})

		assert.Error(t, err) // Context timeout expected.

		// Verify that only 1 out of 3 objects was successfully processed.
		assert.Equal(t, 1, successfulCallbacks, "Only 1 object should have been successfully processed")
		assert.Equal(t, 1, failedCallbacks, "1 object should have had callback failure")

		mockS3.AssertExpectations(t)
		mockSQS.AssertExpectations(t)
		// Verify DeleteMessage was never called - message should remain for retry.
		mockSQS.AssertNotCalled(t, "DeleteMessage", mock.Anything, mock.Anything)
		mockS3.AssertNotCalled(t, "PutObjectTagging", mock.Anything, &s3.PutObjectTaggingInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("s3-failure-key"),
		})
		mockS3.AssertNotCalled(t, "PutObjectTagging", mock.Anything, &s3.PutObjectTaggingInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("callback-failure-key"),
		})
	})

	t.Run("does not delete message or tag object on callback error", func(t *testing.T) {
		mockS3 := new(mockS3ClientSQS)
		mockSQS := new(mockSQSClient)

		reader := &s3SQSNotificationReader{
			logger:                  logger,
			s3Client:                mockS3,
			sqsClient:               mockSQS,
			queueURL:                cfg.SQS.QueueURL,
			s3Bucket:                cfg.S3Downloader.S3Bucket,
			s3Prefix:                cfg.S3Downloader.S3Prefix,
			maxNumberOfMessages:     10,
			waitTimeSeconds:         20,
			tagObjectAfterIngestion: true,
		}

		s3Event := s3EventNotification{
			Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "test-key"},
					},
				},
			},
		}

		eventJSON, err := json.Marshal(s3Event)
		require.NoError(t, err)

		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{
						Body:          aws.String(string(eventJSON)),
						ReceiptHandle: aws.String("test-receipt-handle"),
					},
				},
			},
			nil,
		).Once()

		// After processing one message, return empty results to exit the loop.
		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{},
			},
			nil,
		)

		// S3 object retrieves successfully.
		mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("test-key"),
		}).Return([]byte("test-content"), nil)

		// NOTE: DeleteMessage should NOT be called when callback fails.
		// The message should remain in the queue for retry.

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		callbackCalled := false
		err = reader.readAll(ctx, "test-telemetry", func(context.Context, string, []byte) error {
			callbackCalled = true
			return errors.New("callback processing failed")
		})

		assert.Error(t, err) // Context timeout expected.
		assert.True(t, callbackCalled, "Callback should have been called")

		mockS3.AssertExpectations(t)
		mockSQS.AssertExpectations(t)
		// Verify DeleteMessage was never called - message should remain for retry.
		mockSQS.AssertNotCalled(t, "DeleteMessage", mock.Anything, mock.Anything)
		mockS3.AssertNotCalled(t, "PutObjectTagging", mock.Anything, mock.Anything)
	})
	t.Run("deletes message if object is not found", func(t *testing.T) {
		mockSQS := new(mockSQSClient)
		mockS3 := new(mockS3ClientSQS)

		s3Event := s3EventNotification{
			Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "test-key"},
					},
				},
			},
		}

		s3EventJSON, err := json.Marshal(s3Event)
		require.NoError(t, err)

		receiptHandle := "test-receipt-handle"

		// S3 returns object not found
		mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
			return *input.Bucket == "test-bucket" && *input.Key == "test-key"
		})).Return([]byte(""), &s3types.NoSuchKey{Message: aws.String("The specified key does not exist.")})

		mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
			return *input.QueueUrl == "test-queue-url"
		})).Return(&sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{
					Body:          aws.String(string(s3EventJSON)),
					ReceiptHandle: &receiptHandle,
				},
			},
		}, nil).Once()

		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			(*sqs.ReceiveMessageOutput)(nil), context.DeadlineExceeded,
		)

		// Message should be deleted
		mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
			return *input.QueueUrl == "test-queue-url" && *input.ReceiptHandle == "test-receipt-handle"
		})).Return(&sqs.DeleteMessageOutput{}, nil)

		reader := &s3SQSNotificationReader{
			logger:                  zap.NewNop(),
			s3Client:                mockS3,
			sqsClient:               mockSQS,
			queueURL:                "test-queue-url",
			s3Bucket:                "test-bucket",
			s3Prefix:                "",
			maxNumberOfMessages:     10,
			waitTimeSeconds:         20,
			tagObjectAfterIngestion: true, // Enable deletion
		}

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		callbackCalled := false
		err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, _ string, _ []byte) error {
			callbackCalled = true
			return nil
		})

		// Context deadline exceeded is expected
		assert.Equal(t, context.DeadlineExceeded, err)

		mockSQS.AssertExpectations(t)
		mockS3.AssertExpectations(t)

		// Callback never called because there is no object to process
		assert.False(t, callbackCalled, "Callback should have been called")
	})
	t.Run("deletes message if object is not found and skip tagged objects is enabled", func(t *testing.T) {
		mockSQS := new(mockSQSClient)
		mockS3 := new(mockS3ClientSQS)

		s3Event := s3EventNotification{
			Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "test-key"},
					},
				},
			},
		}

		s3EventJSON, err := json.Marshal(s3Event)
		require.NoError(t, err)

		receiptHandle := "test-receipt-handle"

		// S3 returns object not found
		mockS3.On("GetObjectTagging", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectTaggingInput) bool {
			return *input.Bucket == "test-bucket" && *input.Key == "test-key"
		})).Return((*s3.GetObjectTaggingOutput)(nil), &s3types.NoSuchKey{Message: aws.String("The specified key does not exist.")})

		mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
			return *input.QueueUrl == "test-queue-url"
		})).Return(&sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{
					Body:          aws.String(string(s3EventJSON)),
					ReceiptHandle: &receiptHandle,
				},
			},
		}, nil).Once()

		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			(*sqs.ReceiveMessageOutput)(nil), context.DeadlineExceeded,
		)

		// Message should be deleted
		mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
			return *input.QueueUrl == "test-queue-url" && *input.ReceiptHandle == "test-receipt-handle"
		})).Return(&sqs.DeleteMessageOutput{}, nil)

		reader := &s3SQSNotificationReader{
			logger:                     zap.NewNop(),
			s3Client:                   mockS3,
			sqsClient:                  mockSQS,
			queueURL:                   "test-queue-url",
			s3Bucket:                   "test-bucket",
			s3Prefix:                   "",
			maxNumberOfMessages:        10,
			waitTimeSeconds:            20,
			tagObjectAfterIngestion:    true, // Enable deletion
			skipIngestingTaggedObjects: true,
		}

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		callbackCalled := false
		err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, _ string, _ []byte) error {
			callbackCalled = true
			return nil
		})

		// Context deadline exceeded is expected
		assert.Equal(t, context.DeadlineExceeded, err)

		mockSQS.AssertExpectations(t)
		mockS3.AssertExpectations(t)

		// Callback never called because there is no object to process
		assert.False(t, callbackCalled, "Callback should have been called")
	})

	t.Run("does not delete message if tags cannot be fetched", func(t *testing.T) {
		mockS3 := new(mockS3ClientSQS)
		mockSQS := new(mockSQSClient)

		reader := &s3SQSNotificationReader{
			logger:                     logger,
			s3Client:                   mockS3,
			sqsClient:                  mockSQS,
			queueURL:                   cfg.SQS.QueueURL,
			s3Bucket:                   cfg.S3Downloader.S3Bucket,
			s3Prefix:                   cfg.S3Downloader.S3Prefix,
			maxNumberOfMessages:        10,
			waitTimeSeconds:            20,
			tagObjectAfterIngestion:    true,
			skipIngestingTaggedObjects: true,
		}

		// Create S3 event notification
		s3Event := s3EventNotification{
			Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{
							Name: "test-bucket",
						},
						Object: s3ObjectData{
							Key: "test-key-with-tags",
						},
					},
				},
			},
		}

		eventJSON, err := json.Marshal(s3Event)
		require.NoError(t, err)

		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{
						Body:          aws.String(string(eventJSON)),
						ReceiptHandle: aws.String("test-receipt-handle"),
					},
				},
			},
			nil,
		).Once()

		// After processing one message, return empty results to exit the loop
		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{
				Messages: []types.Message{},
			},
			nil,
		)

		mockS3.On("GetObjectTagging", mock.Anything, &s3.GetObjectTaggingInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("test-key-with-tags"),
		}).Return(
			nil,
			errors.New("failed to get object tags"),
		)
		// NOTE: DeleteMessage should NOT be called when fetching tags fails
		// The message should remain in the queue for retry.

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		callbackCalled := false
		err = reader.readAll(ctx, "test-telemetry", func(context.Context, string, []byte) error {
			callbackCalled = true
			return nil
		})

		// Context timeout is expected since readAll runs until context is done
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.False(t, callbackCalled, "Callback should have not have been called due to GetObjectTagging error")

		mockS3.AssertExpectations(t)
		mockSQS.AssertExpectations(t)
		// Verify DeleteMessage was never called - message should remain for retry.
		mockSQS.AssertNotCalled(t, "DeleteMessage", mock.Anything, mock.Anything)
		mockS3.AssertNotCalled(t, "GetObject", mock.Anything, mock.Anything)
		mockS3.AssertNotCalled(t, "PutObjectTagging", mock.Anything, mock.Anything)
	})
}

func TestS3SQSReader_ReadAllWithPrefix(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{
		S3Downloader: S3DownloaderConfig{
			S3Bucket: "test-bucket",
			S3Prefix: "logs/",
			Region:   "us-east-1",
		},
		SQS: &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			Region:   "us-east-1",
		},
	}

	mockS3 := new(mockS3ClientSQS)
	mockSQS := new(mockSQSClient)

	reader := &s3SQSNotificationReader{
		logger:              logger,
		s3Client:            mockS3,
		sqsClient:           mockSQS,
		queueURL:            cfg.SQS.QueueURL,
		s3Bucket:            cfg.S3Downloader.S3Bucket,
		s3Prefix:            cfg.S3Downloader.S3Prefix,
		maxNumberOfMessages: 10,
		waitTimeSeconds:     20,
	}

	// Create S3 event notification with multiple objects - some matching the prefix, some not
	s3Event := s3EventNotification{
		Records: []s3EventRecord{
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "logs/matched-key-1",
					},
				},
			},
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "data/unmatched-key",
					},
				},
			},
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "logs/matched-key-2",
					},
				},
			},
		},
	}

	eventJSON, err := json.Marshal(s3Event)
	require.NoError(t, err)

	snsNotification := snsMessage{
		Type:    "Notification",
		Message: string(eventJSON),
	}

	snsJSON, err := json.Marshal(snsNotification)
	require.NoError(t, err)

	mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			input.MaxNumberOfMessages == 10 &&
			input.WaitTimeSeconds == 20
	})).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{
					Body:          aws.String(string(snsJSON)),
					ReceiptHandle: aws.String("test-receipt-handle"),
				},
			},
		},
		nil,
	).Once()

	// After processing one message, return empty results to exit the loop
	mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{},
		},
		nil,
	)

	// Mock S3 object retrieval ONLY for keys that match the prefix
	mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("logs/matched-key-1"),
	}).Return(
		[]byte("first-matching-content"),
		nil,
	)

	mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("logs/matched-key-2"),
	}).Return(
		[]byte("second-matching-content"),
		nil,
	)

	// Note: We do NOT set up a mock for "data/unmatched-key" because it should never be called

	// Mock message deletion
	mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			*input.ReceiptHandle == "test-receipt-handle"
	})).Return(
		&sqs.DeleteMessageOutput{},
		nil,
	)

	// Run test with callback
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	processedKeys := make(map[string][]byte)

	err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, content []byte) error {
		processedKeys[key] = content
		return nil
	})

	// Context cancellation is expected
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify that only keys matching the prefix were processed
	assert.Len(t, processedKeys, 2)
	assert.Contains(t, processedKeys, "logs/matched-key-1")
	assert.Contains(t, processedKeys, "logs/matched-key-2")
	assert.NotContains(t, processedKeys, "data/unmatched-key")

	// Verify content for matched keys
	assert.Equal(t, []byte("first-matching-content"), processedKeys["logs/matched-key-1"])
	assert.Equal(t, []byte("second-matching-content"), processedKeys["logs/matched-key-2"])
}

func TestS3SQSReader_PutTag(t *testing.T) {
	testCases := []struct {
		name                  string
		s3Records             []s3EventRecord
		setupMocks            func(*mockS3ClientSQS, *mockSQSClient, map[string]bool)
		expectedProcessedKeys map[string][]byte
		expectedTaggedKeys    []string
	}{
		{
			name: "single object",
			s3Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "test-key"},
					},
				},
			},
			setupMocks: func(mockS3 *mockS3ClientSQS, mockSQS *mockSQSClient, taggedKeys map[string]bool) {
				mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
					return *input.Bucket == "test-bucket" && *input.Key == "test-key"
				})).Return([]byte("test-content"), nil)

				mockS3.On("PutObjectTagging", mock.Anything, mock.MatchedBy(func(input *s3.PutObjectTaggingInput) bool {
					return *input.Bucket == "test-bucket" && *input.Key == "test-key"
				})).Run(func(args mock.Arguments) {
					input := args.Get(1).(*s3.PutObjectTaggingInput)
					taggedKeys[*input.Key] = true
				}).Return(&s3.PutObjectTaggingOutput{}, nil)

				// Mock successful message deletion
				mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
					return *input.QueueUrl == "test-queue-url" && *input.ReceiptHandle == "test-receipt-handle"
				})).Return(&sqs.DeleteMessageOutput{}, nil)
			},
			expectedProcessedKeys: map[string][]byte{
				"test-key": []byte("test-content"),
			},
			expectedTaggedKeys: []string{"test-key"},
		},
		{
			name: "tag error",
			s3Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "test-key"},
					},
				},
			},
			setupMocks: func(mockS3 *mockS3ClientSQS, mockSQS *mockSQSClient, _ map[string]bool) {
				mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
					return *input.Bucket == "test-bucket" && *input.Key == "test-key"
				})).Return([]byte("test-content"), nil)

				// Mock tag failure
				mockS3.On("PutObjectTagging", mock.Anything, mock.MatchedBy(func(input *s3.PutObjectTaggingInput) bool {
					return *input.Bucket == "test-bucket" && *input.Key == "test-key"
				})).Return((*s3.PutObjectTaggingOutput)(nil), errors.New("access denied"))

				// Mock successful message deletion (should still delete message even if S3 tagging fails)
				mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
					return *input.QueueUrl == "test-queue-url" && *input.ReceiptHandle == "test-receipt-handle"
				})).Return(&sqs.DeleteMessageOutput{}, nil)
			},
			expectedProcessedKeys: map[string][]byte{
				"test-key": []byte("test-content"),
			},
			expectedTaggedKeys: []string{},
		},
		{
			name: "multiple objects",
			s3Records: []s3EventRecord{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "key1"},
					},
				},
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "key2"},
					},
				},
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: s3Data{
						Bucket: s3BucketData{Name: "test-bucket"},
						Object: s3ObjectData{Key: "key3"},
					},
				},
			},
			setupMocks: func(mockS3 *mockS3ClientSQS, mockSQS *mockSQSClient, taggedKeys map[string]bool) {
				// Mock GetObject and DeleteObject for multiple keys
				keys := []string{"key1", "key2", "key3"}
				for _, key := range keys {
					keyCopy := key // Capture key in closure
					mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
						return *input.Bucket == "test-bucket" && *input.Key == keyCopy
					})).Return([]byte("content-"+keyCopy), nil)

					// Mock successful tagging for each key
					mockS3.On("PutObjectTagging", mock.Anything, mock.MatchedBy(func(input *s3.PutObjectTaggingInput) bool {
						return *input.Bucket == "test-bucket" && *input.Key == keyCopy
					})).Run(func(args mock.Arguments) {
						input := args.Get(1).(*s3.PutObjectTaggingInput)
						taggedKeys[*input.Key] = true
					}).Return(&s3.PutObjectTaggingOutput{}, nil)
				}

				// Mock successful message deletion
				mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
					return *input.QueueUrl == "test-queue-url" && *input.ReceiptHandle == "test-receipt-handle"
				})).Return(&sqs.DeleteMessageOutput{}, nil)
			},
			expectedProcessedKeys: map[string][]byte{
				"key1": []byte("content-key1"),
				"key2": []byte("content-key2"),
				"key3": []byte("content-key3"),
			},
			expectedTaggedKeys: []string{"key1", "key2", "key3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock clients
			mockSQS := new(mockSQSClient)
			mockS3 := new(mockS3ClientSQS)

			// Track tagged objects
			taggedKeys := make(map[string]bool)

			// Create S3 event notification
			s3Event := s3EventNotification{
				Records: tc.s3Records,
			}

			s3EventJSON, err := json.Marshal(s3Event)
			require.NoError(t, err)

			receiptHandle := "test-receipt-handle"

			// Mock SQS ReceiveMessage to return S3 event, then context timeout
			mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
				return *input.QueueUrl == "test-queue-url"
			})).Return(&sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{
						Body:          aws.String(string(s3EventJSON)),
						ReceiptHandle: &receiptHandle,
					},
				},
			}, nil).Once()

			// Second call returns timeout to end the loop
			mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
				(*sqs.ReceiveMessageOutput)(nil), context.DeadlineExceeded,
			)

			// Setup test-specific mocks
			tc.setupMocks(mockS3, mockSQS, taggedKeys)

			reader := &s3SQSNotificationReader{
				logger:                  zap.NewNop(),
				s3Client:                mockS3,
				sqsClient:               mockSQS,
				queueURL:                "test-queue-url",
				s3Bucket:                "test-bucket",
				s3Prefix:                "",
				maxNumberOfMessages:     10,
				waitTimeSeconds:         20,
				tagObjectAfterIngestion: true, // Enable deletion
			}

			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()

			processedKeys := make(map[string][]byte)
			err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, content []byte) error {
				processedKeys[key] = content
				return nil
			})

			// Context deadline exceeded is expected
			assert.Equal(t, context.DeadlineExceeded, err)

			// Verify processed objects
			assert.Equal(t, tc.expectedProcessedKeys, processedKeys)

			// Verify tagged objects if needed
			assert.Len(t, taggedKeys, len(tc.expectedTaggedKeys))
			for _, key := range tc.expectedTaggedKeys {
				assert.True(t, taggedKeys[key], "Object %s should be tagged", key)
			}

			// Verify all mock expectations were met
			mockSQS.AssertExpectations(t)
			mockS3.AssertExpectations(t)
		})
	}
}

func TestS3SQSReader_ReadAllDirectS3TestEvent(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{
		S3Downloader: S3DownloaderConfig{
			S3Bucket: "test-bucket",
			Region:   "us-east-1",
		},
		SQS: &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			Region:   "us-east-1",
		},
	}

	mockS3 := new(mockS3ClientSQS)
	mockSQS := new(mockSQSClient)

	// Create reader with mocks
	reader := &s3SQSNotificationReader{
		logger:              logger,
		s3Client:            mockS3,
		sqsClient:           mockSQS,
		queueURL:            cfg.SQS.QueueURL,
		s3Bucket:            cfg.S3Downloader.S3Bucket,
		s3Prefix:            cfg.S3Downloader.S3Prefix,
		maxNumberOfMessages: 10,
		waitTimeSeconds:     20,
	}

	// This simulates the exact, complete payload AWS sends for an s3:TestEvent
	testEventJSON := `{"Service":"Amazon S3","Event":"s3:TestEvent","Time":"2023-10-25T12:00:00.000Z","Bucket":"test-bucket","RequestId":"5582815E1EA5EF29","HostId":"8cLeGAmw098X5cv4Zkwcmo8vvZa3eH3eKxsPzbB9wrR+YstdA6Knx4Ip8EXAMPLE"}`

	// Mock SQS message reception with the test event
	mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL
	})).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{
					Body:          aws.String(testEventJSON),
					ReceiptHandle: aws.String("test-event-receipt-handle"),
				},
			},
		},
		nil,
	).Once()

	// After processing one message, return empty results to exit test loop
	mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{},
		},
		nil,
	)

	mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			*input.ReceiptHandle == "test-event-receipt-handle"
	})).Return(
		&sqs.DeleteMessageOutput{},
		nil,
	)

	// Run test with callback
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	var callbackCalled bool

	err := reader.readAll(ctx, "test-telemetry", func(_ context.Context, _ string, _ []byte) error {
		callbackCalled = true
		return nil
	})

	// Context cancellation is expected to break the infinite reader loop
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify the data callback was NOT called (since there are 0 records in a TestEvent)
	assert.False(t, callbackCalled)

	// Verify all expectations (proves DeleteMessage was actually triggered)
	mockS3.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

func TestS3SQSReader_ReadAll_SkipTagged(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{
		S3Downloader: S3DownloaderConfig{
			S3Bucket: "test-bucket",
			Region:   "us-east-1",
		},
		SQS: &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			Region:   "us-east-1",
		},
	}

	mockS3 := new(mockS3ClientSQS)
	mockSQS := new(mockSQSClient)

	reader := &s3SQSNotificationReader{
		logger:                     logger,
		s3Client:                   mockS3,
		sqsClient:                  mockSQS,
		queueURL:                   cfg.SQS.QueueURL,
		s3Bucket:                   cfg.S3Downloader.S3Bucket,
		s3Prefix:                   cfg.S3Downloader.S3Prefix,
		maxNumberOfMessages:        10,
		waitTimeSeconds:            20,
		skipIngestingTaggedObjects: true,
	}

	s3Event := s3EventNotification{
		Records: []s3EventRecord{
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "test-key",
					},
				},
			},
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "test-key-2",
					},
				},
			},
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: s3Data{
					Bucket: s3BucketData{
						Name: "test-bucket",
					},
					Object: s3ObjectData{
						Key: "test-key-3",
					},
				},
			},
		},
	}

	eventJSON, err := json.Marshal(s3Event)
	require.NoError(t, err)

	snsNotification := snsMessage{
		Type:    "Notification",
		Message: string(eventJSON),
	}

	snsJSON, err := json.Marshal(snsNotification)
	require.NoError(t, err)

	mockSQS.On("ReceiveMessage", mock.Anything, mock.MatchedBy(func(input *sqs.ReceiveMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			input.MaxNumberOfMessages == 10 &&
			input.WaitTimeSeconds == 20
	})).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{
					Body:          aws.String(string(snsJSON)),
					ReceiptHandle: aws.String("test-receipt-handle"),
				},
			},
		},
		nil,
	).Once()

	// After processing one message, return empty results to exit the loop
	mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []types.Message{},
		},
		nil,
	)

	// First key is already tagged, so it won't be fetched
	mockS3.On("GetObjectTagging", mock.Anything, &s3.GetObjectTaggingInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key"),
	}).Return(
		&s3.GetObjectTaggingOutput{
			TagSet: []s3types.Tag{{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)}},
		},
		nil,
	)

	mockS3.On("GetObjectTagging", mock.Anything, &s3.GetObjectTaggingInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key-2"),
	}).Return(
		&s3.GetObjectTaggingOutput{
			TagSet: []s3types.Tag{},
		},
		nil,
	)
	mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key-2"),
	}).Return(
		[]byte("test-content-2"),
		nil,
	)

	// last key has a tag, but not set by the receiver
	mockS3.On("GetObjectTagging", mock.Anything, &s3.GetObjectTaggingInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key-3"),
	}).Return(
		&s3.GetObjectTaggingOutput{
			TagSet: []s3types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}},
		},
		nil,
	)
	mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key-3"),
	}).Return(
		[]byte("test-content-3"),
		nil,
	)

	mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			*input.ReceiptHandle == "test-receipt-handle"
	})).Return(
		&sqs.DeleteMessageOutput{},
		nil,
	)

	// Run test with callback
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	var callbackCallCount int
	var receivedKeys []string
	var receivedContents [][]byte

	err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, content []byte) error {
		callbackCallCount++
		receivedKeys = append(receivedKeys, key)
		receivedContents = append(receivedContents, content)
		return nil
	})

	// Context cancellation is expected
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify callback was called with correct data
	assert.Equal(t, 2, callbackCallCount)
	assert.Equal(t, "test-key-2", receivedKeys[0])
	assert.Equal(t, "test-key-3", receivedKeys[1])
	assert.Equal(t, []byte("test-content-2"), receivedContents[0])
	assert.Equal(t, []byte("test-content-3"), receivedContents[1])

	// Verify all expectations
	mockS3.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
	mockS3.AssertNotCalled(t, "GetObject", mock.Anything, &s3.GetObjectTaggingInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key"),
	})
}
