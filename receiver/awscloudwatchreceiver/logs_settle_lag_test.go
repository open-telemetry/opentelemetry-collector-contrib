// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const settleTestGroup = "settle-group"

// lateEventClient returns one event only from the poll after it was ingested, and only
// when the event timestamp lies inside the requested window.
type lateEventClient struct {
	eventTime     time.Time
	visibleOnCall int
	calls         int
	lastWindow    [2]int64
}

func (c *lateEventClient) DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
}

func (c *lateEventClient) FilterLogEvents(_ context.Context, in *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	c.calls++
	c.lastWindow = [2]int64{aws.ToInt64(in.StartTime), aws.ToInt64(in.EndTime)}
	ts := c.eventTime.UnixMilli()
	if c.calls < c.visibleOnCall || ts < c.lastWindow[0] || ts > c.lastWindow[1] {
		return &cloudwatchlogs.FilterLogEventsOutput{}, nil
	}
	return &cloudwatchlogs.FilterLogEventsOutput{Events: []types.FilteredLogEvent{{
		EventId:       aws.String("late-event"),
		LogStreamName: aws.String("stream"),
		Message:       aws.String("late"),
		Timestamp:     aws.Int64(ts),
	}}}, nil
}

func newSettleReceiver(t *testing.T, lag time.Duration, now *time.Time, c client) (*logsReceiver, *consumertest.LogsSink) {
	t.Helper()
	original := nowFunc
	nowFunc = func() time.Time { return *now }
	t.Cleanup(func() { nowFunc = original })

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.SettleLag = lag
	cfg.Logs.InitialLookback = time.Hour
	cfg.Logs.Groups = GroupConfig{NamedConfigs: map[string]StreamConfig{settleTestGroup: {}}}

	sink := &consumertest.LogsSink{}
	rcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	}, sink)
	rcvr.client = c
	return rcvr, sink
}

func TestSettleLagDefaultKeepsWindowEndAtNow(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	c := &lateEventClient{eventTime: now.Add(time.Hour), visibleOnCall: 1}
	rcvr, _ := newSettleReceiver(t, 0, &now, c)

	require.NoError(t, rcvr.poll(t.Context()))

	require.Equal(t, now.UnixMilli(), c.lastWindow[1])
	require.Equal(t, now.UnixMilli(), rcvr.groupNextStartTimes[settleTestGroup].UnixMilli())
}

func TestSettleLagEndsWindowBeforeNow(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	c := &lateEventClient{eventTime: now.Add(time.Hour), visibleOnCall: 1}
	rcvr, _ := newSettleReceiver(t, 30*time.Second, &now, c)

	require.NoError(t, rcvr.poll(t.Context()))

	want := now.Add(-30 * time.Second).UnixMilli()
	require.Equal(t, want, c.lastWindow[1], "FilterLogEvents EndTime")
	require.Equal(t, want, rcvr.groupNextStartTimes[settleTestGroup].UnixMilli(), "empty-window advance")
}

func TestSettleLagPicksUpLateIngestedEvent(t *testing.T) {
	tests := []struct {
		name      string
		lag       time.Duration
		wantCount int
	}{
		{name: "no lag misses the event", lag: 0, wantCount: 0},
		{name: "lag picks it up", lag: 30 * time.Second, wantCount: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
			now := first
			// The event happened 2 s before the first poll and is ingested after it.
			c := &lateEventClient{eventTime: first.Add(-2 * time.Second), visibleOnCall: 2}
			rcvr, sink := newSettleReceiver(t, tt.lag, &now, c)

			require.NoError(t, rcvr.poll(t.Context()))
			now = first.Add(time.Minute)
			require.NoError(t, rcvr.poll(t.Context()))

			require.Equal(t, tt.wantCount, sink.LogRecordCount())
		})
	}
}

func TestSettleLagLongerThanLookbackSkipsThePoll(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	c := &lateEventClient{eventTime: now, visibleOnCall: 1}
	rcvr, _ := newSettleReceiver(t, 2*time.Hour, &now, c)

	require.NoError(t, rcvr.poll(t.Context()))

	require.Zero(t, c.calls, "no request when the window would end before it starts")
	_, advanced := rcvr.groupNextStartTimes[settleTestGroup]
	require.False(t, advanced, "the start time must not move")
}

func TestSettleLagNegativeIsInvalid(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.Groups = GroupConfig{NamedConfigs: map[string]StreamConfig{settleTestGroup: {}}}
	cfg.Logs.SettleLag = -time.Second

	require.ErrorIs(t, cfg.Validate(), errInvalidSettleLag)
}
