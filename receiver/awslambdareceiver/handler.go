// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

// s3StreamBatchSize defines the size of data chunks read from S3 object stream for processing.
const s3StreamBatchSize = 10_000_000 // 10MB chunks

// readerBufferSize defines the buffer size for buffered readers.
const readerBufferSize = 128 * 1024 // 128KB buffer size

type (
	handlerRegistry map[eventType]lambdaEventHandler
)

type handlerProvider interface {
	getHandler(eventType eventType) (lambdaEventHandler, error)
}

// handlerProvider is responsible for providing event handlers based on event types.
// It operates with a registry of handlers and caches loadedHandlers for reuse.
type handlerProviderImpl struct {
	registry   handlerRegistry
	knownTypes []string
}

func newHandlerProvider(registry handlerRegistry) handlerProvider {
	var types []string
	for t := range registry {
		types = append(types, string(t))
	}

	return &handlerProviderImpl{
		registry:   registry,
		knownTypes: types,
	}
}

func (h *handlerProviderImpl) getHandler(eventType eventType) (lambdaEventHandler, error) {
	handler, exists := h.registry[eventType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for event type %s, known types: '%s'", eventType, strings.Join(h.knownTypes, ","))
	}

	return handler, nil
}

// lambdaEventHandler defines the contract for AWS Lambda event handlers
type lambdaEventHandler interface {
	handlerType() eventType
	handle(ctx context.Context, event json.RawMessage) error
}

// s3Handler is specialized in S3 object event handling
type s3Handler struct {
	s3Service internal.S3Service
	logger    *zap.Logger

	decodeF func(ctx context.Context, reader io.Reader, event events.S3EventRecord) error
}

// newS3LogsHandler builds an S3 logs handler. The getDecoder function is called
// per-event with the S3 object key and must return the LogsDecoderFactory to use
// for that object, along with its encoding name for logging purposes.
//
// For single-encoding configs, getDecoder is a closure that returns the same factory
// regardless of the object key. For multi-encoding configs it is the router's GetDecoder.
func newS3LogsHandler(
	service internal.S3Service,
	baseLogger *zap.Logger,
	getDecoder func(objectKey string) (encoding.LogsDecoderFactory, string, error),
	consumer consumer.Logs,
) *s3Handler {
	logger := baseLogger.Named("s3")
	logDecodeF := func(ctx context.Context, reader io.Reader, event events.S3EventRecord) error {
		factory, encodingName, err := getDecoder(event.S3.Object.URLDecodedKey)
		if err != nil {
			return fmt.Errorf("failed to route S3 object: %w", err)
		}
		logger.Debug("Matched encoding for S3 object",
			zap.String("File", event.S3.Object.URLDecodedKey),
			zap.String("Encoding", encodingName),
		)

		var decoder encoding.LogsDecoder
		// Bytes based batching and disable flush on items
		decoder, err = factory.NewLogsDecoder(reader, encoding.WithFlushBytes(s3StreamBatchSize), encoding.WithFlushItems(0))
		if err != nil {
			return fmt.Errorf("failed to derive the extension for S3 logs (encoding %q): %w", encodingName, err)
		}

		for {
			var logs plog.Logs
			logs, err = decoder.DecodeLogs()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return fmt.Errorf("failed to decode S3 logs (encoding %q): %w", encodingName, err)
			}

			enrichS3Logs(logs, event)
			if err = consumer.ConsumeLogs(getEnrichedContext(ctx, event), logs); err != nil {
				return checkConsumerErrorAndWrap(err)
			}
		}

		return nil
	}

	return &s3Handler{
		s3Service: service,
		logger:    logger,
		decodeF:   logDecodeF,
	}
}

func newS3MetricsHandler(
	service internal.S3Service,
	baseLogger *zap.Logger,
	metricsDecoder encoding.MetricsDecoderFactory,
	consumer consumer.Metrics,
) *s3Handler {
	metricDecodeF := func(ctx context.Context, reader io.Reader, event events.S3EventRecord) error {
		var decoder encoding.MetricsDecoder
		// Bytes based batching and disable flush on items
		decoder, err := metricsDecoder.NewMetricsDecoder(reader, encoding.WithFlushBytes(s3StreamBatchSize), encoding.WithFlushItems(0))
		if err != nil {
			return fmt.Errorf("failed to derive the extension for S3 metrics: %w", err)
		}

		for {
			var metrics pmetric.Metrics
			metrics, err = decoder.DecodeMetrics()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return fmt.Errorf("failed to decode S3 metrics: %w", err)
			}

			if err := consumer.ConsumeMetrics(getEnrichedContext(ctx, event), metrics); err != nil {
				return checkConsumerErrorAndWrap(err)
			}
		}

		return nil
	}

	return &s3Handler{
		s3Service: service,
		logger:    baseLogger.Named("s3"),
		decodeF:   metricDecodeF,
	}
}

func (*s3Handler) handlerType() eventType {
	return s3Event
}

