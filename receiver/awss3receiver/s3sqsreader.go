// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/zap"
)

// S3 event notification structure from AWS
// See: https://docs.aws.amazon.com/AmazonS3/latest/userguide/notification-content-structure.html

// s3ObjectData represents an S3 object in the notification
type s3ObjectData struct {
	Key string `json:"key"`
}

// s3BucketData represents an S3 bucket in the notification
type s3BucketData struct {
	Name string `json:"name"`
}

// s3Data represents the S3 specific data in the notification
type s3Data struct {
	Bucket s3BucketData `json:"bucket"`
	Object s3ObjectData `json:"object"`
}

// s3EventRecord represents a single record in an S3 event notification
type s3EventRecord struct {
	EventSource string `json:"eventSource"`
	EventName   string `json:"eventName"`
	S3          s3Data `json:"s3"`
}

// s3EventNotification is the top-level structure for S3 event notifications
type s3EventNotification struct {
	Records []s3EventRecord `json:"Records"`
}

// snsMessage represents the structure of an SNS notification message
type snsMessage struct {
	Type    string `json:"Type"`
	Message string `json:"Message"`
}

// s3SQSNotificationReader listens for SNS notifications about new S3 objects
type s3SQSNotificationReader struct {
	logger              *zap.Logger
	s3Client            GetObjectAPI
	sqsClient           sqsClient
	queueURL            string
	s3Bucket            string
	s3Prefix            string
	maxNumberOfMessages int32
	waitTimeSeconds     int32
}

func newS3SQSReader(ctx context.Context, logger *zap.Logger, cfg *Config) (*s3SQSNotificationReader, error) {
	if cfg.SQS == nil {
		return nil, errors.New("SQS configuration is required")
	}

	sqsAPIClient, err := newSQSClient(ctx, cfg.SQS.Region, cfg.SQS.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQS client: %w", err)
	}

	_, getObjectClient, err := newS3Client(ctx, cfg.S3Downloader)
	if err != nil {
		return nil, err
	}

	// Use configured values or defaults for SQS polling parameters
	maxMessages := int32(10) // Default to 10 messages
	if cfg.SQS.MaxNumberOfMessages != nil {
		maxMessages = int32(*cfg.SQS.MaxNumberOfMessages)
	}

	waitTime := int32(20) // Default to 20 seconds
	if cfg.SQS.WaitTimeSeconds != nil {
		waitTime = int32(*cfg.SQS.WaitTimeSeconds)
	}

	return &s3SQSNotificationReader{
		logger:              logger,
		s3Client:            getObjectClient,
		sqsClient:           sqsAPIClient,
		queueURL:            cfg.SQS.QueueURL,
		s3Bucket:            cfg.S3Downloader.S3Bucket,
		s3Prefix:            cfg.S3Downloader.S3Prefix,
		maxNumberOfMessages: maxMessages,
		waitTimeSeconds:     waitTime,
	}, nil
}

func (r *s3SQSNotificationReader) readAll(ctx context.Context, _ string, callback s3ObjectCallback) error {
	r.logger.Info("Starting SQS notification processing",
		zap.String("queueURL", r.queueURL),
		zap.String("s3Bucket", r.s3Bucket),
		zap.String("s3Prefix", r.s3Prefix),
		zap.Int32("maxNumberOfMessages", r.maxNumberOfMessages),
		zap.Int32("waitTimeSeconds", r.waitTimeSeconds))

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Context canceled, stopping SQS notification processing")
			return ctx.Err()
		default:
			r.logger.Debug("Waiting for messages from SQS")
			result, err := r.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(r.queueURL),
				MaxNumberOfMessages: r.maxNumberOfMessages,
				WaitTimeSeconds:     r.waitTimeSeconds,
			})
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				r.logger.Warn("Error receiving messages from SQS", zap.Error(err))
				// Add a small sleep to avoid tight loops on persistent errors
				time.Sleep(5 * time.Second)
				continue
			}
			r.logger.Debug("Received messages from SQS",
				zap.Int("messageCount", len(result.Messages)))

			for _, message := range result.Messages {
				var s3Event s3EventNotification
				messageBody := *message.Body

				// First try to parse as direct S3 event notification
				if err = json.Unmarshal([]byte(messageBody), &s3Event); err != nil || len(s3Event.Records) == 0 {
					// If direct parsing failed, try to extract from SNS notification format
					r.logger.Debug("Direct parsing as S3 event failed, trying SNS format", zap.Error(err))

					var snsMsg snsMessage

					if err = json.Unmarshal([]byte(messageBody), &snsMsg); err != nil {
						r.logger.Warn("Failed to parse message as SNS notification", zap.Error(err))
						continue
					}

					if snsMsg.Type != "Notification" {
						r.logger.Warn("Message is not a valid S3 notification", zap.String("type", snsMsg.Type))
						continue
					}
					if err = json.Unmarshal([]byte(snsMsg.Message), &s3Event); err != nil {
						r.logger.Warn("Failed to parse S3 event from SNS message", zap.Error(err))
						continue
					}
				}

				// Track whether all records were successfully processed.
				// Only delete the message if all records succeed to prevent data loss.
				allRecordsSucceeded := true

				// Process each S3 object notification
				for _, record := range s3Event.Records {
					if record.EventSource != "aws:s3" || !strings.HasPrefix(record.EventName, "ObjectCreated:") {
						continue
					}
					bucket := record.S3.Bucket.Name
					key := record.S3.Object.Key

					// Decode the URL-encoded S3 key
					decodedKey, decodeErr := url.QueryUnescape(key)
					if decodeErr != nil {
						r.logger.Warn("Failed to decode S3 object key, using original", zap.String("key", key), zap.Error(decodeErr))
						decodedKey = key
					}

					if bucket != r.s3Bucket {
						r.logger.Debug("Skipping object from different bucket",
							zap.String("bucket", bucket),
							zap.String("targetBucket", r.s3Bucket))
						continue
					}

					if r.s3Prefix != "" && !strings.HasPrefix(decodedKey, r.s3Prefix) {
						r.logger.Debug("Skipping object not matching prefix",
							zap.String("key", decodedKey),
							zap.String("prefix", r.s3Prefix))
						continue
					}

					r.logger.Info("Processing new S3 object",
						zap.String("bucket", bucket),
						zap.String("key", decodedKey))

					var content []byte
					content, err = retrieveS3Object(ctx, r.s3Client, bucket, decodedKey)
					if err != nil {
						r.logger.Error("Failed to get S3 object",
							zap.String("bucket", bucket),
							zap.String("key", decodedKey),
							zap.Error(err))
						allRecordsSucceeded = false
						continue
					}

					err = callback(ctx, decodedKey, content)
					if err != nil {
						r.logger.Error("Failed to process S3 object content",
							zap.String("key", decodedKey),
							zap.Error(err))
						allRecordsSucceeded = false
						continue
					}
				}

				// Only delete the message if all records were successfully processed.
				// If any record failed, leave the message in the queue for retry.
				if allRecordsSucceeded {
					_, err = r.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
						QueueUrl:      aws.String(r.queueURL),
						ReceiptHandle: message.ReceiptHandle,
					})
					if err != nil {
						r.logger.Warn("Failed to delete message from SQS queue", zap.Error(err))
					}
				} else {
					r.logger.Warn("Message not deleted due to processing failures, will be retried after visibility timeout",
						zap.String("receiptHandle", *message.ReceiptHandle))
				}
			}
		}
	}
}
