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
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

// Converter converts a batch of entry.Entry into pdata.Logs aggregating translated
// entries into logs coming from the same Resource.
//
// The diagram below illustrates the internal communication inside the Converter:
//
//            ┌─────────────────────────────────┐
//            │ Batch()                         │
//  ┌─────────┤  Ingests batches of log entries │
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
//      │ │   translates received entries to pdata.LogRecords,│
//      └─┤   hashes them to generate an ID, and sends them   │
//        │   onto batchChan                                  │
//        └─────────────────────────┬─────────────────────────┘
//                                  │
//                                  ▼
//      ┌─────────────────────────────────────────────────────┐
//      │ aggregationLoop()                                   │
//      │   consumes from batchChan, aggregates log records   │
//      │   by marshaled Resource and sends the               │
//      │   aggregated buffer to flushChan                    │
//      └───────────────────────────┬─────────────────────────┘
//                                  │
//                                  ▼
//      ┌─────────────────────────────────────────────────────┐
//      │ flushLoop()                                         │
//      │   receives log records from flushChan and sends     │
//      │   them onto pLogsChan which is consumed by          │
//      │   downstream consumers via OutChannel()             │
//      └─────────────────────────────────────────────────────┘
//
type Converter struct {
	// pLogsChan is a channel on which aggregated logs will be sent to.
	pLogsChan chan pdata.Logs

	stopOnce sync.Once
	stopChan chan struct{}

	// workerChan is an internal communication channel that gets the log
	// entries from Batch() calls and it receives the data in workerLoop().
	workerChan chan []*entry.Entry
	// workerCount configures the amount of workers started.
	workerCount int
	// aggregationChan obtains log entries converted by the pool of workers,
	// in a form of logRecords grouped by Resource and then sends aggregated logs
	// on flushChan.
	aggregationChan chan []workerItem

	// flushChan is an internal channel used for transporting batched pdata.Logs.
	flushChan chan pdata.Logs

	// wg is a WaitGroup that makes sure that we wait for spun up goroutines exit
	// when Stop() is called.
	wg sync.WaitGroup

	logger *zap.Logger
}

type ConverterOption interface {
	apply(*Converter)
}

type optionFunc func(*Converter)

func (f optionFunc) apply(c *Converter) {
	f(c)
}

func WithLogger(logger *zap.Logger) ConverterOption {
	return optionFunc(func(c *Converter) {
		c.logger = logger
	})
}

func WithWorkerCount(workerCount int) ConverterOption {
	return optionFunc(func(c *Converter) {
		c.workerCount = workerCount
	})
}

func NewConverter(opts ...ConverterOption) *Converter {
	c := &Converter{
		workerChan:      make(chan []*entry.Entry),
		workerCount:     int(math.Max(1, float64(runtime.NumCPU()/4))),
		aggregationChan: make(chan []workerItem),
		pLogsChan:       make(chan pdata.Logs),
		stopChan:        make(chan struct{}),
		logger:          zap.NewNop(),
		flushChan:       make(chan pdata.Logs),
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	return c
}

func (c *Converter) Start() {
	c.logger.Debug("Starting log converter", zap.Int("worker_count", c.workerCount))

	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.workerLoop()
	}

	c.wg.Add(1)
	go c.aggregationLoop()

	c.wg.Add(1)
	go c.flushLoop()
}

func (c *Converter) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
		c.wg.Wait()
		close(c.pLogsChan)
	})
}

// OutChannel returns the channel on which converted entries will be sent to.
func (c *Converter) OutChannel() <-chan pdata.Logs {
	return c.pLogsChan
}

type workerItem struct {
	Resource   map[string]string
	LogRecord  pdata.LogRecord
	ResourceID uint64
}

// workerLoop is responsible for obtaining log entries from Batch() calls,
// converting them to pdata.LogRecords and sending them together with the
// associated Resource through the aggregationChan for aggregation.
func (c *Converter) workerLoop() {
	defer c.wg.Done()

	for {

		select {
		case <-c.stopChan:
			return

		case entries, ok := <-c.workerChan:
			if !ok {
				return
			}

			workerItems := make([]workerItem, 0, len(entries))

			for _, e := range entries {
				lr := convert(e)
				resourceID := getResourceID(e.Resource)
				workerItems = append(workerItems, workerItem{
					Resource:   e.Resource,
					ResourceID: resourceID,
					LogRecord:  lr,
				})
			}

			select {
			case c.aggregationChan <- workerItems:
			case <-c.stopChan:
			}
		}
	}
}

