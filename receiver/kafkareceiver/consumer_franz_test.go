// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func TestPartitionProcessing(t *testing.T) {
	t.Run("blocked partition", func(t *testing.T) {
		// A slow partition does not stop another partition.
		h := newPartitionProcessingHarness(t, 2, func(cfg *Config) {
			cfg.MessageMarking.After = true
		})
		// Hold partition 0 inside consume until the test ends.
		blocked := make(chan struct{})
		defer close(blocked)
		started := make(chan struct{}, 1)
		healthy := make(chan struct{}, 1)
		h.start(func(ctx context.Context, record *kgo.Record, _ attribute.Set) error {
			switch record.Partition {
			case 0:
				notify(started)
				select {
				case <-blocked:
					return nil
				case <-ctx.Done():
					return context.Cause(ctx)
				}
			case 1:
				notify(healthy)
			}
			return nil
		})

		// Pin partition 0 in consume before partition 1 receives work.
		h.produce(0, "blocked")
		waitSignal(t, started, "partition 0 did not start")
		h.produce(1, "healthy")
		waitSignal(t, healthy, "partition 1 was blocked by partition 0")
	})

	t.Run("rebalance", func(t *testing.T) {
		// Every produced record is processed after forced rebalances.
		const recordCount = 10
		h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
			cfg.ConsumerConfig.GroupRebalanceStrategies = []configkafka.GroupRebalanceStrategy{
				configkafka.RangeBalanceStrategy,
			}
		})
		var (
			mu   sync.Mutex
			seen = make(map[int64]struct{}, recordCount)
		)
		h.start(func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			mu.Lock()
			seen[record.Offset] = struct{}{}
			mu.Unlock()
			return nil
		})
		records := make([]*kgo.Record, recordCount)
		for i := range records {
			records[i] = &kgo.Record{Partition: 0}
		}
		h.produceRecords(records...)
		// ForceRebalance may revoke and reassign the partition while records
		// are still in the mailbox or in the worker.
		for range 3 {
			h.consumer.client.ForceRebalance()
			runtime.Gosched()
		}
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(seen) == recordCount
		}, 5*time.Second, 10*time.Millisecond)
	})

	t.Run("rewind control reports whether offset was applied", func(t *testing.T) {
		cases := []struct {
			name        string
			current     bool
			wantApplied bool
			wantPending bool
			wantPaused  bool
		}{
			{
				name:        "current partition",
				current:     true,
				wantApplied: true,
				wantPending: false,
				wantPaused:  false,
			},
			{
				name:        "stale partition",
				current:     false,
				wantApplied: false,
				wantPending: true,
				wantPaused:  true,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				client, _ := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
				tp := topicPartition{topic: "test", partition: 0}
				partitionConsumer := &pc{
					ctx:     t.Context(),
					mailbox: newPartitionMailbox(t.Context(), 1),
				}
				partitionConsumer.addPauseReason(partitionPauseRewind)
				partitionConsumer.mailbox.requestRewind(
					&kgo.Record{Topic: "test", Partition: 0, Offset: 1},
					true,
					func() {},
				)

				current := partitionConsumer
				if !tc.current {
					current = &pc{ctx: t.Context()}
				}
				consumer := franzConsumer{
					client:      client,
					assignments: map[topicPartition]*pc{tp: current},
					controls:    make(chan partitionControl, 1),
					closing:     make(chan struct{}),
				}

				applied := make(chan bool, 1)
				go func() {
					applied <- consumer.applyMailboxRewind(
						partitionConsumer,
						tp,
						map[string][]int32{"test": {0}},
					)
				}()

				control := <-consumer.controls
				consumer.opsMu.Lock()
				consumer.processControlLocked(control)
				consumer.opsMu.Unlock()

				require.Equal(t, tc.wantApplied, <-applied)
				require.Equal(t, tc.wantPending, partitionConsumer.mailbox.hasPendingOffsetChange())
				require.Equal(t, tc.wantPaused, partitionConsumer.pauseReasons.Load() != 0)
			})
		}
	})

	t.Run("ignores control after cancellation", func(t *testing.T) {
		// A cancelled partition does not apply a rewind.
		client, _ := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
		ctx, cancel := context.WithCancel(t.Context())
		partitionConsumer := &pc{ctx: ctx, mailbox: newPartitionMailbox(ctx, 1)}
		partitionConsumer.mailbox.requestRewind(&kgo.Record{Topic: "test", Partition: 0, Offset: 1}, true, func() {})
		// Cancel before the poll loop sees the control. SetOffsets must not run.
		cancel()
		tp := topicPartition{topic: "test", partition: 0}
		consumer := franzConsumer{
			client:      client,
			assignments: map[topicPartition]*pc{tp: partitionConsumer},
		}
		applied := make(chan bool, 1)
		consumer.opsMu.Lock()
		consumer.processControlLocked(partitionControl{tp: tp, pc: partitionConsumer, applied: applied})
		consumer.opsMu.Unlock()
		// The rewind is still pending, so processControlLocked did not take it.
		require.False(t, <-applied)
		require.NotNil(t, partitionConsumer.mailbox.takeOffsetChange())
	})

	t.Run("full mailbox", func(t *testing.T) {
		// A full mailbox pauses fetch. After drain, fetch resumes. Later
		// records are processed.
		h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
			// One record per fetch so the mailbox fills from later polls.
			cfg.ConsumerConfig.MaxFetchSize = 1
		})
		started := make(chan struct{}, 1)
		release := make(chan struct{})
		var processed atomic.Int64
		h.start(func(ctx context.Context, _ *kgo.Record, _ attribute.Set) error {
			notify(started)
			select {
			case <-release:
			case <-ctx.Done():
			}
			processed.Add(1)
			return nil
		})
		h.produce(0, "first")
		waitSignal(t, started, "first record did not start")
		// Capacity is 1. The first record occupies the worker, so the next
		// fetches fill the mailbox and pause the partition.
		h.produce(0, "second")
		h.produce(0, "third")
		h.waitPaused(1)
		close(release)
		waitAtomic(t, &processed, 3)
		h.waitPaused(0)
		h.produce(0, "after-resume")
		waitAtomic(t, &processed, 4)
	})

	t.Run("transient retry", func(t *testing.T) {
		// A transient error rewinds the partition until processing succeeds.
		h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
			cfg.MessageMarking = MessageMarking{After: true, OnError: false}
			cfg.ErrorBackOff = configretry.BackOffConfig{
				Enabled:             true,
				InitialInterval:     10 * time.Millisecond,
				MaxInterval:         20 * time.Millisecond,
				MaxElapsedTime:      50 * time.Millisecond,
				RandomizationFactor: 0,
				Multiplier:          1,
			}
		})
		var first atomic.Pointer[kgo.Record]
		var processed atomic.Int64
		h.start(func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			// Inner backoff retries the same *kgo.Record. A later PollRecords
			// fetch allocates a new record for the same offset. Franz-go does
			// not pool these pointers. If it did, waitAtomic would time out.
			if first.CompareAndSwap(nil, record) || first.Load() == record {
				return errors.New("transient failure")
			}
			processed.Add(1)
			return nil
		})
		h.produce(0, "retry")
		waitAtomic(t, &processed, 1)
	})

	t.Run("terminal error clears mailbox", func(t *testing.T) {
		// A permanent error discards queued batches and pauses fetch. Later
		// records are not processed.
		h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
			cfg.ConsumerConfig.MaxFetchSize = 1
			cfg.PartitionProcessing.MaxBufferedBatches = 3
			cfg.MessageMarking = MessageMarking{After: true, OnPermanentError: false}
		})
		started := make(chan struct{}, 1)
		release := make(chan struct{})
		var attempts atomic.Int64
		h.start(func(ctx context.Context, _ *kgo.Record, _ attribute.Set) error {
			attempts.Add(1)
			notify(started)
			select {
			case <-release:
			case <-ctx.Done():
				return context.Cause(ctx)
			}
			return consumererror.NewPermanent(errors.New("permanent failure"))
		})
		pc := h.assignment(0)
		h.produce(0, "first")
		waitSignal(t, started, "first record did not start")
		// Queue more batches while the first record is still in consume.
		h.produce(0, "second")
		h.produce(0, "third")
		require.Eventually(t, func() bool {
			return queued(t, pc.mailbox) > 0
		}, 5*time.Second, 10*time.Millisecond)
		// The worker returns a permanent error, discards the queued batches,
		// cancels the partition, and leaves fetch paused.
		close(release)
		h.waitPaused(1)
		require.Eventually(t, func() bool {
			return queued(t, pc.mailbox) == 0
		}, 5*time.Second, 10*time.Millisecond)
		require.Eventually(t, func() bool {
			return pc.ctx.Err() != nil
		}, 5*time.Second, 10*time.Millisecond)
		h.produce(0, "still paused")
		require.Never(t, func() bool {
			return attempts.Load() > 1
		}, 500*time.Millisecond, 10*time.Millisecond)
	})
}

