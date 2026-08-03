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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func TestIndependentPartitionProcessingDoesNotBlockOtherPartitions(t *testing.T) {
	const topic = "otlp_spans"
	_, cfg := mustNewFakeCluster(t, kfake.SeedTopics(2, topic))
	cfg.GroupID = t.Name()
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}
	producer, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RecordPartitioner(kgo.ManualPartitioner()),
	)
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	blocked := make(chan struct{})
	partitionZeroStarted := make(chan struct{})
	partitionOneProcessed := make(chan struct{})
	var (
		partitionZeroOnce sync.Once
		partitionOneOnce  sync.Once
	)
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(ctx context.Context, record *kgo.Record, _ attribute.Set) error {
			switch record.Partition {
			case 0:
				partitionZeroOnce.Do(func() { close(partitionZeroStarted) })
				select {
				case <-blocked:
					return nil
				case <-ctx.Done():
					return context.Cause(ctx)
				}
			case 1:
				partitionOneOnce.Do(func() { close(partitionOneProcessed) })
			}
			return nil
		}, nil
	}

	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, consumer.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		close(blocked)
		require.NoError(t, consumer.Shutdown(t.Context()))
	}()
	require.Eventually(t, func() bool {
		consumer.mu.RLock()
		defer consumer.mu.RUnlock()
		return len(consumer.assignments) == 2
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, producer.ProduceSync(t.Context(), &kgo.Record{
		Topic:     topic,
		Partition: 0,
		Value:     []byte("blocked"),
	}).FirstErr())
	require.Eventually(t, func() bool {
		select {
		case <-partitionZeroStarted:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, producer.ProduceSync(t.Context(), &kgo.Record{
		Topic:     topic,
		Partition: 1,
		Value:     []byte("healthy"),
	}).FirstErr())
	select {
	case <-partitionOneProcessed:
	case <-time.After(time.Second):
		t.Fatal("partition 1 was blocked by partition 0")
	}
}

func TestIndependentPartitionProcessingManualCommitDoesNotCommitBlockedPartition(t *testing.T) {
	const topic = "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(2, topic))
	cfg.GroupID = t.Name()
	cfg.AutoCommit.Enable = false
	cfg.MessageMarking.After = false
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}
	producer, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RecordPartitioner(kgo.ManualPartitioner()),
	)
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	blocked := make(chan struct{})
	partitionZeroStarted := make(chan struct{})
	partitionOneProcessed := make(chan struct{})
	var (
		partitionZeroOnce sync.Once
		partitionOneOnce  sync.Once
	)
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(ctx context.Context, record *kgo.Record, _ attribute.Set) error {
			if record.Partition == 0 {
				partitionZeroOnce.Do(func() { close(partitionZeroStarted) })
				select {
				case <-blocked:
				case <-ctx.Done():
					return context.Cause(ctx)
				}
			} else {
				partitionOneOnce.Do(func() { close(partitionOneProcessed) })
			}
			return nil
		}, nil
	}

	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, consumer.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		close(blocked)
		require.NoError(t, consumer.Shutdown(t.Context()))
	}()
	require.Eventually(t, func() bool {
		consumer.mu.RLock()
		defer consumer.mu.RUnlock()
		return len(consumer.assignments) == 2
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, producer.ProduceSync(t.Context(), &kgo.Record{
		Topic:     topic,
		Partition: 0,
		Value:     []byte("blocked"),
	}).FirstErr())
	require.Eventually(t, func() bool {
		select {
		case <-partitionZeroStarted:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, producer.ProduceSync(t.Context(), &kgo.Record{
		Topic:     topic,
		Partition: 1,
		Value:     []byte("healthy"),
	}).FirstErr())
	require.Eventually(t, func() bool {
		select {
		case <-partitionOneProcessed:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)

	require.Never(t, func() bool {
		offsets, err := kadm.NewClient(kafkaClient).FetchOffsets(t.Context(), cfg.GroupID)
		if err != nil {
			return false
		}
		offset, ok := offsets.Lookup(topic, 0)
		return ok && offset.At > 0
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestPartitionMailboxRepeatedOverflowRetainsEarliestRewind(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	partitionConsumer := pc{
		ctx:           ctx,
		mailbox:       make([]partitionBatch, 0, 1),
		mailboxNotify: make(chan struct{}, 1),
		generation:    1,
	}

	require.True(t, partitionConsumer.enqueueBatch(newTestPartitionBatch(0, 1)))
	require.False(t, partitionConsumer.enqueueBatch(newTestPartitionBatch(1, 1)))
	require.False(t, partitionConsumer.enqueueBatch(newTestPartitionBatch(2, 1)))
	require.Equal(t, int64(1), partitionConsumer.pendingRewind.Offset)
}

func TestPartitionMailboxRejectsOverflowAfterFailureRewindAndRevocation(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	partitionConsumer := pc{
		ctx:           ctx,
		mailbox:       make([]partitionBatch, 0, 2),
		mailboxNotify: make(chan struct{}, 1),
		generation:    4,
	}

	require.True(t, partitionConsumer.enqueueBatch(newTestPartitionBatch(6, 4)))
	partitionConsumer.requestRewind(&kgo.Record{
		Offset: 5,
	})
	require.Empty(t, partitionConsumer.mailbox)

	start := make(chan struct{})
	enqueueResult := make(chan bool, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		enqueueResult <- partitionConsumer.enqueueBatch(newTestPartitionBatch(7, 4))
	})
	wg.Go(func() {
		<-start
		cancel(errors.New("partition revoked"))
	})
	close(start)
	wg.Wait()

	require.False(t, <-enqueueResult)
	require.Equal(t, int64(5), partitionConsumer.pendingRewind.Offset)
	require.False(t, partitionConsumer.enqueueBatch(newTestPartitionBatch(4, 4)))
	require.Equal(t, int64(5), partitionConsumer.pendingRewind.Offset)
}

type blockingFetchController struct {
	pauseStarted chan struct{}
	releasePause chan struct{}
	resumed      chan struct{}
}

func (c *blockingFetchController) PauseFetchPartitions(map[string][]int32) map[string][]int32 {
	close(c.pauseStarted)
	<-c.releasePause
	return nil
}

func (c *blockingFetchController) ResumeFetchPartitions(map[string][]int32) {
	close(c.resumed)
}

func TestDispatchSerializesPauseBeforeMailboxResume(t *testing.T) {
	const topic = "test"
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	controller := &blockingFetchController{
		pauseStarted: make(chan struct{}),
		releasePause: make(chan struct{}),
		resumed:      make(chan struct{}),
	}
	partitionConsumer := &pc{
		ctx:           ctx,
		logger:        zap.NewNop(),
		mailbox:       make([]partitionBatch, 0, 1),
		mailboxNotify: make(chan struct{}, 1),
	}
	tp := topicPartition{topic: topic, partition: 0}
	consumer := franzConsumer{
		fetchController: controller,
	}
	fetch := kgo.Fetches{{
		Topics: []kgo.FetchTopic{{
			Topic: topic,
			Partitions: []kgo.FetchPartition{{
				Partition: 0,
				Records: []*kgo.Record{{
					Topic:     topic,
					Partition: 0,
					Offset:    1,
				}},
			}},
		}},
	}}

	dispatchDone := make(chan struct{})
	go func() {
		consumer.dispatchPartitionBatches(
			fetch,
			map[topicPartition]*pc{tp: partitionConsumer},
			map[topicPartition]partitionSnapshot{tp: {
				pc:         partitionConsumer,
				generation: partitionConsumer.generation,
			}},
		)
		close(dispatchDone)
	}()
	<-controller.pauseStarted

	dequeueStarted := make(chan struct{})
	go func() {
		close(dequeueStarted)
		partitionConsumer.mailboxMu.Lock()
		partitionConsumer.mailbox = partitionConsumer.mailbox[:0]
		controller.ResumeFetchPartitions(nil)
		partitionConsumer.mailboxMu.Unlock()
	}()
	<-dequeueStarted

	require.Never(t, func() bool {
		select {
		case <-controller.resumed:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond)

	close(controller.releasePause)
	<-dispatchDone
	<-controller.resumed
}

func newTestPartitionBatch(offset int64, generation uint64) partitionBatch {
	return partitionBatch{
		fetch: kgo.FetchTopicPartition{
			Topic: "test",
			FetchPartition: kgo.FetchPartition{
				Partition: 0,
				Records: []*kgo.Record{
					{
						Topic:     "test",
						Partition: 0,
						Offset:    offset,
					},
				},
			},
		},
		generation: generation,
	}
}

func TestIndependentPartitionProcessingPausesAndResumesFullMailbox(t *testing.T) {
	const topic = "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.GroupID = t.Name()
	cfg.MaxFetchSize = 1
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}

	release := make(chan struct{})
	firstStarted := make(chan struct{})
	var (
		processed atomic.Int64
		once      sync.Once
	)
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(ctx context.Context, _ *kgo.Record, _ attribute.Set) error {
			once.Do(func() {
				close(firstStarted)
				select {
				case <-release:
				case <-ctx.Done():
				}
			})
			processed.Add(1)
			return nil
		}, nil
	}

	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, consumer.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		require.NoError(t, consumer.Shutdown(t.Context()))
	}()
	require.Eventually(t, func() bool {
		consumer.mu.RLock()
		defer consumer.mu.RUnlock()
		return len(consumer.assignments) == 1
	}, 5*time.Second, 10*time.Millisecond)

	records := make([]*kgo.Record, 10)
	for i := range records {
		records[i] = &kgo.Record{
			Topic:     topic,
			Partition: 0,
			Value:     []byte{byte(i)},
		}
	}
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), records[0]).FirstErr())
	require.Eventually(t, func() bool {
		select {
		case <-firstStarted:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), records[1:]...).FirstErr())
	require.Eventually(t, func() bool {
		paused := consumer.client.PauseFetchPartitions(nil)
		return len(paused[topic]) == 1
	}, 5*time.Second, 10*time.Millisecond)

	close(release)
	require.Eventually(t, func() bool {
		return processed.Load() == int64(len(records))
	}, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		paused := consumer.client.PauseFetchPartitions(nil)
		return len(paused[topic]) == 0
	}, 5*time.Second, 10*time.Millisecond)

	// Verify that a late PauseFetchPartitions call did not overtake the
	// worker's resume and leave an empty mailbox permanently paused.
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic,
		Value: []byte("after-resume"),
	}).FirstErr())
	require.Eventually(t, func() bool {
		return processed.Load() == int64(len(records)+1)
	}, 5*time.Second, 10*time.Millisecond)
}

