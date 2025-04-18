// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type mockCloudWatchClient struct {
	createLogGroupCount atomic.Int32
	createLogGroupFuncs []func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	createLogGroup      func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)

	createLogStreamCount atomic.Int32
	createLogStreamFuncs []func(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	createLogStream      func(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)

	putLogEventsCount atomic.Int32
	putLogEventsFuncs []func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
	putLogEvents      func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)

	putRetentionPolicy func(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	tagResource        func(ctx context.Context, params *cloudwatchlogs.TagResourceInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.TagResourceOutput, error)
}

func (m *mockCloudWatchClient) CreateLogGroup(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	i := int(m.createLogGroupCount.Add(1) - 1)
	if i < len(m.createLogGroupFuncs) {
		return m.createLogGroupFuncs[i](ctx, params, optFns...)
	}

	return m.createLogGroup(ctx, params, optFns...)
}

func (m *mockCloudWatchClient) CreateLogStream(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	i := int(m.createLogStreamCount.Add(1) - 1)
	if i < len(m.createLogStreamFuncs) {
		return m.createLogStreamFuncs[i](ctx, params, optFns...)
	}

	return m.createLogStream(ctx, params, optFns...)
}

func (m *mockCloudWatchClient) PutLogEvents(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	i := int(m.putLogEventsCount.Add(1) - 1)
	if i < len(m.putLogEventsFuncs) {
		return m.putLogEventsFuncs[i](ctx, params, optFns...)
	}

	return m.putLogEvents(ctx, params, optFns...)
}

func (m *mockCloudWatchClient) PutRetentionPolicy(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	return m.putRetentionPolicy(ctx, params, optFns...)
}

func (m *mockCloudWatchClient) TagResource(ctx context.Context, params *cloudwatchlogs.TagResourceInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.TagResourceOutput, error) {
	return m.tagResource(ctx, params, optFns...)
}

// Tests
var (
	logGroup      = "logGroup"
	logStreamName = "logStream"
)

