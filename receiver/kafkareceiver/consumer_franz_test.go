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
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
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

// TestPartitionProcessingBlockedPartition verifies that a blocked partition
// does not stop another partition or commit the blocked record.
func TestPartitionProcessingBlockedPartition(t *testing.T) {
	h := newPartitionProcessingHarness(t, 2, func(cfg *Config) {
		cfg.ConsumerConfig.AutoCommit.Enable = false
		cfg.MessageMarking.After = false
	})

	blocked := make(chan struct{})
	partitionZeroStarted := make(chan struct{})
	partitionOneProcessed := make(chan struct{})
	var (
		partitionZeroOnce sync.Once
		partitionOneOnce  sync.Once
	)
	h.start(func(ctx context.Context, record *kgo.Record, _ attribute.Set) error {
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
	})
	defer func() {
		close(blocked)
		h.shutdown(t.Context())
	}()
	h.waitAssignments(2)

	h.produce(0, "blocked")
	waitSignal(t, partitionZeroStarted, "partition 0 did not start")

	h.produce(1, "healthy")
	waitSignal(t, partitionOneProcessed, "partition 1 was blocked by partition 0")
	require.Eventually(t, func() bool {
		offset, ok := h.committedOffset(1)
		return ok && offset == 1
	}, 5*time.Second, 10*time.Millisecond)

	require.Never(t, func() bool {
		offset, ok := h.committedOffset(0)
		return ok && offset > 0
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestPartitionProcessingCommitDoesNotBlockPolling(t *testing.T) {
	h := newPartitionProcessingHarness(t, 2, func(cfg *Config) {
		cfg.ConsumerConfig.AutoCommit.Enable = false
	})

	processed := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	var processedOnce [2]sync.Once
	h.start(func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
		processedOnce[record.Partition].Do(func() {
			close(processed[record.Partition])
		})
		return nil
	})
	defer h.shutdown(t.Context())
	h.waitAssignments(2)

	var rejectCommits atomic.Bool
	rejectCommits.Store(true)
	commitStarted := make(chan struct{})
	var commitOnce sync.Once
	h.cluster.ControlKey(int16(kmsg.OffsetCommit), func(request kmsg.Request) (kmsg.Response, error, bool) {
		h.cluster.KeepControl()
		if !rejectCommits.Load() {
			return nil, nil, false
		}
		commitOnce.Do(func() { close(commitStarted) })
		return retryOffsetCommitResponse(request.(*kmsg.OffsetCommitRequest)), nil, true
	})
	defer rejectCommits.Store(false)

	h.produce(0, "first")
	waitSignal(t, processed[0], "partition 0 was not processed")
	waitSignal(t, commitStarted, "partition 0 did not attempt an offset commit")

	h.produce(1, "second")
	waitSignal(t, processed[1], "partition 1 was blocked by partition 0's offset commit")
}

func TestPartitionProcessingPendingRewindDoesNotBlockPolling(t *testing.T) {
	h := newPartitionProcessingHarness(t, 3, func(cfg *Config) {
		cfg.ConsumerConfig.AutoCommit.Enable = false
		cfg.MessageMarking.After = true
		cfg.ErrorBackOff = configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     time.Millisecond,
			MaxInterval:         time.Millisecond,
			MaxElapsedTime:      2 * time.Millisecond,
			RandomizationFactor: 0,
			Multiplier:          1,
		}
	})

	healthyProcessed := make(chan struct{})
	var healthyOnce sync.Once
	h.start(func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
		switch record.Partition {
		case 1:
			return errors.New("trigger rewind")
		case 2:
			healthyOnce.Do(func() { close(healthyProcessed) })
		}
		return nil
	})
	defer h.shutdown(t.Context())
	h.waitAssignments(3)

	var rejectCommits atomic.Bool
	rejectCommits.Store(true)
	defer rejectCommits.Store(false)
	commitStarted := make(chan struct{})
	var commitOnce sync.Once
	h.cluster.ControlKey(int16(kmsg.OffsetCommit), func(request kmsg.Request) (kmsg.Response, error, bool) {
		h.cluster.KeepControl()
		if !rejectCommits.Load() {
			return nil, nil, false
		}
		commitOnce.Do(func() { close(commitStarted) })
		return retryOffsetCommitResponse(request.(*kmsg.OffsetCommitRequest)), nil, true
	})

	h.produce(0, "blocked commit")
	waitSignal(t, commitStarted, "partition 0 did not attempt an offset commit")
	h.produce(1, "rewind")
	require.Eventually(t, func() bool {
		return len(h.consumer.controls) == 1
	}, 5*time.Second, 10*time.Millisecond)

	h.produce(2, "healthy")
	waitSignal(t, healthyProcessed, "pending rewind blocked polling healthy partition")
}

