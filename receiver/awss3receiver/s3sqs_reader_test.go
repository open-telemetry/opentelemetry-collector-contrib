// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
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

// Mock GetObjectAPI implementation for testing
type mockS3ClientSNS struct {
	mock.Mock
}

func (m *mockS3ClientSNS) GetObject(ctx context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
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

// mockSQSClient implements the SQSClient interface for testing
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

func (m *mockSQSClient) DeleteQueue(ctx context.Context, params *sqs.DeleteQueueInput, _ ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.DeleteQueueOutput), args.Error(1)
}

func TestNewSNSReader(t *testing.T) {
	logger := zap.NewNop()

	t.Run("fails with nil SNS config", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket: "test-bucket",
				Region:   "us-east-1",
			},
		}

		reader, err := newSQSReader(context.Background(), logger, cfg)
		assert.Error(t, err)
		assert.Nil(t, reader)
	})

	t.Run("succeeds with valid SNS config", func(t *testing.T) {
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

		// This test will fail due to AWS SDK calls, but we're only testing initialization logic
		r, err := newSQSReader(context.Background(), logger, cfg)
		assert.NotNil(t, r)
		// Error is expected because we can't create real AWS sessions in a unit test
		assert.NoError(t, err)
	})
}

func TestSNSReadAll(t *testing.T) {
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

	mockS3 := new(mockS3ClientSNS)
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
		Records: []struct {
			EventSource string `json:"eventSource"`
			EventName   string `json:"eventName"`
			S3          struct {
				Bucket struct {
					Name string `json:"name"`
				} `json:"bucket"`
				Object struct {
					Key string `json:"key"`
				} `json:"object"`
			} `json:"s3"`
		}{
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: struct {
					Bucket struct {
						Name string `json:"name"`
					} `json:"bucket"`
					Object struct {
						Key string `json:"key"`
					} `json:"object"`
				}{
					Bucket: struct {
						Name string `json:"name"`
					}{
						Name: "test-bucket",
					},
					Object: struct {
						Key string `json:"key"`
					}{
						Key: "test-key",
					},
				},
			},
		},
	}

	eventJSON, err := json.Marshal(s3Event)
	require.NoError(t, err)

	snsNotification := struct {
		Type    string `json:"Type"`
		Message string `json:"Message"`
	}{
		Type:    "Notification",
		Message: string(eventJSON),
	}

	snsJSON, err := json.Marshal(snsNotification)
	require.NoError(t, err)

	// Mock SQS message reception
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
	).Once() // Only return the message once

	// After processing one message, return empty results to exit the loop
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
		[]byte("test-content"),
		nil,
	)

	// Mock message deletion
	mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL &&
			*input.ReceiptHandle == "test-receipt-handle"
	})).Return(
		&sqs.DeleteMessageOutput{},
		nil,
	)

	// Mock queue deletion on shutdown
	mockSQS.On("DeleteQueue", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteQueueInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL
	})).Return(
		&sqs.DeleteQueueOutput{},
		nil,
	)

	// Run test with callback
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var callbackCalled bool
	var receivedKey string
	var receivedContent []byte
	var callbackMu sync.Mutex

	err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, content []byte) error {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCalled = true
		receivedKey = key
		receivedContent = content
		return nil
	})

	// Context cancellation is expected
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify callback was called with correct data
	callbackMu.Lock()
	defer callbackMu.Unlock()
	assert.True(t, callbackCalled)
	assert.Equal(t, "test-key", receivedKey)
	assert.Equal(t, []byte("test-content"), receivedContent)

	// Verify all expectations
	mockS3.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

func TestReadAllErrorHandling(t *testing.T) {
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
		mockS3 := new(mockS3ClientSNS)
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

		// Mock error during receive messages
		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			nil,
			errors.New("context canceled"),
		)

		// Mock queue deletion on shutdown
		mockSQS.On("DeleteQueue", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteQueueInput) bool {
			return *input.QueueUrl == cfg.SQS.QueueURL
		})).Return(
			&sqs.DeleteQueueOutput{},
			nil,
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately to trigger error case

		err := reader.readAll(ctx, "test-telemetry", nil)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("handles S3 object retrieval error", func(t *testing.T) {
		mockS3 := new(mockS3ClientSNS)
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
			Records: []struct {
				EventSource string `json:"eventSource"`
				EventName   string `json:"eventName"`
				S3          struct {
					Bucket struct {
						Name string `json:"name"`
					} `json:"bucket"`
					Object struct {
						Key string `json:"key"`
					} `json:"object"`
				} `json:"s3"`
			}{
				{
					EventSource: "aws:s3",
					EventName:   "ObjectCreated:Put",
					S3: struct {
						Bucket struct {
							Name string `json:"name"`
						} `json:"bucket"`
						Object struct {
							Key string `json:"key"`
						} `json:"object"`
					}{
						Bucket: struct {
							Name string `json:"name"`
						}{
							Name: "test-bucket",
						},
						Object: struct {
							Key string `json:"key"`
						}{
							Key: "test-key",
						},
					},
				},
			},
		}

		eventJSON, err := json.Marshal(s3Event)
		require.NoError(t, err)

		snsNotification := struct {
			Type    string `json:"Type"`
			Message string `json:"Message"`
		}{
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

		// After processing one message, simulate context cancellation
		mockSQS.On("ReceiveMessage", mock.Anything, mock.Anything).Return(
			&sqs.ReceiveMessageOutput{},
			errors.New("context canceled"),
		)

		// Mock S3 object retrieval with error
		mockS3.On("GetObject", mock.Anything, &s3.GetObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String("test-key"),
		}).Return(
			[]byte{},
			errors.New("object retrieval failed"),
		)

		// Mock message deletion
		mockSQS.On("DeleteMessage", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteMessageInput) bool {
			return *input.QueueUrl == cfg.SQS.QueueURL &&
				*input.ReceiptHandle == "test-receipt-handle"
		})).Return(
			&sqs.DeleteMessageOutput{},
			nil,
		)

		// Mock queue deletion on shutdown
		mockSQS.On("DeleteQueue", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteQueueInput) bool {
			return *input.QueueUrl == cfg.SQS.QueueURL
		})).Return(
			&sqs.DeleteQueueOutput{},
			nil,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, _ string, _ []byte) error {
			t.Fatal("Callback should not be called when S3 retrieval fails")
			return nil
		})

		assert.Error(t, err)
	})
}