// aggregationLoop is responsible for receiving the converted log entries and aggregating
// them by Resource.
func (c *Converter) aggregationLoop() {
	defer c.wg.Done()

	resourceIDToLogs := make(map[uint64]pdata.Logs)

	for {
		select {
		case workerItems, ok := <-c.aggregationChan:
			if !ok {
				return
			}

			for _, wi := range workerItems {
				pLogs, ok := resourceIDToLogs[wi.ResourceID]
				if ok {
					lr := pLogs.ResourceLogs().
						At(0).InstrumentationLibraryLogs().
						At(0).Logs().AppendEmpty()
					wi.LogRecord.CopyTo(lr)
					continue
				}

				pLogs = pdata.NewLogs()
				logs := pLogs.ResourceLogs()
				rls := logs.AppendEmpty()

				resource := rls.Resource()
				resourceAtts := resource.Attributes()
				resourceAtts.EnsureCapacity(len(wi.Resource))
				for k, v := range wi.Resource {
					resourceAtts.InsertString(k, v)
				}

				ills := rls.InstrumentationLibraryLogs()
				lr := ills.AppendEmpty().Logs().AppendEmpty()
				wi.LogRecord.CopyTo(lr)

				resourceIDToLogs[wi.ResourceID] = pLogs
			}

			for r, pLogs := range resourceIDToLogs {
				c.flushChan <- pLogs
				delete(resourceIDToLogs, r)
			}

		case <-c.stopChan:
			return
		}
	}
}

func (c *Converter) flushLoop() {
	defer c.wg.Done()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-c.stopChan:
			return

		case pLogs := <-c.flushChan:
			if err := c.flush(ctx, pLogs); err != nil {
				c.logger.Debug("Problem sending log entries",
					zap.Error(err),
				)
			}
		}
	}
}

// flush flushes provided pdata.Logs entries onto a channel.
func (c *Converter) flush(ctx context.Context, pLogs pdata.Logs) error {
	doneChan := ctx.Done()

	select {
	case <-doneChan:
		return fmt.Errorf("flushing log entries interrupted, err: %w", ctx.Err())

	case c.pLogsChan <- pLogs:

	// The converter has been stopped so bail the flush.
	case <-c.stopChan:
		return errors.New("logs converter has been stopped")
	}

	return nil
}

// Batch takes in an entry.Entry and sends it to an available worker for processing.
func (c *Converter) Batch(e []*entry.Entry) error {
	select {
	case c.workerChan <- e:
		return nil
	case <-c.stopChan:
		return errors.New("logs converter has been stopped")
	}
}

// convert converts one entry.Entry into pdata.LogRecord allocating it.
func convert(ent *entry.Entry) pdata.LogRecord {
	dest := pdata.NewLogRecord()
	convertInto(ent, dest)
	return dest
}

// Convert converts one entry.Entry into pdata.Logs.
// To be used in a stateless setting like tests where ease of use is more
// important than performance or throughput.
func Convert(ent *entry.Entry) pdata.Logs {
	pLogs := pdata.NewLogs()
	logs := pLogs.ResourceLogs()

	rls := logs.AppendEmpty()

	resource := rls.Resource()
	resourceAtts := resource.Attributes()
	resourceAtts.EnsureCapacity(len(ent.Resource))
	for k, v := range ent.Resource {
		resourceAtts.InsertString(k, v)
	}

	ills := rls.InstrumentationLibraryLogs().AppendEmpty()
	lr := ills.Logs().AppendEmpty()
	convertInto(ent, lr)
	return pLogs
}

