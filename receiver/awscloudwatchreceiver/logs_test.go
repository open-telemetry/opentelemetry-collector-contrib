// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestStart(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.Groups.AutodiscoverConfig = nil

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)

	err := logsRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = logsRcvr.Shutdown(t.Context())
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
	alertRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	alertRcvr.client = defaultMockClient()

	err := alertRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	err = alertRcvr.Shutdown(t.Context())
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
	alertRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	alertRcvr.client = defaultMockClient()

	err := alertRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	groupRequests := alertRcvr.groupRequests
	require.Len(t, groupRequests, 1)
	require.Equal(t, "test-log-group-name", groupRequests[0].groupName())

	err = alertRcvr.Shutdown(t.Context())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	expected, err := golden.ReadLogs(filepath.Join("testdata", "processed", "prefixed.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
}

func TestNamedConfigNoStreamFilter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 1 * time.Second
	cfg.Logs.Groups = GroupConfig{
		NamedConfigs: map[string]StreamConfig{
			testLogGroupName: {},
		},
	}

	sink := &consumertest.LogsSink{}
	alertRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	alertRcvr.client = defaultMockClient()

	err := alertRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	groupRequests := alertRcvr.groupRequests
	require.Len(t, groupRequests, 1)
	require.Equal(t, "test-log-group-name", groupRequests[0].groupName())

	err = alertRcvr.Shutdown(t.Context())
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
	logsRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	logsRcvr.client = defaultMockClient()

	require.NoError(t, logsRcvr.Start(t.Context(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)
	require.Len(t, logsRcvr.groupRequests, 2)
	require.NoError(t, logsRcvr.Shutdown(t.Context()))
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
	alertRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	doneChan := make(chan time.Time, 1)
	mc := &mockClient{}
	mc.On("FilterLogEvents", mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{
		Events:    []types.FilteredLogEvent{},
		NextToken: aws.String("next"),
	}, nil).
		WaitUntil(doneChan)
	alertRcvr.client = mc

	err := alertRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Never(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 3*time.Second, 10*time.Millisecond)

	close(doneChan)
	require.NoError(t, alertRcvr.Shutdown(t.Context()))
}

func TestAutodiscoverPattern(t *testing.T) {
	mc := &mockClient{}

	mc.On(
		"DescribeLogGroups",
		mock.Anything,
		mock.MatchedBy(func(input *cloudwatchlogs.DescribeLogGroupsInput) bool {
			if input.LogGroupNamePattern != nil && strings.Contains(testLogGroupName, *input.LogGroupNamePattern) {
				return true
			}

			return false
		}),
		mock.Anything,
	).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []types.LogGroup{
				{
					LogGroupName: &testLogGroupName,
				},
			},
			NextToken: nil,
		}, nil)

	mc.On(
		"DescribeLogGroups",
		mock.Anything,
		mock.MatchedBy(func(input *cloudwatchlogs.DescribeLogGroupsInput) bool {
			fmt.Printf("The log group name pattern %s", *input.LogGroupNamePattern)
			if input.LogGroupNamePattern == nil || !strings.Contains(testLogGroupName, *input.LogGroupNamePattern) {
				return true
			}

			return false
		}),
		mock.Anything,
	).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []types.LogGroup{},
			NextToken: nil,
		}, nil)

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.Groups = GroupConfig{
		AutodiscoverConfig: &AutodiscoverConfig{
			Limit:   1,
			Pattern: testLogGroupName[2:5],
		},
	}

	sink := &consumertest.LogsSink{}
	alertRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	alertRcvr.client = mc

	grs, err := alertRcvr.discoverGroups(t.Context(), cfg.Logs.Groups.AutodiscoverConfig)
	require.NoError(t, err)
	require.Len(t, grs, 1)

	cfg.Logs.Groups = GroupConfig{
		AutodiscoverConfig: &AutodiscoverConfig{
			Limit:   1,
			Pattern: testLogGroupName[5:] + "-no-matches",
		},
	}

	grs, err = alertRcvr.discoverGroups(t.Context(), cfg.Logs.Groups.AutodiscoverConfig)
	require.NoError(t, err)
	require.Empty(t, grs)
}

