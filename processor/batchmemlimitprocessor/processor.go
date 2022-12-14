package batchmemlimitprocessor

import (
	"context"
	"go.uber.org/multierr"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"go.uber.org/zap"
)

type batchMemoryLimitProcessor struct {
	logger *zap.Logger

	timeout time.Duration

	batch      *batchLogs
	newItem    chan plog.Logs
	shutdownC  chan struct{}
	goroutines sync.WaitGroup
	timer      *time.Timer
}

func newBatchMemoryLimiterProcessor(next consumer.Logs, logger *zap.Logger, cfg *Config) *batchMemoryLimitProcessor {
	return &batchMemoryLimitProcessor{
		logger:    logger,
		batch:     newBatchLogs(next, int(cfg.SendBatchSize), int(cfg.SendMemorySize)),
		timeout:   cfg.Timeout,
		newItem:   make(chan plog.Logs, runtime.NumCPU()),
		shutdownC: make(chan struct{}, 1),
	}
}

func (mp *batchMemoryLimitProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start is invoked during service startup.
func (mp *batchMemoryLimitProcessor) Start(context.Context, component.Host) error {
	mp.goroutines.Add(1)
	go mp.startProcessingCycle()
	return nil
}

// Shutdown is invoked during service shutdown.
func (mp *batchMemoryLimitProcessor) Shutdown(context.Context) error {
	close(mp.shutdownC)

	// Wait until all goroutines are done.
	mp.goroutines.Wait()
	return nil
}

// ConsumeLogs implements LogsProcessor
func (mp *batchMemoryLimitProcessor) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	mp.newItem <- ld
	return nil
}

func (mp *batchMemoryLimitProcessor) startProcessingCycle() {
	defer mp.goroutines.Done()
	mp.timer = time.NewTimer(mp.timeout)
	for {
		select {
		case <-mp.shutdownC:
		DONE:
			for {
				select {
				case item := <-mp.newItem:
					mp.processItem(item)
				default:
					break DONE
				}
			}
			// This is the close of the channel
			if mp.batch.getLogCount() > 0 {
				mp.sendItems()
			}
			return
		case item := <-mp.newItem:
			mp.processItem(item)
		case <-mp.timer.C:
			if mp.batch.getLogCount() > 0 {
				mp.sendItems()
			}
			mp.resetTimer()
		}
	}
}

func (mp *batchMemoryLimitProcessor) sendItems() {
	err := mp.batch.sendAndClear()
	if err != nil {
		mp.logger.Warn("Sender failed", zap.Error(err))
	}
}

func (mp *batchMemoryLimitProcessor) processItem(item plog.Logs) {
	sent, err := mp.batch.add(item)
	if err != nil {
		mp.logger.Warn("Sender failed", zap.Error(err))
	}

	if sent {
		mp.stopTimer()
		mp.resetTimer()
	}
}

func (mp *batchMemoryLimitProcessor) stopTimer() {
	if !mp.timer.Stop() {
		<-mp.timer.C
	}
}

func (mp *batchMemoryLimitProcessor) resetTimer() {
	mp.timer.Reset(mp.timeout)
}

type limits struct {
	size  int
	count int
}

type batchLogs struct {
	nextConsumer consumer.Logs
	logData      plog.Logs
	logCount     int
	logSize      int
	sizer        plog.Sizer
	limits       limits
}

func newBatchLogs(nextConsumer consumer.Logs, batchSize int, memorySize int) *batchLogs {
	return &batchLogs{
		nextConsumer: nextConsumer,
		logData:      plog.NewLogs(),
		sizer:        plog.MarshalSizer(&plog.ProtoMarshaler{}).(plog.Sizer),
		limits: limits{
			size:  memorySize,
			count: batchSize,
		},
	}
}

func (bl *batchLogs) getLogCount() int {
	return bl.logCount
}

func (bl *batchLogs) add(ld plog.Logs) (bool, error) {

	newLogsCount := ld.LogRecordCount()
	newLogsSize := bl.sizer.LogsSize(ld)
	if newLogsCount == 0 || newLogsSize == 0 {
		return false, nil
	}

	var err error
	sent := false

	if bl.logCount+newLogsCount >= bl.limits.count || bl.logSize+newLogsSize >= bl.limits.size {
		sent = true
		err = bl.sendAndClear()
	}

	bl.logCount += newLogsCount
	bl.logSize += newLogsSize
	ld.ResourceLogs().MoveAndAppendTo(bl.logData.ResourceLogs())

	return sent, err
}