// convertInto converts entry.Entry into provided pdata.LogRecord.
func convertInto(ent *entry.Entry, dest pdata.LogRecord) {
	dest.SetTimestamp(pdata.NewTimestampFromTime(ent.Timestamp))
	dest.SetSeverityNumber(sevMap[ent.Severity])
	dest.SetSeverityText(sevTextMap[ent.Severity])

	if l := len(ent.Attributes); l > 0 {
		attributes := dest.Attributes()
		attributes.EnsureCapacity(l)
		for k, v := range ent.Attributes {
			attributes.InsertString(k, v)
		}
	}

	insertToAttributeVal(ent.Body, dest.Body())

	if ent.TraceId != nil {
		var buffer [16]byte
		copy(buffer[0:16], ent.TraceId)
		dest.SetTraceID(pdata.NewTraceID(buffer))
	}
	if ent.SpanId != nil {
		var buffer [8]byte
		copy(buffer[0:8], ent.SpanId)
		dest.SetSpanID(pdata.NewSpanID(buffer))
	}
	if ent.TraceFlags != nil {
		// The 8 least significant bits are the trace flags as defined in W3C Trace
		// Context specification. Don't override the 24 reserved bits.
		flags := dest.Flags()
		flags = flags & 0xFFFFFF00
		flags = flags | uint32(ent.TraceFlags[0])
		dest.SetFlags(flags)
	}
}

func insertToAttributeVal(value interface{}, dest pdata.AttributeValue) {
	switch t := value.(type) {
	case bool:
		dest.SetBoolVal(t)
	case string:
		dest.SetStringVal(t)
	case []byte:
		dest.SetStringVal(string(t))
	case int64:
		dest.SetIntVal(t)
	case int32:
		dest.SetIntVal(int64(t))
	case int16:
		dest.SetIntVal(int64(t))
	case int8:
		dest.SetIntVal(int64(t))
	case int:
		dest.SetIntVal(int64(t))
	case uint64:
		dest.SetIntVal(int64(t))
	case uint32:
		dest.SetIntVal(int64(t))
	case uint16:
		dest.SetIntVal(int64(t))
	case uint8:
		dest.SetIntVal(int64(t))
	case uint:
		dest.SetIntVal(int64(t))
	case float64:
		dest.SetDoubleVal(t)
	case float32:
		dest.SetDoubleVal(float64(t))
	case map[string]interface{}:
		toAttributeMap(t).CopyTo(dest)
	case []interface{}:
		toAttributeArray(t).CopyTo(dest)
	default:
		dest.SetStringVal(fmt.Sprintf("%v", t))
	}
}

func toAttributeMap(obsMap map[string]interface{}) pdata.AttributeValue {
	attVal := pdata.NewAttributeValueMap()
	attMap := attVal.MapVal()
	attMap.EnsureCapacity(len(obsMap))
	for k, v := range obsMap {
		switch t := v.(type) {
		case bool:
			attMap.InsertBool(k, t)
		case string:
			attMap.InsertString(k, t)
		case []byte:
			attMap.InsertString(k, string(t))
		case int64:
			attMap.InsertInt(k, t)
		case int32:
			attMap.InsertInt(k, int64(t))
		case int16:
			attMap.InsertInt(k, int64(t))
		case int8:
			attMap.InsertInt(k, int64(t))
		case int:
			attMap.InsertInt(k, int64(t))
		case uint64:
			attMap.InsertInt(k, int64(t))
		case uint32:
			attMap.InsertInt(k, int64(t))
		case uint16:
			attMap.InsertInt(k, int64(t))
		case uint8:
			attMap.InsertInt(k, int64(t))
		case uint:
			attMap.InsertInt(k, int64(t))
		case float64:
			attMap.InsertDouble(k, t)
		case float32:
			attMap.InsertDouble(k, float64(t))
		case map[string]interface{}:
			subMap := toAttributeMap(t)
			attMap.Insert(k, subMap)
		case []interface{}:
			arr := toAttributeArray(t)
			attMap.Insert(k, arr)
		default:
			attMap.InsertString(k, fmt.Sprintf("%v", t))
		}
	}
	return attVal
}

func toAttributeArray(obsArr []interface{}) pdata.AttributeValue {
	arrVal := pdata.NewAttributeValueArray()
	arr := arrVal.SliceVal()
	arr.EnsureCapacity(len(obsArr))
	for _, v := range obsArr {
		insertToAttributeVal(v, arr.AppendEmpty())
	}
	return arrVal
}

