// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TestClearPauseReasons proves fetch resumes only after the last pause reason is cleared.
func TestClearPauseReasons(t *testing.T) {
	cases := []struct {
		name          string
		current       partitionPauseReason
		clear         partitionPauseReason
		wantResume    bool
		wantRemaining partitionPauseReason
	}{
		{
			name:  "does not resume without matching reason",
			clear: partitionPauseBackpressure,
		},
		{
			name:          "resumes after final reason clears",
			current:       partitionPauseBackpressure,
			clear:         partitionPauseBackpressure,
			wantResume:    true,
			wantRemaining: 0,
		},
		{
			name:          "does not resume while another reason remains",
			current:       partitionPauseBackpressure | partitionPauseRewind,
			clear:         partitionPauseBackpressure,
			wantRemaining: partitionPauseRewind,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			partitionConsumer := &pc{}
			partitionConsumer.pauseReasons.Store(uint32(tc.current))

			require.Equal(t, tc.wantResume, partitionConsumer.clearPauseReasons(tc.clear))
			require.Equal(t, uint32(tc.wantRemaining), partitionConsumer.pauseReasons.Load())
		})
	}
}

func TestProcessPartitionBatchMarkOwnership(t *testing.T) {
	const topic = "test"
	cases := []struct {
		name        string
		independent bool
		after       bool
		owner       bool
		wantMarked  bool
	}{
		{
			name:       "legacy marks after assignment changes",
			wantMarked: true,
		},
		{
			name:       "legacy after marks after assignment changes",
			after:      true,
			wantMarked: true,
		},
		{
			name:        "independent skips after assignment changes",
			independent: true,
		},
		{
			name:        "independent after skips after assignment changes",
			independent: true,
			after:       true,
		},
		{
			name:        "independent marks while owner",
			independent: true,
			owner:       true,
			wantMarked:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kafkaClient, cfg := mustNewMarkedFakeCluster(t, kfake.SeedTopics(1, topic))
			cfg.PartitionProcessing.Independent = tc.independent
			cfg.MessageMarking.After = tc.after
			settings, _, _ := mustNewSettings(t)
			consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, nil)
			require.NoError(t, err)
			consumer.client = kafkaClient
			consumer.consumeMessage = func(context.Context, *kgo.Record, attribute.Set) error {
				return nil
			}

			ctx, cancel := context.WithCancelCause(t.Context())
			t.Cleanup(func() { cancel(nil) })
			partitionConsumer := &pc{
				ctx:    ctx,
				cancel: cancel,
				attrs:  attribute.NewSet(),
			}
			if tc.owner {
				consumer.assignments[topicPartition{topic: topic, partition: 0}] = partitionConsumer
			}

			batch := mailboxBatch(10)
			batch.Topic = topic
			batch.Records[0].Topic = topic
			batch.Records[0].Partition = 0
			consumer.processPartitionBatch(t.Context(), partitionConsumer, batch)

			marked := kafkaClient.MarkedOffsets()[topic]
			if tc.wantMarked {
				require.NotEmpty(t, marked)
				return
			}
			require.Empty(t, marked)
		})
	}
}

