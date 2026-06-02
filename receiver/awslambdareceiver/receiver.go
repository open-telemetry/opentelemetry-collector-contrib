// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

type eventType string

const (
	s3Event           eventType = "S3Event"
	cwEvent           eventType = "CloudWatchEvent"
	customReplayEvent eventType = "replayFailedEvents"

	// defaultMetricsEncodingExtension defines the default encoding extension ID for metrics when none is specified
	defaultMetricsEncodingExtension = "awscloudwatchmetricstreams_encoding"

	// logInvokedTrigger define the string key to convey the invoked trigger(derived by event content) type in logs
	logInvokedTrigger = "invokedTrigger"
)

var (
	errEncoderNotFound                  = errors.New("extension not found")
	_                  receiver.Metrics = (*awsLambdaReceiver)(nil)
	_                  receiver.Logs    = (*awsLambdaReceiver)(nil)
)

type awsLambdaReceiver struct {
	cfg       *Config
	logger    *zap.Logger
	buildInfo component.BuildInfo
	// Note: handlerProvider deriving is deferred to Start method.
	// This is because internal extension loading depends on component.Host.
	handlerProvider func(context.Context, component.Host, internal.S3Provider) (handlerProvider, error)

	// Derived handlerProvider once Start is called.
	hp handlerProvider

	// s3Provider to be reused by any component.
	// Derived once Start is called.
	s3Provider internal.S3Provider
}

func newLogsReceiver(cfg *Config, set receiver.Settings, next consumer.Logs) (receiver.Logs, error) {
	return &awsLambdaReceiver{
		cfg:       cfg,
		logger:    set.Logger,
		buildInfo: set.BuildInfo,
		handlerProvider: func(
			ctx context.Context,
			host component.Host,
			s3Provider internal.S3Provider,
		) (handlerProvider, error) {
			return newLogsHandler(ctx, cfg, set, host, next, s3Provider)
		},
	}, nil
}

func newMetricsReceiver(cfg *Config, set receiver.Settings, next consumer.Metrics) (receiver.Metrics, error) {
	return &awsLambdaReceiver{
		cfg:       cfg,
		logger:    set.Logger,
		buildInfo: set.BuildInfo,
		handlerProvider: func(
			ctx context.Context,
			host component.Host,
			s3Provider internal.S3Provider,
		) (handlerProvider, error) {
			return newMetricsHandler(ctx, cfg, set, host, next, s3Provider)
		},
	}, nil
}

// Start registers the main handler function that get executed when lambda is triggered
func (a *awsLambdaReceiver) Start(ctx context.Context, host component.Host) error {
	// Verify we're running in a Lambda environment
	if os.Getenv("AWS_EXECUTION_ENV") == "" || !strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda_") {
		return errors.New("aws_lambda receiver must be used in an AWS Lambda environment: missing environment variable AWS_EXECUTION_ENV")
	}

	// Initialize S3 provider to be used by implementations
	a.s3Provider = &internal.S3ServiceProvider{}

	var err error
	a.hp, err = a.handlerProvider(ctx, host, a.s3Provider)
	if err != nil {
		return err
	}

	go lambda.Start(a.processLambdaEvent)
	return nil
}

// processLambdaEvent filters trigger events and forward to dedicated processors
func (a *awsLambdaReceiver) processLambdaEvent(ctx context.Context, event json.RawMessage) error {
	triggerType, err := detectTriggerType(event)
	if err != nil {
		// Unknown or invalid event triggers are suppressed so that they do not get replayed by the lambda framework.
		a.logger.Error("Received an event with invalid or unsupported trigger type", zap.Error(err))
		return nil
	}

	if triggerType == customReplayEvent {
		a.logger.Info("Running custom event", zap.String(logInvokedTrigger, string(triggerType)))
		service, err := a.s3Provider.GetService(ctx)
		if err != nil {
			return err
		}

		bucket, err := getBucketNameFromARN(a.cfg.FailureBucketARN)
		if err != nil {
			return fmt.Errorf("unable to determine bucket name from ARN: %w", err)
		}

		handler, err := internal.NewErrorReplayTriggerHandler(a.logger.Named("replayHandler"), event, bucket, service)
		if err != nil {
			a.logger.Error("failed to create error replay handler", zap.Error(err))
			return nil
		}

		return a.handleCustomTriggers(ctx, handler)
	}

	return a.handleEvent(ctx, event, triggerType)
}

