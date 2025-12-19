// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal/metadata"
)

type emits interface {
	plog.Logs | pmetric.Metrics
}

type (
	unmarshalFunc[T emits]       func([]byte) (T, error)
	s3EventConsumerFunc[T emits] func(context.Context, events.S3EventRecord, T) error
	handlerRegistry              map[eventType]func() lambdaEventHandler
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
type s3Handler[T emits] struct {
	s3Service     internal.S3Service
	logger        *zap.Logger
	s3Unmarshaler unmarshalFunc[T]
	consumer      s3EventConsumerFunc[T]
}

func newS3Handler[T emits](
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
		return fmt.Errorf("failed to unmarshal S3 data: %w", err)
	}

	if err := s.consumer(ctx, parsedEvent, data); err != nil {
		return checkConsumerErrorAndWrap(err)
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
	unmarshal unmarshalFunc[plog.Logs]
	consumer  func(context.Context, plog.Logs) error
}

func newCWLogsSubscriptionHandler(
	unmarshal unmarshalFunc[plog.Logs],
	consumer func(context.Context, plog.Logs) error,
) *cwLogsSubscriptionHandler {
	return &cwLogsSubscriptionHandler{
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

	var reader *gzip.Reader
	reader, err = gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return fmt.Errorf("failed to decompress data from cloudwatch subscription event: %w", err)
	}

	defer reader.Close()

	var decodedData bytes.Buffer
	_, err = decodedData.ReadFrom(reader)
	if err != nil {
		return fmt.Errorf("failed to read decompressed data from cloudwatch subscription event: %w", err)
	}

	data, err := c.unmarshal(decodedData.Bytes())
	if err != nil {
		return fmt.Errorf("failed to unmarshal CloudWatch logs: %w", err)
	}
	if err := c.consumer(ctx, data); err != nil {
		return checkConsumerErrorAndWrap(err)
	}

	return nil
}

// cwLogsToPlogs implements unmarshalFunc for plog.Logs.
// This defines the built-in behavior for CloudWatch subscription filter events when no encoding extension is provided.
func cwLogsToPlogs(data []byte) (plog.Logs, error) {
	var cwLog events.CloudwatchLogsData
	err := gojson.Unmarshal(data, &cwLog)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal data from cloudwatch logs event: %w", err)
	}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	resourceAttrs.PutStr(string(conventions.CloudAccountIDKey), cwLog.Owner)
	resourceAttrs.PutStr(string(conventions.AWSLogGroupNamesKey), cwLog.LogGroup)
	resourceAttrs.PutStr(string(conventions.AWSLogStreamNamesKey), cwLog.LogStream)

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)

	for _, event := range cwLog.LogEvents {
		logRecord := sl.LogRecords().AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logRecord.Body().SetStr(event.Message)
	}

	return logs, err
}

// bytesToPlogs implements unmarshalFunc for plog.Logs.
// This defines the built-in behavior for S3 events when no encoding extension is provided.
func bytesToPlogs(data []byte) (plog.Logs, error) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)

	lr := sl.LogRecords().AppendEmpty()
	if utf8.Valid(data) {
		lr.Body().SetStr(string(data))
	} else {
		lr.Body().SetEmptyBytes().FromRaw(data)
	}

	return logs, nil
}

func enrichS3Logs(logs plog.Logs, event events.S3EventRecord) {
	for _, resourceLogs := range logs.ResourceLogs().All() {
		resourceAttrs := resourceLogs.Resource().Attributes()
		resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
		resourceAttrs.PutStr(string(conventions.CloudRegionKey), event.AWSRegion)
		resourceAttrs.PutStr(string(conventions.AWSS3BucketKey), event.S3.Bucket.Name)
		resourceAttrs.PutStr(string(conventions.AWSS3KeyKey), event.S3.Object.Key)

		for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
			for _, logRecord := range scopeLogs.LogRecords().All() {
				logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(event.EventTime))
			}
		}
	}
}

// checkConsumerErrorAndWrap is a helper to process errors returned from consumer functions.
func checkConsumerErrorAndWrap(err error) error {
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