type partitionProcessingHarness struct {
	t          *testing.T
	topic      string
	producer   *kgo.Client
	cfg        *Config
	consumer   *franzConsumer
	partitions int
}

func newPartitionProcessingHarness(t *testing.T, partitions int, configure func(*Config)) *partitionProcessingHarness {
	t.Helper()

	const topic = "otlp_spans"
	cluster, clientConfig := kafkatest.NewCluster(t, kfake.SeedTopics(int32(partitions), topic))
	kafkaClient := mustNewClient(t, cluster)
	t.Cleanup(func() { deleteConsumerGroups(t, kafkaClient) })
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = clientConfig
	cfg.ConsumerConfig.InitialOffset = "earliest"
	cfg.ConsumerConfig.MaxFetchWait = 10 * time.Millisecond
	cfg.ConsumerConfig.GroupID = t.Name()
	// These tests cover independent workers only.
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}
	if configure != nil {
		configure(cfg)
	}

	producer, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.ClientConfig.Brokers...),
		kgo.RecordPartitioner(kgo.ManualPartitioner()),
	)
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	return &partitionProcessingHarness{
		t:          t,
		topic:      topic,
		producer:   producer,
		cfg:        cfg,
		partitions: partitions,
	}
}

func (h *partitionProcessingHarness) start(consume func(context.Context, *kgo.Record, attribute.Set) error) {
	h.t.Helper()

	settings, _, _ := mustNewSettings(h.t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return consumeMessageFunc(consume), nil
	}
	consumer, err := newFranzKafkaConsumer(h.cfg, settings, []string{h.topic}, nil, consumeFn)
	require.NoError(h.t, err)
	require.NoError(h.t, consumer.Start(h.t.Context(), componenttest.NewNopHost()))
	h.consumer = consumer
	h.t.Cleanup(func() {
		// Cleanup runs after t.Context() is canceled. Shutdown needs a live context.
		require.NoError(h.t, h.consumer.Shutdown(context.Background()))
	})
	h.waitAssignments(h.partitions)
}

