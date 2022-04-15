// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stanza // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"

import (
	"encoding/binary"
	"math"
	"runtime"
	"sync"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

// FromPdataConverter converts a set of entry.Entry into pdata.Logs
//
// The diagram below illustrates the internal communication inside the FromPdataConverter:
//
//            ┌─────────────────────────────────┐
//            │ Batch()                         │
//  ┌─────────┤  Ingests pdata.Logs, splits up  │
//  │         │  and sends them onto workerChan │
//  │         └─────────────────────────────────┘
//  │
//  │ ┌───────────────────────────────────────────────────┐
//  ├─► workerLoop()                                      │
//  │ │ ┌─────────────────────────────────────────────────┴─┐
//  ├─┼─► workerLoop()                                      │
//  │ │ │ ┌─────────────────────────────────────────────────┴─┐
//  └─┼─┼─► workerLoop()                                      │
//    └─┤ │   consumes sent log entries from workerChan,      │
//      │ │   translates received logs to entry.Entry,        │
//      └─┤   and sends them along                            │
//        └───────────────────────────────────────────────────┘
//
type FromPdataConverter struct {
	// entriesChan is a channel on which converted logs will be sent out of the converter.
	entriesChan chan []*entry.Entry

	stopOnce sync.Once
	stopChan chan struct{}

	// workerChan is an internal communication channel that gets the log
	// entries from Batch() calls and it receives the data in workerLoop().
	workerChan chan fromConverterWorkerItem
	// workerCount configures the amount of workers started.
	workerCount int

	// wg is a WaitGroup that makes sure that we wait for spun up goroutines exit
	// when Stop() is called.
	wg sync.WaitGroup

	logger *zap.Logger
}

func NewFromPdataConverter(workerCount int, logger *zap.Logger) *FromPdataConverter {
	if logger == nil {
		logger = zap.NewNop()
	}
	if workerCount == 0 {
		workerCount = int(math.Max(1, float64(runtime.NumCPU()/4)))
	}

	return &FromPdataConverter{
		workerChan:  make(chan fromConverterWorkerItem, workerCount),
		workerCount: workerCount,
		entriesChan: make(chan []*entry.Entry),
		stopChan:    make(chan struct{}),
		logger:      logger,
	}
}

func (c *FromPdataConverter) Start() {
	c.logger.Debug("Starting log converter", zap.Int("worker_count", c.workerCount))

	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.workerLoop()
	}
}

func (c *FromPdataConverter) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
		c.wg.Wait()
		close(c.entriesChan)
	})
}

// OutChannel returns the channel on which converted entries will be sent to.
func (c *FromPdataConverter) OutChannel() <-chan []*entry.Entry {
	return c.entriesChan
}

