// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs

import (
	"errors"
	"fmt"
	"net/http"
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
			NextSequenceToken: &expectedNextSequenceToken,
		},
		nil).Run(putLogEventsFunc)

	svc.On("CreateLogGroup", mock.Anything).Return(new(cloudwatchlogs.CreateLogGroupOutput), nil)

	svc.On("CreateLogStream", mock.Anything).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil)

	svc.On("DescribeLogStreams", mock.Anything).Return(
		&cloudwatchlogs.DescribeLogStreamsOutput{
			LogStreams: []*cloudwatchlogs.LogStream{{UploadSequenceToken: &expectedNextSequenceToken}},
		},
		nil)
	return newCloudWatchLogClient(svc, 0, nil, logger)
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

func (svc *mockCloudWatchLogsClient) TagResource(input *cloudwatchlogs.TagResourceInput) (*cloudwatchlogs.TagResourceOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatchlogs.TagResourceOutput), args.Error(1)
}

// Tests
var (
	previousSequenceToken     = "0000"
	expectedNextSequenceToken = "1111"
	logGroup                  = "logGroup"
	logStreamName             = "logStream"
	emptySequenceToken        = ""
)

func TestPutLogEvents_HappyCase(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &previousSequenceToken,
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken,
	}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil)

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
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
		TooOldLogEventEndIndex:   aws.Int64(3),
	}
	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken:     &expectedNextSequenceToken,
		RejectedLogEventsInfo: rejectedLogEventsInfo,
	}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil)

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, errors.New("some random error")).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Error(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}

	invalidParameterException := &cloudwatchlogs.InvalidParameterException{}
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, invalidParameterException).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Error(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}

	operationAbortedException := &cloudwatchlogs.OperationAbortedException{}
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, operationAbortedException).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Error(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}

	serviceUnavailableException := &cloudwatchlogs.ServiceUnavailableException{}
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, serviceUnavailableException).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Error(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}

	unknownException := awserr.New("unknownException", "", nil)
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, unknownException).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Error(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}

	throttlingException := awserr.New(errCodeThrottlingException, "", nil)
	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, throttlingException).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Error(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Once()

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}
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

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
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
		NextSequenceToken: &expectedNextSequenceToken,
	}
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

	client := newCloudWatchLogClient(svc, 365, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestSetTags_NotCalled(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &emptySequenceToken,
	}

	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken,
	}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), awsErr).Once()

	// Tags not added because it is not set

	svc.On("CreateLogGroup",
		&cloudwatchlogs.CreateLogGroupInput{LogGroupName: &logGroup}).Return(new(cloudwatchlogs.CreateLogGroupOutput), nil).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Once()

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil).Once()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestSetTags_Called(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStreamName,
		SequenceToken: &emptySequenceToken,
	}

	putLogEventsOutput := &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: &expectedNextSequenceToken,
	}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	avalue := "avalue"
	sampleTags := map[string]*string{"akey": &avalue}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), awsErr).Once()

	svc.On("CreateLogGroup",
		&cloudwatchlogs.CreateLogGroupInput{LogGroupName: &logGroup, Tags: sampleTags}).Return(new(cloudwatchlogs.CreateLogGroupOutput), nil).Once()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Once()

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, nil).Once()

	client := newCloudWatchLogClient(svc, 0, sampleTags, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
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
		NextSequenceToken: nil,
	}
	awsErr := &cloudwatchlogs.ResourceNotFoundException{}

	svc.On("PutLogEvents", putLogEventsInput).Return(putLogEventsOutput, awsErr).Twice()

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil).Twice()

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.PutLogEvents(putLogEventsInput, defaultRetryCount)

	svc.AssertExpectations(t)
	assert.Error(t, err)
}