func (bl *batchLogs) sendAndClear() error {
	// breaking down the logs if they are above the size or count limit
	var err error
	for bl.logCount >= bl.limits.count || bl.logSize >= bl.limits.size {
		sendLogs := bl.splitLogs()
		if sendLogs.ResourceLogs().Len() == 0 {
			break
		}
		err = multierr.Append(err, bl.nextConsumer.ConsumeLogs(context.Background(), sendLogs))
	}

	err = bl.export()
	bl.resetLogs()
	return err
}

// splitLogs - removes the logs which don't meet the limits criteria and returns them
//
//	ResourceLogs > 1   										- split is directly done on ResourceLogs
//	ResourceLogs == 1 && ScopeLogs > 1 						- split will happen on ScopeLogs
//	ResourceLogs == 1 && ScopeLogs == 1 && LogRecords > 1 	- split will happen on LogRecords
//	If none of the above conditions satisfy then split will not take place
func (bl *batchLogs) splitLogs() plog.Logs {
	if bl.logCount < bl.limits.count && bl.logSize < bl.limits.size {
		dest := bl.logData
		bl.resetLogs()
		return dest
	}

	var totalCopiedCount, totalCopiedSize int
	dest := plog.NewLogs()

	if bl.logData.ResourceLogs().Len() > 1 {

		bl.logData.ResourceLogs().RemoveIf(func(resLogs plog.ResourceLogs) bool {
			count := 0
			for i := 0; i < resLogs.ScopeLogs().Len(); i++ {
				count += resLogs.ScopeLogs().At(i).LogRecords().Len()
			}
			l := plog.NewLogs()
			resLogs.MoveTo(l.ResourceLogs().AppendEmpty())
			size := bl.sizer.LogsSize(l)

			if totalCopiedCount+count >= bl.limits.count || totalCopiedSize+size >= bl.limits.size {
				return false
			}

			totalCopiedCount += count
			totalCopiedSize += size
			l.ResourceLogs().MoveAndAppendTo(dest.ResourceLogs())

			return true
		})

	} else if bl.logData.ResourceLogs().Len() == 1 {

		resLogs := bl.logData.ResourceLogs().At(0)
		scopeLogs := resLogs.ScopeLogs()

		l := plog.NewResourceLogsSlice()
		newResLog := l.AppendEmpty()
		resLogs.Resource().CopyTo(newResLog.Resource()) // copy all the attributes from ResourceLog

		if scopeLogs.Len() > 1 {

			scopeLogs.RemoveIf(func(scoLogs plog.ScopeLogs) bool {
				count := scoLogs.LogRecords().Len()
				tmp := plog.NewLogs()
				scoLogs.CopyTo(tmp.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty())
				size := bl.sizer.LogsSize(tmp)

				if totalCopiedCount+count >= bl.limits.count || totalCopiedSize+size >= bl.limits.size {
					return false
				}

				totalCopiedCount += count
				totalCopiedSize += size
				scoLogs.MoveTo(newResLog.ScopeLogs().AppendEmpty())

				return true
			})

		} else if scopeLogs.Len() == 1 {

			newScoLogs := newResLog.ScopeLogs().AppendEmpty()

			logRec := scopeLogs.At(0).LogRecords()

			logRec.RemoveIf(func(record plog.LogRecord) bool {
				count := 1
				tmp := plog.NewLogs()
				record.CopyTo(tmp.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty())
				size := bl.sizer.LogsSize(tmp)

				if totalCopiedCount+count >= bl.limits.count || totalCopiedSize+size >= bl.limits.size {
					return false
				}

				totalCopiedCount += count
				totalCopiedSize += size
				record.MoveTo(newScoLogs.LogRecords().AppendEmpty())

				return true
			})

		}

		if newResLog.ScopeLogs().Len() > 0 {
			newResLog.MoveTo(dest.ResourceLogs().AppendEmpty())
		}

	}

	bl.logCount -= totalCopiedCount
	bl.logSize -= totalCopiedSize
	return dest
}

func (bl *batchLogs) export() error {
	if bl.logData.ResourceLogs().Len() == 0 {
		return nil
	}

	return bl.nextConsumer.ConsumeLogs(context.Background(), bl.logData)
}

func (bl *batchLogs) resetLogs() {
	bl.logCount = 0
	bl.logSize = 0
	bl.logData = plog.NewLogs()
}