func (s *s3Handler) handle(ctx context.Context, event json.RawMessage) error {
	var err error
	parsedEvent, err := parseS3Event(event)
	if err != nil {
		return fmt.Errorf("failed to parse the event: %w", err)
	}

	s.logger.Debug("Processing S3 event notification.",
		zap.String("File", parsedEvent.S3.Object.URLDecodedKey),
		zap.String("S3Bucket", parsedEvent.S3.Bucket.Arn),
	)

	// Skip processing zero length objects. This includes events from folder creation and empty object.
	if parsedEvent.S3.Object.Size == 0 {
		s.logger.Info("Empty object, skipping download", zap.String("File", parsedEvent.S3.Object.URLDecodedKey))
		return nil
	}

	reader, err := s.s3Service.GetReader(ctx, parsedEvent.S3.Bucket.Name, parsedEvent.S3.Object.URLDecodedKey)
	if err != nil {
		return err
	}

	wrappedReader, err := gunzipIfNeeded(reader)
	if err != nil {
		return fmt.Errorf("failed to derive reader with wrapper: %w", err)
	}

	defer func() {
		if gzReader, ok := wrappedReader.(*gzip.Reader); ok {
			_ = gzReader.Close()
		}
	}()

	err = s.decodeF(ctx, wrappedReader, parsedEvent)
	if err != nil {
		return err
	}

	return nil
}

// parseS3Event parses a raw JSON S3 event notification and returns the single S3 event record.
// S3 event notifications always contain exactly one record.
func parseS3Event(raw json.RawMessage) (events.S3EventRecord, error) {
	var message events.S3Event
	if err := gojson.Unmarshal(raw, &message); err != nil {
		return events.S3EventRecord{}, fmt.Errorf("failed to unmarshal S3 event notification: %w", err)
	}

	// This receiver processes one S3 object per invocation; reject events with != 1 record.
	if len(message.Records) > 1 || len(message.Records) == 0 {
		return events.S3EventRecord{}, fmt.Errorf("s3 event notification should contain one record instead of %d", len(message.Records))
	}

	return message.Records[0], nil
}

// cwLogsSubscriptionHandler is specialized in CloudWatch log stream subscription filter events
type cwLogsSubscriptionHandler struct {
	logsDecoder encoding.LogsDecoderFactory
	consumer    consumer.Logs
}

func newCWLogsSubscriptionHandler(
	logsDecoder encoding.LogsDecoderFactory,
	consumer consumer.Logs,
) *cwLogsSubscriptionHandler {
	return &cwLogsSubscriptionHandler{
		logsDecoder: logsDecoder,
		consumer:    consumer,
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

	decoder, err := c.logsDecoder.NewLogsDecoder(reader, encoding.WithFlushBytes(s3StreamBatchSize))
	if err != nil {
		return err
	}

	for {
		var logs plog.Logs
		logs, err = decoder.DecodeLogs()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("failed to decode S3 logs: %w", err)
		}
		if err = c.consumer.ConsumeLogs(ctx, logs); err != nil {
			return checkConsumerErrorAndWrap(err)
		}
	}

	return nil
}

// gunzipIfNeeded checks if the provided reader is a gzipped stream and returns a reader with gunzip wrapping if needed.
func gunzipIfNeeded(r io.Reader) (io.Reader, error) {
	buf := bufio.NewReaderSize(r, readerBufferSize)
	header, err := buf.Peek(2)
	if err != nil {
		return nil, err
	}
	// gzip magic number: 0x1f 0x8b
	if header[0] == 0x1f && header[1] == 0x8b {
		gr, err := gzip.NewReader(buf)
		if err != nil {
			return nil, err
		}
		return gr, nil
	}
	return buf, nil
}

func enrichS3Logs(logs plog.Logs, event events.S3EventRecord) {
	for _, resourceLogs := range logs.ResourceLogs().All() {
		resourceAttrs := resourceLogs.Resource().Attributes()

		resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
		resourceAttrs.PutStr(string(conventions.CloudRegionKey), event.AWSRegion)
		resourceAttrs.PutStr("aws.s3.bucket.name", event.S3.Bucket.Name)
		resourceAttrs.PutStr("aws.s3.bucket.arn", event.S3.Bucket.Arn)
		resourceAttrs.PutStr(string(conventions.AWSS3KeyKey), event.S3.Object.Key)

		for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
			for _, logRecord := range scopeLogs.LogRecords().All() {
				logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(event.EventTime))
			}
		}
	}
}

// getEnrichedContext creates a new context with metadata extracted from the S3 event,
// which can be used for further processing and correlation in the pipeline.
func getEnrichedContext(ctx context.Context, event events.S3EventRecord) context.Context {
	metadata := map[string][]string{}
	metadata[string(conventions.CloudRegionKey)] = []string{event.AWSRegion}
	metadata["aws.s3.bucket.name"] = []string{event.S3.Bucket.Name}
	metadata["aws.s3.bucket.arn"] = []string{event.S3.Bucket.Arn}
	metadata[string(conventions.AWSS3KeyKey)] = []string{event.S3.Object.Key}

	info := client.Info{}
	info.Metadata = client.NewMetadata(metadata)
	return client.NewContext(ctx, info)
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
