// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/cenkalti/backoff/v4"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

type partitionPauseReason uint32

const (
	partitionPauseBackpressure partitionPauseReason = 1 << iota
	partitionPauseRewind
)

// partitionBatchResult describes the rewind work produced while processing one
// fetched partition batch.
type partitionBatchResult struct {
	rewindRecord *kgo.Record
	terminal     bool
}

// pc represents the partition consumer shared information.
type pc struct {
	logger *zap.Logger
	attrs  attribute.Set

	ctx    context.Context
	cancel context.CancelCauseFunc
	// Not safe for concurrent use. handleMessage clones it when max_in_flight
	// is above 1.
	backOff *backoff.ExponentialBackOff

	mailbox *partitionMailbox
	// skipConsume holds offsets this worker already finished above a rewind
	// hole. It stays nil until a rewind writes it. Entries drop when the
	// marked prefix advances past them, and the map dies with pc on rebalance.
	skipConsume map[int64]struct{}
	// pauseReasons stores a bitmask of partitionPauseReason values.
	pauseReasons atomic.Uint32

	// mu prevents cancellation from racing a new wg.Add.
	mu sync.RWMutex
	// wg tracks the number of in-flight message processing goroutines for this
	// partition. New goroutines must call add() so none are added after the
	// partition consumer starts stopping.
	wg sync.WaitGroup
}

// add increments the wait group counter if the partition consumer is not
// stopping. It returns true if the counter was incremented, false otherwise.
func (p *pc) add() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.ctx.Err() != nil {
		return false
	}
	p.wg.Add(1)
	return true
}

// cancelContext cancels the partition consumer context while holding the write
// lock.
func (p *pc) cancelContext(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancel(err)
}

// addPauseReason records why fetching must remain paused.
func (p *pc) addPauseReason(reason partitionPauseReason) {
	p.pauseReasons.Or(uint32(reason))
}

// clearPauseReasons reports whether the cleared reasons were the final active
// reasons, in which case the caller must resume fetching.
func (p *pc) clearPauseReasons(reasons partitionPauseReason) bool {
	mask := uint32(reasons)
	previous := p.pauseReasons.And(^mask)
	remaining := previous &^ mask
	return previous&mask != 0 && remaining == 0
}

// logProcessError reports a record the pipeline rejected. A cancelled partition
// is an expected shutdown path, so it logs at debug level.
func (p *pc) logProcessError(record *kgo.Record, err error) {
	fields := []zap.Field{zap.Error(err), zap.Int64("offset", record.Offset)}
	if p.ctx.Err() != nil {
		p.logger.Debug("message processing interrupted", fields...)
		return
	}
	p.logger.Error("unable to process message", fields...)
}

// runPartitionWorker processes exactly one partition in offset order. Workers
// are independent, so downstream backpressure on one partition does not stop
// the shared poll loop or workers for other partitions.
func (c *franzConsumer) runPartitionWorker(pc *pc, tp topicPartition) {
	defer pc.wg.Done()
	partition := map[string][]int32{tp.topic: {tp.partition}}
	for {
		select {
		case <-pc.ctx.Done():
			return
		case <-pc.mailbox.notify:
		}

		for {
			if pc.ctx.Err() != nil {
				return
			}
			batch, ok := pc.mailbox.dequeue(func() {
				if pc.clearPauseReasons(partitionPauseBackpressure) {
					c.client.ResumeFetchPartitions(partition)
				}
			})
			if !ok {
				if !pc.mailbox.hasPendingOffsetChange() {
					break
				}
				if !c.applyMailboxRewind(pc, tp, partition) {
					return
				}
				break
			}

			result := c.processPartitionBatch(pc.ctx, pc, batch)
			if result.rewindRecord != nil {
				pc.mailbox.requestRewind(result.rewindRecord, true, func() {
					pc.addPauseReason(partitionPauseRewind)
					c.client.PauseFetchPartitions(partition)
				})
			}
			if result.rewindRecord != nil {
				if !c.applyMailboxRewind(pc, tp, partition) {
					return
				}
				break
			}
			if result.terminal {
				pc.cancelContext(errors.New("stopping processing: terminal partition error"))
				pc.mailbox.discard()
				return
			}
		}
	}
}

// applyMailboxRewind asks the poll loop to apply the pending rewind, then
// resumes fetching. Returns false if this partition or the receiver is stopping.
func (c *franzConsumer) applyMailboxRewind(pc *pc, tp topicPartition, partition map[string][]int32) bool {
	if !c.sendControl(partitionControl{tp: tp, pc: pc}) {
		return false
	}
	pc.mailbox.resumeAfterOffsetChange(func() {
		if pc.clearPauseReasons(partitionPauseRewind | partitionPauseBackpressure) {
			c.client.ResumeFetchPartitions(partition)
		}
	})
	return true
}