func TestProcessPartitionBatchMaxInFlight(t *testing.T) {
	const topic = "test"
	const inFlight = 4
	cases := []struct {
		name string
		// failAt is the offset that fails, or -1 when every record succeeds.
		failAt int64
		// failTimes is how many Consume calls at failAt return an error.
		// 0 means every call at failAt fails.
		failTimes  int
		records    int
		wantRewind int64
		wantMarked int64
	}{
		{
			name:       "overlaps Consume calls and marks the whole batch",
			failAt:     -1,
			wantRewind: -1,
			wantMarked: inFlight,
		},
		{
			name:       "retries the hole in memory and does not rewind",
			failAt:     1,
			failTimes:  1,
			wantRewind: -1,
			wantMarked: inFlight,
		},
		{
			name:       "retries the hole then processes the rest of the batch",
			failAt:     1,
			failTimes:  1,
			records:    2 * inFlight,
			wantRewind: -1,
			wantMarked: 2 * inFlight,
		},
		{
			name:       "marks the successful prefix and rewinds when the hole retry fails",
			failAt:     1,
			wantRewind: 1,
			wantMarked: 1,
		},
		{
			name:       "stops starting calls and marks nothing when the first record fails",
			records:    2 * inFlight,
			failAt:     0,
			wantRewind: 0,
			wantMarked: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records := tc.records
			if records == 0 {
				records = inFlight
			}
			consumer, kafkaClient, partitionConsumer := newMaxInFlightConsumer(t, inFlight)
			entered := make(chan struct{}, records+1)
			releases := make([]chan struct{}, records)
			for i := range releases {
				releases[i] = make(chan struct{})
			}
			calls := make([]atomic.Int64, records)
			consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
				n := calls[record.Offset].Add(1)
				entered <- struct{}{}
				<-releases[record.Offset]
				if record.Offset == tc.failAt && (tc.failTimes == 0 || n <= int64(tc.failTimes)) {
					return errors.New("boom")
				}
				return nil
			}

			done := make(chan partitionBatchResult, 1)
			go func() {
				done <- consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatch(records))
			}()

			// Every in-flight call must be inside Consume before any returns.
			// This proves the overlap.
			timeout := time.After(10 * time.Second)
			for range inFlight {
				select {
				case <-entered:
				case <-timeout:
					t.Fatalf("fewer than %d Consume calls overlapped", inFlight)
				}
			}
			if records > inFlight {
				requireNoEntry(t, entered, "started more than max_in_flight calls")
			}
			if tc.failAt == 0 {
				close(releases[0])
				requireNoEntry(t, entered, "started a call after a record failed")
				for _, ch := range releases[1:] {
					close(ch)
				}
			} else {
				for _, ch := range releases {
					close(ch)
				}
			}

			result := <-done
			if tc.wantRewind < 0 {
				require.Nil(t, result.rewindRecord)
			} else {
				require.NotNil(t, result.rewindRecord)
				require.Equal(t, tc.wantRewind, result.rewindRecord.Offset)
			}

			marked := kafkaClient.MarkedOffsets()[topic]
			if tc.wantMarked < 0 {
				require.Empty(t, marked)
			} else {
				require.Len(t, marked, 1)
				require.Equal(t, tc.wantMarked, marked[0].Offset)
			}
			if tc.failAt >= 0 {
				require.Equal(t, int64(2), calls[tc.failAt].Load())
			}
			if tc.failAt == 1 {
				require.Equal(t, int64(1), calls[2].Load())
				require.Equal(t, int64(1), calls[3].Load())
			}
			if tc.failAt < 0 {
				requireNoEntry(t, entered, "started extra Consume calls after the batch returned")
			}
		})
	}
}

func TestProcessPartitionBatchPrefixMark(t *testing.T) {
	const topic = "test"
	const inFlight = 4
	const records = 8
	consumer, kafkaClient, partitionConsumer := newMaxInFlightConsumer(t, inFlight)
	entered := make(chan struct{}, records+1)
	releases := make([]chan struct{}, records)
	for i := range releases {
		releases[i] = make(chan struct{})
	}
	consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
		entered <- struct{}{}
		<-releases[record.Offset]
		return nil
	}

	done := make(chan partitionBatchResult, 1)
	go func() {
		done <- consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatch(records))
	}()

	timeout := time.After(10 * time.Second)
	for range inFlight {
		select {
		case <-entered:
		case <-timeout:
			t.Fatalf("fewer than %d Consume calls overlapped", inFlight)
		}
	}
	requireNoEntry(t, entered, "started more than max_in_flight calls")
	require.Empty(t, kafkaClient.MarkedOffsets()[topic], "must not mark while the first wave is still in Consume")

	for i := range inFlight {
		close(releases[i])
	}
	for range inFlight {
		select {
		case <-entered:
		case <-timeout:
			t.Fatal("second wave did not start")
		}
	}
	requireNoEntry(t, entered, "started more than max_in_flight calls in the second wave")

	marked := kafkaClient.MarkedOffsets()[topic]
	require.Len(t, marked, 1)
	require.Equal(t, int64(inFlight), marked[0].Offset)

	for i := inFlight; i < records; i++ {
		close(releases[i])
	}
	result := <-done
	require.Nil(t, result.rewindRecord)
	marked = kafkaClient.MarkedOffsets()[topic]
	require.Len(t, marked, 1)
	require.Equal(t, int64(records), marked[0].Offset)
}