func (h *partitionProcessingHarness) produce(partition int32, value string) {
	h.t.Helper()
	h.produceRecords(&kgo.Record{
		Topic:     h.topic,
		Partition: partition,
		Value:     []byte(value),
	})
}

func (h *partitionProcessingHarness) produceRecords(records ...*kgo.Record) {
	h.t.Helper()
	for _, record := range records {
		if record.Topic == "" {
			record.Topic = h.topic
		}
	}
	require.NoError(h.t, h.producer.ProduceSync(h.t.Context(), records...).FirstErr())
}

func (h *partitionProcessingHarness) waitAssignments(want int) {
	h.t.Helper()
	require.Eventually(h.t, func() bool {
		h.consumer.mu.RLock()
		defer h.consumer.mu.RUnlock()
		return len(h.consumer.assignments) == want
	}, 5*time.Second, 10*time.Millisecond)
}

func (h *partitionProcessingHarness) waitPaused(want int) {
	h.t.Helper()
	require.Eventually(h.t, func() bool {
		// PauseFetchPartitions(nil) returns the current paused set without pausing more.
		paused := h.consumer.client.PauseFetchPartitions(nil)
		return len(paused[h.topic]) == want
	}, 5*time.Second, 10*time.Millisecond)
}

func (h *partitionProcessingHarness) assignment(partition int32) *pc {
	h.t.Helper()
	tp := topicPartition{topic: h.topic, partition: partition}
	h.consumer.mu.RLock()
	defer h.consumer.mu.RUnlock()
	partitionConsumer := h.consumer.assignments[tp]
	require.NotNil(h.t, partitionConsumer)
	return partitionConsumer
}

func waitSignal(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond, msg)
}

func newStaleWorkerConsumer(t *testing.T, kafkaClient *kgo.Client, cfg *Config, topic string) *franzConsumer {
	t.Helper()
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 2,
	}
	cfg.ConsumerConfig.SessionTimeout = 20 * time.Millisecond
	cfg.MessageMarking.After = true
	settings, _, _ := mustNewSettings(t)
	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, nil)
	require.NoError(t, err)
	consumer.client = kafkaClient
	return consumer
}

func runStaleWorkerReplacement(
	t *testing.T,
	consumer *franzConsumer,
	client *kgo.Client,
	topic string,
	batch kgo.FetchTopicPartition,
	consumeErr error,
	onEnqueue func(*pc, partitionPauseReason),
	afterReplacement func(),
) {
	t.Helper()
	processingStarted := make(chan struct{})
	releaseProcessing := make(chan struct{})
	consumer.consumeMessage = func(context.Context, *kgo.Record, attribute.Set) error {
		close(processingStarted)
		<-releaseProcessing
		return consumeErr
	}

	assignmentCtx, cancelAssignment := context.WithCancel(t.Context())
	defer cancelAssignment()
	partitions := map[string][]int32{topic: {0}}
	consumer.assigned(assignmentCtx, client, partitions)
	stale := consumer.assignments[topicPartition{topic: topic, partition: 0}]
	if onEnqueue == nil {
		onEnqueue = func(*pc, partitionPauseReason) {}
	}
	require.True(t, stale.mailbox.enqueue(batch, func(reason partitionPauseReason) {
		onEnqueue(stale, reason)
	}))
	waitSignal(t, processingStarted, "stale worker did not start processing")

	consumer.lost(t.Context(), nil, partitions, true)
	consumer.assigned(assignmentCtx, client, partitions)
	require.NotSame(t, stale, consumer.assignments[topicPartition{topic: topic, partition: 0}])
	if afterReplacement != nil {
		afterReplacement()
	}

	staleDone := make(chan struct{})
	go func() {
		stale.wg.Wait()
		close(staleDone)
	}()
	close(releaseProcessing)
	waitSignal(t, staleDone, "stale worker did not stop")
}

func waitAtomic(t *testing.T, got *atomic.Int64, want int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		return got.Load() == want
	}, 5*time.Second, 10*time.Millisecond)
}

func notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func TestConsumerShutdownConsuming(t *testing.T) {
	type tCfg struct {
		mark        MessageMarking
		backOff     configretry.BackOffConfig
		returnError bool
	}
	type assertions struct {
		firstBatchProcessedCount  int64
		secondBatchProcessedCount int64
		committedOffset           int64
	}
	type testCase struct {
		name       string
		testConfig tCfg
		want       assertions
	}
	testCases := []testCase{
		{
			name:       "BackOff default marking",
			testConfig: tCfg{MessageMarking{}, configretry.NewDefaultBackOffConfig(), false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "NoBackoff default marking",
			testConfig: tCfg{MessageMarking{}, configretry.BackOffConfig{Enabled: false}, false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "BackOff default marking with error",
			testConfig: tCfg{MessageMarking{}, configretry.NewDefaultBackOffConfig(), true},
			want: assertions{
				firstBatchProcessedCount:  1,
				secondBatchProcessedCount: 2,
				committedOffset:           2,
			},
		},
		{
			name:       "NoBackoff default marking with error",
			testConfig: tCfg{MessageMarking{}, configretry.BackOffConfig{Enabled: false}, true},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "BackOff after marking",
			testConfig: tCfg{MessageMarking{After: true}, configretry.NewDefaultBackOffConfig(), false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "NoBackoff after marking",
			testConfig: tCfg{MessageMarking{After: true}, configretry.BackOffConfig{Enabled: false}, false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		// With error
		{
			name:       "BackOff after marking with error",
			testConfig: tCfg{MessageMarking{After: true}, configretry.NewDefaultBackOffConfig(), true},
			want: assertions{
				firstBatchProcessedCount:  1,
				secondBatchProcessedCount: 2,
				committedOffset:           0,
			},
		},
		{
			name:       "NoBackoff after marking with error",
			testConfig: tCfg{MessageMarking{After: true}, configretry.BackOffConfig{Enabled: false}, true},
			want: assertions{
				firstBatchProcessedCount:  1,
				secondBatchProcessedCount: 2,
				committedOffset:           0,
			},
		},
		// WithError OnError=true
		{
			name:       "BackOff after marking with error and OnError=true",
			testConfig: tCfg{MessageMarking{After: true, OnError: true}, configretry.NewDefaultBackOffConfig(), true},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "NoBackoff after marking with error and OnError=true",
			testConfig: tCfg{MessageMarking{After: true, OnError: true}, configretry.BackOffConfig{Enabled: false}, true},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
	}

	// Create some traces for sending to the otlp_spans topic.
	const topic = "otlp_spans"
	traces := testdata.GenerateTraces(5)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	rs := []*kgo.Record{
		{Topic: topic, Value: data},
		{Topic: topic, Value: data},
	}

	testShutdown := func(tb testing.TB, testConfig tCfg, want assertions) {
		// Test that the consumer shuts down while consuming a message and
		// commits the offset after it's left the group.

		kafkaClient, cfg := mustNewFakeCluster(tb, kfake.SeedTopics(1, topic))
		cfg.ConsumerConfig.GroupID = tb.Name()
		cfg.ConsumerConfig.AutoCommit = configkafka.AutoCommitConfig{Enable: true, Interval: 10 * time.Second}
		// Set MinFetchSize to ensure all records are fetched at once
		cfg.ConsumerConfig.MinFetchSize = int32(len(data) * len(rs))
		// Use a very short MaxFetchWait to avoid delays when MinFetchSize cannot be met
		cfg.ConsumerConfig.MaxFetchWait = 10 * time.Millisecond
		cfg.ErrorBackOff = testConfig.backOff
		cfg.MessageMarking = testConfig.mark

		var called atomic.Int64
		var wg sync.WaitGroup
		settings, _, _ := mustNewSettings(tb)
		newConsumeFunc := func() (newConsumeMessageFunc, chan<- struct{}) {
			consuming := make(chan struct{})
			return func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
				return func(ctx context.Context, _ *kgo.Record, _ attribute.Set) error {
					wg.Add(1)
					defer wg.Done()

					<-consuming
					called.Add(1)
					// Wait for the consumer to shutdown.
					<-ctx.Done()
					if testConfig.returnError {
						return errors.New("error")
					}
					return nil
				}, nil
			}, consuming
		}

		test := func(tb testing.TB, expected int64) {
			ctx := t.Context()
			consumeFn, consuming := newConsumeFunc()
			consumer, e := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
			require.NoError(tb, e)
			require.NoError(tb, consumer.Start(ctx, componenttest.NewNopHost()))
			require.NoError(tb, kafkaClient.ProduceSync(ctx, rs...).FirstErr())

			// Use longer timeout on Windows due to tick granularity and slower CI
			timeout := 2 * time.Second
			if runtime.GOOS == "windows" {
				timeout = 5 * time.Second
			}

			select {
			case consuming <- struct{}{}:
				close(consuming) // Close the channel so the rest exit.
			case <-time.After(timeout):
				tb.Fatal("expected to consume a message")
			}

			require.NoError(tb, consumer.Shutdown(ctx))
			wg.Wait() // Wait for the consume functions to exit.
			// Ensure that the consume function was called twice.
			assert.Equal(tb, expected, called.Load(), "consume function processed calls mismatch")
		}

		test(tb, want.firstBatchProcessedCount)
		test(tb, want.secondBatchProcessedCount)

		offsets, err := kadm.NewClient(kafkaClient).FetchOffsets(t.Context(), tb.Name())
		require.NoError(tb, err)
		// Lookup the last committed offset for partition 0
		offset, _ := offsets.Lookup(topic, 0)
		assert.Equal(tb, want.committedOffset, offset.At)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testShutdown(t, tc.testConfig, tc.want)
		})
	}
}

func TestConsumerShutdownNotStarted(t *testing.T) {
	_, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			return nil
		}, nil
	}
	c, err := newFranzKafkaConsumer(cfg, settings, []string{"test"}, nil, consumeFn)
	require.NoError(t, err)

	for range 2 {
		require.NoError(t, c.Shutdown(t.Context()))
	}

	// Verify internal signal that there's nothing to shut down.
	// (Same package, so we can call the unexported helper.)
	require.False(t, c.triggerShutdown(), "triggerShutdown should indicate no-op when never started")
}

// TestRaceLostVsConsume verifies no data race occurs between concurrent
// message processing (which calls pc.add / pc.done) and partition revocation
// handling (lost() → pc.wait). It spins up a kfake cluster, floods them with
// records, and repeatedly invokes lost() while consumption is in-flight.
func TestRaceLostVsConsume(t *testing.T) {
	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.ConsumerConfig.GroupID = t.Name()
	cfg.ConsumerConfig.MaxFetchSize = 1 // Force a lot of iterations of consume()
	cfg.ConsumerConfig.AutoCommit = configkafka.AutoCommitConfig{
		Enable: true, Interval: 100 * time.Millisecond,
	}

	// Produce records.
	var rs []*kgo.Record
	for range 500 {
		traces := testdata.GenerateTraces(5)
		data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
		require.NoError(t, err)
		rs = append(rs, &kgo.Record{Topic: topic, Value: data})
	}
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), rs...).FirstErr())
	settings, _, _ := mustNewSettings(t)

	// Noop consume function.
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(context.Context, *kgo.Record, attribute.Set) error {
			return nil
		}, nil
	}

	c, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))

	done := make(chan struct{})
	// Hammer lost/assigned and rebalance in a goroutine.
	go func() {
		defer close(done)
		topicMap := map[string][]int32{topic: {0}}
		for range 2000 {
			c.lost(t.Context(), nil, topicMap, false)
			c.assigned(t.Context(), kafkaClient, topicMap)
			c.client.ForceRebalance()
		}
	}()

	<-done
	require.NoError(t, c.Shutdown(t.Context()))
}