func TestPartitionProcessingRebalance(t *testing.T) {
	const recordCount = 100
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
	defer h.shutdown(t.Context())
	h.waitAssignments(1)

	records := make([]*kgo.Record, 0, recordCount)
	for range recordCount {
		records = append(records, &kgo.Record{Partition: 0})
	}
	h.produceRecords(records...)
	for range 10 {
		h.consumer.client.ForceRebalance()
		runtime.Gosched()
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == recordCount
	}, 5*time.Second, 10*time.Millisecond)
}

func TestPartitionProcessingCommitSerializesRewind(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.ConsumerConfig.AutoCommit.Enable = false
	})
	h.start(func(context.Context, *kgo.Record, attribute.Set) error {
		return nil
	})
	defer h.shutdown(t.Context())
	h.waitAssignments(1)

	var rejectCommits atomic.Bool
	rejectCommits.Store(true)
	defer rejectCommits.Store(false)
	commitStarted := make(chan struct{})
	var commitOnce sync.Once
	h.cluster.ControlKey(int16(kmsg.OffsetCommit), func(request kmsg.Request) (kmsg.Response, error, bool) {
		h.cluster.KeepControl()
		if !rejectCommits.Load() {
			return nil, nil, false
		}
		commitOnce.Do(func() { close(commitStarted) })
		return retryOffsetCommitResponse(request.(*kmsg.OffsetCommitRequest)), nil, true
	})

	h.produce(0, "record")
	waitSignal(t, commitStarted, "partition did not attempt an offset commit")

	tp := topicPartition{topic: h.topic, partition: 0}
	h.consumer.mu.RLock()
	partitionConsumer := h.consumer.assignments[tp]
	h.consumer.mu.RUnlock()
	require.NotNil(t, partitionConsumer)

	partitionConsumer.mailbox.requestRewind(&kgo.Record{
		Topic:       h.topic,
		Partition:   0,
		Offset:      1,
		LeaderEpoch: 0,
	}, true, func() {})
	done := make(chan struct{})
	go func() {
		h.consumer.opsMu.Lock()
		defer h.consumer.opsMu.Unlock()
		h.consumer.processControlLocked(partitionControl{
			tp:   tp,
			pc:   partitionConsumer,
			done: done,
		})
	}()

	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 200*time.Millisecond, 10*time.Millisecond)

	rejectCommits.Store(false)
	waitSignal(t, done, "rewind remained blocked after the commit completed")
}

func TestPartitionProcessingIgnoresControlAfterCancellation(t *testing.T) {
	client, _ := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
	ctx, cancel := context.WithCancel(t.Context())
	partitionConsumer := &pc{
		ctx:     ctx,
		mailbox: newPartitionMailbox(ctx, 1),
	}
	partitionConsumer.mailbox.requestRewind(&kgo.Record{
		Topic:     "test",
		Partition: 0,
		Offset:    1,
	}, true, func() {})
	cancel()

	tp := topicPartition{topic: "test", partition: 0}
	consumer := franzConsumer{
		client:      client,
		assignments: map[topicPartition]*pc{tp: partitionConsumer},
	}
	consumer.opsMu.Lock()
	consumer.processControlLocked(partitionControl{
		tp:   tp,
		pc:   partitionConsumer,
		done: make(chan struct{}),
	})
	consumer.opsMu.Unlock()

	require.NotNil(t, partitionConsumer.mailbox.takeOffsetChange())
}

