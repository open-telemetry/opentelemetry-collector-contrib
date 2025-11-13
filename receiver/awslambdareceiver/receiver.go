// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"
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

type extensionFactory interface {
	Create(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error)
}

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
				s3Provider, awslogsencodingextension.NewFactory(),
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
	handler, err := a.newHandler(ctx, host, a.s3Provider)
	if err != nil {
		return fmt.Errorf("failed to create the lambda event handler: %w", err)
	}
	a.handler = handler

	go lambda.StartWithOptions(a.processLambdaEvent, lambda.WithContext(ctx))
	return nil
}

// processLambdaEvent filters trigger events and forward to dedicated processors
func (a *awsLambdaReceiver) processLambdaEvent(ctx context.Context, event json.RawMessage) error {
	triggerType, err := detectTriggerType(event)
	if err != nil {
		// fail fast: Unknown or Invalid event triggers are suppressed so that they do not get replayed by the lambda framework.
		a.logger.Error("Received an event with invalid OR unsupported trigger type", zap.String(logDeployedTrigger, string(a.handler.handlerType())), zap.Error(err))
		return nil
	}

	if triggerType == customReplayEvent {
		// Note - custom event handler is derived on-demand. This is to avoid unnecessary S3 service initialization
		// for non-error replay scenarios, which is the common case.
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
func (a *awsLambdaReceiver) handleCustomTriggers(ctx context.Context, handler internal.CustomEventHandler[*internal.ErrorReplayResponse]) error {
	if handler.IsDryRun() {
		a.logger.Info("Running in dryrun mode, processing is skipped")
	}

	var count int
	for handler.HasNext() {
		content, err := handler.GetNext(ctx)
		if err != nil {
			a.logger.Error("error while executing custom event", zap.Error(err))
			return err
		}

		count++
		if handler.IsDryRun() {
			a.logger.Info(fmt.Sprintf("(dryrun) object key:  %s", content.Key))
			continue
		}

		tType, err := detectTriggerType(content.Content)
		if err != nil {
			// Note - Manual triggers are synchronous.
			// Errors for synchronous invocations are not retried by Lambda not stored at error destination.
			return fmt.Errorf("invalid lambda trigger event from custom trigger: %w", err)
		}

		err = a.handleEvent(ctx, content.Content, tType)
		if err != nil {
			a.logger.Error("error while executing custom event", zap.Error(err))
			return err
		}

		handler.PostProcess(ctx, content)
	}

	a.logger.Info(fmt.Sprintf("Processed %d events", count))
	return nil
}

// handleEvent is specialized for processing events and extracting signals.
// Handling of the event is done using provided eventKey.
// See payload types content,
//   - For S3: https://pkg.go.dev/github.com/aws/aws-lambda-go/events#S3Event
//   - For CloudWatch: https://pkg.go.dev/github.com/aws/aws-lambda-go/events#CloudwatchLogsEvent
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

func (a *awsLambdaReceiver) Shutdown(_ context.Context) error {
	return nil
}

func newLogsHandler(
	ctx context.Context,
	cfg *Config,
	set receiver.Settings,
	host component.Host,
	next consumer.Logs,
	s3Provider internal.S3Provider,
	factory extensionFactory,
) (lambdaEventHandler, error) {
	// If S3Encoding is not set, then we are dealing with CW subscription filters
	if cfg.S3Encoding == "" {
		unmarshaler, err := loadSubFilterLogUnmarshaler(ctx, factory)
		if err != nil {
			return nil, err
		}
		return newCWLogsSubscriptionHandler(set.Logger, unmarshaler.UnmarshalLogs, next.ConsumeLogs), nil
	}
	// Then prioritize encoding loading with S3Encoding ID
	encodingExtension, err := loadEncodingExtension[plog.Unmarshaler](host, cfg.S3Encoding, "logs")
	if err != nil {
		return nil, err
	}
	s3Service, err := s3Provider.GetService(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load the S3 service: %w", err)
	}
	return newS3Handler(s3Service, set.Logger, encodingExtension.UnmarshalLogs, next.ConsumeLogs), nil
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
	return newS3Handler(s3Service, set.Logger, encodingExtension.UnmarshalMetrics, next.ConsumeMetrics), nil
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

// loadSubFilterLogUnmarshaler loads unmarshaler for cloudwatch subscription filter logs
func loadSubFilterLogUnmarshaler(ctx context.Context, factory extensionFactory) (plog.Unmarshaler, error) {
	var extensionID component.ID
	err := extensionID.UnmarshalText([]byte(awsLogsEncoding))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal identifier: %w", err)
	}

	// Create the extension using the factory
	ext, err := factory.Create(ctx, extension.Settings{ID: extensionID}, &awslogsencodingextension.Config{
		Format: "cloudwatch",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create extension: %w", err)
	}

	// Assert that the extension implements plog.Unmarshaler
	subscriptionFilterLogUnmarshaler, ok := ext.(plog.Unmarshaler)
	if !ok {
		return nil, fmt.Errorf("extension %q does not implement plog.Unmarshaler", extensionID.String())
	}

	// Return the unmarshaler
	return subscriptionFilterLogUnmarshaler, nil
}

// detectTriggerType is a helper to derive the eventType based on the payload content.
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