func TestLost(t *testing.T) {
	// It is possible that lost is called multiple times for the same partition
	// or called with a topic/partition that hasn't been assigned. This test
	// ensures that `lost` works without error in both cases.
	_, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
	settings, _, _ := mustNewSettings(t)

	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			return nil
		}, nil
	}
	c, err := newFranzKafkaConsumer(cfg, settings, []string{"test"}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, c.Shutdown(t.Context())) }()

	// Call lost couple of times for same partition
	lostM := map[string][]int32{"test": {0}}
	c.lost(t.Context(), nil, lostM, false)
	c.lost(t.Context(), nil, lostM, false)

	// Call lost for a topic and partition that was not assigned
	c.lost(t.Context(), nil, map[string][]int32{"404": {0}}, true)
}

func TestLostFatalWait(t *testing.T) {
	cases := []struct {
		name           string
		independent    bool
		sessionTimeout time.Duration
		wantWait       bool
	}{
		{
			name: "legacy returns immediately",
		},
		{
			name:        "independent waits for worker",
			independent: true,
			wantWait:    true,
		},
		{
			name:           "independent wait is bounded",
			independent:    true,
			sessionTimeout: 20 * time.Millisecond,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
			cfg.PartitionProcessing.Independent = tc.independent
			if tc.sessionTimeout > 0 {
				cfg.ConsumerConfig.SessionTimeout = tc.sessionTimeout
			}
			settings, _, _ := mustNewSettings(t)
			c, err := newFranzKafkaConsumer(cfg, settings, []string{"test"}, nil, nil)
			require.NoError(t, err)

			ctx, cancel := context.WithCancelCause(t.Context())
			partitionConsumer := &pc{
				ctx:    ctx,
				cancel: cancel,
			}
			partitionConsumer.wg.Add(1)
			c.assignments[topicPartition{topic: "test", partition: 0}] = partitionConsumer

			done := make(chan struct{})
			go func() {
				defer close(done)
				c.lost(t.Context(), nil, map[string][]int32{"test": {0}}, true)
			}()
			t.Cleanup(func() {
				partitionConsumer.wg.Done()
				<-done
			})
			// lost() cancels the partition before it waits or returns.
			require.Eventually(t, func() bool {
				return partitionConsumer.ctx.Err() != nil
			}, 5*time.Second, 10*time.Millisecond)

			if tc.wantWait {
				require.Never(t, func() bool {
					select {
					case <-done:
						return true
					default:
						return false
					}
				}, 200*time.Millisecond, 10*time.Millisecond)
			} else {
				select {
				case <-done:
				case <-time.After(200 * time.Millisecond):
					t.Fatal("fatal partition loss exceeded its wait bound")
				}
			}
		})
	}
}