func TestIndependentPartitionProcessingPreservesPartitionOrder(t *testing.T) {
	const topic = "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.GroupID = t.Name()
	cfg.AutoCommit.Enable = false
	cfg.MessageMarking.After = true
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}

	const recordCount = 20
	var (
		mu      sync.Mutex
		offsets []int64
		active  atomic.Int64
		max     atomic.Int64
	)
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			current := active.Add(1)
			defer active.Add(-1)
			for {
				previous := max.Load()
				if current <= previous || max.CompareAndSwap(previous, current) {
					break
				}
			}
			mu.Lock()
			offsets = append(offsets, record.Offset)
			mu.Unlock()
			return nil
		}, nil
	}

	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, consumer.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, consumer.Shutdown(t.Context())) }()

	records := make([]*kgo.Record, recordCount)
	for i := range records {
		records[i] = &kgo.Record{
			Topic: topic,
			Value: []byte{byte(i)},
		}
	}
	producer, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RecordPartitioner(kgo.ManualPartitioner()),
	)
	require.NoError(t, err)
	t.Cleanup(producer.Close)
	require.NoError(t, producer.ProduceSync(t.Context(), records...).FirstErr())
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(offsets) == recordCount
	}, 5*time.Second, 10*time.Millisecond)

	mu.Lock()
	require.Equal(t, int64(1), max.Load())
	for i, offset := range offsets {
		require.Equal(t, int64(i), offset)
	}
	mu.Unlock()
	require.Eventually(t, func() bool {
		committed, err := kadm.NewClient(kafkaClient).FetchOffsets(t.Context(), cfg.GroupID)
		if err != nil {
			return false
		}
		offset, ok := committed.Lookup(topic, 0)
		return ok && offset.At == recordCount
	}, 5*time.Second, 10*time.Millisecond)
}

