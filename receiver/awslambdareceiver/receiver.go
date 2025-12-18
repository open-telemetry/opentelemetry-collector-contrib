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

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

type eventType string

const (
	s3Event eventType = "S3Event"
	cwEvent eventType = "CloudWatchEvent"

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
	handlerProvider func(context.Context, component.Host) (handlerProvider, error)

	// Derived handlerProvider once Start is called.
	hp handlerProvider
}

func newLogsReceiver(cfg *Config, set receiver.Settings, next consumer.Logs) (receiver.Logs, error) {
	return &awsLambdaReceiver{
		cfg:       cfg,
		logger:    set.Logger,
		buildInfo: set.BuildInfo,
		handlerProvider: func(
			ctx context.Context,
			host component.Host,
		) (handlerProvider, error) {
			return newLogsHandler(
				ctx, cfg, set, host, next, &internal.S3ServiceProvider{},
			)
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
		) (handlerProvider, error) {
			return newMetricsHandler(ctx, cfg, set, host, next, &internal.S3ServiceProvider{})
		},
	}, nil
}

// Start registers the main handler function that get executed when lambda is triggered
func (a *awsLambdaReceiver) Start(ctx context.Context, host component.Host) error {
	// Verify we're running in a Lambda environment
	if os.Getenv("AWS_EXECUTION_ENV") == "" || !strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda_") {
		return errors.New("awslambdareceiver must be used in an AWS Lambda environment: missing environment variable AWS_EXECUTION_ENV")
	}

	var err error
	a.hp, err = a.handlerProvider(ctx, host)
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

	return a.handleEvent(ctx, event, triggerType)
}

// handleEvent is specialized for processing events and extracting signals.
// Handling of the event is done using provided eventKey.
func (a *awsLambdaReceiver) handleEvent(ctx context.Context, event json.RawMessage, et eventType) error {
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
	var s3Unmarshaler unmarshalFunc[plog.Logs] = bytesToPlogs
	if cfg.S3.Encoding != "" {
		logger.Info("Using configured S3 encoding for logs", zap.String("encoding", cfg.S3.Encoding))
		extension, err := loadEncodingExtension[plog.Unmarshaler](host, cfg.S3.Encoding, "logs")
		if err != nil {
			return nil, err
		}

		s3Unmarshaler = extension.UnmarshalLogs
	}

	var cwUnmarshaler unmarshalFunc[plog.Logs] = cwLogsToPlogs
	if cfg.CloudWatch.Encoding != "" {
		logger.Info("Using configured CloudWatch encoding for logs", zap.String("encoding", cfg.CloudWatch.Encoding))
		extension, err := loadEncodingExtension[plog.Unmarshaler](host, cfg.CloudWatch.Encoding, "logs")
		if err != nil {
			return nil, err
		}

		cwUnmarshaler = extension.UnmarshalLogs
	}

	s3Service, err := s3Provider.GetService(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load the S3 service: %w", err)
	}

	// Register handlers. Logs supports S3 and CloudWatch Logs subscription events.
	registry := make(handlerRegistry)
	registry[s3Event] = func() lambdaEventHandler {
		// Wrapper function that sets observed timestamp for S3 logs
		logsConsumer := func(ctx context.Context, event events.S3EventRecord, logs plog.Logs) error {
			enrichS3Logs(logs, event)
			return next.ConsumeLogs(ctx, logs)
		}

		return newS3Handler(s3Service, logger, s3Unmarshaler, logsConsumer)
	}

	registry[cwEvent] = func() lambdaEventHandler {
		return newCWLogsSubscriptionHandler(cwUnmarshaler, next.ConsumeLogs)
	}

	return newHandlerProvider(registry), nil
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
	extensionID := defaultMetricsEncodingExtension
	// Note: for metrics, we currently support S3 trigger only.
	if cfg.S3.Encoding != "" {
		logger.Info("Using configured S3 encoding for metrics", zap.String("encoding", cfg.S3.Encoding))
		extensionID = cfg.S3.Encoding
	}

	encodingExtension, err := loadEncodingExtension[pmetric.Unmarshaler](host, extensionID, "metrics")
	if err != nil {
		return nil, err
	}

	s3Service, err := s3Provider.GetService(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load the S3 service: %w", err)
	}

	// Register handlers. Metrics supports S3 events.
	registry := make(handlerRegistry)
	registry[s3Event] = func() lambdaEventHandler {
		metricConsumer := func(ctx context.Context, _ events.S3EventRecord, metrics pmetric.Metrics) error {
			return next.ConsumeMetrics(ctx, metrics)
		}

		return newS3Handler(s3Service, set.Logger, encodingExtension.UnmarshalMetrics, metricConsumer)
	}

	return newHandlerProvider(registry), nil
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

	// todo : use trigger key for custom event handling
	return "", fmt.Errorf("unknown event type with key: %s", key)
}

// extractFirstKey extracts the first JSON key from byte array without parsing it.
func extractFirstKey(data []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	// Read opening brace
	t, err := dec.Token()
	if err != nil {
		return "", errors.New("invalid JSON payload, failed to find the opening bracket")
	}
	if t != json.Delim('{') {
		return "", errors.New("invalid JSON payload, failed to find the opening bracket")
	}

	// Read first key
	t, err = dec.Token()
	if err != nil {
		return "", errors.New("invalid JSON payload")
	}

	key, ok := t.(string)
	if !ok {
		return "", errors.New("invalid JSON payload, expected a key but found none")
	}

	return key, nil
}
