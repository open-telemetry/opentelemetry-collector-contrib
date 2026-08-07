// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
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
	partitionPauseProcessingError
)

// partitionBatchResult returns offset operations to the poll loop. franz-go
// offset mutation and synchronous commits are serialized there instead of
// racing PollRecords from partition workers.
type partitionBatchResult struct {
	rewindRecord *kgo.Record
	commitRecord *kgo.Record
	terminal     bool
}

// pc represents the partition consumer shared information.
type pc struct {
	logger *zap.Logger
	attrs  attribute.Set

	ctx    context.Context
	cancel context.CancelCauseFunc
	// Not safe for concurrent use, this field is never accessed concurrently.
	backOff *backoff.ExponentialBackOff

	mailbox *partitionMailbox
	// pauseReasons stores a bitmask of partitionPauseReason values.
	pauseReasons atomic.Uint32

	mu sync.RWMutex // protects the fields below
	// wg tracks the number of in-flight message processing goroutines for this
	// partition. The wg must not be used directly; instead, the helper methods
	// add() and done() should be called to safely mutate it. These methods ensure
	// that no new goroutines are added once the partition consumer is stopping
	// (i.e. after the partition is lost / revoked).
	wg sync.WaitGroup
}

// add increments the wait group counter if the partition consumer is not
// stopping. It returns true if the counter was incremented, false otherwise.
func (p *pc) add(delta int) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	select {
	case <-p.ctx.Done():
		return false
	default:
	}
	p.wg.Add(delta)
	return true
}

// cancelContext cancels the partition consumer context while holding the write
// lock.
func (p *pc) cancelContext(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancel(err)
}

// done decrements the wait group counter.
func (p *pc) done() { p.wg.Done() }

// wait waits for all in-flight goroutines to finish.
func (p *pc) wait() { p.wg.Wait() }

// addPauseReason records why fetching must remain paused.
func (p *pc) addPauseReason(reason partitionPauseReason) {
	p.pauseReasons.Or(uint32(reason))
}

// clearPauseReasons reports whether the cleared reasons were the final active
// reasons, in which case the caller must resume fetching.
func (p *pc) clearPauseReasons(reasons partitionPauseReason) bool {
	mask := uint32(reasons)
	previous := p.pauseReasons.And(^mask)
	return previous&mask != 0 && previous&^mask == 0
}

// runPartitionWorker processes exactly one partition in offset order. Workers
// are independent, so downstream backpressure on one partition does not stop
// the shared poll loop or workers for other partitions.
func (c *franzConsumer) runPartitionWorker(pc *pc, tp topicPartition) {
	defer pc.done()
	partition := map[string][]int32{tp.topic: {tp.partition}}
	for {
		select {
		case <-pc.ctx.Done():
			return
		case <-pc.mailbox.notify:
		}

		for {
			select {
			case <-pc.ctx.Done():
				return
			default:
			}
			batch, ok := pc.mailbox.dequeue(func() {
				if pc.clearPauseReasons(partitionPauseBackpressure) {
					c.client.ResumeFetchPartitions(partition)
				}
			})
			if !ok {
				if pc.mailbox.hasPendingRewind() {
					if !c.sendControl(partitionControl{
						tp:     tp,
						pc:     pc,
						rewind: true,
						done:   make(chan struct{}),
					}) {
						return
					}
					if pc.clearPauseReasons(partitionPauseRewind | partitionPauseBackpressure) {
						c.client.ResumeFetchPartitions(partition)
					}
					break
				}
				break
			}

			result := c.processPartitionBatch(context.Background(), pc, batch)
			if result.rewindRecord != nil {
				pc.mailbox.requestRewind(result.rewindRecord, true, func() {
					pc.addPauseReason(partitionPauseRewind)
					c.client.PauseFetchPartitions(partition)
				})
				if !pc.mailbox.hasPendingRewind() {
					break
				}
				if !c.sendControl(partitionControl{
					tp:           tp,
					pc:           pc,
					commitRecord: result.commitRecord,
					rewind:       true,
					done:         make(chan struct{}),
				}) {
					return
				}
				if pc.clearPauseReasons(partitionPauseRewind | partitionPauseBackpressure) {
					c.client.ResumeFetchPartitions(partition)
				}
				break
			}
			if result.commitRecord != nil && !c.sendControl(partitionControl{
				tp:           tp,
				pc:           pc,
				commitRecord: result.commitRecord,
				done:         make(chan struct{}),
			}) {
				return
			}
			if result.terminal {
				return
			}
		}
	}
}

// processPartitionBatch applies message-marking semantics and returns
// partition-scoped offset work required by independent workers.
func (c *franzConsumer) processPartitionBatch(ctx context.Context, pc *pc, p kgo.FetchTopicPartition) partitionBatchResult {
	partitionScopedCommit := c.config.PartitionProcessing.Independent && !c.config.ConsumerConfig.AutoCommit.Enable
	var fatalRecord *kgo.Record
	fatalIsPermanent := false
	var lastProcessed *kgo.Record
	for _, msg := range p.Records {
		if !c.config.MessageMarking.After && !partitionScopedCommit {
			c.client.MarkCommitRecords(msg)
		}
		c.telemetryBuilder.KafkaReceiverCurrentOffset.Record(ctx, msg.Offset, metric.WithAttributeSet(pc.attrs))
		if err := c.handleMessage(pc, msg); err != nil {
			if pc.ctx.Err() != nil {
				pc.logger.Debug("message processing interrupted",
					zap.Error(err),
					zap.Int64("offset", msg.Offset),
				)
			} else {
				pc.logger.Error("unable to process message",
					zap.Error(err),
					zap.Int64("offset", msg.Offset),
				)
			}
			isPermanent := consumererror.IsPermanent(err)
			shouldMark := (!isPermanent && c.config.MessageMarking.OnError) || (isPermanent && c.config.MessageMarking.OnPermanentError)
			if !shouldMark {
				fatalRecord = msg
				fatalIsPermanent = isPermanent
				break
			}
		}
		lastProcessed = msg
	}

	result := partitionBatchResult{}
	if fatalRecord != nil {
		switch {
		case c.config.ErrorBackOff.Enabled && !fatalIsPermanent:
			select {
			case <-pc.ctx.Done():
				result.terminal = true
			case <-c.closing:
				result.terminal = true
			default:
				if c.config.PartitionProcessing.Independent {
					result.rewindRecord = fatalRecord
				} else {
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
		case fatalIsPermanent:
			pc.addPauseReason(partitionPauseProcessingError)
			c.client.PauseFetchPartitions(map[string][]int32{p.Topic: {p.Partition}})
			pc.logger.Error("pausing partition due to permanent processing error, partition will remain paused until rebalance",
				zap.Int64("offset", fatalRecord.Offset),
			)
			result.terminal = true
		default:
			pc.addPauseReason(partitionPauseProcessingError)
			c.client.PauseFetchPartitions(map[string][]int32{p.Topic: {p.Partition}})
			pc.logger.Error("pausing partition due to processing error (no backoff configured), partition will remain paused until rebalance",
				zap.Int64("offset", fatalRecord.Offset),
			)
			result.terminal = true
		}
	}
	if lastProcessed == nil {
		return result
	}
	c.telemetryBuilder.KafkaReceiverOffsetLag.Record(
		context.Background(),
		(p.HighWatermark-1)-lastProcessed.Offset,
		metric.WithAttributeSet(pc.attrs),
	)
	if partitionScopedCommit {
		result.commitRecord = lastProcessed
	} else if c.config.MessageMarking.After {
		c.client.MarkCommitRecords(lastProcessed)
	}
	return result
}