func TestPartitionProcessingRevocationCommitsProgressAfterRewind(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.ConsumerConfig.AutoCommit.Enable = false
		cfg.MessageMarking.After = true
		cfg.ErrorBackOff = configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     time.Millisecond,
			MaxInterval:         time.Millisecond,
			MaxElapsedTime:      time.Nanosecond,
			RandomizationFactor: 0,
			Multiplier:          1,
		}
	})
	retried := make(chan struct{})
	var attempts atomic.Int64
	h.start(func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
		if record.Offset == 0 {
			return nil
		}
		if attempts.Add(1) == 2 {
			close(retried)
		}
		return errors.New("trigger rewind")
	})
	defer h.shutdown(t.Context())
	h.waitAssignments(1)

	var rejectFirstCommit atomic.Bool
	rejectFirstCommit.Store(true)
	h.cluster.ControlKey(int16(kmsg.OffsetCommit), func(request kmsg.Request) (kmsg.Response, error, bool) {
		h.cluster.KeepControl()
		if !rejectFirstCommit.Swap(false) {
			return nil, nil, false
		}
		response := retryOffsetCommitResponse(request.(*kmsg.OffsetCommitRequest))
		for i := range response.Topics {
			for j := range response.Topics[i].Partitions {
				response.Topics[i].Partitions[j].ErrorCode = kerr.GroupAuthorizationFailed.Code
			}
		}
		return response, nil, true
	})

	h.produceRecords(
		&kgo.Record{Partition: 0, Value: []byte("accepted")},
		&kgo.Record{Partition: 0, Value: []byte("rewound")},
	)
	waitSignal(t, retried, "failed record was not fetched again")
	h.consumer.lost(t.Context(), nil, map[string][]int32{h.topic: {0}}, false)

	require.Eventually(t, func() bool {
		offset, ok := h.committedOffset(0)
		return ok && offset == 1
	}, 5*time.Second, 10*time.Millisecond)
}

func TestPartitionProcessingBeforeMarkingCommitsFailedRecord(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.ConsumerConfig.AutoCommit.Enable = false
		cfg.MessageMarking.After = false
	})
	h.start(func(context.Context, *kgo.Record, attribute.Set) error {
		return errors.New("processing failed")
	})
	defer h.shutdown(t.Context())

	h.produce(0, "failed")

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		offset, ok := h.committedOffset(0)
		assert.True(collect, ok)
		assert.Equal(collect, int64(1), offset)
	}, 5*time.Second, 10*time.Millisecond)
}

func TestPartitionProcessingBeforeMarkingCommitsInterruptedRecord(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.ConsumerConfig.AutoCommit.Enable = false
		cfg.MessageMarking.After = false
		cfg.ErrorBackOff = configretry.BackOffConfig{
			Enabled:         true,
			InitialInterval: time.Hour,
			MaxInterval:     time.Hour,
			MaxElapsedTime:  2 * time.Hour,
		}
	})
	processing := make(chan struct{})
	var once sync.Once
	h.start(func(context.Context, *kgo.Record, attribute.Set) error {
		once.Do(func() { close(processing) })
		return errors.New("trigger backoff")
	})
	defer h.shutdown(t.Context())
	h.waitAssignments(1)

	h.produce(0, "interrupted")
	waitSignal(t, processing, "record did not start processing")
	h.consumer.lost(t.Context(), nil, map[string][]int32{h.topic: {0}}, false)

	require.Eventually(t, func() bool {
		offset, ok := h.committedOffset(0)
		return ok && offset == 1
	}, 5*time.Second, 10*time.Millisecond)
}