// processPartitionBatch processes one partition batch in both legacy and
// independent modes.
//
// Marking and committing depend on the processing mode:
//   - Legacy with autocommit enabled marks before or after processing according
//     to MessageMarking.After. franz-go commits the marks periodically.
//   - Independent with autocommit enabled behaves the same after a worker
//     dequeues the batch. Records still waiting in the mailbox are not marked.
//   - Legacy with autocommit disabled marks records here. consume commits all
//     marked partitions after the fetched batch finishes.
//   - Independent with max_in_flight above 1 always marks after processing,
//     because concurrent calls have no single record to mark before.
//     processRecordsConcurrent also marks the contiguous prefix as soon as
//     earlier in-flight records finish.
func (c *franzConsumer) processPartitionBatch(ctx context.Context, pc *pc, p kgo.FetchTopicPartition) partitionBatchResult {
	var fatalRecord *kgo.Record
	fatalIsPermanent := false
	var lastProcessed *kgo.Record
	concurrent := c.maxInFlight() > 1
	if concurrent {
		fatalRecord, fatalIsPermanent, lastProcessed = c.processRecordsConcurrent(ctx, pc, p)
	} else {
		for _, msg := range p.Records {
			if !c.config.MessageMarking.After {
				c.markCommitRecords(pc, p.Topic, p.Partition, msg)
			}
			c.telemetryBuilder.KafkaReceiverCurrentOffset.Record(ctx, msg.Offset, metric.WithAttributeSet(pc.attrs))
			if err := c.handleMessage(pc, msg); err != nil {
				pc.logProcessError(msg, err)
				if !c.shouldMark(err) {
					fatalRecord = msg
					fatalIsPermanent = consumererror.IsPermanent(err)
					break
				}
			}
			lastProcessed = msg
		}
	}

	result := partitionBatchResult{}
	if fatalRecord != nil {
		switch {
		case c.config.ErrorBackOff.Enabled && !fatalIsPermanent:
			// Skip rewind if the consumer is shutting down or the partition was
			// lost. In these cases the error is from context cancellation, not
			// a real processing failure, and calling SetOffsets could interfere
			// with the final offset commit.
			select {
			case <-pc.ctx.Done():
				result.terminal = true
			case <-c.closing:
				result.terminal = true
			default:
				if c.config.PartitionProcessing.Independent {
					result.rewindRecord = fatalRecord
				} else {
					// PollRecords is blocked on wg.Wait() until this goroutine
					// finishes, so SetOffsets is enough. The next poll retries
					// the failed record without a pause.
					c.client.SetOffsets(map[string]map[int32]kgo.EpochOffset{
						p.Topic: {p.Partition: {
							Epoch:  fatalRecord.LeaderEpoch,
							Offset: fatalRecord.Offset,
						}},
					})
				}
				pc.logger.Info("rewinding partition to retry failed record on next poll",
					zap.Int64("offset", fatalRecord.Offset),
				)
			}
		default:
			// No pause reason is needed because this worker exits permanently.
			// On reassignment, a new partition consumer resumes fetching with
			// no pause reasons.
			c.mu.RLock()
			current := c.assignments[topicPartition{topic: p.Topic, partition: p.Partition}]
			// A worker can pause only while it owns the assignment and remains active.
			canPause := current == pc && pc.ctx.Err() == nil
			if canPause {
				// Pause to prevent later records from passing the failed,
				// unmarked record.
				c.client.PauseFetchPartitions(map[string][]int32{p.Topic: {p.Partition}})
			}
			c.mu.RUnlock()
			if !canPause {
				// Exit without pausing because a replacement worker can now own
				// the partition.
				result.terminal = true
				break
			}
			if fatalIsPermanent {
				pc.logger.Error("pausing partition due to permanent processing error, partition will remain paused until rebalance",
					zap.Int64("offset", fatalRecord.Offset),
				)
			} else {
				pc.logger.Error("pausing partition due to processing error (no backoff configured), partition will remain paused until rebalance",
					zap.Int64("offset", fatalRecord.Offset),
				)
			}
			result.terminal = true
		}
	}
	if lastProcessed == nil {
		return result
	}
	c.telemetryBuilder.KafkaReceiverOffsetLag.Record(
		ctx,
		(p.HighWatermark-1)-lastProcessed.Offset,
		metric.WithAttributeSet(pc.attrs),
	)
	if c.config.MessageMarking.After || concurrent {
		// Marking one record commits the contiguous accepted prefix.
		c.markCommitRecords(pc, p.Topic, p.Partition, lastProcessed)
	}
	return result
}

// markCommitRecords marks records for later commit. Independent mode also
// checks assignment ownership so a timed-out worker cannot mark after a
// replacement starts. Marks are keyed by topic-partition.
func (c *franzConsumer) markCommitRecords(pc *pc, topic string, partition int32, records ...*kgo.Record) {
	if c.config.PartitionProcessing.Independent {
		c.mu.RLock()
		defer c.mu.RUnlock()
		if c.assignments[topicPartition{topic: topic, partition: partition}] != pc {
			return
		}
	}
	c.client.MarkCommitRecords(records...)
}