func TestPutLogEvents(t *testing.T) {
	tests := []struct {
		name      string
		client    cloudWatchClient
		expectErr bool
	}{
		{
			name: "Happy path",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return &cloudwatchlogs.PutLogEventsOutput{}, nil
				},
			},
			expectErr: false,
		},
		{
			name: "Some rejected info",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return &cloudwatchlogs.PutLogEventsOutput{
						RejectedLogEventsInfo: &types.RejectedLogEventsInfo{
							ExpiredLogEventEndIndex:  aws.Int32(1),
							TooNewLogEventStartIndex: aws.Int32(2),
							TooOldLogEventEndIndex:   aws.Int32(3),
						},
					}, nil
				},
			},
			expectErr: false,
		},
		{
			name: "Non-AWS error",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return nil, errors.New("some random error")
				},
			},
			expectErr: true,
		},
		{
			name: "InvalidParameterException",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return nil, &types.InvalidParameterException{}
				},
			},
			expectErr: true,
		},
		{
			name: "OperationAbortedException",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return nil, &types.OperationAbortedException{}
				},
			},
			expectErr: true,
		},
		{
			name: "ServiceUnavailableException",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return nil, &types.ServiceUnavailableException{}
				},
			},
			expectErr: true,
		},
		{
			name: "UnknownException",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return nil, &smithy.OperationError{
						Err: errors.New("unknownException"),
					}
				},
			},
			expectErr: true,
		},
		{
			name: "ThrottlingException",
			client: &mockCloudWatchClient{
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return nil, &types.ThrottlingException{}
				},
			},
			expectErr: true,
		},
		{
			name: "Successful after ResourceNotFoundException",
			client: &mockCloudWatchClient{
				createLogStream: func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
					return nil, nil
				},
				putLogEventsFuncs: []func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return nil, nil
					},
				},
			},
			expectErr: false,
		},
		{
			name: "All retries fail",
			client: &mockCloudWatchClient{
				createLogGroup: func(_ context.Context, _ *cloudwatchlogs.CreateLogGroupInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
					return nil, &types.ResourceNotFoundException{}
				},
				createLogStream: func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
					return nil, &types.ResourceNotFoundException{}
				},
				putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
					return nil, &types.ResourceNotFoundException{}
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := zap.NewNop()
			client := newCloudWatchLogClient(test.client, 0, nil, logger)
			err := client.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{
				LogGroupName:  aws.String(logGroup),
				LogStreamName: aws.String(logStreamName),
			}, defaultRetryCount)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPutLogEvents_WithOpts(t *testing.T) {
	tests := []struct {
		name         string
		logRetention int32
		tags         map[string]string
		client       cloudWatchClient
		expectErr    bool
	}{
		{
			name:         "Log retention - never expire",
			logRetention: 0,
			client: &mockCloudWatchClient{
				createLogGroup: func(_ context.Context, _ *cloudwatchlogs.CreateLogGroupInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
					return &cloudwatchlogs.CreateLogGroupOutput{}, nil
				},
				createLogStreamFuncs: []func(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return &cloudwatchlogs.CreateLogStreamOutput{}, nil
					},
				},
				putLogEventsFuncs: []func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return &cloudwatchlogs.PutLogEventsOutput{}, nil
					},
				},
			},
			expectErr: false,
		},
		{
			name:         "Log retention - set",
			logRetention: 365,
			client: &mockCloudWatchClient{
				createLogGroup: func(_ context.Context, _ *cloudwatchlogs.CreateLogGroupInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
					return &cloudwatchlogs.CreateLogGroupOutput{}, nil
				},
				createLogStreamFuncs: []func(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return &cloudwatchlogs.CreateLogStreamOutput{}, nil
					},
				},
				putLogEventsFuncs: []func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return &cloudwatchlogs.PutLogEventsOutput{}, nil
					},
				},
				putRetentionPolicy: func(_ context.Context, _ *cloudwatchlogs.PutRetentionPolicyInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
					return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
				},
			},
			expectErr: false,
		},
		{
			name:         "With tags",
			logRetention: 0,
			tags: map[string]string{
				"key": "value",
			},
			client: &mockCloudWatchClient{
				createLogGroup: func(_ context.Context, _ *cloudwatchlogs.CreateLogGroupInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
					return &cloudwatchlogs.CreateLogGroupOutput{}, nil
				},
				createLogStreamFuncs: []func(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return &cloudwatchlogs.CreateLogStreamOutput{}, nil
					},
				},
				putLogEventsFuncs: []func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
						return &cloudwatchlogs.PutLogEventsOutput{}, nil
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := zap.NewNop()
			client := newCloudWatchLogClient(test.client, test.logRetention, nil, logger)
			err := client.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{}, defaultRetryCount)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateStream(t *testing.T) {
	tests := []struct {
		name      string
		client    cloudWatchClient
		expectErr bool
	}{
		{
			name: "Happy path",
			client: &mockCloudWatchClient{
				createLogStream: func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
					return &cloudwatchlogs.CreateLogStreamOutput{}, nil
				},
			},
			expectErr: false,
		},
		{
			name: "Already exists",
			client: &mockCloudWatchClient{
				createLogStream: func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
					return nil, &types.ResourceAlreadyExistsException{}
				},
			},
			expectErr: false,
		},
		{
			name: "Resource not found",
			client: &mockCloudWatchClient{
				createLogGroup: func(_ context.Context, _ *cloudwatchlogs.CreateLogGroupInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
					return &cloudwatchlogs.CreateLogGroupOutput{}, nil
				},
				createLogStreamFuncs: []func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error){
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return nil, &types.ResourceNotFoundException{}
					},
					func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
						return &cloudwatchlogs.CreateLogStreamOutput{}, nil
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := zap.NewNop()
			client := newCloudWatchLogClient(test.client, 0, nil, logger)
			err := client.CreateStream(context.Background(), &logGroup, &logStreamName)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := newCollectorUserAgent(tc.buildInfo, tc.logGroupName, expectedComponentName, tc.clientOptions...)
			assert.Contains(t, tc.expectedUserAgentStr, actual)
		})
	}
}