// handleCustomTriggers handles custom invocations of the Lambda.
// It works over internal.CustomTriggerHandler interface to iterate over events.
func (a *awsLambdaReceiver) handleCustomTriggers(ctx context.Context, customEvent internal.CustomTriggerHandler) error {
	var count int
	for customEvent.HasNext(ctx) {
		content, err := customEvent.GetNext(ctx)
		if err != nil {
			a.logger.Error("error while iterating over custom event", zap.Error(err))
			return err
		}

		if customEvent.IsDryRun() {
			continue
		}

		tType, err := detectTriggerType(content)
		if err != nil {
			// Note - Manual triggers are synchronous.
			// Errors for synchronous invocations are not retried by Lambda & not stored at error destination.
			return fmt.Errorf("invalid lambda trigger event from custom trigger: %w", err)
		}

		err = a.handleEvent(ctx, content, tType)
		if err != nil {
			a.logger.Error("error while processing content of the custom trigger", zap.Error(err))
			return err
		}

		customEvent.PostProcess(ctx)
		count++
	}

	// validate for any error during iteration
	if err := customEvent.Error(); err != nil {
		a.logger.Error("error occurred during custom trigger iteration", zap.Error(err))
		return err
	}

	a.logger.Info(fmt.Sprintf("Processed %d events", count))
	return nil
}

// handleEvent is specialized for processing events and extracting signals.
// Handling of the event is done using provided eventKey.
func (a *awsLambdaReceiver) handleEvent(ctx context.Context, event []byte, et eventType) error {
	a.logger.Info("Lambda triggered", zap.String(logInvokedTrigger, string(et)))

	handler, err := a.hp.getHandler(et)
	if err != nil {
		// fail fast: if there is no handler for invoked trigger, skip processing, log and return an error.
		a.logger.Error("Cannot handle event type", zap.String(logInvokedTrigger, string(et)), zap.Error(err))
		return fmt.Errorf("cannot handle event type %s: %w", et, err)
	}

	err = handler.handle(ctx, event)
	if err != nil {
		a.logger.Error("Failed to process event", zap.Error(err))

		var c *consumererror.Error
		ok := errors.As(err, &c)
		if ok && c.IsRetryable() {
			a.logger.Info("Retryable error, returning error to lambda layer.")
			// return the error to lambda layer so that the event can be retried
			return fmt.Errorf("error handling the event: %w", err)
		}

		a.logger.Warn("Non-retryable error, event will be ignored")
	}
	return nil
}

func (*awsLambdaReceiver) Shutdown(_ context.Context) error {
	return nil
}

