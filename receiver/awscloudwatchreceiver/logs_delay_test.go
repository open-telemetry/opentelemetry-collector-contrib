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

// lateEvent is an event that FilterLogEvents only returns once the clock reaches visibleAt,
// like an event that CloudWatch Logs ingests some time after its own timestamp.
type lateEvent struct {
	timestamp time.Time
	visibleAt time.Time
}

// lateClient answers FilterLogEvents from a fixed set of events, honoring the request window
// and each event's visibility time. It records the requested end times.
type lateClient struct {
	now      func() time.Time
	events   []lateEvent
	endTimes []time.Time
}

func (*lateClient) DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
}

func (c *lateClient) FilterLogEvents(_ context.Context, in *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	start, end := time.UnixMilli(*in.StartTime), time.UnixMilli(*in.EndTime)
	c.endTimes = append(c.endTimes, end)
	out := &cloudwatchlogs.FilterLogEventsOutput{}
	for i, e := range c.events {
		if e.timestamp.Before(start) || e.timestamp.After(end) || c.now().Before(e.visibleAt) {
			continue
		}
		out.Events = append(out.Events, types.FilteredLogEvent{
			EventId:       aws.String(string(rune('a' + i))),
			LogStreamName: aws.String("stream"),
			Message:       aws.String("late"),
			Timestamp:     aws.Int64(e.timestamp.UnixMilli()),
		})
	}
	return out, nil
}

func newDelayTestReceiver(t *testing.T, delay time.Duration, start time.Time, sink *consumertest.LogsSink) *logsReceiver {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.Delay = delay
	cfg.Logs.Groups = GroupConfig{NamedConfigs: map[string]StreamConfig{"group": {}}}
	r := newLogsReceiver(cfg, receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, sink)
	r.initialStartTime = start
	return r
}

func TestLogsDelayReadsLateIngestedEvent(t *testing.T) {
	originalNow := nowFunc
	t.Cleanup(func() { nowFunc = originalNow })

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Timestamped 2s before the first poll, visible to FilterLogEvents 10s after it.
	ev := lateEvent{timestamp: base.Add(58 * time.Second), visibleAt: base.Add(70 * time.Second)}

	for _, tc := range []struct {
		name     string
		delay    time.Duration
		expected int
	}{
		{name: "no delay misses the event", delay: 0, expected: 0},
		{name: "delay reads the event", delay: 30 * time.Second, expected: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := base.Add(time.Minute)
			nowFunc = func() time.Time { return current }
			sink := &consumertest.LogsSink{}
			r := newDelayTestReceiver(t, tc.delay, base, sink)
			r.client = &lateClient{now: nowFunc, events: []lateEvent{ev}}

			for range 3 {
				require.NoError(t, r.poll(t.Context()))
				current = current.Add(time.Minute)
			}
			require.Equal(t, tc.expected, sink.LogRecordCount())
		})
	}
}

func TestLogsDelayShiftsWindowEnd(t *testing.T) {
	originalNow := nowFunc
	t.Cleanup(func() { nowFunc = originalNow })

	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }

	for _, delay := range []time.Duration{0, 90 * time.Second} {
		sink := &consumertest.LogsSink{}
		r := newDelayTestReceiver(t, delay, now.Add(-time.Hour), sink)
		c := &lateClient{now: nowFunc}
		r.client = c

		require.NoError(t, r.poll(t.Context()))
		require.Len(t, c.endTimes, 1)
		require.Equal(t, now.Add(-delay).UnixMilli(), c.endTimes[0].UnixMilli())
		// An empty window advances the start to the window end, not to now.
		require.Equal(t, now.Add(-delay), r.groupNextStartTimes["group"])
	}
}

func TestLogsDelaySkipsGroupWhenWindowIsEmpty(t *testing.T) {
	originalNow := nowFunc
	t.Cleanup(func() { nowFunc = originalNow })

	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }

	// The start is only 10s ago, so with a 60s delay the window end is before the start.
	start := now.Add(-10 * time.Second)
	sink := &consumertest.LogsSink{}
	r := newDelayTestReceiver(t, time.Minute, start, sink)
	c := &lateClient{now: nowFunc}
	r.client = c

	require.NoError(t, r.poll(t.Context()))
	require.Empty(t, c.endTimes, "no request should be sent for an empty window")
	require.Equal(t, start, r.groupNextStartTimes["group"])
}
