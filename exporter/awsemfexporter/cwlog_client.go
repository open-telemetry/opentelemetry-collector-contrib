// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/handler"
)

var collectorDistribution = "opentelemetry-collector-contrib"

const (
	// this is the retry count, the total attempts will be at most retry count + 1.
	defaultRetryCount          = 1
	errCodeThrottlingException = "ThrottlingException"
)

// Possible exceptions are combination of common errors (https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/CommonErrors.html)
// and API specific erros (e.g. https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html#API_PutLogEvents_Errors)
type cloudWatchLogClient struct {
	svc    cloudwatchlogsiface.CloudWatchLogsAPI
	logger *zap.Logger
}

//Create a log client based on the actual cloudwatch logs client.
func newCloudWatchLogClient(svc cloudwatchlogsiface.CloudWatchLogsAPI, logger *zap.Logger) *cloudWatchLogClient {
	logClient := &cloudWatchLogClient{svc: svc,
		logger: logger}
	return logClient
}

// newCloudWatchLogsClient create cloudWatchLogClient
func newCloudWatchLogsClient(logger *zap.Logger, awsConfig *aws.Config, buildInfo component.BuildInfo, logGroupName string, sess *session.Session) *cloudWatchLogClient {
	client := cloudwatchlogs.New(sess, awsConfig)
	client.Handlers.Build.PushBackNamed(handler.RequestStructuredLogHandler)
	client.Handlers.Build.PushFrontNamed(newCollectorUserAgentHandler(buildInfo, logGroupName))
	return newCloudWatchLogClient(client, logger)
}

//Put log events. The method mainly handles different possible error could be returned from server side, and retries them
//if necessary.
func (client *cloudWatchLogClient) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput, retryCnt int) (*string, error) {
	var response *cloudwatchlogs.PutLogEventsOutput
	var err error
	var token = input.SequenceToken

	for i := 0; i <= retryCnt; i++ {
		input.SequenceToken = token
		response, err = client.svc.PutLogEvents(input)
		if err != nil {
			awsErr, ok := err.(awserr.Error)
			if !ok {
				client.logger.Error("Cannot cast PutLogEvents error into awserr.Error.", zap.Error(err))
				return token, err
			}
			switch e := awsErr.(type) {
			case *cloudwatchlogs.InvalidParameterException:
				client.logger.Error("cwlog_client: Error occurs in PutLogEvents, will not retry the request", zap.Error(e), zap.String("LogGroupName", *input.LogGroupName), zap.String("LogStreamName", *input.LogStreamName))
				return token, err
			case *cloudwatchlogs.InvalidSequenceTokenException: //Resend log events with new sequence token when InvalidSequenceTokenException happens
				client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, will search the next token and retry the request", zap.Error(e))
				token = e.ExpectedSequenceToken
				continue
			case *cloudwatchlogs.DataAlreadyAcceptedException: //Skip batch if DataAlreadyAcceptedException happens
				client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, drop this request and continue to the next request", zap.Error(e))
				token = e.ExpectedSequenceToken
				return token, err
			case *cloudwatchlogs.OperationAbortedException: //Retry request if OperationAbortedException happens
				client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, will retry the request", zap.Error(e))
				return token, err
			case *cloudwatchlogs.ServiceUnavailableException: //Retry request if ServiceUnavailableException happens
				client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, will retry the request", zap.Error(e))
				return token, err
			case *cloudwatchlogs.ResourceNotFoundException:
				tmpToken, tmpErr := client.CreateStream(input.LogGroupName, input.LogStreamName)
				if tmpErr == nil {
					if tmpToken == "" {
						token = nil
					}
				}
				continue
			default:
				// ThrottlingException is handled here because the type cloudwatch.ThrottlingException is not yet available in public SDK
				// Drop request if ThrottlingException happens
				if awsErr.Code() == errCodeThrottlingException {
					client.logger.Warn("cwlog_client: Error occurs in PutLogEvents, will not retry the request", zap.Error(awsErr), zap.String("LogGroupName", *input.LogGroupName), zap.String("LogStreamName", *input.LogStreamName))
					return token, err
				}
				client.logger.Error("cwlog_client: Error occurs in PutLogEvents", zap.Error(awsErr))
				return token, err
			}

		}

		//TODO: Should have metrics to provide visibility of these failures
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
				token = response.NextSequenceToken
				break
			}
		}
	}
	if err != nil {
		client.logger.Error("All retries failed for PutLogEvents. Drop this request.", zap.Error(err))
	}
	return token, err
}

//Prepare the readiness for the log group and log stream.
func (client *cloudWatchLogClient) CreateStream(logGroup, streamName *string) (token string, e error) {
	//CreateLogStream / CreateLogGroup
	_, err := client.svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  logGroup,
		LogStreamName: streamName,
	})
	if err != nil {
		client.logger.Debug("cwlog_client: creating stream fail", zap.Error(err))
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
			_, err = client.svc.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
				LogGroupName: logGroup,
			})
			if err == nil {
				_, err = client.svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
					LogGroupName:  logGroup,
					LogStreamName: streamName,
				})
			}
		}
	}

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
			return "", nil
		}
		client.logger.Debug("CreateLogStream / CreateLogGroup has errors.", zap.String("LogGroupName", *logGroup), zap.String("LogStreamName", *streamName), zap.Error(e))
		return token, err
	}

	//After a log stream is created the token is always empty.
	return "", nil
}

func newCollectorUserAgentHandler(buildInfo component.BuildInfo, logGroupName string) request.NamedHandler {
	fn := request.MakeAddToUserAgentHandler(collectorDistribution, buildInfo.Version)
	if matchContainerInsightsPattern(logGroupName) {
		fn = request.MakeAddToUserAgentHandler(collectorDistribution, buildInfo.Version, "ContainerInsights")
	}
	return request.NamedHandler{
		Name: "otel.collector.UserAgentHandler",
		Fn:   fn,
	}
}

func matchContainerInsightsPattern(logGroupName string) bool {
	regexP := "^/aws/.*containerinsights/.*/(performance|prometheus)$"
	r, _ := regexp.Compile(regexP)
	return r.MatchString(logGroupName)
}
