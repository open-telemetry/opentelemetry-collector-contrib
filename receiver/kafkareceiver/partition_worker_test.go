// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"bytes"
	"context"
	"errors"
	"runtime"
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

// TestProcessPartitionBatchStopsWhenCancelled checks that the batch loop stops
// once the partition consumer is cancelled, so the rest of the batch stays
// unmarked. A revocation hands it to the next owner, a shutdown to the next run.
func TestProcessPartitionBatchStopsWhenCancelled(t *testing.T) {
	const topic = "test"
	const records = 10

	cases := []struct {
		name             string
		independent      bool
		marking          MessageMarking
		shutdown         bool
		cancelBeforeLoop bool
		wantConsumed     int
		wantMarkedOffset int64
	}{
		{
			// The default marks a record before processing it, so nothing but
			// the loop itself keeps the rest of the batch from being marked.
			name:             "legacy default marking",
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			name:             "independent default marking",
			independent:      true,
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			// on_error marks records the pipeline refused, but a cancelled
			// record was never offered to it, so the interrupted one stays
			// unmarked too.
			name:             "legacy after marking with on_error",
			marking:          MessageMarking{After: true, OnError: true},
			wantConsumed:     1,
			wantMarkedOffset: -1,
		},
		{
			name:             "independent after marking with on_error",
			independent:      true,
			marking:          MessageMarking{After: true, OnError: true},
			wantConsumed:     1,
			wantMarkedOffset: -1,
		},
		{
			// Cancelled before the first record: nothing is consumed and
			// nothing is marked, so the whole batch is redelivered.
			name:             "cancelled before the first record",
			cancelBeforeLoop: true,
			wantConsumed:     0,
			wantMarkedOffset: -1,
		},
		{
			// Shutdown reaches this loop as a cancelled context, not as a
			// closed c.closing: triggerShutdown closes c.closing, closes the
			// client, and franz-go then calls lost(), which cancels. So a
			// pending shutdown must not change the outcome here. Reading
			// c.closing to finish the batch instead would mark records against
			// a context that is already cancelled, and the next run would never
			// see them again.
			name:             "pending shutdown does not resume the batch",
			shutdown:         true,
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			name:             "independent pending shutdown does not resume the batch",
			independent:      true,
			shutdown:         true,
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kafkaClient, cfg := mustNewMarkedFakeCluster(t, kfake.SeedTopics(1, topic))
			cfg.PartitionProcessing.Independent = tc.independent
			cfg.MessageMarking = tc.marking
			settings, _, _ := mustNewSettings(t)
			consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, nil)
			require.NoError(t, err)
			consumer.client = kafkaClient
			if tc.shutdown {
				close(consumer.closing)
			}

			ctx, cancel := context.WithCancelCause(t.Context())
			t.Cleanup(func() { cancel(nil) })
			partitionConsumer := &pc{
				logger: settings.Logger,
				ctx:    ctx,
				cancel: cancel,
				attrs:  attribute.NewSet(),
			}
			consumer.assignments[topicPartition{topic: topic, partition: 0}] = partitionConsumer

			// Stands in for lost(), which cancels the partition context before
			// it waits for the in-flight call.
			cancelPartition := func() {
				partitionConsumer.cancelContext(errors.New("stopping processing"))
			}
			if tc.cancelBeforeLoop {
				cancelPartition()
			}

			consumed := 0
			consumer.consumeMessage = func(msgCtx context.Context, _ *kgo.Record, _ attribute.Set) error {
				consumed++
				if consumed == 1 && !tc.cancelBeforeLoop {
					cancelPartition()
				}
				return msgCtx.Err()
			}

			batch := kgo.FetchTopicPartition{Topic: topic}
			for i := range records {
				batch.Records = append(batch.Records, &kgo.Record{
					Topic: topic, Partition: 0, Offset: int64(i),
				})
			}
			batch.HighWatermark = records

			consumer.processPartitionBatch(partitionConsumer, batch)

			require.Equal(t, tc.wantConsumed, consumed,
				"records sent to the pipeline")
			marked := kafkaClient.MarkedOffsets()[topic]
			if tc.wantMarkedOffset < 0 {
				// Nothing counted as processed, so nothing may be marked and
				// there is no lag to report.
				require.Empty(t, marked, "an unprocessed batch must stay unmarked")
				return
			}
			require.Len(t, marked, 1)
			require.Equal(t, tc.wantMarkedOffset, marked[0].Offset,
				"marked records are committed and never redelivered")

			// The loop breaks instead of returning, so the records it did
			// process still report their lag.
			require.True(t, partitionConsumer.offsetLagReportable.Load(),
				"a partition that processed records reports its lag")
			require.Equal(t, (records-1)-(tc.wantMarkedOffset-1),
				partitionConsumer.offsetLag.Load())
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
			consumer.processPartitionBatch(partitionConsumer, batch)

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
		failAt     int64
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
			name:       "marks the successful prefix and rewinds when a record fails",
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
				calls[record.Offset].Add(1)
				entered <- struct{}{}
				<-releases[record.Offset]
				if record.Offset == tc.failAt {
					return errors.New("boom")
				}
				return nil
			}

			done := make(chan partitionBatchResult, 1)
			go func() {
				done <- consumer.processPartitionBatch(partitionConsumer, offsetBatch(records))
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
				require.Equal(t, int64(1), calls[tc.failAt].Load())
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

func TestProcessPartitionBatchMaxInFlightBytes(t *testing.T) {
	const recordSize = 10
	const records = 4
	cases := []struct {
		name string
		// bytes is the max_in_flight.bytes cap.
		bytes int
		// wantConcurrent is how many calls may be in Consume at once.
		wantConcurrent int
	}{
		{name: "byte cap limits calls below the record cap", bytes: 2 * recordSize, wantConcurrent: 2},
		{name: "record above the byte cap runs alone", bytes: recordSize - 1, wantConcurrent: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The record cap is above the batch size, so only bytes can limit.
			consumer, _, partitionConsumer := newMaxInFlightConsumer(t, 2*records)
			consumer.config.PartitionProcessing.MaxInFlight.Bytes = tc.bytes
			entered := make(chan struct{}, records)
			release := make(chan struct{}, records)
			consumer.consumeMessage = func(context.Context, *kgo.Record, attribute.Set) error {
				entered <- struct{}{}
				<-release
				return nil
			}
			batch := offsetBatch(records)
			for _, r := range batch.Records {
				// Key and value both count towards the size.
				r.Key = make([]byte, recordSize/2)
				r.Value = make([]byte, recordSize/2)
			}

			done := make(chan partitionBatchResult, 1)
			go func() { done <- consumer.processPartitionBatch(partitionConsumer, batch) }()

			// Each wave must fill the cap again once the previous one releases. A
			// leaked byte count would shrink the later waves.
			for range records / tc.wantConcurrent {
				for range tc.wantConcurrent {
					select {
					case <-entered:
					case <-time.After(10 * time.Second):
						t.Fatalf("fewer than %d Consume calls overlapped", tc.wantConcurrent)
					}
				}
				requireNoEntry(t, entered, "started more calls than the byte cap allows")
				for range tc.wantConcurrent {
					release <- struct{}{}
				}
			}
			require.Nil(t, (<-done).rewindRecord)
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
		done <- consumer.processPartitionBatch(partitionConsumer, offsetBatch(records))
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
			done <- consumer.processPartitionBatch(partitionConsumer, offsetBatch(5))
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
		// Offset 4 may start if record 3 frees a slot before record 2 fails.
		// Later asserts do not read that offset.
		close(releases[2])
		close(releases[3])
		close(releases[4])

		result := <-done
		require.NotNil(t, result.rewindRecord)
		require.Equal(t, int64(2), result.rewindRecord.Offset)
		marked := kafkaClient.MarkedOffsets()[topic]
		require.Len(t, marked, 1)
		require.Equal(t, int64(2), marked[0].Offset)
		require.Equal(t, int64(1), calls[2].Load())
		require.Equal(t, int64(1), calls[3].Load())

		secondBatch.Store(true)
		result = consumer.processPartitionBatch(partitionConsumer, offsetBatchFrom(2, 3))
		require.NotNil(t, result.rewindRecord)
		require.Equal(t, int64(1), calls[3].Load(), "same-worker rewind must not Consume offset 3 again")

		failAt.Store(-1)
		result = consumer.processPartitionBatch(partitionConsumer, offsetBatchFrom(2, 3))
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
		entered := make(chan struct{}, 8)
		releases := make([]chan struct{}, 5)
		for i := range releases {
			releases[i] = make(chan struct{})
		}
		calls := make([]atomic.Int64, 5)
		consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
			calls[record.Offset].Add(1)
			entered <- struct{}{}
			<-releases[record.Offset]
			if record.Offset == 2 {
				return errors.New("boom")
			}
			return nil
		}

		done := make(chan partitionBatchResult, 1)
		go func() {
			done <- consumer.processPartitionBatch(partitionConsumer, offsetBatch(5))
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
		// Offset 4 may start if record 3 frees a slot before record 2 fails.
		// Later asserts do not read that offset.
		close(releases[2])
		close(releases[3])
		close(releases[4])

		result := <-done
		require.True(t, result.terminal)
		require.Nil(t, result.rewindRecord)
		require.Equal(t, int64(1), calls[3].Load())
		require.Nil(t, partitionConsumer.skipConsume, "pause path must not remember finished offsets")
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

		result := consumer.processPartitionBatch(partitionConsumer, offsetBatchFrom(2, 3))
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

func TestProcessPartitionBatchCancelledSkipsExtraConsume(t *testing.T) {
	consumer, _, partitionConsumer := newMaxInFlightConsumer(t, 2)
	entered := make(chan struct{})
	releases := make([]chan struct{}, 2)
	for i := range releases {
		releases[i] = make(chan struct{})
	}
	var calls atomic.Int64
	consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
		calls.Add(1)
		if record.Offset >= 2 {
			return nil
		}
		entered <- struct{}{}
		<-releases[record.Offset]
		return nil
	}

	done := make(chan partitionBatchResult, 1)
	go func() {
		done <- consumer.processPartitionBatch(partitionConsumer, offsetBatch(3))
	}()

	timeout := time.After(10 * time.Second)
	for range 2 {
		select {
		case <-entered:
		case <-timeout:
			t.Fatal("in-flight Consume calls did not start")
		}
	}
	// The next acquire must already be waiting for room. A cancel
	// before that wait ends at the loop's context check and never enters acquire.
	waitAcquireBlocked(t)
	partitionConsumer.cancel(context.Canceled)
	close(releases[0])
	close(releases[1])

	var result partitionBatchResult
	select {
	case result = <-done:
	case <-timeout:
		t.Fatal("batch did not return")
	}
	require.False(t, result.terminal)
	require.Nil(t, result.rewindRecord)
	require.Equal(t, int64(2), calls.Load())
	require.Nil(t, partitionConsumer.skipConsume)
}

func TestProcessPartitionBatchCancelDoesNotStartNext(t *testing.T) {
	consumer, _, partitionConsumer := newMaxInFlightConsumer(t, 1)
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	consumer.consumeMessage = func(_ context.Context, record *kgo.Record, _ attribute.Set) error {
		calls.Add(1)
		if record.Offset == 0 {
			entered <- struct{}{}
			<-release
		}
		return nil
	}

	done := make(chan partitionBatchResult, 1)
	go func() {
		done <- consumer.processPartitionBatch(partitionConsumer, offsetBatch(2))
	}()

	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("first Consume did not start")
	}
	partitionConsumer.cancel(context.Canceled)
	close(release)

	var result partitionBatchResult
	select {
	case result = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("batch did not return")
	}
	require.Nil(t, result.rewindRecord)
	require.Equal(t, int64(1), calls.Load())
}

// waitAcquireBlocked returns once a goroutine is blocked in inflightBatch.acquire.
func waitAcquireBlocked(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	buf := make([]byte, 64<<10)
	for time.Now().Before(deadline) {
		for {
			n := runtime.Stack(buf, true)
			if n < len(buf) {
				if bytes.Contains(buf[:n], []byte("inflightBatch).acquire")) {
					return
				}
				break
			}
			buf = make([]byte, len(buf)*2)
		}
		runtime.Gosched()
	}
	t.Fatal("timed out waiting for acquire to block")
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
	cfg.PartitionProcessing.MaxInFlight.Records = maxInFlight
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
		ctx:    ctx,
		cancel: cancel,
		attrs:  attribute.NewSet(),
		logger: zap.NewNop(),
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
				consumer.processPartitionBatch(partitionConsumer, batch)
			}
		})
	}
}