func TestIndependentPartitionProcessingShutdownCancelsWorker(t *testing.T) {
	const topic = "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.GroupID = t.Name()
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}

	started := make(chan struct{})
	cancelled := make(chan struct{})
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(ctx context.Context, _ *kgo.Record, _ attribute.Set) error {
			close(started)
			<-ctx.Done()
			close(cancelled)
			return context.Cause(ctx)
		}, nil
	}

	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, consumer.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic,
		Value: []byte("blocked"),
	}).FirstErr())
	require.Eventually(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, consumer.Shutdown(shutdownCtx))
	select {
	case <-cancelled:
	default:
		t.Fatal("partition worker was not cancelled during shutdown")
	}
}

func TestIndependentPartitionProcessingRewindsAfterBackoff(t *testing.T) {
	const topic = "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.GroupID = t.Name()
	cfg.MessageMarking = MessageMarking{
		After:   true,
		OnError: false,
	}
	cfg.ErrorBackOff = configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         20 * time.Millisecond,
		MaxElapsedTime:      50 * time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          1,
	}
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}

	var (
		fail      atomic.Bool
		attempts  atomic.Int64
		processed atomic.Int64
	)
	fail.Store(true)
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			attempts.Add(1)
			if fail.Load() {
				return errors.New("transient failure")
			}
			processed.Add(1)
			return nil
		}, nil
	}

	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, consumer.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, consumer.Shutdown(t.Context())) }()
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic,
		Value: []byte("retry"),
	}).FirstErr())
	require.Eventually(t, func() bool {
		return attempts.Load() >= 4
	}, 5*time.Second, 10*time.Millisecond)

	fail.Store(false)
	require.Eventually(t, func() bool {
		return processed.Load() == 1
	}, 5*time.Second, 10*time.Millisecond)
}

