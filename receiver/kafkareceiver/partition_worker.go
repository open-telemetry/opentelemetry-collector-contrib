// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/otel/attribute"
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

	mailbox *partitionMailbox
	// skipConsume holds offsets this worker already finished above a rewind
	// hole. It stays nil until a rewind writes it. Entries drop when the
	// marked prefix advances past them, and the map dies with pc on rebalance.
	skipConsume map[int64]struct{}
	// pauseReasons stores a bitmask of partitionPauseReason values.
	pauseReasons atomic.Uint32

	// offsetLag holds the last reported offset lag
	offsetLag atomic.Int64
	// offsetLagReportable flag to track when an active partition has a reportable offset lag
	offsetLagReportable atomic.Bool

	// currentOffset holds the offset of the last record handed to message processing
	currentOffset atomic.Int64
	// currentOffsetReportable flag to track when an active partition has a reportable current offset
	currentOffsetReportable atomic.Bool

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

			result := c.processPartitionBatch(pc, batch)
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
func (c *franzConsumer) processPartitionBatch(pc *pc, p kgo.FetchTopicPartition) partitionBatchResult {
	var fatalRecord *kgo.Record
	fatalIsPermanent := false
	var lastProcessed *kgo.Record
	concurrent := c.maxInFlight() > 1
	if concurrent {
		fatalRecord, fatalIsPermanent, lastProcessed = c.processRecordsConcurrent(pc, p)
	} else {
		for _, msg := range p.Records {
			// Stop before marking once the partition consumer is cancelled. The
			// records left here are still committable, and lost() commits the marks,
			// so marking one now drops it: nothing processed it and nothing
			// redelivers it. Leaving them unmarked hands them to the next owner
			// after a revocation, or to the next run after a shutdown.
			//
			// This also keeps the wait in lost() down to the in-flight record
			// instead of the whole batch, and that wait has to fit in the re-balance
			// timeout.
			//
			// break, not return, so processed records still get their After mark and
			// lag telemetry below.
			if pc.ctx.Err() != nil {
				pc.logger.Debug("stopped processing records, leaving the rest of the batch unmarked",
					zap.Int64("offset", msg.Offset),
				)
				break
			}
			if !c.config.MessageMarking.After {
				c.markCommitRecords(pc, p.Topic, p.Partition, msg)
			}
			// Record the current consumer offset.
			pc.currentOffset.Store(msg.Offset)
			if err := c.handleMessage(pc, msg); err != nil {
				pc.logProcessError(msg, err)
				fatalRecord = msg
				fatalIsPermanent = consumererror.IsPermanent(err)
				break
			}
			lastProcessed = msg
		}
	}

	result := partitionBatchResult{}
	terminallyPaused := false
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

				// Stop reporting lag and current offset because this partition
				// cannot resume without reassignment.
				pc.offsetLagReportable.Store(false)
				pc.currentOffsetReportable.Store(false)
				terminallyPaused = true
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

	if len(p.Records) > 0 && !terminallyPaused {
		// Every loop iteration stores the record offset before any break, so
		// currentOffset is set before the flag becomes observable.
		pc.currentOffsetReportable.Store(true)
	}

	if lastProcessed == nil {
		// no messages were processed, return early
		return result
	}

	// Record the current consumer lag.
	// Skip for terminally paused partitions. Reporting resumes after rebalance.
	if !terminallyPaused {
		pc.offsetLag.Store((p.HighWatermark - 1) - lastProcessed.Offset)
		pc.offsetLagReportable.Store(true)
	}

	if c.config.MessageMarking.After && !concurrent {
		// Mark the latest accepted record after processing. This also covers
		// every earlier record in the batch. The concurrent path marks as it goes.
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

// maxInFlight returns how many handleMessage calls one partition worker may run
// at once. Only independent workers go above 1.
func (c *franzConsumer) maxInFlight() int {
	if c.config.PartitionProcessing.Independent {
		return c.config.PartitionProcessing.MaxInFlight.Records
	}
	return 1
}

// processRecordsConcurrent runs up to maxInFlight handleMessage calls, and
// max_in_flight.bytes bytes of records, at once on one partition. It marks the
// contiguous accepted prefix as records finish. That prefix includes failures
// message marking skips. After an unmarked failure it starts no new calls and
// waits for the in-flight ones. It returns that record, whether the error is
// permanent, and the last record of the accepted prefix. Before a rewind it
// remembers the offsets it already finished above the hole, so this worker
// skips Consume for them on the next fetch.
func (c *franzConsumer) processRecordsConcurrent(pc *pc, p kgo.FetchTopicPartition) (fatalRecord *kgo.Record, fatalIsPermanent bool, lastProcessed *kgo.Record) {
	b := &inflightBatch{
		c:             c,
		pc:            pc,
		p:             p,
		errs:          make([]error, len(p.Records)),
		done:          make([]bool, len(p.Records)),
		markedThrough: -1,
	}
	b.room = sync.NewCond(&b.mu)
	for i, msg := range p.Records {
		if pc.ctx.Err() != nil {
			break
		}
		pc.currentOffset.Store(msg.Offset)
		if _, ok := pc.skipConsume[msg.Offset]; ok {
			b.complete(i, nil)
			continue
		}
		size := len(msg.Key) + len(msg.Value)
		if !b.acquire(size) {
			break
		}
		b.start(i, size)
	}
	b.wg.Wait()

	// wg.Wait orders every write to b before the reads below.
	if b.markedThrough >= 0 {
		lastProcessed = p.Records[b.markedThrough]
	}
	for i, err := range b.errs {
		if err == nil {
			continue
		}
		fatalRecord, fatalIsPermanent = p.Records[i], consumererror.IsPermanent(err)
		pc.logProcessError(fatalRecord, err)
		// Only a rewind on this pc retries the hole. A pause or a cancelled
		// partition hands the offsets to a new owner, which must Consume them.
		if c.config.ErrorBackOff.Enabled && !fatalIsPermanent && pc.ctx.Err() == nil {
			b.rememberFinished(i + 1)
		}
		break
	}
	pc.dropSkipConsume(lastProcessed)
	return fatalRecord, fatalIsPermanent, lastProcessed
}

// inflightBatch holds the state of one batch in processRecordsConcurrent.
type inflightBatch struct {
	c  *franzConsumer
	pc *pc
	p  kgo.FetchTopicPartition
	wg sync.WaitGroup

	mu sync.Mutex
	// room is signaled when a call finishes.
	room *sync.Cond
	// inflight and inflightBytes count the calls between acquire and release.
	inflight      int
	inflightBytes int
	// errs holds failures message marking does not skip. A non-nil entry is a
	// hole the accepted prefix cannot pass.
	errs []error
	done []bool
	// failed stops new calls once any record leaves a hole.
	failed bool
	// markedThrough is the index of the last marked record, or -1.
	markedThrough int
}

// acquire waits until a record of size bytes fits under both max_in_flight
// limits. It returns false when a record has failed or the partition context is
// done, so a revoked partition does not start another Consume after this wait.
func (b *inflightBatch) acquire(size int) bool {
	cfg := b.c.config.PartitionProcessing.MaxInFlight
	b.mu.Lock()
	defer b.mu.Unlock()
	// An empty window always admits one record, so a record above bytes runs alone
	// instead of blocking the partition. A waiter is woken by a call finishing,
	// so it sees a cancelled context at the next release.
	for !b.failed && b.pc.ctx.Err() == nil &&
		b.inflight > 0 && (b.inflight >= cfg.Records || (cfg.Bytes > 0 && b.inflightBytes+size > cfg.Bytes)) {
		b.room.Wait()
	}
	if b.failed || b.pc.ctx.Err() != nil {
		return false
	}
	b.inflight++
	b.inflightBytes += size
	return true
}

// start runs handleMessage for record i on its own goroutine. The caller must
// have called acquire with the same size.
func (b *inflightBatch) start(i, size int) {
	b.wg.Go(func() {
		b.complete(i, b.c.handleMessage(b.pc, b.p.Records[i]))
		// Release after complete, so the next acquire sees failed.
		b.mu.Lock()
		b.inflight--
		b.inflightBytes -= size
		b.mu.Unlock()
		b.room.Signal()
	})
}

// complete records that record i finished. A non-nil err is a failure message
// marking does not skip. It marks the accepted prefix when it advances.
func (b *inflightBatch) complete(i int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.errs[i], b.done[i] = err, true
	if err != nil {
		b.failed = true
	}
	start := b.markedThrough
	for next := start + 1; next < len(b.done) && b.done[next] && b.errs[next] == nil; next++ {
		b.markedThrough = next
	}
	if b.markedThrough > start {
		b.c.markCommitRecords(b.pc, b.p.Topic, b.p.Partition, b.p.Records[b.markedThrough])
	}
}

// rememberFinished adds the accepted records from index from onward that
// already finished to pc.skipConsume.
func (b *inflightBatch) rememberFinished(from int) {
	for i := from; i < len(b.done); i++ {
		if !b.done[i] || b.errs[i] != nil {
			continue
		}
		if b.pc.skipConsume == nil {
			b.pc.skipConsume = make(map[int64]struct{})
		}
		b.pc.skipConsume[b.p.Records[i].Offset] = struct{}{}
	}
}

// dropSkipConsume removes skip entries at or below the last marked record. The
// map returns to nil when it is empty so later success batches do not keep a set.
func (p *pc) dropSkipConsume(marked *kgo.Record) {
	if p.skipConsume == nil {
		return
	}
	if marked != nil {
		for off := range p.skipConsume {
			if off <= marked.Offset {
				delete(p.skipConsume, off)
			}
		}
	}
	if len(p.skipConsume) == 0 {
		p.skipConsume = nil
	}
}