func TestProcessPartitionBatchSkipConsume(t *testing.T) {
	const (
		topic    = "test"
		inFlight = 2
	)

	t.Run("rewind skips later success", func(t *testing.T) {
		consumer, kafkaClient, partitionConsumer := newMaxInFlightConsumer(t, inFlight)
		entered := make(chan struct{}, 8)
		releases := make([]chan struct{}, 5)
		for i := range releases {
			releases[i] = make(chan struct{})
		}
		var failAt atomic.Int64
		failAt.Store(2)
		var secondBatch atomic.Bool
		calls := make([]atomic.Int64, 5)
		consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			calls[record.Offset].Add(1)
			if !secondBatch.Load() {
				entered <- struct{}{}
				<-releases[record.Offset]
			}
			if record.Offset == failAt.Load() {
				return errors.New("boom")
			}
			return nil
		}

		done := make(chan partitionBatchResult, 1)
		go func() {
			done <- consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatch(5))
		}()

		timeout := time.After(10 * time.Second)
		for range inFlight {
			select {
			case <-entered:
			case <-timeout:
				t.Fatal("first pair did not start")
			}
		}
		close(releases[0])
		close(releases[1])
		for range inFlight {
			select {
			case <-entered:
			case <-timeout:
				t.Fatal("second pair did not start")
			}
		}
		requireNoEntry(t, entered, "started offset 4 before the hole failed")
		close(releases[2])
		close(releases[3])
		close(releases[4])

		result := <-done
		require.NotNil(t, result.rewindRecord)
		require.Equal(t, int64(2), result.rewindRecord.Offset)
		marked := kafkaClient.MarkedOffsets()[topic]
		require.Len(t, marked, 1)
		require.Equal(t, int64(2), marked[0].Offset)
		require.Equal(t, int64(2), calls[2].Load())
		require.Equal(t, int64(1), calls[3].Load())

		secondBatch.Store(true)
		result = consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatchFrom(2, 3))
		require.NotNil(t, result.rewindRecord)
		require.Equal(t, int64(1), calls[3].Load(), "same-worker rewind must not Consume offset 3 again")
		require.LessOrEqual(t, calls[4].Load(), int64(1))

		result = consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatchFrom(2, 3))
		require.NotNil(t, result.rewindRecord)
		require.Equal(t, int64(1), calls[3].Load(), "repeated rewind must still skip offset 3")

		failAt.Store(-1)
		result = consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatchFrom(2, 3))
		require.Nil(t, result.rewindRecord)
		require.Equal(t, int64(1), calls[3].Load())
		require.Nil(t, partitionConsumer.skipConsume)
		marked = kafkaClient.MarkedOffsets()[topic]
		require.Len(t, marked, 1)
		require.Equal(t, int64(5), marked[0].Offset)
	})

	t.Run("pause does not remember", func(t *testing.T) {
		consumer, _, partitionConsumer := newMaxInFlightConsumer(t, inFlight)
		consumer.config.ErrorBackOff.Enabled = false
		partitionConsumer.backOff = nil
		entered := make(chan struct{}, 8)
		releases := make([]chan struct{}, 5)
		for i := range releases {
			releases[i] = make(chan struct{})
		}
		var secondBatch atomic.Bool
		calls := make([]atomic.Int64, 5)
		consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			calls[record.Offset].Add(1)
			if !secondBatch.Load() {
				entered <- struct{}{}
				<-releases[record.Offset]
			}
			if record.Offset == 2 {
				return errors.New("boom")
			}
			return nil
		}

		done := make(chan partitionBatchResult, 1)
		go func() {
			done <- consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatch(5))
		}()

		timeout := time.After(10 * time.Second)
		for range inFlight {
			select {
			case <-entered:
			case <-timeout:
				t.Fatal("first pair did not start")
			}
		}
		close(releases[0])
		close(releases[1])
		for range inFlight {
			select {
			case <-entered:
			case <-timeout:
				t.Fatal("second pair did not start")
			}
		}
		close(releases[2])
		close(releases[3])
		close(releases[4])

		result := <-done
		require.True(t, result.terminal)
		require.Nil(t, result.rewindRecord)
		require.Equal(t, int64(1), calls[3].Load())

		secondBatch.Store(true)
		result = consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatchFrom(2, 3))
		require.True(t, result.terminal)
		require.Equal(t, int64(2), calls[3].Load(), "pause path must not skip Consume on a later batch")
	})

	t.Run("later fail drops prefix skip", func(t *testing.T) {
		consumer, kafkaClient, partitionConsumer := newMaxInFlightConsumer(t, inFlight)
		partitionConsumer.skipConsume = map[int64]struct{}{3: {}}
		calls := make([]atomic.Int64, 5)
		consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			calls[record.Offset].Add(1)
			if record.Offset == 4 {
				return errors.New("boom")
			}
			return nil
		}

		result := consumer.processPartitionBatch(t.Context(), partitionConsumer, offsetBatchFrom(2, 3))
		require.NotNil(t, result.rewindRecord)
		require.Equal(t, int64(4), result.rewindRecord.Offset)
		require.Equal(t, int64(0), calls[3].Load())
		_, skipped3 := partitionConsumer.skipConsume[3]
		require.False(t, skipped3)
		marked := kafkaClient.MarkedOffsets()[topic]
		require.Len(t, marked, 1)
		require.Equal(t, int64(4), marked[0].Offset)
	})
}