func TestStaleWorkerDoesNotMutateReplacement(t *testing.T) {
	// A worker that ignores cancellation can still run after fatal-loss timeout.
	// A replacement then owns the same topic-partition. The stale worker must
	// not pause fetch or mark commits for that partition.
	const topic = "test"

	t.Run("pause", func(t *testing.T) {
		kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
		cfg.MessageMarking.OnPermanentError = false
		consumer := newStaleWorkerConsumer(t, kafkaClient, cfg, topic)
		batch := mailboxBatch(0)
		batch.Topic = topic
		partitions := map[string][]int32{topic: {0}}
		runStaleWorkerReplacement(
			t,
			consumer,
			kafkaClient,
			topic,
			batch,
			consumererror.NewPermanent(errors.New("permanent failure")),
			func(stale *pc, reason partitionPauseReason) {
				stale.addPauseReason(reason)
				kafkaClient.PauseFetchPartitions(partitions)
			},
			func() {
				require.Empty(t, kafkaClient.PauseFetchPartitions(nil)[topic])
			},
		)
		require.Empty(t, kafkaClient.PauseFetchPartitions(nil)[topic])
	})

	t.Run("mark", func(t *testing.T) {
		kafkaClient, cfg := mustNewMarkedFakeCluster(t, kfake.SeedTopics(1, topic))
		consumer := newStaleWorkerConsumer(t, kafkaClient, cfg, topic)
		batch := mailboxBatch(10)
		batch.Topic = topic
		batch.Records[0].Topic = topic
		batch.Records[0].Partition = 0
		runStaleWorkerReplacement(t, consumer, kafkaClient, topic, batch, nil, nil, func() {
			require.Empty(t, kafkaClient.MarkedOffsets()[topic])
		})
		require.Empty(t, kafkaClient.MarkedOffsets()[topic])
	})
}

func TestLostDiscardsQueuedBatches(t *testing.T) {
	// Fatal loss may return while a worker is still inside consumeMessage.
	// Queued batches must be dropped at cancel time, not when that worker exits.
	const topic = "test"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 2,
	}
	cfg.ConsumerConfig.SessionTimeout = 20 * time.Millisecond

	settings, _, _ := mustNewSettings(t)
	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, nil)
	require.NoError(t, err)
	consumer.client = kafkaClient

	processingStarted := make(chan struct{})
	releaseProcessing := make(chan struct{})
	consumer.consumeMessage = func(context.Context, *kgo.Record, attribute.Set) error {
		close(processingStarted)
		<-releaseProcessing
		return nil
	}

	assignmentCtx, cancelAssignment := context.WithCancel(t.Context())
	defer cancelAssignment()
	partitions := map[string][]int32{topic: {0}}
	consumer.assigned(assignmentCtx, kafkaClient, partitions)
	pc := consumer.assignments[topicPartition{topic: topic, partition: 0}]

	inFlight := mailboxBatch(0)
	inFlight.Topic = topic
	require.True(t, pc.mailbox.enqueue(inFlight, func(partitionPauseReason) {}))
	waitSignal(t, processingStarted, "worker did not start processing")

	queued := mailboxBatch(1)
	queued.Topic = topic
	require.True(t, pc.mailbox.enqueue(queued, func(partitionPauseReason) {}))

	consumer.lost(t.Context(), nil, partitions, true)

	_, ok := pc.mailbox.dequeue(func() {})
	require.False(t, ok)

	close(releaseProcessing)
	staleDone := make(chan struct{})
	go func() {
		pc.wg.Wait()
		close(staleDone)
	}()
	waitSignal(t, staleDone, "worker did not stop")
}

// TestResumePartitionsAfterRebalance verifies that partitions paused due to
// processing errors are resumed when they are reassigned after a rebalance.
func TestResumePartitionsAfterRebalance(t *testing.T) {
	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.ConsumerConfig.GroupID = t.Name()
	cfg.MessageMarking = MessageMarking{
		After:   true,
		OnError: false, // errors are NOT marked -> triggers PauseFetchPartitions
	}
	cfg.ErrorBackOff = configretry.BackOffConfig{Enabled: false}

	var (
		consumeCount atomic.Int64
		shouldError  atomic.Bool
		errored      = make(chan struct{}, 1)
	)
	shouldError.Store(true)

	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			if shouldError.Load() {
				select {
				case errored <- struct{}{}:
				default:
				}
				return errors.New("simulated processing error")
			}
			consumeCount.Add(1)
			return nil
		}, nil
	}

	c, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, c.Shutdown(t.Context())) }()

	// Produce a record to trigger the error path.
	traces := testdata.GenerateTraces(1)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic, Value: data,
	}).FirstErr())

	// Wait for the consume function to error at least once.
	select {
	case <-errored:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consume error")
	}

	// Wait for PauseFetchPartitions to be called after the error
	// propagates through handleMessage -> the consume loop.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		paused := c.client.PauseFetchPartitions(nil)
		assert.NotEmpty(ct, paused, "expected partition to be paused")
	}, 5*time.Second, 10*time.Millisecond)

	// Switch to success mode and simulate a cooperative-sticky rebalance:
	// the partition is lost (revoked) and then reassigned to the same consumer.
	shouldError.Store(false)
	partitions := map[string][]int32{topic: {0}}
	c.lost(t.Context(), nil, partitions, false)
	c.assigned(t.Context(), c.client, partitions)

	// Produce new records after the resume. The client's internal fetch offset
	// has already advanced past the error record (offset 0), so we need fresh
	// records at offset 1+ for the consumer to pick up.
	// Without the fix, the partition stays paused and these records are never consumed.
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic, Value: data,
	}).FirstErr())

	assert.Eventually(t, func() bool {
		return consumeCount.Load() == 1
	}, 5*time.Second, 50*time.Millisecond,
		"expected partition to resume consuming after rebalance, but it stayed paused")
}