func newLogsHandler(
	ctx context.Context,
	cfg *Config,
	set receiver.Settings,
	host component.Host,
	next consumer.Logs,
	s3Provider internal.S3Provider,
) (handlerProvider, error) {
	logger := set.Logger

	s3Service, err := s3Provider.GetService(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load the S3 service: %w", err)
	}

	registry := make(handlerRegistry)

	// S3: multi-encoding or single-encoding. Both paths resolve to newS3LogsHandler,
	// which accepts a per-event getDecoder function.
	var getLogsDecoder func(objectKey string) (encoding.LogsDecoderFactory, string, error)
	if len(cfg.S3.Encodings) > 0 {
		s3Router, buildErr := buildS3LogsRouter(host, cfg.S3, logger)
		if buildErr != nil {
			return nil, fmt.Errorf("failed to build S3 multi-encoding router: %w", buildErr)
		}
		getLogsDecoder = s3Router.GetDecoder
	} else {
		s3LogsDecoder := internal.NewDefaultS3LogsDecoder()
		if cfg.S3.Encoding != "" {
			logger.Info("Using configured S3 encoding for logs", zap.String("encoding", cfg.S3.Encoding))
			s3LogsDecoder, err = resolveLogsDecoder(host, cfg.S3.Encoding)
			if err != nil {
				return nil, err
			}
		}
		encodingName := cfg.S3.Encoding
		getLogsDecoder = func(_ string) (encoding.LogsDecoderFactory, string, error) {
			return s3LogsDecoder, encodingName, nil
		}
	}
	registry[s3Event] = newS3LogsHandler(s3Service, logger, getLogsDecoder, next)

	// CloudWatch: single-encoding path unchanged in this PR.
	cwDecoder := internal.NewDefaultCWLogsDecoder()
	if cfg.CloudWatch.Encoding != "" {
		logger.Info("Using configured CloudWatch encoding for logs", zap.String("encoding", cfg.CloudWatch.Encoding))
		cwDecoder, err = resolveLogsDecoder(host, cfg.CloudWatch.Encoding)
		if err != nil {
			return nil, err
		}
	}
	registry[cwEvent] = newCWLogsSubscriptionHandler(cwDecoder, next)

	return newHandlerProvider(registry), nil
}

// buildS3LogsRouter constructs a logsDecoderRouter from the S3 encodings config.
// Encodings are sorted by path pattern specificity before being passed to the router.
func buildS3LogsRouter(host component.Host, cfg S3Config, logger *zap.Logger) (*logsDecoderRouter, error) {
	sortedEncodings := cfg.sortedEncodings()
	decoders := make(map[string]encoding.LogsDecoderFactory, len(sortedEncodings))

	defaultDecoder := internal.NewDefaultS3LogsDecoder()
	for _, enc := range sortedEncodings {
		if enc.Encoding == "" {
			// No extension configured: use the raw-passthrough decoder.
			decoders[enc.Name] = defaultDecoder
			continue
		}
		decoder, err := resolveLogsDecoder(host, enc.Encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve encoding for S3 entry %q: %w", enc.Name, err)
		}
		logger.Info("Registered decoder for S3 encoding entry",
			zap.String("name", enc.Name),
			zap.String("encoding", enc.Encoding),
			zap.String("pattern", enc.resolvePathPattern()),
		)
		decoders[enc.Name] = decoder
	}

	return newLogsDecoderRouter(sortedEncodings, decoders), nil
}

func newMetricsHandler(
	ctx context.Context,
	cfg *Config,
	set receiver.Settings,
	host component.Host,
	next consumer.Metrics,
	s3Provider internal.S3Provider,
) (handlerProvider, error) {
	logger := set.Logger
	// Multi-format routing via 's3.encodings' is only supported for logs.
	if len(cfg.S3.Encodings) > 0 {
		return nil, errors.New("'s3.encodings' is only supported for the logs signal type; use 's3.encoding' for metrics")
	}
	extensionID := defaultMetricsEncodingExtension
	// Note: for metrics, we currently support S3 trigger only.
	if cfg.S3.Encoding != "" {
		logger.Info("Using configured S3 encoding for metrics", zap.String("encoding", cfg.S3.Encoding))
		extensionID = cfg.S3.Encoding
	}

	ext, err := loadEncodingExtension[extension.Extension](host, extensionID, "metrics")
	if err != nil {
		return nil, err
	}

	var decoder encoding.MetricsDecoderFactory
	decoder, ok := ext.(encoding.MetricsDecoderExtension)
	if !ok {
		// derive a decoder wrapper if extension is of encoding.MetricsUnmarshalerExtension type
		metricsUnmarshaler, t := ext.(encoding.MetricsUnmarshalerExtension)
		if !t {
			return nil, errors.New("provided extension does not implement MetricsDecoder or MetricsUnmarshalerExtension interfaces")
		}

		decoder = xstreamencoding.NewMetricsUnmarshalerDecoderFactory(metricsUnmarshaler)
	}

	s3Service, err := s3Provider.GetService(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load the S3 service: %w", err)
	}

	// Register handlers. Metrics supports S3 events.
	registry := make(handlerRegistry)
	registry[s3Event] = newS3MetricsHandler(s3Service, set.Logger, decoder, next)

	return newHandlerProvider(registry), nil
}

