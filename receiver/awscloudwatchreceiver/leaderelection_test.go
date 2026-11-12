// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
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
	rcvr.cloudwatchCheckpointPersister = newCloudwatchCheckpointPersister(newMockStorageClient(), zap.NewNop())
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
	storageID := component.MustNewID("file_storage")
	cfg.StorageID = &storageID
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
	cfg.StorageID = nil

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

// exportingConsumer holds a batch the way a slow exporter does, and gives up only if the
// receiver cancels the batch's context.
type exportingConsumer struct {
	consumertest.LogsSink
	exporting chan struct{}
	release   chan struct{}
}

func (*exportingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *exportingConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	close(c.exporting)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.release:
		return c.LogsSink.ConsumeLogs(ctx, ld)
	}
}

func TestStoppingCollectionDeliversBatchInFlight(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	cfg := leaderElectedLogsConfig()
	cfg.K8sLeaderElector = nil
	cfg.StorageID = nil

	sink := &exportingConsumer{exporting: make(chan struct{}), release: make(chan struct{})}
	rcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.New(core),
		},
	}, sink)
	rcvr.client = &pagingClient{emitted: make(chan struct{})}

	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	<-sink.exporting

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- rcvr.Shutdown(t.Context()) }()
	require.Never(t, func() bool { return len(shutdownDone) > 0 }, 100*time.Millisecond, 10*time.Millisecond,
		"shutdown must wait for the batch being exported instead of canceling it")

	close(sink.release)
	require.NoError(t, <-shutdownDone)
	require.Equal(t, 1, sink.LogRecordCount())
	require.Zero(t, logs.Len(), "stopping collection must not fail the batch in flight: %v", logs.All())
}

func TestLogsStartRejectsLeaderElectionWithoutStorage(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	cfg := leaderElectedLogsConfig()
	cfg.StorageID = nil

	rcvr := newLeaderElectedLogsReceiver(t, cfg, &consumertest.LogsSink{})

	require.ErrorIs(t, rcvr.Start(t.Context(), host), errLeaderElectionWithoutStorage)
}

func TestLeadershipAfterShutdownDoesNotRestartPolling(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	sink := &consumertest.LogsSink{}
	rcvr := newLeaderElectedLogsReceiver(t, leaderElectedLogsConfig(), sink)

	require.NoError(t, rcvr.Start(t.Context(), host))
	require.NoError(t, rcvr.Shutdown(t.Context()))

	// The extension keeps callbacks registered, so leadership can still arrive after the
	// receiver has shut down and closed its checkpoint storage.
	le.InvokeOnLeading()
	require.Never(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "a shut-down receiver must not start polling again")
}

func TestMetricsLeadershipAfterShutdownDoesNotStartController(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	inner := &countingReceiver{}

	r := newLeaderElectedMetrics(
		component.MustNewID("k8s_leader_elector"),
		zap.NewNop(),
		func() (receiver.Metrics, error) { return inner, nil },
	)

	require.NoError(t, r.Start(t.Context(), host))
	require.NoError(t, r.Shutdown(t.Context()))

	// The extension keeps callbacks registered, so leadership can still arrive after the
	// receiver has shut down and the pipeline below it is gone.
	le.InvokeOnLeading()
	require.Zero(t, inner.starts.Load(), "a shut-down receiver must not start a controller")
}

// cancelAwareStorageClient refuses every operation once its context is cancelled, the way
// redis_storage does. The shared mock ignores the context, so it cannot catch this.
type cancelAwareStorageClient struct {
	*mockStorageClient
}

func (c *cancelAwareStorageClient) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.mockStorageClient.Get(ctx, key)
}

func (c *cancelAwareStorageClient) Set(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.mockStorageClient.Set(ctx, key, value)
}

// pagingClient returns one page of events and then blocks until the poll is cancelled, so
// cancellation lands after events were emitted but before the poll finishes.
type pagingClient struct {
	emitted chan struct{}
}

func (*pagingClient) DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
}