func TestCreateStream_HappyCase(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)

	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(new(cloudwatchlogs.CreateLogStreamOutput), nil)

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.CreateStream(&logGroup, &logStreamName)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestCreateStream_CreateLogStream_ResourceAlreadyExists(t *testing.T) {
	logger := zap.NewNop()
	svc := new(mockCloudWatchLogsClient)

	resourceAlreadyExistsException := &cloudwatchlogs.ResourceAlreadyExistsException{}
	svc.On("CreateLogStream",
		&cloudwatchlogs.CreateLogStreamInput{LogGroupName: &logGroup, LogStreamName: &logStreamName}).Return(
		new(cloudwatchlogs.CreateLogStreamOutput), resourceAlreadyExistsException)

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.CreateStream(&logGroup, &logStreamName)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
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

	client := newCloudWatchLogClient(svc, 0, nil, logger)
	err := client.CreateStream(&logGroup, &logStreamName)

	svc.AssertExpectations(t)
	assert.NoError(t, err)
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
	return errors.New("OrigErr")
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
	expectedComponentName := "mockComponentName"
	tests := []struct {
		name                 string
		buildInfo            component.BuildInfo
		logGroupName         string
		clientOptions        []ClientOption
		expectedUserAgentStr string
	}{
		{
			"emptyLogGroupAndEmptyClientOptions",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"",
			[]ClientOption{},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s)", expectedComponentName),
		},
		{
			"emptyLogGroupWithEmptyUserAgentExtras",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"",
			[]ClientOption{WithUserAgentExtras()},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s)", expectedComponentName),
		},
		{
			"buildInfoCommandUsed",
			component.BuildInfo{Command: "test-collector-contrib", Version: "1.0"},
			"",
			[]ClientOption{},
			fmt.Sprintf("test-collector-contrib/1.0 (%s)", expectedComponentName),
		},
		{
			"buildInfoCommandUsedWithEmptyUserAgentExtras",
			component.BuildInfo{Command: "test-collector-contrib", Version: "1.0"},
			"",
			[]ClientOption{WithUserAgentExtras()},
			fmt.Sprintf("test-collector-contrib/1.0 (%s)", expectedComponentName),
		},
		{
			"nonContainerInsights",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.1"},
			"test-group",
			[]ClientOption{},
			fmt.Sprintf("opentelemetry-collector-contrib/1.1 (%s)", expectedComponentName),
		},
		{
			"containerInsightsEKS",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/containerinsights/eks-cluster-name/performance",
			[]ClientOption{},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s; ContainerInsights)", expectedComponentName),
		},
		{
			"containerInsightsECS",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/ecs/containerinsights/ecs-cluster-name/performance",
			[]ClientOption{},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s; ContainerInsights)", expectedComponentName),
		},
		{
			"containerInsightsPrometheus",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/containerinsights/cluster-name/prometheus",
			[]ClientOption{},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s; ContainerInsights)", expectedComponentName),
		},
		{
			"validAppSignalsLogGroupAndAgentString",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/application-signals",
			[]ClientOption{WithUserAgentExtras("AppSignals")},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s; AppSignals)", expectedComponentName),
		},
		{
			"multipleAgentStringExtras",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/application-signals",
			[]ClientOption{WithUserAgentExtras("abcde", "vwxyz", "12345")},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s; abcde; vwxyz; 12345)", expectedComponentName),
		},
		{
			"containerInsightsEKSWithMultipleAgentStringExtras",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/containerinsights/eks-cluster-name/performance",
			[]ClientOption{WithUserAgentExtras("extra0", "extra1", "extra2")},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s; extra0; extra1; extra2; ContainerInsights)", expectedComponentName),
		},
		{
			"validAppSignalsEMFEnabled",
			component.BuildInfo{Command: "opentelemetry-collector-contrib", Version: "1.0"},
			"/aws/application-signals",
			[]ClientOption{WithUserAgentExtras("AppSignals")},
			fmt.Sprintf("opentelemetry-collector-contrib/1.0 (%s; AppSignals)", expectedComponentName),
		},
	}

	testSession, _ := session.NewSession()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cwlog := NewClient(logger, &aws.Config{}, tc.buildInfo, tc.logGroupName, 0, map[string]*string{}, testSession, expectedComponentName, tc.clientOptions...)
			logClient := cwlog.svc.(*cloudwatchlogs.CloudWatchLogs)

			req := request.New(aws.Config{}, metadata.ClientInfo{}, logClient.Handlers, nil, &request.Operation{
				HTTPMethod: http.MethodGet,
				HTTPPath:   "/",
			}, nil, nil)

			logClient.Handlers.Build.Run(req)
			assert.Contains(t, req.HTTPRequest.UserAgent(), tc.expectedUserAgentStr)
		})
	}
}
