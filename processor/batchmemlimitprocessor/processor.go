package batchmemlimitprocessor

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var sizer = plog.NewProtoMarshaler().(plog.Sizer)

type batchMemoryLimitProcessor struct {
	logger       *zap.Logger
	config       *Config
	nextConsumer consumer.Logs
}

func newBatchMemoryLimiterProcessor(next consumer.Logs, logger *zap.Logger, cfg *Config) *batchMemoryLimitProcessor {
	return &batchMemoryLimitProcessor{
		logger:       logger,
		config:       cfg,
		nextConsumer: next,
	}
}

func (mp *batchMemoryLimitProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start is invoked during service startup.
func (mp *batchMemoryLimitProcessor) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (mp *batchMemoryLimitProcessor) Shutdown(context.Context) error {
	return nil
}

// ConsumeLogs implements LogsProcessor
func (mp *batchMemoryLimitProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {

	if sizer.LogsSize(ld) <= mp.config.MemoryLimit {
		mp.nextConsumer.ConsumeLogs(ctx, ld)
		return nil
	}

	for {
		dest := mp.extractBatch(ld)
		if err := mp.nextConsumer.ConsumeLogs(ctx, dest); err != nil {
			return err
		}

		if ld.LogRecordCount() == 0 {
			break
		}
	}

	return nil
}

func (mp *batchMemoryLimitProcessor) extractBatch(ld plog.Logs) plog.Logs {
	dest := plog.NewLogs()
	size := 0

	ld.ResourceLogs().RemoveIf(func(rLog plog.ResourceLogs) bool {
		if rLog.ScopeLogs().Len() == 0 {
			return true
		}

		destRes := dest.ResourceLogs().AppendEmpty()
		rLog.Resource().CopyTo(destRes.Resource())

		rLog.ScopeLogs().RemoveIf(func(scLog plog.ScopeLogs) bool {
			if scLog.LogRecords().Len() == 0 {
				return true
			}

			destScope := destRes.ScopeLogs().AppendEmpty()
			scLog.Scope().CopyTo(destScope.Scope())

			scLog.LogRecords().RemoveIf(func(rec plog.LogRecord) bool {
				recordSize := getLogRecordProtoSize(rec)
				if recordSize > mp.config.MemoryLimit {
					mp.logger.Warn("log record size exceeds memory batch limit", zap.Int("log record size", recordSize), zap.Int("memory batch limit", mp.config.MemoryLimit))
					return true
				}
				if size+recordSize > mp.config.MemoryLimit {
					return false
				}
				size = size + recordSize
				rec.CopyTo(destScope.LogRecords().AppendEmpty())
				return true

			})
			return false
		})
		return false
	})

	return dest
}

func getLogRecordProtoSize(record plog.LogRecord) int {
	tempLD := plog.NewLogs()
	tempRec := tempLD.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	record.CopyTo(tempRec)
	return sizer.LogsSize(tempLD)
}
