// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestStart(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.Groups.AutodiscoverConfig = nil

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, zap.NewNop(), sink)

	err := logsRcvr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = logsRcvr.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPrefixedConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 1 * time.Second
	cfg.Logs.Groups = GroupConfig{
		NamedConfigs: map[string]StreamConfig{
			testLogGroupName: {
				Names: []*string{&testLogStreamName},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	alertRcvr := newLogsReceiver(cfg, zap.NewNop(), sink)
	alertRcvr.client = defaultMockClient()

	err := alertRcvr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	err = alertRcvr.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	expected, err := golden.ReadLogs(filepath.Join("testdata", "processed", "prefixed.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
}

func TestPrefixedNamedStreamsConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 1 * time.Second
	cfg.Logs.Groups = GroupConfig{
		NamedConfigs: map[string]StreamConfig{
			testLogGroupName: {
				Prefixes: []*string{&testLogStreamPrefix},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	alertRcvr := newLogsReceiver(cfg, zap.NewNop(), sink)
	alertRcvr.client = defaultMockClient()

	err := alertRcvr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	groupRequests := alertRcvr.groupRequests
	require.Len(t, groupRequests, 1)
	require.Equal(t, groupRequests[0].groupName(), "test-log-group-name")

	err = alertRcvr.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	expected, err := golden.ReadLogs(filepath.Join("testdata", "processed", "prefixed.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
}

func TestDiscovery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 1 * time.Second
	cfg.Logs.Groups = GroupConfig{
		AutodiscoverConfig: &AutodiscoverConfig{
			Limit: 1,
			Streams: StreamConfig{
				Prefixes: []*string{&testLogStreamPrefix},
				Names:    []*string{&testLogStreamMessage},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, zap.NewNop(), sink)
	logsRcvr.client = defaultMockClient()

	require.NoError(t, logsRcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, len(logsRcvr.groupRequests), 2)
	require.NoError(t, logsRcvr.Shutdown(context.Background()))
}

// Test to ensure that mid collection while streaming results we will
// return early if Shutdown is called
func TestShutdownWhileCollecting(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 1 * time.Second
	cfg.Logs.Groups = GroupConfig{
		NamedConfigs: map[string]StreamConfig{
			testLogGroupName: {
				Names: []*string{&testLogStreamName},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	alertRcvr := newLogsReceiver(cfg, zap.NewNop(), sink)
	doneChan := make(chan time.Time, 1)
	mc := &mockClient{}
	mc.On("FilterLogEventsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{
		Events:    []*cloudwatchlogs.FilteredLogEvent{},
		NextToken: aws.String("next"),
	}, nil).
		WaitUntil(doneChan)
	alertRcvr.client = mc

	err := alertRcvr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Never(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 3*time.Second, 10*time.Millisecond)

	close(doneChan)
	require.NoError(t, alertRcvr.Shutdown(context.Background()))
}

func TestAutodiscoverLimit(t *testing.T) {
	mc := &mockClient{}

	logGroups := []*cloudwatchlogs.LogGroup{}
	for i := 0; i <= 100; i++ {
		logGroups = append(logGroups, &cloudwatchlogs.LogGroup{
			LogGroupName: aws.String(fmt.Sprintf("test log group: %d", i)),
		})
	}
	token := "token"
	mc.On("DescribeLogGroupsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: logGroups[:50],
			NextToken: &token,
		}, nil).Once()

	mc.On("DescribeLogGroupsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: logGroups[50:],
			NextToken: nil,
		}, nil)

	numGroups := 100

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.Groups = GroupConfig{
		AutodiscoverConfig: &AutodiscoverConfig{
			Prefix: "/aws/",
			Limit:  numGroups,
		},
	}

	sink := &consumertest.LogsSink{}
	alertRcvr := newLogsReceiver(cfg, zap.NewNop(), sink)
	alertRcvr.client = mc

	grs, err := alertRcvr.discoverGroups(context.Background(), cfg.Logs.Groups.AutodiscoverConfig)
	require.NoError(t, err)
	require.Len(t, grs, cfg.Logs.Groups.AutodiscoverConfig.Limit)
}

func defaultMockClient() client {
	mc := &mockClient{}
	mc.On("DescribeLogGroupsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{
				{
					LogGroupName: &testLogGroupName,
				},
			},
			NextToken: nil,
		}, nil)
	mc.On("FilterLogEventsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.FilterLogEventsOutput{
			Events: []*cloudwatchlogs.FilteredLogEvent{
				{
					EventId:       &testEventIDs[0],
					IngestionTime: aws.Int64(testIngestionTime),
					LogStreamName: aws.String(testLogStreamName),
					Message:       aws.String(testLogStreamMessage),
					Timestamp:     aws.Int64(testTimeStamp),
				},
				{
					EventId:       &testEventIDs[1],
					IngestionTime: aws.Int64(testIngestionTime),
					LogStreamName: aws.String(testLogStreamName),
					Message:       aws.String(testLogStreamMessage),
					Timestamp:     aws.Int64(testTimeStamp),
				},
				{
					EventId:       &testEventIDs[2],
					IngestionTime: aws.Int64(testIngestionTime),
					LogStreamName: aws.String(testLogStreamName2),
					Message:       aws.String(testLogStreamMessage),
					Timestamp:     aws.Int64(testTimeStamp),
				},
				{
					EventId:       &testEventIDs[3],
					IngestionTime: aws.Int64(testIngestionTime),
					LogStreamName: aws.String(testLogStreamName2),
					Message:       aws.String(testLogStreamMessage),
					Timestamp:     aws.Int64(testTimeStamp),
				},
			},
			NextToken: nil,
		}, nil)
	return mc
}

var (
	testLogGroupName    = "test-log-group-name"
	testLogStreamName   = "test-log-stream-name"
	testLogStreamName2  = "test-log-stream-name-2"
	testLogStreamPrefix = "test-log-stream"
	testEventIDs        = []string{
		"37134448277055698880077365577645869800162629528367333379",
		"37134448277055698880077365577645869800162629528367333380",
		"37134448277055698880077365577645869800162629528367333381",
		"37134448277055698880077365577645869800162629528367333382",
	}
	testIngestionTime    = int64(1665166252124)
	testTimeStamp        = int64(1665166251014)
	testLogStreamMessage = `"time=\"2022-10-07T18:10:46Z\" level=info msg=\"access granted\" arn=\"arn:aws:iam::892146088969:role/AWSWesleyClusterManagerLambda-NodeManagerRole-16UPVDKA1KBGI\" client=\"127.0.0.1:50252\" groups=\"[]\" method=POST path=/authenticate uid=\"aws-iam-authenticator:892146088969:AROA47OAM7QE2NWPDFDCW\" username=\"eks:node-manager\""`
)

type mockClient struct {
	mock.Mock
}

func (mc *mockClient) DescribeLogGroupsWithContext(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...request.Option) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	args := mc.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.DescribeLogGroupsOutput), args.Error(1)
}

func (mc *mockClient) FilterLogEventsWithContext(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput, opts ...request.Option) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	args := mc.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.FilterLogEventsOutput), args.Error(1)
}
