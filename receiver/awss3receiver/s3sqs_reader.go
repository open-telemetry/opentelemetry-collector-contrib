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

// S3ObjectData represents an S3 object in the notification
type S3ObjectData struct {
	Key string `json:"key"`
}

// S3BucketData represents an S3 bucket in the notification
type S3BucketData struct {
	Name string `json:"name"`
}

// S3Data represents the S3 specific data in the notification
type S3Data struct {
	Bucket S3BucketData `json:"bucket"`
	Object S3ObjectData `json:"object"`
}

// S3EventRecord represents a single record in an S3 event notification
type S3EventRecord struct {
	EventSource string `json:"eventSource"`
	EventName   string `json:"eventName"`
	S3          S3Data `json:"s3"`
}

// s3EventNotification is the top-level structure for S3 event notifications
type s3EventNotification struct {
	Records []S3EventRecord `json:"Records"`
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
	r.logger.Info("Starting SQS notification processing",
		zap.String("queueURL", r.queueURL),
		zap.String("s3Bucket", r.s3Bucket),
		zap.String("s3Prefix", r.s3Prefix),
		zap.Int32("maxNumberOfMessages", r.maxNumberOfMessages),
		zap.Int32("waitTimeSeconds", r.waitTimeSeconds))
	// Process messages until context is canceled
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Context canceled, stopping SQS notification processing")
			return ctx.Err()
		default:
			r.logger.Info("Waiting for messages from SQS")
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
			r.logger.Info("Received messages from SQS",
				zap.Int("messageCount", len(result.Messages)))

			// Process each message
			for _, message := range result.Messages {
				var s3Event s3EventNotification
				messageBody := *message.Body

				// First try to parse as direct S3 event notification
				err := json.Unmarshal([]byte(messageBody), &s3Event)
				if err != nil || len(s3Event.Records) == 0 {
					// If direct parsing failed, try to extract from SNS notification format
					r.logger.Debug("Direct parsing as S3 event failed, trying SNS format", zap.Error(err))

					var snsMessage struct {
						Type    string `json:"Type"`
						Message string `json:"Message"`
					}

					if err := json.Unmarshal([]byte(messageBody), &snsMessage); err != nil {
						r.logger.Warn("Failed to parse message as SNS notification", zap.Error(err))
						continue
					}

					if snsMessage.Type == "Notification" {
						if err := json.Unmarshal([]byte(snsMessage.Message), &s3Event); err != nil {
							r.logger.Warn("Failed to parse S3 event from SNS message", zap.Error(err))
							continue
						}
					} else {
						r.logger.Warn("Message is not a valid S3 notification",
							zap.String("type", snsMessage.Type))
						continue
					}
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
