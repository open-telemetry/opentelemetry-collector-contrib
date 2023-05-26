// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestLoggingIntegration(t *testing.T) {
	mc := &mockClient{}
	mc.On("DescribeLogGroupsWithContext", mock.Anything, mock.Anything, mock.Anything).
		Return(loadLogGroups(t), nil)

	mc.On("FilterLogEventsWithContext", mock.Anything, mock.Anything, mock.Anything).
		Return(loadLogEvents(t), nil)

	sink := &consumertest.LogsSink{}
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-east-2"
	cfg.Logs.PollInterval = time.Second
	cfg.Logs.Groups.AutodiscoverConfig = &AutodiscoverConfig{
		Limit: 1,
	}
	recv, err := NewFactory().CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		sink,
	)
	require.NoError(t, err)

	rcvr, ok := recv.(*logsReceiver)
	require.True(t, ok)
	rcvr.client = mc

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 5*time.Second, 10*time.Millisecond)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]

	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "golden", "autodiscovered.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))
}

var (
	logGroupFiles = []string{
		filepath.Join("testdata", "log-groups", "group-1.json"),
	}
	logEventsFiles = []string{
		filepath.Join("testdata", "events", "event-1.json"),
	}
)

func loadLogGroups(t *testing.T) *cloudwatchlogs.DescribeLogGroupsOutput {
	var output []*cloudwatchlogs.LogGroup
	for _, lg := range logGroupFiles {
		bytes, err := os.ReadFile(lg)
		require.NoError(t, err)
		var logGroup cloudwatchlogs.LogGroup
		err = json.Unmarshal(bytes, &logGroup)
		require.NoError(t, err)
		output = append(output, &logGroup)
	}

	return &cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: output,
		NextToken: nil,
	}
}

func loadLogEvents(t *testing.T) *cloudwatchlogs.FilterLogEventsOutput {
	var output []*cloudwatchlogs.FilteredLogEvent
	for _, lg := range logEventsFiles {
		bytes, err := os.ReadFile(lg)
		require.NoError(t, err)
		var event cloudwatchlogs.FilteredLogEvent
		err = json.Unmarshal(bytes, &event)
		require.NoError(t, err)
		output = append(output, &event)
	}

	return &cloudwatchlogs.FilterLogEventsOutput{
		Events:    output,
		NextToken: nil,
	}
}