func TestAutodiscoverLimit(t *testing.T) {
	mc := &mockClient{}

	logGroups := []types.LogGroup{}
	for i := 0; i <= 100; i++ {
		logGroups = append(logGroups, types.LogGroup{
			LogGroupName: aws.String(fmt.Sprintf("test log group: %d", i)),
		})
	}
	token := "token"
	mc.On("DescribeLogGroups", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: logGroups[:50],
			NextToken: &token,
		}, nil).Once()

	mc.On("DescribeLogGroups", mock.Anything, mock.Anything, mock.Anything).Return(
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
	alertRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	alertRcvr.client = mc

	grs, err := alertRcvr.discoverGroups(t.Context(), cfg.Logs.Groups.AutodiscoverConfig)
	require.NoError(t, err)
	require.Len(t, grs, cfg.Logs.Groups.AutodiscoverConfig.Limit)
}

func TestAutodiscoverAccountIdentifiers(t *testing.T) {
	mc := &mockClient{}

	testAccountID := "123456789012"

	mc.On(
		"DescribeLogGroups",
		mock.Anything,
		mock.MatchedBy(func(input *cloudwatchlogs.DescribeLogGroupsInput) bool {
			// Ensure the request includes the expected account ID and that IncludeLinkedAccounts is enabled.
			if input.IncludeLinkedAccounts == nil || !*input.IncludeLinkedAccounts {
				return false
			}
			return slices.Contains(input.AccountIdentifiers, testAccountID)
		}),
		mock.Anything,
	).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []types.LogGroup{
				{
					LogGroupName: &testLogGroupName,
				},
			},
			NextToken: nil,
		}, nil)

	// Otherwise, return no groups
	mc.On("DescribeLogGroups", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []types.LogGroup{},
			NextToken: nil,
		}, nil)

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.Groups = GroupConfig{
		AutodiscoverConfig: &AutodiscoverConfig{
			Limit:                 1,
			AccountIdentifiers:    []string{testAccountID},
			IncludeLinkedAccounts: aws.Bool(true),
		},
	}

	sink := &consumertest.LogsSink{}
	alertRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	alertRcvr.client = mc

	grs, err := alertRcvr.discoverGroups(t.Context(), cfg.Logs.Groups.AutodiscoverConfig)
	require.NoError(t, err)
	require.Len(t, grs, 1)
	require.Equal(t, testLogGroupName, grs[0].groupName())
	mc.AssertExpectations(t)

	// Filtering by a different account ID does not return any groups
	cfg.Logs.Groups.AutodiscoverConfig.AccountIdentifiers = []string{"987654321098"}
	grs, err = alertRcvr.discoverGroups(t.Context(), cfg.Logs.Groups.AutodiscoverConfig)
	require.NoError(t, err)
	require.Empty(t, grs)
	mc.AssertExpectations(t)
}

func TestShutdownCheckpointer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)

	mockStorage := newMockStorageClient()
	logsRcvr.cloudwatchCheckpointPersister = newCloudwatchCheckpointPersister(mockStorage, zap.NewNop())

	err := logsRcvr.Shutdown(t.Context())
	require.NoError(t, err)

	require.Nil(t, mockStorage.cache)
}

func TestShutdownCheckpointerWithError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)

	// Create a mock storage client that returns an error on Close
	mockStorage := newMockStorageClient()
	mockStorage.forceError = true

	cloudwatchCheckpointPersister := newCloudwatchCheckpointPersister(mockStorage, zap.NewNop())
	logsRcvr.cloudwatchCheckpointPersister = cloudwatchCheckpointPersister

	err := logsRcvr.Shutdown(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "forced storage error")
}