var sevMap = map[entry.Severity]pdata.SeverityNumber{
	entry.Default: pdata.SeverityNumberUNDEFINED,
	entry.Trace:   pdata.SeverityNumberTRACE,
	entry.Trace2:  pdata.SeverityNumberTRACE2,
	entry.Trace3:  pdata.SeverityNumberTRACE3,
	entry.Trace4:  pdata.SeverityNumberTRACE4,
	entry.Debug:   pdata.SeverityNumberDEBUG,
	entry.Debug2:  pdata.SeverityNumberDEBUG2,
	entry.Debug3:  pdata.SeverityNumberDEBUG3,
	entry.Debug4:  pdata.SeverityNumberDEBUG4,
	entry.Info:    pdata.SeverityNumberINFO,
	entry.Info2:   pdata.SeverityNumberINFO2,
	entry.Info3:   pdata.SeverityNumberINFO3,
	entry.Info4:   pdata.SeverityNumberINFO4,
	entry.Warn:    pdata.SeverityNumberWARN,
	entry.Warn2:   pdata.SeverityNumberWARN2,
	entry.Warn3:   pdata.SeverityNumberWARN3,
	entry.Warn4:   pdata.SeverityNumberWARN4,
	entry.Error:   pdata.SeverityNumberERROR,
	entry.Error2:  pdata.SeverityNumberERROR2,
	entry.Error3:  pdata.SeverityNumberERROR3,
	entry.Error4:  pdata.SeverityNumberERROR4,
	entry.Fatal:   pdata.SeverityNumberFATAL,
	entry.Fatal2:  pdata.SeverityNumberFATAL2,
	entry.Fatal3:  pdata.SeverityNumberFATAL3,
	entry.Fatal4:  pdata.SeverityNumberFATAL4,
}

var sevTextMap = map[entry.Severity]string{
	entry.Default: "",
	entry.Trace:   "Trace",
	entry.Trace2:  "Trace2",
	entry.Trace3:  "Trace3",
	entry.Trace4:  "Trace4",
	entry.Debug:   "Debug",
	entry.Debug2:  "Debug2",
	entry.Debug3:  "Debug3",
	entry.Debug4:  "Debug4",
	entry.Info:    "Info",
	entry.Info2:   "Info2",
	entry.Info3:   "Info3",
	entry.Info4:   "Info4",
	entry.Warn:    "Warn",
	entry.Warn2:   "Warn2",
	entry.Warn3:   "Warn3",
	entry.Warn4:   "Warn4",
	entry.Error:   "Error",
	entry.Error2:  "Error2",
	entry.Error3:  "Error3",
	entry.Error4:  "Error4",
	entry.Fatal:   "Fatal",
	entry.Fatal2:  "Fatal2",
	entry.Fatal3:  "Fatal3",
	entry.Fatal4:  "Fatal4",
}

// pairSep is chosen to be an invalid byte for a utf-8 sequence
// making it very unlikely to be present in the resource maps keys or values
var pairSep = []byte{0xfe}

// emptyResourceID is the ID returned by getResourceID when it is passed an empty resource.
// This specific number is chosen as it is the starting offset of fnv64.
const emptyResourceID uint64 = 14695981039346656037

func getResourceID(resource map[string]string) uint64 {
	if len(resource) == 0 {
		return emptyResourceID
	}

	var fnvHash = fnv.New64a()
	var fnvHashOut = make([]byte, 0, 16)
	var keySlice = make([]string, 0, len(resource))

	for k := range resource {
		keySlice = append(keySlice, k)
	}

	if len(keySlice) > 1 {
		// In order for this to be deterministic, we need to sort the map. Using range, like above,
		// has no guarantee about order.
		sort.Strings(keySlice)
	}

	for _, k := range keySlice {
		fnvHash.Write([]byte(k))
		fnvHash.Write(pairSep)

		fnvHash.Write([]byte(resource[k]))
		fnvHash.Write(pairSep)
	}

	fnvHashOut = fnvHash.Sum(fnvHashOut)
	return binary.BigEndian.Uint64(fnvHashOut)
}
