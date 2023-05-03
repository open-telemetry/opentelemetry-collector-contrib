// Copyright The OpenTelemetry Authors
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

package cwlogs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

func newAlwaysPassMockLogClient(putLogEventsFunc func(args mock.Arguments)) *Client {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)

	svc.On("PutLogEvents", mock.Anything).Return(
		&cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: &expectedNextSequenceToken},
		nil).Run(putLogEventsFunc)

	svc.On("CreateLogGroup", mock.Anything).Return(new(cloudwatchlogs.CreateLogGroupOutput), nil)

	svc.On("CreateLogStream", mock.Anything).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil)

	svc.On("DescribeLogStreams", mock.Anything).Return(
		&cloudwatchlogs.DescribeLogStreamsOutput{
			LogStreams: []*cloudwatchlogs.LogStream{{UploadSequenceToken: &expectedNextSequenceToken}}},
		nil)
	return newCloudWatchLogClient(svc, 0, logger)
}

type mockCloudWatchLogsClient struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
	mock.Mock
}

func (svc *mockCloudWatchLogsClient) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatchlogs.PutLogEventsOutput), args.Error(1)
}

func (svc *mockCloudWatchLogsClient) CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogGroupOutput), args.Error(1)
}

func (svc *mockCloudWatchLogsClient) CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogStreamOutput), args.Error(1)
}

func (svc *mockCloudWatchLogsClient) DescribeLogStreams(input *cloudwatchlogs.DescribeLogStreamsInput) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatchlogs.DescribeLogStreamsOutput), args.Error(1)
}

func (svc *mockCloudWatchLogsClient) PutRetentionPolicy(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatchlogs.PutRetentionPolicyOutput), args.Error(1)
}

// Tests
var previousSequenceToken = "0000"
var expectedNextSequenceToken = "1111"
var logGroup = "logGroup"
var logStreamName = "logStream"
var emptySequenceToken = ""

func TestPutLogEvents_HappyCase(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil)

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, expectedNextSequenceToken, *tokenP)
}

func TestPutLogEvents_HappyCase_SomeRejectedInfo(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	rejectedLogEventsInfo := &cloudwatchlogs.RejectedLogEventsInfo{
		ExpiredLogEventEndIndex:  aws.Int64(1),
		TooNewLogEventStartIndex: aws.Int64(2),
		TooOldLogEventEndIndex:   aws.Int64(3)}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken:     &expectedNextSequenceToken,
		RejectedLogEventsInfo: rejectedLogEventsInfo,
	}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil)

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, expectedNextSequenceToken, *tokenP)
}

func TestPutLogEvents_NonAWSError(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, errors.New("some random error")).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, previousSequenceToken, *tokenP)
}

func TestPutLogEvents_InvalidParameterException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}

	invalidParameterException := &cloudwatchlogs.InvalidParameterException{}
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, invalidParameterException).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, previousSequenceToken, *tokenP)
}

func TestPutLogEvents_InvalidSequenceTokenException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}
	awsErr := &cloudwatchlogs.InvalidSequenceTokenException{ExpectedSequenceToken: &expectedNextSequenceToken}

	// the test framework does not support return different result sequentially for the same method call.
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, expectedNextSequenceToken, *tokenP)
}

func TestPutLogEvents_DataAlreadyAcceptedException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}
	awsErr := &cloudwatchlogs.DataAlreadyAcceptedException{ExpectedSequenceToken: &expectedNextSequenceToken}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, expectedNextSequenceToken, *tokenP)
}

func TestPutLogEvents_OperationAbortedException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}

	operationAbortedException := &cloudwatchlogs.OperationAbortedException{}
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, operationAbortedException).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, previousSequenceToken, *tokenP)
}

func TestPutLogEvents_ServiceUnavailableException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}

	serviceUnavailableException := &cloudwatchlogs.ServiceUnavailableException{}
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, serviceUnavailableException).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, previousSequenceToken, *tokenP)
}

func TestPutLogEvents_UnknownException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}

	unknownException := awserr.New("unknownException", "", nil)
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, unknownException).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, previousSequenceToken, *tokenP)
}

func TestPutLogEvents_ThrottlingException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}

	throttlingException := awserr.New(errCodeThrottlingException, "", nil)
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, throttlingException).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, previousSequenceToken, *tokenP)
}

func TestPutLogEvents_ResourceNotFoundException(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &emptySequenceToken,
	}

	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Once()

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, expectedNextSequenceToken, *tokenP)
}

func TestLogRetention_NeverExpire(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &emptySequenceToken,
	}

	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), awsErr).Once()

	svc.On("CreateLogGroup",
		&cloudwatchlogs.CreateLogGroupInput{LogGroupName: &logGroup}).Return(new(cloudwatchlogs.CreateLogGroupOutput), nil).Once()

	// PutRetentionPolicy is not called because it is set to 0

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Once()

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, expectedNextSequenceToken, *tokenP)
}