func TestShutdownWithoutCheckpointer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)

	// Don't set a checkpointer (should be nil by default)
	require.Nil(t, logsRcvr.cloudwatchCheckpointPersister)

	// Call Shutdown and verify it doesn't error when checkpointer is nil
	err := logsRcvr.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestDeletedLogGroupContinuesPolling(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 1 * time.Second
	cfg.Logs.Groups = GroupConfig{
		NamedConfigs: map[string]StreamConfig{
			"existing-group": {
				Names: []*string{aws.String("stream1")},
			},
			"deleted-group": {
				Names: []*string{aws.String("stream2")},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	mc := &mockClient{}

	mc.On("FilterLogEvents", mock.Anything, mock.MatchedBy(func(input *cloudwatchlogs.FilterLogEventsInput) bool {
		return *input.LogGroupName == "existing-group"
	}), mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{
		Events: []types.FilteredLogEvent{
			{
				EventId:       aws.String("event1"),
				LogStreamName: aws.String("stream1"),
				Message:       aws.String("test message"),
				Timestamp:     aws.Int64(time.Now().UnixMilli()),
			},
		},
		NextToken: nil,
	}, nil)

	mc.On("FilterLogEvents", mock.Anything, mock.MatchedBy(func(input *cloudwatchlogs.FilterLogEventsInput) bool {
		return *input.LogGroupName == "deleted-group"
	}), mock.Anything).Return((*cloudwatchlogs.FilterLogEventsOutput)(nil), &types.ResourceNotFoundException{
		Message: aws.String("The specified log group does not exist"),
	})

	logsRcvr.client = mc

	err := logsRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)
	logs := sink.AllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, 1, logs[0].LogRecordCount())

	logRecord := logs[0].ResourceLogs().At(0)
	require.Equal(t, "existing-group", logRecord.Resource().Attributes().AsRaw()["cloudwatch.log.group.name"])

	err = logsRcvr.Shutdown(t.Context())
	require.NoError(t, err)

	// Verify all mock expectations were met
	mc.AssertExpectations(t)
}

func TestDeletedLogGroupDuringAutodiscover(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 1 * time.Second
	cfg.Logs.Groups = GroupConfig{
		AutodiscoverConfig: &AutodiscoverConfig{
			Limit:  3,
			Prefix: "/aws/",
		},
	}

	firstPollDone := make(chan struct{}, 1)

	sink := &consumertest.LogsSink{}
	logsRcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)

	mc := &mockClient{}

	// First DescribeLogGroups call returns two groups
	mc.On("DescribeLogGroups", mock.Anything, mock.MatchedBy(func(input *cloudwatchlogs.DescribeLogGroupsInput) bool {
		return input.LogGroupNamePrefix != nil && *input.LogGroupNamePrefix == "/aws/"
	}), mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: []types.LogGroup{
			{
				LogGroupName: aws.String("/aws/working-group"),
			},
			{
				LogGroupName: aws.String("/aws/delete-group"),
			},
			{
				LogGroupName: aws.String("/aws/third-group"),
			},
		},
		NextToken: nil,
	}, nil).Once()

	// Second DescribeLogGroups call returns working group plus a new group
	mc.On("DescribeLogGroups", mock.Anything, mock.MatchedBy(func(input *cloudwatchlogs.DescribeLogGroupsInput) bool {
		return input.LogGroupNamePrefix != nil && *input.LogGroupNamePrefix == "/aws/"
	}), mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: []types.LogGroup{
			{
				LogGroupName: aws.String("/aws/working-group"),
			},
			{
				LogGroupName: aws.String("/aws/third-group"),
			},
		},
		NextToken: nil,
	}, nil).Run(func(_ mock.Arguments) {
		close(firstPollDone)
	})

	// Setup mock for working-group to return normal logs
	mc.On("FilterLogEvents", mock.Anything, mock.MatchedBy(func(input *cloudwatchlogs.FilterLogEventsInput) bool {
		return input.LogGroupName != nil && *input.LogGroupName == "/aws/working-group"
	}), mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{
		Events: []types.FilteredLogEvent{
			{
				EventId:       aws.String("event1"),
				LogStreamName: aws.String("stream1"),
				Message:       aws.String("test message"),
				Timestamp:     aws.Int64(time.Now().UnixMilli()),
			},
		},
		NextToken: nil,
	}, nil)

	// Setup mock for /aws/delete-group to return ResourceNotFoundException
	mc.On("FilterLogEvents", mock.Anything, mock.MatchedBy(func(input *cloudwatchlogs.FilterLogEventsInput) bool {
		return input.LogGroupName != nil && *input.LogGroupName == "/aws/delete-group"
	}), mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{
		Events: []types.FilteredLogEvent{
			{
				EventId:       aws.String("event1"),
				LogStreamName: aws.String("stream1"),
				Message:       aws.String("test message"),
				Timestamp:     aws.Int64(time.Now().UnixMilli()),
			},
		},
		NextToken: aws.String("next"),
	}, nil).Once()

	mc.On("FilterLogEvents", mock.Anything, mock.MatchedBy(func(input *cloudwatchlogs.FilterLogEventsInput) bool {
		return input.LogGroupName != nil && *input.LogGroupName == "/aws/delete-group" && input.NextToken != nil && *input.NextToken == "next"
	}), mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{}, &types.ResourceNotFoundException{
		Message: aws.String("The specified log group does not exist"),
	}).Once()

	// Setup mock for /aws/working-group or /aws/third-group to return normal logs
	mc.On("FilterLogEvents",
		mock.Anything,
		mock.MatchedBy(func(input *cloudwatchlogs.FilterLogEventsInput) bool {
			return input.LogGroupName != nil && (*input.LogGroupName == "/aws/working-group" || *input.LogGroupName == "/aws/third-group")
		}), mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{
		Events: []types.FilteredLogEvent{
			{
				EventId:       aws.String("event2"),
				LogStreamName: aws.String("stream2"),
				Message:       aws.String("test message from group"),
				Timestamp:     aws.Int64(time.Now().UnixMilli()),
			},
		},
		NextToken: nil,
	}, nil)

	logsRcvr.client = mc

	err := logsRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Wait for first poll to complete
	require.Eventually(t, func() bool {
		select {
		case <-firstPollDone:
			return true
		default:
			return false
		}
	}, 15*time.Second, 10*time.Millisecond)

	err = logsRcvr.Shutdown(t.Context())
	require.NoError(t, err)

	groupNames := make([]string, 0, len(logsRcvr.groupRequests))
	for _, gr := range logsRcvr.groupRequests {
		groupNames = append(groupNames, gr.groupName())
	}
	require.ElementsMatch(t, []string{"/aws/working-group", "/aws/third-group"}, groupNames)
	logs := sink.AllLogs()
	require.GreaterOrEqual(t, len(logs), 3)

	firstWorkingGroupLog := logs[0].ResourceLogs().At(0)
	require.Equal(t, "/aws/working-group", firstWorkingGroupLog.Resource().Attributes().AsRaw()["cloudwatch.log.group.name"])
	require.Equal(t, 1, firstWorkingGroupLog.ScopeLogs().Len())

	secondWorkingGroupLog := logs[1].ResourceLogs().At(0)
	require.Equal(t, "/aws/delete-group", secondWorkingGroupLog.Resource().Attributes().AsRaw()["cloudwatch.log.group.name"])
	require.Equal(t, 1, secondWorkingGroupLog.ScopeLogs().Len())

	thirdWorkingGroupLog := logs[2].ResourceLogs().At(0)
	require.Equal(t, "/aws/third-group", thirdWorkingGroupLog.Resource().Attributes().AsRaw()["cloudwatch.log.group.name"])
	require.Equal(t, 1, thirdWorkingGroupLog.ScopeLogs().Len())

	mc.AssertExpectations(t)
}

func defaultMockClient() client {
	mc := &mockClient{}
	mc.On("DescribeLogGroups", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []types.LogGroup{
				{
					LogGroupName: &testLogGroupName,
				},
			},
			NextToken: nil,
		}, nil)
	mc.On("FilterLogEvents", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatchlogs.FilterLogEventsOutput{
			Events: []types.FilteredLogEvent{
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

func (mc *mockClient) DescribeLogGroups(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...func(options *cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	args := mc.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.DescribeLogGroupsOutput), args.Error(1)
}

func (mc *mockClient) FilterLogEvents(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput, opts ...func(options *cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	args := mc.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.FilterLogEventsOutput), args.Error(1)
}