// shouldMark reports whether message_marking allows marking a record that
// failed with err. handleMessage returns an error only when it already refused
// to mark, so batch callers repeat this check to stay consistent.
func (c *franzConsumer) shouldMark(err error) bool {
	if consumererror.IsPermanent(err) {
		return c.config.MessageMarking.OnPermanentError
	}
	return c.config.MessageMarking.OnError
}

// maxInFlight returns how many handleMessage calls one partition worker may run
// at once. Only independent workers go above 1.
func (c *franzConsumer) maxInFlight() int {
	if c.config.PartitionProcessing.Independent {
		return c.config.PartitionProcessing.MaxInFlight
	}
	return 1
}

// processRecordsConcurrent runs up to maxInFlight handleMessage calls at once
// on one partition. It marks the successful prefix as records finish. An
// unmarked failure is retried once in memory. If that retry fails, this worker
// rewinds and skips Consume for offsets it already finished above the hole. It
// returns that unmarked record, whether the error is permanent, and the last
// record of the successful prefix.
func (c *franzConsumer) processRecordsConcurrent(ctx context.Context, pc *pc, p kgo.FetchTopicPartition) (fatalRecord *kgo.Record, fatalIsPermanent bool, lastProcessed *kgo.Record) {
	records := p.Records
	sem := make(chan struct{}, c.maxInFlight())
	finished := make([]atomic.Bool, len(records))
	errs := make([]error, len(records))
	var markMu sync.Mutex
	markedThrough := -1
	advanceMark := func() {
		markMu.Lock()
		defer markMu.Unlock()
		start := markedThrough
		base := markedThrough + 1
		for k := range records[base:] {
			next := base + k
			if !finished[next].Load() {
				break
			}
			if err := errs[next]; err != nil && !c.shouldMark(err) {
				break
			}
			markedThrough = next
		}
		if markedThrough > start {
			c.markCommitRecords(pc, p.Topic, p.Partition, records[markedThrough])
		}
	}

	for pos := 0; pos < len(records); {
		var (
			wg      sync.WaitGroup
			failed  atomic.Bool
			started int
		)
		for rel, msg := range records[pos:] {
			i := pos + rel
			if _, ok := pc.skipConsume[msg.Offset]; ok {
				finished[i].Store(true)
				started++
				advanceMark()
				continue
			}
			sem <- struct{}{}
			// Stop starting once a record fails. The hole is retried below.
			if failed.Load() {
				<-sem
				break
			}
			c.telemetryBuilder.KafkaReceiverCurrentOffset.Record(ctx, msg.Offset, metric.WithAttributeSet(pc.attrs))
			started++
			wg.Go(func() {
				err := c.handleMessage(pc, msg)
				if err != nil {
					errs[i] = err
					if !c.shouldMark(err) {
						failed.Store(true)
					} else {
						pc.logProcessError(msg, err)
					}
				}
				finished[i].Store(true)
				advanceMark()
				<-sem
			})
		}
		wg.Wait()

		for j, msg := range records[pos : pos+started] {
			idx := pos + j
			err := errs[idx]
			if err != nil && !c.shouldMark(err) {
				if c.config.ErrorBackOff.Enabled && !consumererror.IsPermanent(err) {
					// One extra Consume, no new backoff. handleMessage already
					// used the configured max_elapsed_time.
					err = c.consumeMessage(pc.ctx, msg, pc.attrs)
					if err == nil || c.shouldMark(err) {
						errs[idx] = err
						lastProcessed = msg
						continue
					}
					c.rememberSkipConsume(pc, records[idx+1:pos+started], errs[idx+1:pos+started])
				}
				pc.logProcessError(msg, err)
				pc.dropSkipConsume(markedOffset(records, markedThrough))
				return msg, consumererror.IsPermanent(err), lastProcessed
			}
			lastProcessed = msg
		}
		advanceMark()
		pc.dropSkipConsume(markedOffset(records, markedThrough))
		if started == 0 {
			break
		}
		pos += started
	}
	return nil, false, lastProcessed
}

func markedOffset(records []*kgo.Record, markedThrough int) int64 {
	if markedThrough < 0 {
		return -1
	}
	return records[markedThrough].Offset
}

// dropSkipConsume removes skip entries at or below the marked prefix. The map
// returns to nil when it is empty so later success batches do not keep a set.
func (p *pc) dropSkipConsume(through int64) {
	if p.skipConsume == nil {
		return
	}
	if through >= 0 {
		for off := range p.skipConsume {
			if off <= through {
				delete(p.skipConsume, off)
			}
		}
	}
	if len(p.skipConsume) == 0 {
		p.skipConsume = nil
	}
}

// rememberSkipConsume records offsets above a rewind hole that this worker
// already finished, so the next fetch on this pc does not Consume them again.
func (c *franzConsumer) rememberSkipConsume(pc *pc, records []*kgo.Record, errs []error) {
	for i, rec := range records {
		if errs[i] != nil && !c.shouldMark(errs[i]) {
			continue
		}
		if pc.skipConsume == nil {
			pc.skipConsume = make(map[int64]struct{}, len(records))
		}
		pc.skipConsume[rec.Offset] = struct{}{}
	}
}