func retryOffsetCommitResponse(request *kmsg.OffsetCommitRequest) *kmsg.OffsetCommitResponse {
	response := &kmsg.OffsetCommitResponse{
		Version: request.Version,
		Topics:  make([]kmsg.OffsetCommitResponseTopic, 0, len(request.Topics)),
	}
	for _, topic := range request.Topics {
		responseTopic := kmsg.OffsetCommitResponseTopic{
			Topic:      topic.Topic,
			Partitions: make([]kmsg.OffsetCommitResponseTopicPartition, 0, len(topic.Partitions)),
		}
		for _, partition := range topic.Partitions {
			responseTopic.Partitions = append(responseTopic.Partitions, kmsg.OffsetCommitResponseTopicPartition{
				Partition: partition.Partition,
				ErrorCode: kerr.CoordinatorLoadInProgress.Code,
			})
		}
		response.Topics = append(response.Topics, responseTopic)
	}
	return response
}

func TestPartitionProcessingFullMailbox(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.ConsumerConfig.MaxFetchSize = 1
	})

	release := make(chan struct{})
	firstStarted := make(chan struct{})
	var (
		processed atomic.Int64
		once      sync.Once
	)
	h.start(func(ctx context.Context, _ *kgo.Record, _ attribute.Set) error {
		once.Do(func() {
			close(firstStarted)
			select {
			case <-release:
			case <-ctx.Done():
			}
		})
		processed.Add(1)
		return nil
	})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		h.shutdown(t.Context())
	}()
	h.waitAssignments(1)

	records := make([]*kgo.Record, 10)
	for i := range records {
		records[i] = &kgo.Record{
			Topic:     h.topic,
			Partition: 0,
			Value:     []byte{byte(i)},
		}
	}
	h.produceRecords(records[0])
	waitSignal(t, firstStarted, "first record did not start")
	h.produceRecords(records[1:]...)
	h.waitPaused(1)

	close(release)
	waitAtomic(t, &processed, int64(len(records)))
	h.waitPaused(0)

	// A drained mailbox must not remain paused after a previous pause decision.
	h.produce(0, "after-resume")
	waitAtomic(t, &processed, int64(len(records)+1))
}

func TestPartitionProcessingTransientRetry(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
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
	})

	var (
		fail      atomic.Bool
		attempts  atomic.Int64
		processed atomic.Int64
	)
	fail.Store(true)
	h.start(func(context.Context, *kgo.Record, attribute.Set) error {
		attempts.Add(1)
		if fail.Load() {
			return errors.New("transient failure")
		}
		processed.Add(1)
		return nil
	})
	defer h.shutdown(t.Context())
	h.produce(0, "retry")
	require.Eventually(t, func() bool {
		return attempts.Load() >= 4
	}, 5*time.Second, 10*time.Millisecond)

	fail.Store(false)
	waitAtomic(t, &processed, 1)
}

