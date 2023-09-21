// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"encoding/binary"
	"math"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// FromPdataConverter converts plog.Logs into a set of entry.Entry
//
// The diagram below illustrates the internal communication inside the FromPdataConverter:
//
//	          ┌─────────────────────────────────┐
//	          │ Batch()                         │
//	┌─────────┤  Ingests plog.Logs, splits up   │
//	│         │  and places them on workerChan  │
//	│         └─────────────────────────────────┘
//	│
//	│ ┌───────────────────────────────────────────────────┐
//	├─► workerLoop()                                      │
//	│ │ ┌─────────────────────────────────────────────────┴─┐
//	├─┼─► workerLoop()                                      │
//	│ │ │ ┌─────────────────────────────────────────────────┴─┐
//	└─┼─┼─► workerLoop()                                      │
//	  └─┤ │   consumes sent log entries from workerChan,      │
//	    │ │   translates received logs to entry.Entry,        │
//	    └─┤   and sends them along entriesChan                │
//	      └───────────────────────────────────────────────────┘
type FromPdataConverter struct {
	// entriesChan is a channel on which converted logs will be sent out of the converter.
	entriesChan chan []*entry.Entry

	stopOnce sync.Once
	stopChan chan struct{}

	// workerChan is an internal communication channel that gets the log
	// entries from Batch() calls and it receives the data in workerLoop().
	workerChan chan fromConverterWorkerItem

	// wg is a WaitGroup that makes sure that we wait for spun up goroutines exit
	// when Stop() is called.
	wg sync.WaitGroup

	logger *zap.Logger
}

func NewFromPdataConverter(workerCount int, logger *zap.Logger) *FromPdataConverter {
	if logger == nil {
		logger = zap.NewNop()
	}
	if workerCount <= 0 {
		workerCount = int(math.Max(1, float64(runtime.NumCPU())))
	}

	return &FromPdataConverter{
		workerChan:  make(chan fromConverterWorkerItem, workerCount),
		entriesChan: make(chan []*entry.Entry),
		stopChan:    make(chan struct{}),
		logger:      logger,
	}
}

func (c *FromPdataConverter) Start() {
	c.logger.Debug("Starting log converter from pdata", zap.Int("worker_count", cap(c.workerChan)))

	for i := 0; i < cap(c.workerChan); i++ {
		c.wg.Add(1)
		go c.workerLoop()
	}
}

func (c *FromPdataConverter) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
		c.wg.Wait()
		close(c.entriesChan)
		close(c.workerChan)
	})
}

// OutChannel returns the channel on which converted entries will be sent to.
func (c *FromPdataConverter) OutChannel() <-chan []*entry.Entry {
	return c.entriesChan
}

type fromConverterWorkerItem struct {
	Resource       pcommon.Resource
	LogRecordSlice plog.LogRecordSlice
	Scope          plog.ScopeLogs
}

// workerLoop is responsible for obtaining pdata logs from Batch() calls,
// converting them to []*entry.Entry and sending them out
func (c *FromPdataConverter) workerLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return

		case workerItem, ok := <-c.workerChan:
			if !ok {
				return
			}

			select {
			case c.entriesChan <- convertFromLogs(workerItem):
			case <-c.stopChan:
				return
			}
		}
	}
}

// Batch takes in an set of plog.Logs and sends it to an available worker for processing.
func (c *FromPdataConverter) Batch(pLogs plog.Logs) error {
	for i := 0; i < pLogs.ResourceLogs().Len(); i++ {
		rls := pLogs.ResourceLogs().At(i)
		for j := 0; j < rls.ScopeLogs().Len(); j++ {
			scope := rls.ScopeLogs().At(j)
			item := fromConverterWorkerItem{
				Resource:       rls.Resource(),
				Scope:          scope,
				LogRecordSlice: scope.LogRecords(),
			}
			select {
			case c.workerChan <- item:
				continue
			case <-c.stopChan:
				return nil
			}
		}
	}

	return nil
}

// convertFromLogs converts the contents of a fromConverterWorkerItem into a slice of entry.Entry
func convertFromLogs(workerItem fromConverterWorkerItem) []*entry.Entry {
	result := make([]*entry.Entry, 0, workerItem.LogRecordSlice.Len())
	for i := 0; i < workerItem.LogRecordSlice.Len(); i++ {
		record := workerItem.LogRecordSlice.At(i)
		entry := entry.Entry{}

		entry.ScopeName = workerItem.Scope.Scope().Name()
		entry.Resource = workerItem.Resource.Attributes().AsRaw()
		convertFrom(record, &entry)
		result = append(result, &entry)
	}
	return result
}

// convertFrom converts plog.LogRecord into provided entry.Entry.
func convertFrom(src plog.LogRecord, ent *entry.Entry) {
	// if src.Timestamp == 0, then leave ent.Timestamp as nil
	if src.Timestamp() != 0 {
		ent.Timestamp = src.Timestamp().AsTime()
	}

	if src.ObservedTimestamp() == 0 {
		ent.ObservedTimestamp = time.Now()
	} else {
		ent.ObservedTimestamp = src.ObservedTimestamp().AsTime()
	}

	ent.Severity = fromPdataSevMap[src.SeverityNumber()]
	ent.SeverityText = src.SeverityText()

	ent.Attributes = src.Attributes().AsRaw()
	ent.Body = src.Body().AsRaw()

	if !src.TraceID().IsEmpty() {
		buffer := src.TraceID()
		ent.TraceID = buffer[:]
	}
	if !src.SpanID().IsEmpty() {
		buffer := src.SpanID()
		ent.SpanID = buffer[:]
	}
	if src.Flags() != 0 {
		a := make([]byte, 4)
		binary.LittleEndian.PutUint32(a, uint32(src.Flags()))
		ent.TraceFlags = []byte{a[0]}
	}
}

var fromPdataSevMap = map[plog.SeverityNumber]entry.Severity{
	plog.SeverityNumberUnspecified: entry.Default,
	plog.SeverityNumberTrace:       entry.Trace,
	plog.SeverityNumberTrace2:      entry.Trace2,
	plog.SeverityNumberTrace3:      entry.Trace3,
	plog.SeverityNumberTrace4:      entry.Trace4,
	plog.SeverityNumberDebug:       entry.Debug,
	plog.SeverityNumberDebug2:      entry.Debug2,
	plog.SeverityNumberDebug3:      entry.Debug3,
	plog.SeverityNumberDebug4:      entry.Debug4,
	plog.SeverityNumberInfo:        entry.Info,
	plog.SeverityNumberInfo2:       entry.Info2,
	plog.SeverityNumberInfo3:       entry.Info3,
	plog.SeverityNumberInfo4:       entry.Info4,
	plog.SeverityNumberWarn:        entry.Warn,
	plog.SeverityNumberWarn2:       entry.Warn2,
	plog.SeverityNumberWarn3:       entry.Warn3,
	plog.SeverityNumberWarn4:       entry.Warn4,
	plog.SeverityNumberError:       entry.Error,
	plog.SeverityNumberError2:      entry.Error2,
	plog.SeverityNumberError3:      entry.Error3,
	plog.SeverityNumberError4:      entry.Error4,
	plog.SeverityNumberFatal:       entry.Fatal,
	plog.SeverityNumberFatal2:      entry.Fatal2,
	plog.SeverityNumberFatal3:      entry.Fatal3,
	plog.SeverityNumberFatal4:      entry.Fatal4,
}