func TestSNSReadAllWithPrefix(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{
		S3Downloader: S3DownloaderConfig{
			S3Bucket: "test-bucket",
			S3Prefix: "logs/", // Setting a prefix
			Region:   "us-east-1",
		},
		SQS: &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			Region:   "us-east-1",
		},
	}

	mockS3 := new(mockS3ClientSNS)
	mockSQS := new(mockSQSClient)

	// Create reader with mocks and prefix
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
		Records: []struct {
			EventSource string `json:"eventSource"`
			EventName   string `json:"eventName"`
			S3          struct {
				Bucket struct {
					Name string `json:"name"`
				} `json:"bucket"`
				Object struct {
					Key string `json:"key"`
				} `json:"object"`
			} `json:"s3"`
		}{
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: struct {
					Bucket struct {
						Name string `json:"name"`
					} `json:"bucket"`
					Object struct {
						Key string `json:"key"`
					} `json:"object"`
				}{
					Bucket: struct {
						Name string `json:"name"`
					}{
						Name: "test-bucket",
					},
					Object: struct {
						Key string `json:"key"`
					}{
						Key: "logs/matched-key-1", // This matches the prefix
					},
				},
			},
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: struct {
					Bucket struct {
						Name string `json:"name"`
					} `json:"bucket"`
					Object struct {
						Key string `json:"key"`
					} `json:"object"`
				}{
					Bucket: struct {
						Name string `json:"name"`
					}{
						Name: "test-bucket",
					},
					Object: struct {
						Key string `json:"key"`
					}{
						Key: "data/unmatched-key", // This doesn't match the prefix
					},
				},
			},
			{
				EventSource: "aws:s3",
				EventName:   "ObjectCreated:Put",
				S3: struct {
					Bucket struct {
						Name string `json:"name"`
					} `json:"bucket"`
					Object struct {
						Key string `json:"key"`
					} `json:"object"`
				}{
					Bucket: struct {
						Name string `json:"name"`
					}{
						Name: "test-bucket",
					},
					Object: struct {
						Key string `json:"key"`
					}{
						Key: "logs/matched-key-2", // This matches the prefix
					},
				},
			},
		},
	}

	eventJSON, err := json.Marshal(s3Event)
	require.NoError(t, err)

	snsNotification := struct {
		Type    string `json:"Type"`
		Message string `json:"Message"`
	}{
		Type:    "Notification",
		Message: string(eventJSON),
	}

	snsJSON, err := json.Marshal(snsNotification)
	require.NoError(t, err)

	// Mock SQS message reception
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
	).Once() // Only return the message once

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

	// Mock queue deletion on shutdown
	mockSQS.On("DeleteQueue", mock.Anything, mock.MatchedBy(func(input *sqs.DeleteQueueInput) bool {
		return *input.QueueUrl == cfg.SQS.QueueURL
	})).Return(
		&sqs.DeleteQueueOutput{},
		nil,
	)

	// Run test with callback
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	processedKeys := make(map[string][]byte)
	var mu sync.Mutex

	err = reader.readAll(ctx, "test-telemetry", func(_ context.Context, key string, content []byte) error {
		mu.Lock()
		defer mu.Unlock()
		processedKeys[key] = content
		return nil
	})

	// Context cancellation is expected
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify that only keys matching the prefix were processed
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, processedKeys, 2)
	assert.Contains(t, processedKeys, "logs/matched-key-1")
	assert.Contains(t, processedKeys, "logs/matched-key-2")
	assert.NotContains(t, processedKeys, "data/unmatched-key")

	// Verify content for matched keys
	assert.Equal(t, []byte("first-matching-content"), processedKeys["logs/matched-key-1"])
	assert.Equal(t, []byte("second-matching-content"), processedKeys["logs/matched-key-2"])
}