// TestResumePartitionsAfterBackoff verifies that when a non-permanent error
// occurs with message_marking.after=true and message_marking.on_error=false,
// SetOffsets rewinds the fetch cursor to the failed record after inner retries
// are exhausted. On the next PollRecords call, the same record is retried,
// consistent with how a rebalance restarts from the last committed offset.
func TestResumePartitionsAfterBackoff(t *testing.T) {
	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.ConsumerConfig.GroupID = t.Name()
	cfg.MessageMarking = MessageMarking{
		After:   true,
		OnError: false, // errors are NOT marked -> triggers SetOffsets rewind
	}
	cfg.ErrorBackOff = configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      100 * time.Millisecond, // exhaust inner retries quickly
		RandomizationFactor: 0,                      // deterministic for testing
		Multiplier:          1.5,
	}

	var (
		consumeCount atomic.Int64
		shouldError  atomic.Bool
		errored      = make(chan struct{}, 1)
	)
	shouldError.Store(true)

	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			if shouldError.Load() {
				select {
				case errored <- struct{}{}:
				default:
				}
				return errors.New("simulated transient error")
			}
			consumeCount.Add(1)
			return nil
		}, nil
	}

	c, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, c.Shutdown(t.Context())) }()

	// Produce a single record to trigger the error -> rewind -> retry cycle.
	traces := testdata.GenerateTraces(1)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic, Value: data,
	}).FirstErr())

	// Wait for the consume function to error at least once.
	select {
	case <-errored:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consume error")
	}

	// Switch to success mode. After inner retries exhaust (MaxElapsedTime),
	// SetOffsets rewinds the cursor to the failed record. The next
	// PollRecords call retries it successfully.
	shouldError.Store(false)

	assert.Eventually(t, func() bool {
		return consumeCount.Load() == 1
	}, 5*time.Second, 50*time.Millisecond,
		"expected the failed record to be retried after SetOffsets rewind, but it was not")
}

// TestNoResumePartitionsAfterPermanentError verifies that when a permanent
// error occurs, the partition is paused but NOT automatically resumed even
// with error_backoff enabled. This is consistent with handleMessage which
// only retries non-permanent errors.
func TestNoResumePartitionsAfterPermanentError(t *testing.T) {
	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.ConsumerConfig.GroupID = t.Name()
	cfg.MessageMarking = MessageMarking{
		After:            true,
		OnError:          false,
		OnPermanentError: false, // permanent errors are NOT marked -> triggers PauseFetchPartitions
	}
	cfg.ErrorBackOff = configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      5 * time.Second,
		RandomizationFactor: 0,
		Multiplier:          1.5,
	}

	var (
		consumeCount atomic.Int64
		errored      = make(chan struct{}, 1)
	)

	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			consumeCount.Add(1)
			select {
			case errored <- struct{}{}:
			default:
			}
			// Always return a permanent error. handleMessage won't retry it,
			// and the outer layer should NOT resume the partition.
			return consumererror.NewPermanent(errors.New("simulated permanent error"))
		}, nil
	}

	c, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, c.Shutdown(t.Context())) }()

	// Produce a record to trigger the permanent error -> pause path.
	traces := testdata.GenerateTraces(1)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic, Value: data,
	}).FirstErr())

	// Wait for the consume function to error.
	select {
	case <-errored:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consume error")
	}

	// Wait for PauseFetchPartitions to be called after the permanent error.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		paused := c.client.PauseFetchPartitions(nil)
		assert.NotEmpty(ct, paused, "expected partition to be paused")
	}, 5*time.Second, 10*time.Millisecond)

	// Produce another record. If the partition were incorrectly resumed,
	// this would be consumed. With the fix, it stays paused.
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic, Value: data,
	}).FirstErr())

	// Partition is confirmed paused, so the record cannot be consumed.
	assert.Equal(t, int64(1), consumeCount.Load(),
		"expected partition to remain paused after permanent error, but additional records were consumed")
}