func TestLogRetention_RetentionDaysInputted(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &emptySequenceToken,
	}

	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), awsErr).Once()

	svc.On("CreateLogGroup",
		&cloudwatchlogs.CreateLogGroupInput{LogGroupName: &logGroup}).Return(new(cloudwatchlogs.CreateLogGroupOutput), nil).Once()

	svc.On("PutRetentionPolicy",
		&cloudwatchlogs.PutRetentionPolicyInput{LogGroupName: &logGroup, RetentionInDays: aws.Int64(365)}).Return(new(cloudwatchlogs.PutRetentionPolicyOutput), nil).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Once()

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil).Once()

	client := newCloudWatchLogClient(svc, 365, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Equal(t, expectedNextSequenceToken, *tokenP)
}

func TestPutLogEvents_AllRetriesFail(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &emptySequenceToken,
	}

	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: nil}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Twice()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Twice()

	client := newCloudWatchLogClient(svc, 0, logger)
	tokenP, _ := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Nil(t, tokenP)
}

func TestCreateStream_HappyCase(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil)

	client := newCloudWatchLogClient(svc, 0, logger)
	token, err := client.CreateStream(&logGroup, &logStreamName)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
	assert.Equal(t, emptySequenceToken, token)
}

func TestCreateStream_CreateLogStream_ResourceAlreadyExists(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)

	resourceAlreadyExistsException := &cloudwatchlogs.ResourceAlreadyExistsException{}
	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(
		new(cloudwatchlogs.CreateLogStreamOutput), resourceAlreadyExistsException)

	client := newCloudWatchLogClient(svc, 0, logger)
	token, err := client.CreateStream(&logGroup, &logStreamName)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
	assert.Equal(t, emptySequenceToken, token)
}

func TestCreateStream_CreateLogStream_ResourceNotFound(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)

	resourceNotFoundException := &cloudwatchlogs.ResourceNotFoundException{}
	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(
		new(cloudwatchlogs.CreateLogStreamOutput), resourceNotFoundException).Once()

	svc.On("CreateLogGroup",
		&cloudwatchlogs.CreateLogGroupInput{LogGroupName: &logGroup}).Return(
		new(cloudwatchlogs.CreateLogGroupOutput), nil)

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(
		new(cloudwatchlogs.CreateLogStreamOutput), nil).Once()

	client := newCloudWatchLogClient(svc, 0, logger)
	token, err := client.CreateStream(&logGroup, &logStreamName)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
	assert.Equal(t, emptySequenceToken, token)
}

type UnknownError struct {
	otherField string
}

func (err *UnknownError) Error() string {
	return "Error"
}

func (err *UnknownError) Code() string {
	return "Code"
}

func (err *UnknownError) Message() string {
	return "Message"
}

func (err *UnknownError) OrigErr() error {
	return fmt.Errorf("OrigErr")
}

func TestLogUnknownError(t *testing.T) {
	err := &UnknownError{
		otherField: "otherFieldValue",
	}
	actualLog := fmt.Sprintf("E! cloudwatchlogs: code: %s, message: %s, original error: %+v, %#v", err.Code(), err.Message(), err.OrigErr(), err)
	expectedLog := "E! cloudwatchlogs: code: Code, message: Message, original error: OrigErr, &cwlogs.UnknownError{otherField:\"otherFieldValue\"}"
	assert.Equal(t, expectedLog, actualLog)
}

func TestUserAgent(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name                 string
		buildInfo            component.BuildInfo
		logGroupName         string
		expectedUserAgentStr string
	}{
		{
			"emptyLogGroup",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"",
			"opentelemetry-collector-contrib/1.0",
		},
		{
			"buildInfoCommandUsed",
			component.BuildInfo{Command: "test-collector-contrib", Version: "1.0"},
			"",
			"test-collector-contrib/1.0",
		},
		{
			"non container insights",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.1"},
			"test-group",
			"opentelemetry-collector-contrib/1.1",
		},
		{
			"container insights EKS",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/containerinsights/eks-cluster-name/performance",
			"opentelemetry-collector-contrib/1.0 (ContainerInsights)",
		},
		{
			"container insights ECS",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/ecs/containerinsights/ecs-cluster-name/performance",
			"opentelemetry-collector-contrib/1.0 (ContainerInsights)",
		},
		{
			"container insights prometheus",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/containerinsights/cluster-name/prometheus",
			"opentelemetry-collector-contrib/1.0 (ContainerInsights)",
		},
	}

	session, _ := session.NewSession()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cwlog := NewClient(logger, &aws.Config{}, tc.buildInfo, tc.logGroupName, 0, session)
			logClient := cwlog.svc.(*cloudwatchlogs.CloudWatchLogs)

			req := request.New(aws.Config{}, metadata.ClientInfo{}, logClient.Handlers, nil, &request.Operation{
				HTTPMethod: "GET",
				HTTPPath:   "/",
			}, nil, nil)

			logClient.Handlers.Build.Run(req)
			assert.Contains(t, req.HTTPRequest.UserAgent(), tc.expectedUserAgentStr)
		})
	}
}
