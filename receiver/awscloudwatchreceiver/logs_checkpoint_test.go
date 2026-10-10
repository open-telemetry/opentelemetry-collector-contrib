// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

func TestPollPersistsAcknowledgedCheckpoint(t *testing.T) {
	end := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	start := end.Add(-time.Hour).Add(123 * time.Millisecond)
	eventTime := start.Add(10 * time.Minute)
	accepted := eventTime.Add(time.Millisecond)

	originalNow := nowFunc
	nowFunc = func() time.Time { return end }
	t.Cleanup(func() { nowFunc = originalNow })

	eventsPage := func(nextToken *string) *cloudwatchlogs.FilterLogEventsOutput {
		return &cloudwatchlogs.FilterLogEventsOutput{
			Events: []types.FilteredLogEvent{{
				EventId:   aws.String("event"),
				Message:   aws.String("message"),
				Timestamp: aws.Int64(eventTime.UnixMilli()),
			}},
			NextToken: nextToken,
		}
	}
	startsAt := func(ts time.Time) any {
		return mock.MatchedBy(func(input *cloudwatchlogs.FilterLogEventsInput) bool {
			return aws.ToInt64(input.StartTime) == ts.UnixMilli()
		})
	}
	apiErr := errors.New("API unavailable")

	const group = "checkpoint-group"
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.StartFrom = start.Format(time.RFC3339Nano)
	cfg.Logs.Groups = GroupConfig{NamedConfigs: map[string]StreamConfig{group: {}}}
	telemetry := receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}

	type page struct {
		output *cloudwatchlogs.FilterLogEventsOutput
		err    error
	}

	for _, tc := range []struct {
		name       string
		consumeErr error
		pages      []page
		wantErr    bool
		want       time.Time
	}{
		{
			name:    "API failure",
			pages:   []page{{err: apiErr}},
			wantErr: true,
			want:    start,
		},
		{
			name:    "accepted page before API failure",
			pages:   []page{{output: eventsPage(aws.String("next-page"))}, {err: apiErr}},
			wantErr: true,
			want:    accepted,
		},
		{
			name:       "consumer failure",
			consumeErr: errors.New("consumer unavailable"),
			pages:      []page{{output: eventsPage(nil)}},
			wantErr:    true,
			want:       start,
		},
		{
			name:  "accepted events",
			pages: []page{{output: eventsPage(nil)}},
			want:  accepted,
		},
		{
			name:  "successful empty scan",
			pages: []page{{output: &cloudwatchlogs.FilterLogEventsOutput{}}},
			want:  end,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := newMockStorageClient()
			newReceiver := func(sink consumer.Logs, client *mockClient) *logsReceiver {
				r := newLogsReceiver(cfg, telemetry, sink)
				r.client = client
				r.cloudwatchCheckpointPersister = newCloudwatchCheckpointPersister(storage, zap.NewNop())
				return r
			}

			mc := &mockClient{}
			for _, p := range tc.pages {
				mc.On("FilterLogEvents", mock.Anything, startsAt(start), mock.Anything).Return(p.output, p.err).Once()
			}
			rcvr := newReceiver(consumertest.NewErr(tc.consumeErr), mc)

			err := rcvr.poll(t.Context())
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			checkpoint, err := rcvr.cloudwatchCheckpointPersister.GetCheckpoint(t.Context(), group)
			require.NoError(t, err)
			require.Equal(t, tc.want.Format(time.RFC3339Nano), checkpoint)
			require.True(t, tc.want.Equal(rcvr.groupNextStartTimes[group]))
			mc.AssertExpectations(t)

			resumeClient := &mockClient{}
			resumeClient.On("FilterLogEvents", mock.Anything, startsAt(tc.want), mock.Anything).
				Return(&cloudwatchlogs.FilterLogEventsOutput{}, nil).Once()
			require.NoError(t, newReceiver(consumertest.NewNop(), resumeClient).poll(t.Context()))
			resumeClient.AssertExpectations(t)
		})
	}
}
