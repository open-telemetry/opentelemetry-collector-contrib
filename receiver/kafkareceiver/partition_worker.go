// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// pc represents the partition consumer shared information.
type pc struct {
	logger *zap.Logger
	attrs  attribute.Set

	ctx    context.Context
	cancel context.CancelCauseFunc
	// Not safe for concurrent use, this field is never accessed concurrently.
	backOff *backoff.ExponentialBackOff

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

// processPartitionBatch processes one fetched partition batch in offset order.
func (c *franzConsumer) processPartitionBatch(ctx context.Context, pc *pc, p kgo.FetchTopicPartition) {
	var fatalRecord *kgo.Record
	fatalIsPermanent := false
	var lastProcessed *kgo.Record
	for _, msg := range p.Records {
		if !c.config.MessageMarking.After {
			c.client.MarkCommitRecords(msg)
		}
		c.telemetryBuilder.KafkaReceiverCurrentOffset.Record(ctx, msg.Offset, metric.WithAttributeSet(pc.attrs))
		if err := c.handleMessage(pc, msg); err != nil {
			// Log at DEBUG level for shutdown/rebalance interruptions
			// (context cancellation), ERROR for real processing failures.
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
			// Pause consumption for partitions that have fatal errors.
			// handleMessage only returns an error when After=true and
			// the message should not be marked, so checking !shouldMark
			// here is consistent with that contract.
			isPermanent := consumererror.IsPermanent(err)
			shouldMark := (!isPermanent && c.config.MessageMarking.OnError) || (isPermanent && c.config.MessageMarking.OnPermanentError)

			if !shouldMark {
				fatalRecord = msg
				fatalIsPermanent = isPermanent
				break // Stop processing messages.
			}
		}
		lastProcessed = msg // Store so we can commit later.
	}
	// Handle fatal processing errors. For non-permanent errors
	// with backoff enabled, rewind the fetch cursor via SetOffsets
	// so the failed record is retried on the next PollRecords call,
	// consistent with how a rebalance restarts from the last
	// committed offset. No pause/resume is needed because
	// PollRecords is blocked on wg.Wait() until this goroutine
	// finishes.
	// Permanent errors and partitions without backoff configured
	// are paused until a rebalance triggers assigned(), which
	// calls ResumeFetchPartitions.
	if fatalRecord != nil {
		switch {
		case c.config.ErrorBackOff.Enabled && !fatalIsPermanent:
			// Skip rewind if the consumer is shutting down or the
			// partition was lost. In these cases the error is from
			// context cancellation, not a real processing failure,
			// and calling SetOffsets could interfere with the final
			// offset commit.
			select {
			case <-pc.ctx.Done():
			case <-c.closing:
			default:
				c.client.SetOffsets(map[string]map[int32]kgo.EpochOffset{
					p.Topic: {p.Partition: {
						Epoch:  fatalRecord.LeaderEpoch,
						Offset: fatalRecord.Offset,
					}},
				})
				pc.logger.Info("rewinding partition to retry failed record on next poll",
					zap.Int64("offset", fatalRecord.Offset),
				)
			}
		case fatalIsPermanent:
			tp := map[string][]int32{p.Topic: {p.Partition}}
			c.client.PauseFetchPartitions(tp)
			pc.logger.Error("pausing partition due to permanent processing error, partition will remain paused until rebalance",
				zap.Int64("offset", fatalRecord.Offset),
			)
		default:
			tp := map[string][]int32{p.Topic: {p.Partition}}
			c.client.PauseFetchPartitions(tp)
			pc.logger.Error("pausing partition due to processing error (no backoff configured), partition will remain paused until rebalance",
				zap.Int64("offset", fatalRecord.Offset),
			)
		}
	}
	if lastProcessed == nil {
		return // No metrics nor marks to update.
	}
	// Otherwise, publish consumer lag.
	c.telemetryBuilder.KafkaReceiverOffsetLag.Record(
		context.Background(),
		(p.HighWatermark-1)-lastProcessed.Offset,
		metric.WithAttributeSet(pc.attrs),
	)
	if c.config.MessageMarking.After {
		c.client.MarkCommitRecords(lastProcessed)
	}
}