// requireNoEntry fails if a Consume call arrives within a short wait.
func requireNoEntry(t *testing.T, entered <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-entered:
		t.Fatal(msg)
	case <-time.After(50 * time.Millisecond):
	}
}

// newMaxInFlightConsumer returns a consumer owning partition 0 of topic, with
// independent processing and error backoff so failures rewind instead of pause.
func newMaxInFlightConsumer(tb testing.TB, maxInFlight int) (*franzConsumer, *kgo.Client, *pc) {
	return newIndependentConsumer(tb, "test", maxInFlight, MessageMarking{After: true}, true)
}

func newIndependentConsumer(
	tb testing.TB,
	topic string,
	maxInFlight int,
	marking MessageMarking,
	backoff bool,
) (*franzConsumer, *kgo.Client, *pc) {
	kafkaClient, cfg := mustNewMarkedFakeCluster(tb, kfake.SeedTopics(1, topic))
	cfg.PartitionProcessing.Independent = true
	cfg.PartitionProcessing.MaxInFlight = maxInFlight
	cfg.MessageMarking = marking
	cfg.MessageMarking.OnPermanentError = marking.OnError
	if backoff {
		cfg.ErrorBackOff = configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     time.Millisecond,
			RandomizationFactor: 0,
			Multiplier:          1.5,
			MaxInterval:         time.Millisecond,
			MaxElapsedTime:      time.Nanosecond,
		}
	}
	settings, _, _ := mustNewSettings(tb)
	consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, nil)
	require.NoError(tb, err)
	consumer.client = kafkaClient

	ctx, cancel := context.WithCancelCause(tb.Context())
	tb.Cleanup(func() { cancel(nil) })
	partitionConsumer := &pc{
		ctx:     ctx,
		cancel:  cancel,
		attrs:   attribute.NewSet(),
		logger:  zap.NewNop(),
		backOff: newExponentialBackOff(cfg.ErrorBackOff),
	}
	consumer.assignments[topicPartition{topic: topic, partition: 0}] = partitionConsumer
	return consumer, kafkaClient, partitionConsumer
}

// offsetBatch builds a fetched batch of n records on partition 0, offsets 0..n-1.
func offsetBatch(n int) kgo.FetchTopicPartition {
	return offsetBatchFrom(0, n)
}

// offsetBatchFrom builds n records on partition 0 starting at offset start.
func offsetBatchFrom(start, n int) kgo.FetchTopicPartition {
	topic := "test"
	records := make([]*kgo.Record, n)
	for i := range records {
		records[i] = &kgo.Record{Topic: topic, Partition: 0, Offset: int64(start + i)}
	}
	return kgo.FetchTopicPartition{
		Topic: topic,
		FetchPartition: kgo.FetchPartition{
			Partition:     0,
			HighWatermark: int64(start + n),
			Records:       records,
		},
	}
}

// BenchmarkProcessPartitionBatchMaxInFlight compares default max_in_flight 1
// with 4. One op is one partition fetch. Consume waits, as a blocked next
// consumer does.
func BenchmarkProcessPartitionBatchMaxInFlight(b *testing.B) {
	const (
		topic       = "test"
		records     = 32
		consumeWait = 2 * time.Millisecond
	)
	batch := offsetBatch(records)
	consume := func(context.Context, *kgo.Record, attribute.Set) error {
		time.Sleep(consumeWait)
		return nil
	}
	for _, n := range []int{1, 4} {
		b.Run("max_in_flight_"+strconv.Itoa(n), func(b *testing.B) {
			consumer, _, partitionConsumer := newMaxInFlightConsumer(b, n)
			consumer.consumeMessage = consume
			for b.Loop() {
				consumer.processPartitionBatch(b.Context(), partitionConsumer, batch)
			}
		})
	}
}