func TestPartitionProcessingPermanentError(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.MessageMarking = MessageMarking{
			After:            true,
			OnPermanentError: false,
		}
	})

	var attempts atomic.Int64
	h.start(func(context.Context, *kgo.Record, attribute.Set) error {
		attempts.Add(1)
		return consumererror.NewPermanent(errors.New("permanent failure"))
	})
	defer h.shutdown(t.Context())
	h.produce(0, "permanent")
	h.waitPaused(1)

	h.produce(0, "still paused")
	require.Never(t, func() bool {
		return attempts.Load() > 1
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestPartitionProcessingTerminalErrorClearsMailbox(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.ConsumerConfig.MaxFetchSize = 1
		cfg.PartitionProcessing.MaxBufferedBatches = 3
		cfg.MessageMarking = MessageMarking{
			After:            true,
			OnPermanentError: false,
		}
	})

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	h.start(func(ctx context.Context, _ *kgo.Record, _ attribute.Set) error {
		once.Do(func() { close(started) })
		select {
		case <-release:
			return consumererror.NewPermanent(errors.New("permanent failure"))
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		h.shutdown(t.Context())
	}()
	h.waitAssignments(1)

	h.consumer.mu.RLock()
	partitionConsumer := h.consumer.assignments[topicPartition{topic: h.topic, partition: 0}]
	h.consumer.mu.RUnlock()
	require.NotNil(t, partitionConsumer)

	h.produce(0, "first")
	waitSignal(t, started, "first record did not start")
	h.produceRecords(
		&kgo.Record{Partition: 0, Value: []byte("second")},
		&kgo.Record{Partition: 0, Value: []byte("third")},
	)
	require.Eventually(t, func() bool {
		partitionConsumer.mailbox.mu.Lock()
		defer partitionConsumer.mailbox.mu.Unlock()
		return len(partitionConsumer.mailbox.batches) > 0
	}, 5*time.Second, 10*time.Millisecond)

	close(release)
	h.waitPaused(1)
	require.Eventually(t, func() bool {
		partitionConsumer.mailbox.mu.Lock()
		defer partitionConsumer.mailbox.mu.Unlock()
		return len(partitionConsumer.mailbox.batches) == 0
	}, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return partitionConsumer.ctx.Err() != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestPartitionProcessingMailboxCapacity(t *testing.T) {
	h := newPartitionProcessingHarness(t, 1, func(cfg *Config) {
		cfg.PartitionProcessing.MaxBufferedBatches = 3
	})
	h.start(func(context.Context, *kgo.Record, attribute.Set) error {
		return nil
	})
	defer h.shutdown(t.Context())
	h.waitAssignments(1)

	h.consumer.mu.RLock()
	defer h.consumer.mu.RUnlock()
	for _, partitionConsumer := range h.consumer.assignments {
		require.NotNil(t, partitionConsumer.mailbox)
		require.Equal(t, 3, cap(partitionConsumer.mailbox.batches))
	}
}

type partitionProcessingHarness struct {
	t           *testing.T
	topic       string
	cluster     *kfake.Cluster
	kafkaClient *kgo.Client
	producer    *kgo.Client
	cfg         *Config
	consumer    *franzConsumer
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
	cfg.Telemetry.Metrics.KafkaReceiverRecordsDelay.Enabled = true
	cfg.ConsumerConfig.GroupID = t.Name()
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
		t:           t,
		topic:       topic,
		cluster:     cluster,
		kafkaClient: kafkaClient,
		producer:    producer,
		cfg:         cfg,
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
}

func (h *partitionProcessingHarness) shutdown(ctx context.Context) {
	h.t.Helper()
	require.NoError(h.t, h.consumer.Shutdown(ctx))
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
		paused := h.consumer.client.PauseFetchPartitions(nil)
		return len(paused[h.topic]) == want
	}, 5*time.Second, 10*time.Millisecond)
}

func (h *partitionProcessingHarness) committedOffset(partition int32) (int64, bool) {
	offsets, err := kadm.NewClient(h.kafkaClient).FetchOffsets(h.t.Context(), h.cfg.ConsumerConfig.GroupID)
	if err != nil {
		return 0, false
	}
	offset, ok := offsets.Lookup(h.topic, partition)
	return offset.At, ok
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

func waitAtomic(t *testing.T, got *atomic.Int64, want int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		return got.Load() == want
	}, 5*time.Second, 10*time.Millisecond)
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

func TestLostFatalWaitsOnlyForIndependentProcessing(t *testing.T) {
	cases := []struct {
		name        string
		independent bool
		wantWait    bool
	}{
		{
			name: "legacy returns immediately",
		},
		{
			name:        "independent waits for worker",
			independent: true,
			wantWait:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
			cfg.PartitionProcessing.Independent = tc.independent
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
				waitSignal(t, done, "fatal partition loss waited for legacy processing")
			}

			partitionConsumer.wg.Done()
			waitSignal(t, done, "fatal partition loss did not finish")
		})
	}
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