func TestIndependentPartitionProcessingKeepsPermanentErrorPaused(t *testing.T) {
	const topic = "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.GroupID = t.Name()
	cfg.MessageMarking = MessageMarking{
		After:            true,
		OnPermanentError: false,
	}
	cfg.PartitionProcessing = PartitionProcessing{
		Independent:        true,
		MaxBufferedBatches: 1,
	}

	var attempts atomic.Int64
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ *kgo.Record, _ attribute.Set) error {
			attempts.Add(1)
			return consumererror.NewPermanent(errors.New("permanent failure"))
		}, nil
	}

	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, consumeFn)
	require.NoError(t, err)
	require.NoError(t, consumer.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, consumer.Shutdown(t.Context())) }()
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic,
		Value: []byte("permanent"),
	}).FirstErr())
	require.Eventually(t, func() bool {
		paused := consumer.client.PauseFetchPartitions(nil)
		return len(paused[topic]) == 1
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: topic,
		Value: []byte("still paused"),
	}).FirstErr())
	require.Never(t, func() bool {
		return attempts.Load() > 1
	}, 500*time.Millisecond, 10*time.Millisecond)
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
		cfg.GroupID = tb.Name()
		cfg.AutoCommit = configkafka.AutoCommitConfig{Enable: true, Interval: 10 * time.Second}
		// Set MinFetchSize to ensure all records are fetched at once
		cfg.MinFetchSize = int32(len(data) * len(rs))
		// Use a very short MaxFetchWait to avoid delays when MinFetchSize cannot be met
		cfg.MaxFetchWait = 10 * time.Millisecond
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
	cfg.GroupID = t.Name()
	cfg.MaxFetchSize = 1 // Force a lot of iterations of consume()
	cfg.AutoCommit = configkafka.AutoCommitConfig{
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

// TestResumePartitionsAfterRebalance verifies that partitions paused due to
// processing errors are resumed when they are reassigned after a rebalance.
func TestResumePartitionsAfterRebalance(t *testing.T) {
	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.GroupID = t.Name()
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
	cfg.GroupID = t.Name()
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
	cfg.GroupID = t.Name()
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
	cfg.UseLeaderEpoch = false // <-- exercise the option
	cfg.GroupID = t.Name()
	cfg.AutoCommit = configkafka.AutoCommitConfig{Enable: true, Interval: 100 * time.Millisecond}

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
	cfg.GroupID = t.Name()
	cfg.AutoCommit = configkafka.AutoCommitConfig{Enable: true, Interval: 100 * time.Millisecond}

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
