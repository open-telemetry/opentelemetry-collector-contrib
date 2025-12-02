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
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal/metadata"
)

type emits interface {
	plog.Logs | pmetric.Metrics
}

type (
	unmarshalFunc[T emits]       func([]byte) (T, error)
	s3EventConsumerFunc[T emits] func(context.Context, time.Time, T) error
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
	emitType      T
}

func newS3Handler[T emits](
	service internal.S3Service,
	baseLogger *zap.Logger,
	unmarshal unmarshalFunc[T],
	consumer s3EventConsumerFunc[T],
	emitType T,
) *s3Handler[T] {
	return &s3Handler[T]{
		s3Service:     service,
		logger:        baseLogger.Named("s3"),
		s3Unmarshaler: unmarshal,
		consumer:      consumer,
		emitType:      emitType,
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

	var data T
	if s.s3Unmarshaler == nil {
		switch any(s.emitType).(type) {
		case plog.Logs:
			data = bytesToPlogs(body, parsedEvent).(T)
		case pmetric.Metrics:
			data = bytesToPMetrics(body, parsedEvent).(T)
		}
	} else {
		data, err = s.s3Unmarshaler(body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal logs: %w", err)
		}
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

	var data plog.Logs
	if c.unmarshal == nil {
		var reader *gzip.Reader
		reader, err = gzip.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return fmt.Errorf("failed to decompress data from cloudwatch subscription event: %w", err)
		}

		var cwLog events.CloudwatchLogsData
		decoder := gojson.NewDecoder(reader)
		err = decoder.Decode(&cwLog)
		if err != nil {
			return fmt.Errorf("failed to unmarshal data from cloudwatch logs event: %w", err)
		}

		data = cwLogsToPlogs(cwLog)
	} else {
		data, err = c.unmarshal(decoded)
		if err != nil {
			return fmt.Errorf("failed to unmarshal logs: %w", err)
		}
	}

	if err := c.consumer(ctx, data); err != nil {
		// consumer errors are marked for retrying
		return consumererror.NewRetryableError(err)
	}

	return nil
}

func cwLogsToPlogs(cwLog events.CloudwatchLogsData) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	resourceAttrs.PutStr(string(conventions.CloudAccountIDKey), cwLog.Owner)
	resourceAttrs.PutEmptySlice(string(conventions.AWSLogGroupNamesKey)).AppendEmpty().SetStr(cwLog.LogGroup)
	resourceAttrs.PutEmptySlice(string(conventions.AWSLogStreamNamesKey)).AppendEmpty().SetStr(cwLog.LogStream)
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

	return logs
}

func bytesToPlogs(data []byte, event events.S3EventRecord) any {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	resourceAttrs.PutStr(string(conventions.CloudRegionKey), event.AWSRegion)
	resourceAttrs.PutStr(string(conventions.AWSS3BucketKey), event.S3.Bucket.Arn)
	resourceAttrs.PutStr(string(conventions.AWSS3KeyKey), event.S3.Object.Key)

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)

	rec := sl.LogRecords().AppendEmpty()
	rec.SetTimestamp(pcommon.NewTimestampFromTime(event.EventTime))
	rec.Attributes().PutInt("content.length", int64(len(data)))

	if utf8.Valid(data) {
		rec.Body().SetStr(string(data))
	} else {
		rec.Attributes().PutStr("encoding", "base64")
		rec.Body().SetStr(base64.StdEncoding.EncodeToString(data))
	}

	return logs
}

func bytesToPMetrics(data []byte, event events.S3EventRecord) any {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	resourceAttrs.PutStr(string(conventions.CloudRegionKey), event.AWSRegion)
	resourceAttrs.PutStr(string(conventions.AWSS3BucketKey), event.S3.Bucket.Arn)
	resourceAttrs.PutStr(string(conventions.AWSS3KeyKey), event.S3.Object.Key)

	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("s3.object.data")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(event.EventTime))
	dp.Attributes().PutInt("content.length", int64(len(data)))
	dp.Attributes().PutEmptyBytes("raw_payload").FromRaw(data)
	return metrics
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