func TestFranzConsumer_UseLeaderEpoch_Smoke(t *testing.T) {
	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.ClientConfig.UseLeaderEpoch = false // <-- exercise the option
	cfg.ConsumerConfig.GroupID = t.Name()
	cfg.ConsumerConfig.AutoCommit = configkafka.AutoCommitConfig{Enable: true, Interval: 100 * time.Millisecond}

	var called atomic.Int64
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			called.Add(1)
			return nil
		}, nil
	}

	// produce a couple of records
	traces := testdata.GenerateTraces(5)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	rs := []*kgo.Record{
		{Topic: topic, Value: data},
		{Topic: topic, Value: data},
	}

	c, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), rs...).FirstErr())

	// wait briefly for consumption
	deadline := time.After(2 * time.Second)
	for called.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("expected to consume 2 records, got %d", called.Load())
		case <-time.After(25 * time.Millisecond):
		}
	}

	require.NoError(t, c.Shutdown(t.Context()))
}

func TestMakeUseLeaderEpochAdjuster_ClearsEpoch(t *testing.T) {
	adj := makeClearLeaderEpochAdjuster()

	input := map[string]map[int32]kgo.Offset{
		"t": {
			0: kgo.NewOffset().At(42).WithEpoch(7),
			1: kgo.NewOffset().At(100), // no epoch set
		},
	}
	out, err := adj(t.Context(), input)
	require.NoError(t, err)

	require.Equal(t, kgo.NewOffset().At(42).WithEpoch(-1), out["t"][0])
	require.Equal(t, kgo.NewOffset().At(100).WithEpoch(-1), out["t"][1])
}

// TestExcludeTopicWithRegex tests that exclude_topic works correctly with regex topic patterns.
// It creates three topics (logs-a, logs-b, logs-c) matching the pattern ^logs-.*
// and excludes logs-a and logs-b using ^logs-(a|b)$, expecting only logs-c to be consumed.
func TestExcludeTopicWithRegex(t *testing.T) {
	// Create three topics: logs-a, logs-b, logs-c
	kafkaClient, cfg := mustNewFakeCluster(t,
		kfake.SeedTopics(1, "logs-a"),
		kfake.SeedTopics(1, "logs-b"),
		kfake.SeedTopics(1, "logs-c"),
	)
	cfg.ConsumerConfig.GroupID = t.Name()
	cfg.ConsumerConfig.AutoCommit = configkafka.AutoCommitConfig{Enable: true, Interval: 100 * time.Millisecond}

	// Prepare test data
	traces := testdata.GenerateTraces(5)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)

	// Produce records to all three topics
	rs := []*kgo.Record{
		{Topic: "logs-a", Value: data},
		{Topic: "logs-b", Value: data},
		{Topic: "logs-c", Value: data},
	}
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), rs...).FirstErr())

	// Track which topics were consumed
	consumedTopics := make(map[string]int)
	var mu sync.Mutex
	var called atomic.Int64

	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			mu.Lock()
			consumedTopics[record.Topic]++
			mu.Unlock()
			called.Add(1)
			return nil
		}, nil
	}

	// Create consumer with regex topic pattern and exclude pattern
	c, err := newFranzKafkaConsumer(
		cfg,
		settings,
		[]string{"^logs-.*"},     // Match all logs-* topics
		[]string{"^logs-(a|b)$"}, // Exclude logs-a and logs-b
		consumeFn,
	)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, c.Shutdown(t.Context())) }()

	// Wait for consumption (should only consume 1 record from logs-c)
	deadline := time.After(5 * time.Second)
	for called.Load() < 1 {
		select {
		case <-deadline:
			t.Fatalf("expected to consume 1 record, got %d", called.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Give it a bit more time to ensure no other messages are consumed
	time.Sleep(500 * time.Millisecond)

	// Verify results
	mu.Lock()
	defer mu.Unlock()

	require.Equal(t, int64(1), called.Load(), "should consume exactly 1 record")
	require.Equal(t, 0, consumedTopics["logs-a"], "logs-a should be excluded")
	require.Equal(t, 0, consumedTopics["logs-b"], "logs-b should be excluded")
	require.Equal(t, 1, consumedTopics["logs-c"], "logs-c should be consumed")
}

func TestFranzConsumerBrokerCacheEvictOnDisconnect(t *testing.T) {
	testTel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
	require.NoError(t, err)
	defer tb.Shutdown()

	c := &franzConsumer{
		telemetryBuilder: tb,
		brokerReadOpts:   make(map[brokerReadKey]metric.MeasurementOption),
	}
	meta := kgo.BrokerMetadata{NodeID: 1, Host: "broker1"}

	// Populate the broker read cache for both outcomes.
	c.OnBrokerRead(meta, 0, 0, time.Millisecond, time.Millisecond, nil)
	c.OnBrokerRead(meta, 0, 0, time.Millisecond, time.Millisecond, errors.New("oops"))
	require.Len(t, c.brokerReadOpts, 2)

	// Disconnect should evict both entries.
	c.OnBrokerDisconnect(meta, nil)
	require.Empty(t, c.brokerReadOpts)
}
