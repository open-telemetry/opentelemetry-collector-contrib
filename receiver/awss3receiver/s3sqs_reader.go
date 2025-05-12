// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/zap"
)

// S3 event notification structure from AWS
// See: https://docs.aws.amazon.com/AmazonS3/latest/userguide/notification-content-structure.html
type s3EventNotification struct {
	Records []struct {
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
	} `json:"Records"`
}

// s3SQSNotificationReader listens for SNS notifications about new S3 objects
type s3SQSNotificationReader struct {
	logger              *zap.Logger
	s3Client            GetObjectAPI
	sqsClient           SQSClient
	queueURL            string
	s3Bucket            string
	s3Prefix            string
	maxNumberOfMessages int32
	waitTimeSeconds     int32
}

func newSQSReader(ctx context.Context, logger *zap.Logger, cfg *Config) (*s3SQSNotificationReader, error) {
	if cfg.SQS == nil {
		return nil, errors.New("SNS configuration is required")
	}

	// Create SQS client
	sqsClient, err := newSQSClient(ctx, cfg.SQS.Region, cfg.SQS.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQS client: %w", err)
	}

	// Create S3 client with specific configuration
	_, getObjectClient, err := newS3Client(ctx, cfg.S3Downloader)
	if err != nil {
		return nil, err
	}

	// Use configured values or defaults for SQS polling parameters
	maxMessages := int32(10) // Default to 10 messages
	if cfg.SQS.MaxNumberOfMessages > 0 && cfg.SQS.MaxNumberOfMessages <= 10 {
		maxMessages = int32(cfg.SQS.MaxNumberOfMessages)
	}

	waitTime := int32(20) // Default to 20 seconds
	if cfg.SQS.WaitTimeSeconds >= 0 && cfg.SQS.WaitTimeSeconds <= 20 {
		waitTime = int32(cfg.SQS.WaitTimeSeconds)
	}

	return &s3SQSNotificationReader{
		logger:              logger,
		s3Client:            getObjectClient,
		sqsClient:           sqsClient,
		queueURL:            cfg.SQS.QueueURL,
		s3Bucket:            cfg.S3Downloader.S3Bucket,
		s3Prefix:            cfg.S3Downloader.S3Prefix,
		maxNumberOfMessages: maxMessages,
		waitTimeSeconds:     waitTime,
	}, nil
}

// readAll monitors SNS notifications and processes new S3 objects
func (r *s3SQSNotificationReader) readAll(ctx context.Context, _ string, callback s3ObjectCallback) error {
	// Process messages until context is canceled
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Context canceled, stopping SNS notification processing")
			// Clean up the SQS queue
			if r.queueURL != "" {
				_, err := r.sqsClient.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
					QueueUrl: aws.String(r.queueURL),
				})
				if err != nil {
					r.logger.Warn("Failed to delete SQS queue during shutdown", zap.Error(err))
				}
			}
			return ctx.Err()
		default:
			// Receive messages from SQS
			result, err := r.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(r.queueURL),
				MaxNumberOfMessages: r.maxNumberOfMessages,
				WaitTimeSeconds:     r.waitTimeSeconds, // Long polling
			})
			if err != nil {
				if ctx.Err() != nil {
					// Context canceled, exit gracefully
					return ctx.Err()
				}
				r.logger.Warn("Error receiving messages from SQS", zap.Error(err))
				// Add a small sleep to avoid tight loops on persistent errors
				time.Sleep(5 * time.Second)
				continue
			}

			// Process each message
			for _, message := range result.Messages {
				// Parse the SNS notification from the SQS message
				var snsMessage struct {
					Type    string `json:"Type"`
					Message string `json:"Message"`
				}

				err := json.Unmarshal([]byte(*message.Body), &snsMessage)
				if err != nil {
					r.logger.Warn("Failed to parse SNS message", zap.Error(err))
					continue
				}

				// Extract S3 event from the SNS message
				if snsMessage.Type == "Notification" {
					var s3Event s3EventNotification
					err = json.Unmarshal([]byte(snsMessage.Message), &s3Event)
					if err != nil {
						r.logger.Warn("Failed to parse S3 event notification", zap.Error(err))
						continue
					}

					// Process each S3 object notification
					for _, record := range s3Event.Records {
						if record.EventSource == "aws:s3" && strings.HasPrefix(record.EventName, "ObjectCreated:") {
							bucket := record.S3.Bucket.Name
							key := record.S3.Object.Key

							// Skip if this object is not in our target bucket
							if bucket != r.s3Bucket {
								r.logger.Debug("Skipping object from different bucket",
									zap.String("bucket", bucket),
									zap.String("targetBucket", r.s3Bucket))
								continue
							}

							// Skip if this is not in our target prefix
							if r.s3Prefix != "" && !strings.HasPrefix(key, r.s3Prefix) {
								r.logger.Debug("Skipping object not matching prefix",
									zap.String("key", key),
									zap.String("prefix", r.s3Prefix))
								continue
							}

							r.logger.Info("Processing new S3 object",
								zap.String("bucket", bucket),
								zap.String("key", key))

							var content []byte
							// Get S3 object content
							content, err = retrieveS3Object(ctx, r.s3Client, bucket, key)
							if err != nil {
								r.logger.Error("Failed to get S3 object",
									zap.String("bucket", bucket),
									zap.String("key", key),
									zap.Error(err))
								continue
							}

							// Process the object content
							err = callback(ctx, key, content)
							if err != nil {
								r.logger.Error("Failed to process S3 object content",
									zap.String("key", key),
									zap.Error(err))
							}
						}
					}
				}

				// Delete processed message from queue
				_, err = r.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(r.queueURL),
					ReceiptHandle: message.ReceiptHandle,
				})
				if err != nil {
					r.logger.Warn("Failed to delete message from SQS queue", zap.Error(err))
				}
			}
		}
	}
}
