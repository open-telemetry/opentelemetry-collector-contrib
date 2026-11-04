// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sleaderelectortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

// countingReceiver records how often it was started and shut down.
type countingReceiver struct {
	starts    atomic.Int64
	shutdowns atomic.Int64
}

func (c *countingReceiver) Start(context.Context, component.Host) error {
	c.starts.Add(1)
	return nil
}

func (c *countingReceiver) Shutdown(context.Context) error {
	c.shutdowns.Add(1)
	return nil
}

var _ receiver.Metrics = (*countingReceiver)(nil)

func TestMetricsControllerStartsOnlyOnceLeadershipIsAcquired(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	inner := &countingReceiver{}

	r := newLeaderElectedMetrics(
		component.MustNewID("k8s_leader_elector"),
		zap.NewNop(),
		func() (receiver.Metrics, error) { return inner, nil },
	)

	require.NoError(t, r.Start(t.Context(), host))
	require.Zero(t, inner.starts.Load(), "controller must not scrape before leadership is acquired")

	le.InvokeOnLeading()
	require.Equal(t, int64(1), inner.starts.Load(), "controller must start once leadership is acquired")
}

func newLeaderElectedLogsReceiver(t *testing.T, cfg *Config, sink *consumertest.LogsSink) *logsReceiver {
	t.Helper()
	rcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, sink)
	rcvr.client = defaultMockClient()
	return rcvr
}

func leaderElectedLogsConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-1"
	cfg.Logs.PollInterval = 10 * time.Millisecond
	cfg.Logs.Groups = GroupConfig{
		NamedConfigs: map[string]StreamConfig{
			testLogGroupName: {
				Names: []*string{&testLogStreamName},
			},
		},
	}
	electorID := component.MustNewID("k8s_leader_elector")
	cfg.K8sLeaderElector = &electorID
	return cfg
}

func TestLogsPollingStartsOnlyOnceLeadershipIsAcquired(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	sink := &consumertest.LogsSink{}
	rcvr := newLeaderElectedLogsReceiver(t, leaderElectedLogsConfig(), sink)

	require.NoError(t, rcvr.Start(t.Context(), host))
	require.Never(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "receiver must not poll CloudWatch before leadership is acquired")

	le.InvokeOnLeading()
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, rcvr.Shutdown(t.Context()))
}

func TestLogsPollingResumesAfterLeadershipIsRegained(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	sink := &consumertest.LogsSink{}
	rcvr := newLeaderElectedLogsReceiver(t, leaderElectedLogsConfig(), sink)

	require.NoError(t, rcvr.Start(t.Context(), host))

	le.InvokeOnLeading()
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	le.InvokeOnStopping()
	afterLoss := sink.LogRecordCount()

	le.InvokeOnLeading()
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > afterLoss
	}, 2*time.Second, 10*time.Millisecond, "receiver must poll again once leadership is regained")

	require.NoError(t, rcvr.Shutdown(t.Context()))
}

func TestMetricsReceiverFailsOnUnknownLeaderElector(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-2"
	electorID := component.MustNewID("k8s_leader_elector")
	cfg.K8sLeaderElector = &electorID

	rcvr, err := NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	err = rcvr.Start(t.Context(), componenttest.NewNopHost())
	require.ErrorContains(t, err, `unknown k8s leader elector "k8s_leader_elector"`)
}

func TestLogsReceiverFailsOnUnknownLeaderElector(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-2"
	electorID := component.MustNewID("k8s_leader_elector")
	cfg.K8sLeaderElector = &electorID

	rcvr, err := NewFactory().CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	err = rcvr.Start(t.Context(), componenttest.NewNopHost())
	require.ErrorContains(t, err, `unknown k8s leader elector "k8s_leader_elector"`)
}

func TestMetricsControllerIsRebuiltForEveryLeadershipTerm(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}

	var built []*countingReceiver
	r := newLeaderElectedMetrics(
		component.MustNewID("k8s_leader_elector"),
		zap.NewNop(),
		func() (receiver.Metrics, error) {
			c := &countingReceiver{}
			built = append(built, c)
			return c, nil
		},
	)

	require.NoError(t, r.Start(t.Context(), host))
	le.InvokeOnLeading()
	le.InvokeOnLeading() // a duplicate callback must not start a second controller
	le.InvokeOnStopping()
	le.InvokeOnLeading()
	require.NoError(t, r.Shutdown(t.Context()))

	require.Len(t, built, 2)
	require.Equal(t, int64(1), built[0].starts.Load())
	require.Equal(t, int64(1), built[0].shutdowns.Load())
	require.Equal(t, int64(1), built[1].starts.Load())
	require.Equal(t, int64(1), built[1].shutdowns.Load())
}

func TestShutdownWithoutEverBecomingLeader(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	sink := &consumertest.LogsSink{}
	rcvr := newLeaderElectedLogsReceiver(t, leaderElectedLogsConfig(), sink)

	require.NoError(t, rcvr.Start(t.Context(), host))
	require.NoError(t, rcvr.Shutdown(t.Context()))
	le.InvokeOnStopping() // losing a lease that was never held must stay harmless
}

// blockingClient keeps a FilterLogEvents call in flight until its context is cancelled.
type blockingClient struct {
	started chan struct{}
	once    sync.Once
}

func (*blockingClient) DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
}

func (c *blockingClient) FilterLogEvents(ctx context.Context, _ *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	c.once.Do(func() { close(c.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestStoppingCollectionDoesNotLogPollErrors(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	cfg := leaderElectedLogsConfig()
	cfg.K8sLeaderElector = nil

	client := &blockingClient{started: make(chan struct{})}
	sink := &consumertest.LogsSink{}
	rcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.New(core),
		},
	}, sink)
	rcvr.client = client

	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	<-client.started
	require.NoError(t, rcvr.Shutdown(t.Context()))

	require.Zero(t, logs.Len(), "canceling an in-flight poll is not an error: %v", logs.All())
}
