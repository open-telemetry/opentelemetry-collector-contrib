// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscwotlpbatchsplitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awscwotlpbatchsplitprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// an intermediate logs processor that manages log record batching with size-based constraints
// to prevent exceeding AWS CloudWatch Logs OTLP endpoint request size limits.
//
// Rather than exporting all logs in a single batch, this processor estimates log
// sizes and performs multiple batch exports where each exported batch is constrained
// to maxRequestByteSize. If a sub-batch would exceed the limit, it is exported before
// adding the next log. If a single log exceeds the limit on its own, it is exported
// as a batch of 1.
type awsCWOTLPBatchLogProcessor struct {
	logger             *zap.Logger
	nextConsumer       consumer.Logs
	maxRequestByteSize int
	baseLogBufferSize  int
}

type queueItem struct {
	val   pcommon.Value
	depth int
}

func (p *awsCWOTLPBatchLogProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *awsCWOTLPBatchLogProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *awsCWOTLPBatchLogProcessor) Shutdown(_ context.Context) error {
	return nil
}

func (p *awsCWOTLPBatchLogProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	subBatch := plog.NewLogs()
	subBatchSize := 0
	isOversized := false

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)

			var destRL plog.ResourceLogs
			var destSL plog.ScopeLogs
			newGroup := true

			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				logSize := p.estimateLogSize(rl.Resource(), sl.Scope(), lr)

				if subBatch.LogRecordCount() > 0 && subBatchSize+logSize > p.maxRequestByteSize {
					isOversized = true
					p.logger.Debug("exporting sub-batch",
						zap.Int("size_bytes", subBatchSize),
						zap.Int("log_count", subBatch.LogRecordCount()),
					)
					if err := p.nextConsumer.ConsumeLogs(ctx, subBatch); err != nil {
						return err
					}
					subBatch = plog.NewLogs()
					subBatchSize = 0
					newGroup = true
				}

				if newGroup {
					destRL = subBatch.ResourceLogs().AppendEmpty()
					rl.Resource().CopyTo(destRL.Resource())
					destSL = destRL.ScopeLogs().AppendEmpty()
					sl.Scope().CopyTo(destSL.Scope())
					newGroup = false
				}

				lr.CopyTo(destSL.LogRecords().AppendEmpty())
				subBatchSize += logSize
			}
		}
	}

	if !isOversized {
		p.logger.Debug("passing through batch",
			zap.Int("size_bytes", subBatchSize),
			zap.Int("log_count", ld.LogRecordCount()),
		)
		return p.nextConsumer.ConsumeLogs(ctx, ld)
	}

	if subBatch.LogRecordCount() > 0 {
		p.logger.Debug("exporting final sub-batch",
			zap.Int("size_bytes", subBatchSize),
			zap.Int("log_count", subBatch.LogRecordCount()),
		)
		return p.nextConsumer.ConsumeLogs(ctx, subBatch)
	}
	return nil
}

func (p *awsCWOTLPBatchLogProcessor) estimateLogSize(resource pcommon.Resource, scope pcommon.InstrumentationScope, lr plog.LogRecord) int {
	size := p.baseLogBufferSize

	size += estimateUTF8Size(scope.Name())
	size += estimateUTF8Size(scope.Version())

	queue := make([]queueItem, 0)
	queue = appendMapEntries(queue, resource.Attributes(), 0, &size)
	queue = appendMapEntries(queue, scope.Attributes(), 0, &size)
	queue = append(queue, queueItem{val: lr.Body(), depth: 0})
	queue = appendMapEntries(queue, lr.Attributes(), 0, &size)

	for len(queue) > 0 {
		if size >= p.maxRequestByteSize {
			return size
		}

		var next []queueItem
		for _, entry := range queue {
			v := entry.val
			switch v.Type() {
			case pcommon.ValueTypeStr:
				size += estimateUTF8Size(v.Str())
			case pcommon.ValueTypeInt, pcommon.ValueTypeDouble, pcommon.ValueTypeBool:
				size += estimateUTF8Size(v.AsString())
			case pcommon.ValueTypeBytes:
				size += v.Bytes().Len()
			case pcommon.ValueTypeMap:
				if entry.depth < defaultMaxDepth {
					next = appendMapEntries(next, v.Map(), entry.depth+1, &size)
				}
			case pcommon.ValueTypeSlice:
				if entry.depth < defaultMaxDepth {
					sl := v.Slice()
					for i := 0; i < sl.Len(); i++ {
						next = append(next, queueItem{val: sl.At(i), depth: entry.depth + 1})
					}
				}
			}
		}
		queue = next
	}

	return size
}

func appendMapEntries(queue []queueItem, m pcommon.Map, depth int, size *int) []queueItem {
	m.Range(func(k string, v pcommon.Value) bool {
		*size += len(k)
		queue = append(queue, queueItem{val: v, depth: depth})
		return true
	})
	return queue
}

func estimateUTF8Size(s string) int {
	ascii := 0
	nonASCII := 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			nonASCII++
		}
	}
	return ascii + (nonASCII * 4)
}
