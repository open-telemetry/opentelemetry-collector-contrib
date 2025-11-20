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
	s3Event           eventType = "S3Event"
	cwEvent           eventType = "CloudWatchEvent"
	customReplayEvent eventType = "replayFailedEvents"

	// logDeployedTrigger define the string key to convey the deployed trigger type in logs
	logDeployedTrigger = "deployedTrigger"
	// logInvokedTrigger define the string key to convey the invoked trigger(derived by event content) type in logs
	logInvokedTrigger = "invokedTrigger"
)

var (
	errEncoderNotFound                  = errors.New("extension not found")
	_                  receiver.Metrics = (*awsLambdaReceiver)(nil)
	_                  receiver.Logs    = (*awsLambdaReceiver)(nil)
)

type awsLambdaReceiver struct {
	cfg        *Config
	logger     *zap.Logger
	buildInfo  component.BuildInfo
	s3Provider internal.S3Provider
	newHandler func(context.Context, component.Host, internal.S3Provider) (lambdaEventHandler, error)

	handler lambdaEventHandler
}

func newLogsReceiver(cfg *Config, set receiver.Settings, next consumer.Logs) (receiver.Logs, error) {
	return &awsLambdaReceiver{
		cfg:        cfg,
		logger:     set.Logger,
		buildInfo:  set.BuildInfo,
		s3Provider: &internal.S3ServiceProvider{},
		newHandler: func(
			ctx context.Context,
			host component.Host,
			s3Provider internal.S3Provider,
		) (lambdaEventHandler, error) {
			return newLogsHandler(
				ctx, cfg, set, host, next,
				s3Provider,
			)
		},
	}, nil
}

func newMetricsReceiver(cfg *Config, set receiver.Settings, next consumer.Metrics) (receiver.Metrics, error) {
	if cfg.S3Encoding == "" {
		return nil, errors.New("s3_encoding is required for metrics")
	}
	return &awsLambdaReceiver{
		cfg:        cfg,
		logger:     set.Logger,
		buildInfo:  set.BuildInfo,
		s3Provider: &internal.S3ServiceProvider{},
		newHandler: func(
			ctx context.Context,
			host component.Host,
			s3Provider internal.S3Provider,
		) (lambdaEventHandler, error) {
			return newMetricsHandler(ctx, cfg, set, host, next, s3Provider)
		},
	}, nil
}

// Start registers the main handler function for
// when lambda is triggered
func (a *awsLambdaReceiver) Start(ctx context.Context, host component.Host) error {
	// Verify we're running in a Lambda environment
	if os.Getenv("_LAMBDA_SERVER_PORT") == "" || os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" {
		return errors.New("receiver must be used in an AWS Lambda environment: required environment variables _LAMBDA_SERVER_PORT and AWS_LAMBDA_RUNTIME_API are not set")
	}

	handler, err := a.newHandler(ctx, host, a.s3Provider)
	if err != nil {
		return fmt.Errorf("failed to create the lambda event handler: %w", err)
	}
	a.handler = handler

	go lambda.Start(a.processLambdaEvent)
	return nil
}

// processLambdaEvent filters trigger events and forward to dedicated processors
func (a *awsLambdaReceiver) processLambdaEvent(ctx context.Context, event json.RawMessage) error {
	triggerType, err := detectTriggerType(event)
	if err != nil {
		// Unknown or invalid event triggers are suppressed so that they do not get replayed by the lambda framework.
		a.logger.Error("Received an event with invalid OR unsupported trigger type", zap.String(logDeployedTrigger, string(a.handler.handlerType())), zap.Error(err))
		return nil
	}

	return a.handleEvent(ctx, event, triggerType)
}

// handleEvent is specialized for processing events and extracting signals.
// Handling of the event is done using provided eventKey.
// See payload types content,
//   - For S3: https://pkg.go.dev/github.com/aws/aws-lambda-go/events#S3Event
//   - For CloudWatch: https://pkg.go.dev/github.com/aws/aws-lambda-go/events#CloudwatchLogsEvent (TODO)
func (a *awsLambdaReceiver) handleEvent(ctx context.Context, event json.RawMessage, et eventType) error {
	a.logger.Info("Lambda triggered", zap.String(logInvokedTrigger, string(et)))

	if et != a.handler.handlerType() {
		// fail fast: if handler does not support the event type, log and return an error without parsing the event
		a.logger.Error("Cannot handle event type", zap.String(logInvokedTrigger, string(et)), zap.String(logDeployedTrigger, string(a.handler.handlerType())))
		return fmt.Errorf("cannot handle event type %s, deployment configured to handler %s", et, a.handler.handlerType())
	}

	err := a.handler.handle(ctx, event)
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
) (lambdaEventHandler, error) {
	// If S3Encoding is not set, return an error
	if cfg.S3Encoding == "" {
		return nil, errors.New("s3_encoding is required for logs stored in S3")
	}

	encodingExtension, err := loadEncodingExtension[plog.Unmarshaler](host, cfg.S3Encoding, "logs")
	if err != nil {
		return nil, err
	}
	s3Service, err := s3Provider.GetService(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load the S3 service: %w", err)
	}

	// Wrapper function that sets observed timestamp for logs
	logsConsumer := func(ctx context.Context, event events.S3EventRecord, logs plog.Logs) error {
		setObservedTimestampForAllLogs(logs, event.EventTime)
		return next.ConsumeLogs(ctx, logs)
	}

	return newS3Handler(s3Service, set.Logger, encodingExtension.UnmarshalLogs, logsConsumer), nil
}

func newMetricsHandler(
	ctx context.Context,
	cfg *Config,
	set receiver.Settings,
	host component.Host,
	next consumer.Metrics,
	s3Provider internal.S3Provider,
) (lambdaEventHandler, error) {
	encodingExtension, err := loadEncodingExtension[pmetric.Unmarshaler](host, cfg.S3Encoding, "metrics")
	if err != nil {
		return nil, err
	}
	s3Service, err := s3Provider.GetService(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load the S3 service: %w", err)
	}

	// Create a wrapper function for metrics (no special processing needed)
	metricsConsumer := func(ctx context.Context, _ events.S3EventRecord, metrics pmetric.Metrics) error {
		return next.ConsumeMetrics(ctx, metrics)
	}

	return newS3Handler(s3Service, set.Logger, encodingExtension.UnmarshalMetrics, metricsConsumer), nil
}

// loadEncodingExtension tries to load an available extension for the given encoding.
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
func detectTriggerType(data []byte) (eventType, error) {
	if bytes.HasPrefix(data, []byte(`{"Records"`)) {
		return s3Event, nil
	}

	// fallback for possible manual trigger cases
	key, err := extractFirstKey(data)
	if err != nil {
		return "", err
	}

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
