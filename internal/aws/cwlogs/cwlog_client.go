// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs/handler"
)

const (
	// this is the retry count, the total attempts will be at most retry count + 1.
	defaultRetryCount          = 1
	errCodeThrottlingException = "ThrottlingException"
)

var containerInsightsRegexPattern = regexp.MustCompile(`^/aws/.*containerinsights/.*/(performance|prometheus)$`)

type cloudWatchClient interface {
	CreateLogGroup(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	CreateLogStream(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	PutLogEvents(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
	PutRetentionPolicy(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	TagResource(ctx context.Context, params *cloudwatchlogs.TagResourceInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.TagResourceOutput, error)
}

// Possible exceptions are combination of common errors (https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/CommonErrors.html)
// and API specific erros (e.g. https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html#API_PutLogEvents_Errors)
type Client struct {
	svc          cloudWatchClient
	logRetention int32
	tags         map[string]string
	logger       *zap.Logger
}

type ClientOption func(*cwLogClientConfig)

type cwLogClientConfig struct {
	userAgentExtras []string
}

func WithUserAgentExtras(userAgentExtras ...string) ClientOption {
	return func(config *cwLogClientConfig) {
		config.userAgentExtras = append(config.userAgentExtras, userAgentExtras...)
	}
}

// Create a log client based on the actual cloudwatch logs client.
func newCloudWatchLogClient(svc cloudWatchClient, logRetention int32, tags map[string]string, logger *zap.Logger) *Client {
	logClient := &Client{
		svc:          svc,
		logRetention: logRetention,
		tags:         tags,
		logger:       logger,
	}
	return logClient
}

func newCollectorUserAgent(buildInfo component.BuildInfo, logGroupName string, componentName string, opts ...ClientOption) string {
	// Loop through each option
	option := &cwLogClientConfig{
		userAgentExtras: []string{},
	}
	for _, opt := range opts {
		opt(option)
	}

	extraStrs := []string{componentName}
	extraStrs = append(extraStrs, option.userAgentExtras...)

	if containerInsightsRegexPattern.MatchString(logGroupName) {
		extraStrs = append(extraStrs, "ContainerInsights")
	}

	userAgentHeader := fmt.Sprintf("%s/%s", buildInfo.Command, buildInfo.Version)
	if len(extraStrs) > 0 {
		userAgentHeader = fmt.Sprintf("%s (%s)", userAgentHeader, strings.Join(extraStrs, "; "))
	}

	return userAgentHeader
}

// NewClient create Client
func NewClient(logger *zap.Logger, awsConfig aws.Config, buildInfo component.BuildInfo, logGroupName string, logRetention int32, tags map[string]string, componentName string, opts ...ClientOption) *Client {
	client := cloudwatchlogs.NewFromConfig(awsConfig,
		handler.WithStructuredLogHeader(middleware.After),
		AddToUserAgentHeader("otel.collector.UserAgentHandler", newCollectorUserAgent(buildInfo, logGroupName, componentName, opts...), middleware.Before),
	)

	return newCloudWatchLogClient(client, logRetention, tags, logger)
}

// PutLogEvents mainly handles different possible error could be returned from server side, and retries them
// if necessary.
func (client *Client) PutLogEvents(ctx context.Context, input *cloudwatchlogs.PutLogEventsInput, retryCnt int) error {
	var response *cloudwatchlogs.PutLogEventsOutput
	var err error
	// CloudWatch Logs API was changed to ignore the sequenceToken
	// PutLogEvents actions are now accepted and never return
	// InvalidSequenceTokenException or DataAlreadyAcceptedException even
	// if the sequence token is not valid.
	// Finally, InvalidSequenceTokenException and DataAlreadyAcceptedException are
	// never returned by the PutLogEvents action.
	for i := 0; i <= retryCnt; i++ {
		response, err = client.svc.PutLogEvents(ctx, input)
		if err != nil {
			var ae smithy.APIError
			if !errors.As(err, &ae) {
				// Should never happen
				client.logger.Error("unexpectedly cannot cast PutLogEvents error into smithy.APIError.", zap.Error(err))
				return err
			}

			var (
				ipe *types.InvalidParameterException
				oae *types.OperationAbortedException
				sue *types.ServiceUnavailableException
				rnf *types.ResourceNotFoundException
				te  *types.ThrottlingException
			)

			switch {
			case errors.As(err, &ipe):
				client.logger.Error("cwlog_client: Error occurs in PutLogEvents, will not retry the request", zap.Error(err), zap.String("LogGroupName", *input.LogGroupName), zap.String("LogStreamName", *input.LogStreamName))
				return err
			case errors.As(err, &oae):
				// Retry request if OperationAbortedException happens
				client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, will retry the request", zap.Error(err))
				return err
			case errors.As(err, &sue):
				// Retry request if ServiceUnavailableException happens
				client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, will retry the request", zap.Error(err))
				return err
			case errors.As(err, &rnf):
				if tmpErr := client.CreateStream(ctx, input.LogGroupName, input.LogStreamName); tmpErr != nil {
					return tmpErr
				}
			case errors.As(err, &te):
				// ThrottlingException is handled here because the type cloudwatch.ThrottlingException is not yet available in public SDK
				// Drop request if ThrottlingException happens
				client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, will not retry the request", zap.Error(err), zap.String("LogGroupName", *input.LogGroupName), zap.String("LogStreamName", *input.LogStreamName))
				return err
			default:
				client.logger.Error("cwlog_client: Error occurs in PutLogEvents", zap.Error(err))
				return err
			}
		}

		// TODO: Should have metrics to provide visibility of these failures
		if response != nil {
			if response.RejectedLogEventsInfo != nil {
				rejectedLogEventsInfo := response.RejectedLogEventsInfo
				if rejectedLogEventsInfo.TooOldLogEventEndIndex != nil {
					client.logger.Warn(fmt.Sprintf("%d log events for log group name are too old", *rejectedLogEventsInfo.TooOldLogEventEndIndex), zap.String("LogGroupName", *input.LogGroupName))
				}
				if rejectedLogEventsInfo.TooNewLogEventStartIndex != nil {
					client.logger.Warn(fmt.Sprintf("%d log events for log group name are too new", *rejectedLogEventsInfo.TooNewLogEventStartIndex), zap.String("LogGroupName", *input.LogGroupName))
				}
				if rejectedLogEventsInfo.ExpiredLogEventEndIndex != nil {
					client.logger.Warn(fmt.Sprintf("%d log events for log group name are expired", *rejectedLogEventsInfo.ExpiredLogEventEndIndex), zap.String("LogGroupName", *input.LogGroupName))
				}
			}

			if response.NextSequenceToken != nil {
				break
			}
		}
	}
	if err != nil {
		client.logger.Error("All retries failed for PutLogEvents. Drop this request.", zap.Error(err))
	}
	return err
}

// Prepare the readiness for the log group and log stream.
func (client *Client) CreateStream(ctx context.Context, logGroup, streamName *string) error {
	// CreateLogStream / CreateLogGroup
	_, err := client.svc.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  logGroup,
		LogStreamName: streamName,
	})
	if err != nil {
		client.logger.Debug("cwlog_client: creating stream fail", zap.Error(err))
		var rnf *types.ResourceNotFoundException
		if errors.As(err, &rnf) {
			// Create Log Group with tags if they exist and were specified in the config
			_, err = client.svc.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
				LogGroupName: logGroup,
				Tags:         client.tags,
			})
			if err == nil {
				// For newly created log groups, set the log retention policy if specified or non-zero. Otherwise, set to Never Expire
				if client.logRetention != 0 {
					_, err = client.svc.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
						LogGroupName:    logGroup,
						RetentionInDays: &client.logRetention,
					})
					if err != nil {
						var ae smithy.APIError
						if errors.As(err, &ae) {
							client.logger.Debug("CreateLogStream / CreateLogGroup has errors related to log retention policy.", zap.String("LogGroupName", *logGroup), zap.String("LogStreamName", *streamName), zap.Error(err))
							return err
						}
					}
				}
				_, err = client.svc.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
					LogGroupName:  logGroup,
					LogStreamName: streamName,
				})
			}
		}
	}

	if err != nil {
		var rae *types.ResourceAlreadyExistsException
		if errors.As(err, &rae) {
			return nil
		}
		client.logger.Debug("CreateLogStream / CreateLogGroup has errors.", zap.String("LogGroupName", *logGroup), zap.String("LogStreamName", *streamName), zap.Error(err))
		return err
	}

	// After a log stream is created the token is always empty.
	return nil
}

type addToUserAgentHeader struct {
	id, val string
}

var _ middleware.SerializeMiddleware = (*addToUserAgentHeader)(nil)

func (a *addToUserAgentHeader) ID() string {
	return a.id
}

func (a *addToUserAgentHeader) HandleSerialize(ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler) (out middleware.SerializeOutput, metadata middleware.Metadata, err error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unreconized transport type: %T", in.Request)
	}

	val := a.val
	curUA := req.Header.Get("User-Agent")
	if len(curUA) > 0 {
		val = curUA + " " + val
	}
	req.Header.Set("User-Agent", val)
	return next.HandleSerialize(ctx, in)
}

func AddToUserAgentHeader(id, val string, pos middleware.RelativePosition) func(options *cloudwatchlogs.Options) {
	return func(o *cloudwatchlogs.Options) {
		o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
			return s.Serialize.Add(&addToUserAgentHeader{id: id, val: val}, pos)
		})
	}
}
