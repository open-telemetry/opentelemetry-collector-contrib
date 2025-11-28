// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

type (
	unmarshalFunc[T any]       func([]byte) (T, error)
	s3EventConsumerFunc[T any] func(context.Context, time.Time, T) error
	handlerRegistry            map[eventType]func() lambdaEventHandler
)

type handlerProvider interface {
	getHandler(eventType eventType) (lambdaEventHandler, error)
}

// handlerProvider is responsible for providing event handlers based on event types.
// It operates with a registry of handler factories and caches loadedHandlers for reuse.
type handlerProviderImpl struct {
	registry       handlerRegistry
	loadedHandlers map[eventType]lambdaEventHandler
	knownTypes     []string
}

func newHandlerProvider(registry handlerRegistry) handlerProvider {
	var types []string
	for t := range registry {
		types = append(types, string(t))
	}

	return &handlerProviderImpl{
		loadedHandlers: map[eventType]lambdaEventHandler{},
		registry:       registry,
		knownTypes:     types,
	}
}

func (h *handlerProviderImpl) getHandler(eventType eventType) (lambdaEventHandler, error) {
	if loaded, exists := h.loadedHandlers[eventType]; exists {
		return loaded, nil
	}

	factory, exists := h.registry[eventType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for event type %s, known types: '%s'", eventType, strings.Join(h.knownTypes, ","))
	}

	handler := factory()
	h.loadedHandlers[eventType] = handler
	return handler, nil
}

// lambdaEventHandler defines the contract for AWS Lambda event handlers
type lambdaEventHandler interface {
	handlerType() eventType
	handle(ctx context.Context, event json.RawMessage) error
}

// s3Handler is specialized in S3 object event handling
type s3Handler[T any] struct {
	s3Service     internal.S3Service
	logger        *zap.Logger
	s3Unmarshaler unmarshalFunc[T]
	consumer      s3EventConsumerFunc[T]
}

func newS3Handler[T any](
	service internal.S3Service,
	baseLogger *zap.Logger,
	unmarshal unmarshalFunc[T],
	consumer s3EventConsumerFunc[T],
) *s3Handler[T] {
	return &s3Handler[T]{
		s3Service:     service,
		logger:        baseLogger.Named("s3"),
		s3Unmarshaler: unmarshal,
		consumer:      consumer,
	}
}

func (*s3Handler[T]) handlerType() eventType {
	return s3Event
}

func (s *s3Handler[T]) handle(ctx context.Context, event json.RawMessage) error {
	parsedEvent, err := s.parseEvent(event)
	if err != nil {
		return fmt.Errorf("failed to parse the event: %w", err)
	}

	s.logger.Debug("Processing S3 event notification.",
		zap.String("File", parsedEvent.S3.Object.Key),
		zap.String("S3Bucket", parsedEvent.S3.Bucket.Arn),
	)

	// Skip processing zero length objects. This includes events from folder creation and empty object.
	if parsedEvent.S3.Object.Size == 0 {
		s.logger.Info("Empty object, skipping download", zap.String("File", parsedEvent.S3.Object.Key))
		return nil
	}

	body, err := s.s3Service.ReadObject(ctx, parsedEvent.S3.Bucket.Name, parsedEvent.S3.Object.Key)
	if err != nil {
		return err
	}

	data, err := s.s3Unmarshaler(body)
	if err != nil {
		return fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	if err := s.consumer(ctx, parsedEvent.EventTime, data); err != nil {
		// If permanent, return as-is (don't retry)
		if consumererror.IsPermanent(err) {
			return err
		}

		// If already wrapped as a consumererror, return as-is
		var consumerErr *consumererror.Error
		if errors.As(err, &consumerErr) {
			return err
		}

		// Plain error - wrap as retryable
		return consumererror.NewRetryableError(err)
	}

	return nil
}

func (*s3Handler[T]) parseEvent(raw json.RawMessage) (event events.S3EventRecord, err error) {
	var message events.S3Event
	if err := gojson.Unmarshal(raw, &message); err != nil {
		return events.S3EventRecord{}, fmt.Errorf("failed to unmarshal S3 event notification: %w", err)
	}

	// Records cannot be more than 1 in case of s3 event notifications
	if len(message.Records) > 1 || len(message.Records) == 0 {
		return events.S3EventRecord{}, fmt.Errorf("s3 event notification should contain one record instead of %d", len(message.Records))
	}

	return message.Records[0], nil
}

// cwLogsSubscriptionHandler is specialized in CloudWatch log stream subscription filter events
type cwLogsSubscriptionHandler struct {
	logger    *zap.Logger
	unmarshal unmarshalFunc[plog.Logs]
	consumer  func(context.Context, plog.Logs) error
}

func newCWLogsSubscriptionHandler(
	baseLogger *zap.Logger,
	unmarshal unmarshalFunc[plog.Logs],
	consumer func(context.Context, plog.Logs) error,
) *cwLogsSubscriptionHandler {
	return &cwLogsSubscriptionHandler{
		logger:    baseLogger.Named("cw-logs-subscription"),
		unmarshal: unmarshal,
		consumer:  consumer,
	}
}

func (*cwLogsSubscriptionHandler) handlerType() eventType {
	return cwEvent
}

func (c *cwLogsSubscriptionHandler) handle(ctx context.Context, event json.RawMessage) error {
	var log events.CloudwatchLogsEvent
	if err := gojson.Unmarshal(event, &log); err != nil {
		return fmt.Errorf("failed to unmarshal cloudwatch event log: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(log.AWSLogs.Data)
	if err != nil {
		return fmt.Errorf("failed to decode data from cloudwatch logs event: %w", err)
	}

	data, err := c.unmarshal(decoded)
	if err != nil {
		return fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	if err := c.consumer(ctx, data); err != nil {
		// consumer errors are marked for retrying
		return consumererror.NewRetryableError(err)
	}

	return nil
}

// setObservedTimestampForAllLogs adds observedTimestamp to all logs
func setObservedTimestampForAllLogs(logs plog.Logs, observedTimestamp time.Time) {
	for _, resourceLogs := range logs.ResourceLogs().All() {
		for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
			for _, logRecord := range scopeLogs.LogRecords().All() {
				logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(observedTimestamp))
			}
		}
	}
}