type fromConverterWorkerItem struct {
	Resource       pdata.Resource
	LogRecordSlice pdata.LogRecordSlice
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

// Batch takes in an set of pdata.Logs and sends it to an available worker for processing.
func (c *FromPdataConverter) Batch(pLogs pdata.Logs) error {
	for i := 0; i < pLogs.ResourceLogs().Len(); i++ {
		rls := pLogs.ResourceLogs().At(i)
		for j := 0; j < rls.ScopeLogs().Len(); j++ {
			scope := rls.ScopeLogs().At(j)
			select {
			case c.workerChan <- fromConverterWorkerItem{Resource: rls.Resource(), LogRecordSlice: scope.LogRecords()}:
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

		entry.Resource = valueToMap(workerItem.Resource.Attributes())
		convertFrom(record, &entry)
		result = append(result, &entry)
	}
	return result
}

// ConvertFrom converts pdata.Logs into a slice of entry.Entry
// To be used in a stateless setting like tests where ease of use is more
// important than performance or throughput.
func ConvertFrom(pLogs pdata.Logs) []*entry.Entry {
	result := []*entry.Entry{}
	for i := 0; i < pLogs.ResourceLogs().Len(); i++ {
		rls := pLogs.ResourceLogs().At(i)
		for j := 0; j < rls.ScopeLogs().Len(); j++ {
			scope := rls.ScopeLogs().At(j)
			result = append(result, convertFromLogs(fromConverterWorkerItem{Resource: rls.Resource(), LogRecordSlice: scope.LogRecords()})...)
		}
	}
	return result
}

// convertFrom converts pdata.LogRecord into provided entry.Entry.
func convertFrom(src pdata.LogRecord, ent *entry.Entry) {
	ent.Timestamp = src.Timestamp().AsTime()
	ent.Severity = fromPdataSevMap[src.SeverityNumber()]

	ent.Attributes = valueToMap(src.Attributes())
	ent.Body = valueToInterface(src.Body())

	if !src.TraceID().IsEmpty() {
		buffer := src.TraceID().Bytes()
		ent.TraceId = buffer[:]
	}
	if !src.SpanID().IsEmpty() {
		buffer := src.SpanID().Bytes()
		ent.SpanId = buffer[:]
	}
	if src.Flags() != 0 {
		a := make([]byte, 4)
		binary.LittleEndian.PutUint32(a, src.Flags())
		ent.TraceFlags[0] = a[0]
	}
}

func valueToMap(value pdata.Map) map[string]interface{} {
	rawMap := map[string]interface{}{}
	value.Range(func(k string, v pdata.Value) bool {
		rawMap[k] = valueToInterface(v)
		return true
	})
	return rawMap
}

func valueToInterface(value pdata.Value) interface{} {
	switch value.Type() {
	case pdata.ValueTypeEmpty:
		return nil
	case pdata.ValueTypeString:
		return value.StringVal()
	case pdata.ValueTypeBool:
		return value.BoolVal()
	case pdata.ValueTypeDouble:
		return value.DoubleVal()
	case pdata.ValueTypeInt:
		return value.IntVal()
	case pdata.ValueTypeBytes:
		return value.BytesVal()
	case pdata.ValueTypeMap:
		return value.MapVal().AsRaw()
	case pdata.ValueTypeSlice:
		arr := make([]interface{}, 0, value.SliceVal().Len())
		for i := 0; i < value.SliceVal().Len(); i++ {
			arr = append(arr, valueToInterface(value.SliceVal().At(i)))
		}
		return arr
	default:
		return value.AsString()
	}
}

var fromPdataSevMap = map[pdata.SeverityNumber]entry.Severity{
	pdata.SeverityNumberUNDEFINED: entry.Default,
	pdata.SeverityNumberTRACE:     entry.Trace,
	pdata.SeverityNumberTRACE2:    entry.Trace2,
	pdata.SeverityNumberTRACE3:    entry.Trace3,
	pdata.SeverityNumberTRACE4:    entry.Trace4,
	pdata.SeverityNumberDEBUG:     entry.Debug,
	pdata.SeverityNumberDEBUG2:    entry.Debug2,
	pdata.SeverityNumberDEBUG3:    entry.Debug3,
	pdata.SeverityNumberDEBUG4:    entry.Debug4,
	pdata.SeverityNumberINFO:      entry.Info,
	pdata.SeverityNumberINFO2:     entry.Info2,
	pdata.SeverityNumberINFO3:     entry.Info3,
	pdata.SeverityNumberINFO4:     entry.Info4,
	pdata.SeverityNumberWARN:      entry.Warn,
	pdata.SeverityNumberWARN2:     entry.Warn2,
	pdata.SeverityNumberWARN3:     entry.Warn3,
	pdata.SeverityNumberWARN4:     entry.Warn4,
	pdata.SeverityNumberERROR:     entry.Error,
	pdata.SeverityNumberERROR2:    entry.Error2,
	pdata.SeverityNumberERROR3:    entry.Error3,
	pdata.SeverityNumberERROR4:    entry.Error4,
	pdata.SeverityNumberFATAL:     entry.Fatal,
	pdata.SeverityNumberFATAL2:    entry.Fatal2,
	pdata.SeverityNumberFATAL3:    entry.Fatal3,
	pdata.SeverityNumberFATAL4:    entry.Fatal4,
}