func resolveLogsDecoder(host component.Host, encoderName string) (encoding.LogsDecoderFactory, error) {
	var ext extension.Extension
	ext, err := loadEncodingExtension[extension.Extension](host, encoderName, "logs")
	if err != nil {
		return nil, err
	}

	var decoderFactory encoding.LogsDecoderFactory
	var ok bool
	decoderFactory, ok = ext.(encoding.LogsDecoderExtension)
	if !ok {
		// derive a decoder wrapper if extension is of encoding.LogsUnmarshalerExtension type
		logsUnmarshaler, t := ext.(encoding.LogsUnmarshalerExtension)
		if !t {
			return nil, errors.New("provided extension does not implement LogsDecoder or LogsUnmarshalerExtension interfaces")
		}

		decoderFactory = xstreamencoding.NewLogsUnmarshalerDecoderFactory(logsUnmarshaler)
	}

	return decoderFactory, nil
}

// loadEncodingExtension attempts to load an available extension for the given name.
func loadEncodingExtension[T any](host component.Host, encoding, signalType string) (T, error) {
	var zero T
	var extensionID component.ID
	err := extensionID.UnmarshalText([]byte(encoding))
	if err != nil {
		return zero, fmt.Errorf("failed to unmarshal identifier: %w", err)
	}
	encodingExtension, ok := host.GetExtensions()[extensionID]
	if !ok {
		return zero, errors.Join(fmt.Errorf("unknown extension: %s", extensionID), errEncoderNotFound)
	}
	unmarshaler, ok := encodingExtension.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a %s unmarshaler", encoding, signalType)
	}
	return unmarshaler, nil
}

// detectTriggerType is a helper to derive the eventType based on the payload content.
// Supported trigger types are:
// - S3Event
// - CloudWatchEvent
// See payload content at official documentation,
//   - For S3: https://pkg.go.dev/github.com/aws/aws-lambda-go/events#S3Event
//   - For CloudWatch: https://pkg.go.dev/github.com/aws/aws-lambda-go/events#CloudwatchLogsEvent
//
// Suppoerted custom trigger type:
// - replayFailedEvents
func detectTriggerType(data []byte) (eventType, error) {
	switch {
	case bytes.HasPrefix(data, []byte(`{"Records"`)):
		return s3Event, nil
	case bytes.HasPrefix(data, []byte(`{"awslogs"`)):
		return cwEvent, nil
	}

	// fallback for possible manual trigger cases
	key, err := extractFirstKey(data)
	if err != nil {
		return "", err
	}

	if key == string(customReplayEvent) {
		return customReplayEvent, nil
	}

	return "", fmt.Errorf("unknown event type with key: %s", key)
}

// extractFirstKey extracts the first JSON key from byte array without parsing it.
// This improves performance as there's no need to parse the entire JSON structure to extract the first key.
func extractFirstKey(data []byte) (string, error) {
	pos := 0
	n := len(data)

	// skip any spaces
	for pos < n && data[pos] <= ' ' {
		pos++
	}
	if pos >= n || data[pos] != '{' {
		return "", errors.New("invalid JSON payload, failed to find the opening bracket")
	}

	// advance to opening quote
	pos++
	for pos < n && data[pos] <= ' ' {
		pos++
	}
	if pos >= n || data[pos] != '"' {
		return "", errors.New("invalid JSON payload, expected a key but found none")
	}

	// extract the first key
	pos++
	keyStart := pos
	for pos < n {
		if data[pos] == '"' {
			return string(data[keyStart:pos]), nil
		}
		pos++
	}

	return "", errors.New("invalid JSON payload")
}
