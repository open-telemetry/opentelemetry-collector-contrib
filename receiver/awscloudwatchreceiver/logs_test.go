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

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
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
	expected, err := readLogs(filepath.Join("testdata", "processed", "prefixed.json"))
	require.NoError(t, err)
	require.NoError(t, compareLogs(expected, logs))
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

	err = alertRcvr.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	expected, err := readLogs(filepath.Join("testdata", "processed", "prefixed.json"))
	require.NoError(t, err)
	require.NoError(t, compareLogs(expected, logs))
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
					EventId:       &testEventID,
					IngestionTime: aws.Int64(testIngestionTime),
					LogStreamName: aws.String(testLogStreamName),
					Message:       aws.String(testLogStreamMessage),
					Timestamp:     aws.Int64(testTimeStamp),
				},
			},
			NextToken: nil,
		}, nil)
	return mc
}

func compareLogs(expected, actual plog.Logs) error {
	if expected.ResourceLogs().Len() != actual.ResourceLogs().Len() {
		return fmt.Errorf("amount of ResourceLogs between Logs are not equal (expected: %d, actual: %d)",
			expected.ResourceLogs().Len(),
			actual.ResourceLogs().Len())
	}

	for i := 0; i < expected.ResourceLogs().Len(); i++ {
		err := compareResourceLogs(expected.ResourceLogs().At(i), actual.ResourceLogs().At(i))
		if err != nil {
			return fmt.Errorf("resource logs at index %d: %w", i, err)
		}
	}
	return nil
}

func compareResourceLogs(expected, actual plog.ResourceLogs) error {
	if expected.SchemaUrl() != actual.SchemaUrl() {
		return fmt.Errorf("resource logs SchemaUrl doesn't match (expected: %s, actual: %s)",
			expected.SchemaUrl(),
			actual.SchemaUrl())
	}

	if !reflect.DeepEqual(expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw()) {
		return fmt.Errorf("resource logs Attributes doesn't match (expected: %+v, actual: %+v)",
			expected.Resource().Attributes().AsRaw(),
			actual.Resource().Attributes().AsRaw())
	}

	if expected.Resource().DroppedAttributesCount() != actual.Resource().DroppedAttributesCount() {
		return fmt.Errorf("resource logs DroppedAttributesCount doesn't match (expected: %d, actual: %d)",
			expected.Resource().DroppedAttributesCount(),
			actual.Resource().DroppedAttributesCount())
	}

	if expected.ScopeLogs().Len() != actual.ScopeLogs().Len() {
		return fmt.Errorf("amount of ScopeLogs between ResourceLogs are not equal (expected: %d, actual: %d)",
			expected.ScopeLogs().Len(),
			actual.ScopeLogs().Len())
	}

	for i := 0; i < expected.ScopeLogs().Len(); i++ {
		err := compareScopeLogs(expected.ScopeLogs().At(i), actual.ScopeLogs().At(i))
		if err != nil {
			return fmt.Errorf("scope logs at index %d: %w", i, err)
		}
	}

	return nil
}

func compareScopeLogs(expected, actual plog.ScopeLogs) error {
	if expected.SchemaUrl() != actual.SchemaUrl() {
		return fmt.Errorf("log scope SchemaUrl doesn't match (expected: %s, actual: %s)",
			expected.SchemaUrl(),
			actual.SchemaUrl())
	}

	if expected.Scope().Name() != actual.Scope().Name() {
		return fmt.Errorf("log scope Name doesn't match (expected: %s, actual: %s)",
			expected.Scope().Name(),
			actual.Scope().Name())
	}

	if expected.Scope().Version() != actual.Scope().Version() {
		return fmt.Errorf("log scope Version doesn't match (expected: %s, actual: %s)",
			expected.Scope().Version(),
			actual.Scope().Version())
	}

	if expected.LogRecords().Len() != actual.LogRecords().Len() {
		return fmt.Errorf("amount of log records between ScopeLogs are not equal (expected: %d, actual: %d)",
			expected.LogRecords().Len(),
			actual.LogRecords().Len())
	}

	for i := 0; i < expected.LogRecords().Len(); i++ {
		err := compareLogRecord(expected.LogRecords().At(i), actual.LogRecords().At(i))
		if err != nil {
			return fmt.Errorf("log record at index %d: %w", i, err)
		}
	}

	return nil
}

func compareLogRecord(expected, actual plog.LogRecord) error {
	if expected.Flags() != actual.Flags() {
		return fmt.Errorf("log record Flags doesn't match (expected: %d, actual: %d)",
			expected.Flags(),
			actual.Flags())
	}

	if expected.DroppedAttributesCount() != actual.DroppedAttributesCount() {
		return fmt.Errorf("log record DroppedAttributesCount doesn't match (expected: %d, actual: %d)",
			expected.DroppedAttributesCount(),
			actual.DroppedAttributesCount())
	}

	if expected.Timestamp() != actual.Timestamp() {
		return fmt.Errorf("log record Timestamp doesn't match (expected: %d, actual: %d)",
			expected.Timestamp(),
			actual.Timestamp())
	}

	if expected.SeverityNumber() != actual.SeverityNumber() {
		return fmt.Errorf("log record SeverityNumber doesn't match (expected: %d, actual: %d)",
			expected.SeverityNumber(),
			actual.SeverityNumber())
	}

	if expected.SeverityText() != actual.SeverityText() {
		return fmt.Errorf("log record SeverityText doesn't match (expected: %s, actual: %s)",
			expected.SeverityText(),
			actual.SeverityText())
	}

	if expected.TraceID() != actual.TraceID() {
		return fmt.Errorf("log record TraceID doesn't match (expected: %d, actual: %d)",
			expected.TraceID(),
			actual.TraceID())
	}

	if expected.SpanID() != actual.SpanID() {
		return fmt.Errorf("log record SpanID doesn't match (expected: %d, actual: %d)",
			expected.SpanID(),
			actual.SpanID())
	}

	if !expected.Body().Equal(actual.Body()) {
		return fmt.Errorf("log record Body doesn't match (expected: %s, actual: %s)",
			expected.Body().AsString(),
			actual.Body().AsString())
	}

	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("log record Attributes doesn't match (expected: %#v, actual: %#v)",
			expected.Attributes().AsRaw(),
			actual.Attributes().AsRaw())
	}

	return nil
}

var (
	testLogGroupName     = "test-log-group-name"
	testLogStreamName    = "test-log-stream-name"
	testLogStreamPrefix  = "test-log-stream"
	testEventID          = "37134448277055698880077365577645869800162629528367333379"
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

func readLogs(path string) (plog.Logs, error) {
	f, err := os.Open(path)
	if err != nil {
		return plog.Logs{}, err
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return plog.Logs{}, err
	}

	unmarshaler := plog.JSONUnmarshaler{}
	return unmarshaler.UnmarshalLogs(b)
}