func (c *pagingClient) FilterLogEvents(ctx context.Context, _ *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	select {
	case <-c.emitted:
		<-ctx.Done()
		return nil, ctx.Err()
	default:
	}

	defer close(c.emitted)
	return &cloudwatchlogs.FilterLogEventsOutput{
		Events: []types.FilteredLogEvent{
			{
				EventId:       &testEventIDs[0],
				IngestionTime: aws.Int64(testIngestionTime),
				LogStreamName: aws.String(testLogStreamName),
				Message:       aws.String(testLogStreamMessage),
				Timestamp:     aws.Int64(testTimeStamp),
			},
		},
		NextToken: aws.String("next-page"),
	}, nil
}

func TestCheckpointIsPersistedWhenCollectionStops(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	sink := &consumertest.LogsSink{}

	storageClient := &cancelAwareStorageClient{mockStorageClient: newMockStorageClient()}
	rcvr := newLeaderElectedLogsReceiver(t, leaderElectedLogsConfig(), sink)
	rcvr.client = &pagingClient{emitted: make(chan struct{})}
	rcvr.cloudwatchCheckpointPersister = newCloudwatchCheckpointPersister(storageClient, zap.NewNop())

	require.NoError(t, rcvr.Start(t.Context(), host))
	le.InvokeOnLeading()
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	// The poll is parked on its next page. Losing the lease cancels it, and the events it
	// already emitted still have to be checkpointed.
	le.InvokeOnStopping()

	// GetCheckpoint reports the start of the stream when nothing was stored, so the value
	// has to be compared against the position the poll reached.
	checkpoint, err := rcvr.cloudwatchCheckpointPersister.GetCheckpoint(t.Context(), testLogGroupName)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(testTimeStamp+1).UTC().Format(time.RFC3339Nano), checkpoint,
		"events were emitted, so the next leader must not re-read them")

	require.NoError(t, rcvr.Shutdown(t.Context()))
}

func TestStartCollectingIgnoresCancelledLeadership(t *testing.T) {
	rcvr := newLeaderElectedLogsReceiver(t, leaderElectedLogsConfig(), &consumertest.LogsSink{})

	// client-go runs the start callback in its own goroutine, so a term can be over before
	// the callback runs. Such a callback must not start a polling loop.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rcvr.startCollecting(ctx)

	rcvr.mu.Lock()
	defer rcvr.mu.Unlock()
	require.Nil(t, rcvr.pollCancel, "a callback for an ended term must not start polling")
}

func TestStartLeadingIgnoresCancelledLeadership(t *testing.T) {
	inner := &countingReceiver{}
	r := newLeaderElectedMetrics(
		component.MustNewID("k8s_leader_elector"),
		zap.NewNop(),
		func() (receiver.Metrics, error) { return inner, nil },
	)
	r.host = componenttest.NewNopHost()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r.startLeading(ctx)

	require.Zero(t, inner.starts.Load(), "a callback for an ended term must not start a controller")
}

// cancellingStorageClient cancels the poll while the checkpoint is being read, the way a lease
// loss landing mid-poll does, and then fails the read like redis_storage would.
type cancellingStorageClient struct {
	*mockStorageClient
	cancel context.CancelFunc
}

func (c *cancellingStorageClient) Get(ctx context.Context, _ string) ([]byte, error) {
	c.cancel()
	return nil, ctx.Err()
}

func TestPollCutShortBeforeReadingCheckpointWritesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	store := newMockStorageClient()
	rcvr := newLeaderElectedLogsReceiver(t, leaderElectedLogsConfig(), &consumertest.LogsSink{})
	rcvr.cloudwatchCheckpointPersister = newCloudwatchCheckpointPersister(
		&cancellingStorageClient{mockStorageClient: store, cancel: cancel}, zap.NewNop(),
	)

	require.NoError(t, rcvr.poll(ctx))

	// The read failed, so the poll fell back to the configured start time and then fetched
	// nothing. Writing that position would move the shared checkpoint backwards.
	store.cacheMux.Lock()
	defer store.cacheMux.Unlock()
	require.Empty(t, store.cache, "a poll that made no progress must not write a checkpoint")
}
