package batchmemlimitprocessor

import (
	"context"
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

	sendBatchSize  int
	sendMemorySize int
	timeout        time.Duration

	batch      *batchLogs
	newItem    chan plog.Logs
	shutdownC  chan struct{}
	goroutines sync.WaitGroup
	timer      *time.Timer
}

func newBatchMemoryLimiterProcessor(next consumer.Logs, logger *zap.Logger, cfg *Config) *batchMemoryLimitProcessor {
	return &batchMemoryLimitProcessor{
		logger: logger,

		sendBatchSize:  int(cfg.SendBatchSize),
		sendMemorySize: int(cfg.SendMemorySize),

		batch:     newBatchLogs(next),
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
func (mp *batchMemoryLimitProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
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
	sent, err := mp.batch.add(item, mp.sendBatchSize, mp.sendMemorySize)
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

type batchLogs struct {
	nextConsumer consumer.Logs
	logData      plog.Logs
	logCount     int
	logSize      int
	sizer        plog.Sizer
}

func newBatchLogs(nextConsumer consumer.Logs) *batchLogs {
	return &batchLogs{
		nextConsumer: nextConsumer,
		logData:      plog.NewLogs(),
		sizer:        plog.MarshalSizer(&plog.ProtoMarshaler{}).(plog.Sizer),
	}
}

func (bl *batchLogs) getLogCount() int {
	return bl.logCount
}

func (bl *batchLogs) getLogSize() int {
	return bl.logSize
}

func (bl *batchLogs) add(ld plog.Logs, sendBatchSize int, sendMemorySize int) (bool, error) {

	var err error
	sent := false
	newLogsCount := ld.LogRecordCount()
	if newLogsCount == 0 {
		return sent, nil
	}
	newLogsSize := bl.sizer.LogsSize(ld)
	if newLogsSize == 0 {
		return sent, nil
	}

	if bl.logCount+newLogsCount >= sendBatchSize || bl.logSize+newLogsSize >= sendMemorySize {
		sent = true
		err = bl.sendAndClear()
	}

	bl.logCount += newLogsCount
	bl.logSize += newLogsSize
	ld.ResourceLogs().MoveAndAppendTo(bl.logData.ResourceLogs())

	return sent, err
}

func (bl *batchLogs) sendAndClear() error {
	err := bl.export()
	bl.resetLogs()
	return err
}

func (bl *batchLogs) export() error {
	return bl.nextConsumer.ConsumeLogs(context.Background(), bl.logData)
}

func (bl *batchLogs) resetLogs() {
	bl.logCount = 0
	bl.logSize = 0
	bl.logData = plog.NewLogs()
}
