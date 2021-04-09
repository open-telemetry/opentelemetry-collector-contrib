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

package stanza

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	DefaultFlushInterval = 100 * time.Millisecond
	DefaultMaxFlushCount = 100
)

// Converter converts entry.Entry into pdata.Logs aggregating translated
// entries into logs coming from the same Resource.
// Logs are being sent out based on the flush interval and/or the maximum
// batch size.
//
// The diagram below illustrates the internal communication inside the Converter.
//
//        ┌─────────────────────────────────┐
//        │ Batch()                         │
//        │  Ingests and converts log       │
//        │  entries and then spawns        │
//    ┌───┼─ go queueForFlush()             │
//    │   │  if maxFlushCount was reached   │
//    │   └─────────────────────────────────┘
//    │
//    │  ┌──────────────────────────────────────┐
//    ├──► queueForFlush goroutine(s)           │
//    │  │   Spawned whenever a batch of logs   │
//    │  │ ┌─is queued to be flushed.           │
//    │  │ │ Sends received logs onto flushChan │
//    │  └─┼────────────────────────────────────┘
//    │    │
//    │    │
//    │  ┌─┼───────────────────────────────────┐
//    │  │ │ Start()                           │
//    │  │ │  Starts a goroutine listening on: │
//    │  │ │                                   │
//    │  │ └► * flushChan   ───────────────────┼──┐
//    │  │                                     │  │
//    │  │    * ticker.C                       │  │
//    └──┼───── calls go queueForFlush() if    │  │
//       │      there's anything in the buffer │  │
//       └─────────────────────────────────────┘  │
//                                                │
//                                                │
//       ┌──────────────────────────────────────┐ │
//       │ flush()   ◄──────────────────────────┼─┘
//       │   Flushes converted and aggregated   │
//       │   logs onto pLogsChan which is       │
//       │   consumed by downstream consumers   │
//       │   viaoOutChannel()                   │
//       └──────────────────────────────────────┘
//
type Converter struct {
	// pLogsChan is a channel on which batched logs will be sent to.
	pLogsChan chan pdata.Logs

	stopOnce sync.Once
	stopChan chan struct{}

	// flushInterval defines how often we flush the aggregated log entries.
	flushInterval time.Duration
	// maxFlushCount defines what's the amount of entries in the buffer that
	// will trigger a flush of log entries.
	maxFlushCount uint
	// flushChan is an internal channel used for transporting batched pdata.Logs.
	flushChan chan []pdata.Logs

	// data is the internal cache which is flushed regularly, either when
	// flushInterval ticker ticks or when max number of entries for a
	// particular Resource is reached.
	data      map[string][]*entry.Entry
	dataMutex sync.RWMutex
	dataCount uint

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

func WithFlushInterval(interval time.Duration) ConverterOption {
	return optionFunc(func(c *Converter) {
		c.flushInterval = interval
	})
}

func WithMaxFlushCount(count uint) ConverterOption {
	return optionFunc(func(c *Converter) {
		c.maxFlushCount = count
	})
}

func WithLogger(logger *zap.Logger) ConverterOption {
	return optionFunc(func(c *Converter) {
		c.logger = logger
	})
}

func NewConverter(opts ...ConverterOption) *Converter {
	c := &Converter{
		pLogsChan:     make(chan pdata.Logs),
		stopChan:      make(chan struct{}),
		logger:        zap.NewNop(),
		flushChan:     make(chan []pdata.Logs),
		flushInterval: DefaultFlushInterval,
		maxFlushCount: DefaultMaxFlushCount,
		data:          make(map[string][]*entry.Entry),
		wg:            sync.WaitGroup{},
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	return c
}

func (c *Converter) Start() {
	c.logger.Debug("Starting log converter")

	c.wg.Add(1)
	go func(c *Converter) {
		defer c.wg.Done()

		ticker := time.NewTicker(c.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopChan:
				return

			case pLogs := <-c.flushChan:
				if err := c.flush(context.Background(), pLogs); err != nil {
					c.logger.Debug("Problem sending log entries",
						zap.Error(err),
					)
				}
				// NOTE:
				// Since we've received a flush signal independently of flush
				// ticker do we want to reset the flush ticker?

			case <-ticker.C:
				c.dataMutex.Lock()
				count := c.dataCount
				if count > 0 {
					pLogs := c.convertBuffer()
					go c.queueForFlush(pLogs)
				}
				c.dataMutex.Unlock()
			}
		}
	}(c)
}

func (c *Converter) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
		c.wg.Wait()
		close(c.pLogsChan)
	})
}

// Channel returns the channel on which converted entries will be sent to.
func (c *Converter) OutChannel() <-chan pdata.Logs {
	return c.pLogsChan
}

// flush flushes provided pdata.Logs entries onto a channel.
func (c *Converter) flush(ctx context.Context, pLogs []pdata.Logs) error {
	doneChan := ctx.Done()

	for _, pLog := range pLogs {
		select {
		case <-doneChan:
			return fmt.Errorf("flushing log entries interrupted, err: %v", ctx.Err())

		case c.pLogsChan <- pLog:

		// The converter has been stopped so bail the flush.
		case <-c.stopChan:
			return nil
		}
	}

	return nil
}

// Batch takes in an entry.Entry and aggregates it with other entries
// that came from the same Resource.
// If the maxFlushCount has been reached then trigger a flush via the flushChan.
func (c *Converter) Batch(e *entry.Entry) error {
	b, err := json.Marshal(e.Resource)
	if err != nil {
		return err
	}

	resource := string(b)

	// This is locked also for the possible conversion so that no entries are
	// added in the meantime so that the expected maximum batch size is not
	// exceeded.
	c.dataMutex.Lock()

	resourceEntries, ok := c.data[resource]
	if !ok {
		// If we don't have any log entries for this Resource then create
		// the provider entry in the cache for it.
		resourceEntries = make([]*entry.Entry, 0, 1)
	}

	c.data[resource] = append(resourceEntries, e)
	c.dataCount++

	needToFlush := c.dataCount >= c.maxFlushCount

	if needToFlush {
		// Flush max size has been reached: schedule a log flush.
		pLogs := c.convertBuffer()
		go c.queueForFlush(pLogs)
	}
	c.dataMutex.Unlock()

	return nil
}

// convertBuffer converts the accumulated entries in the buffer and empties it.
//
// NOTE: The caller needs to ensure that c.dataMutex is locked when this is called.
func (c *Converter) convertBuffer() []pdata.Logs {
	pLogs := make([]pdata.Logs, 0, len(c.data))
	for h, entries := range c.data {
		pLogs = append(pLogs, convertEntries(entries))
		delete(c.data, h)
	}
	c.dataCount = 0

	return pLogs
}

// queueForFlush queues the provided slice of pdata.Logs for flushing.
func (c *Converter) queueForFlush(pLogs []pdata.Logs) {
	select {
	case c.flushChan <- pLogs:
	case <-c.stopChan:
	}
}

// convertEntries converts takes in a slice of entries coming from the same
// Resource and converts them into a pdata.Logs.
func convertEntries(entries []*entry.Entry) pdata.Logs {
	out := pdata.NewLogs()
	if len(entries) == 0 {
		return out
	}

	logs := out.ResourceLogs()
	logs.Resize(1)
	rls := logs.At(0)

	// NOTE: This assumes that passed in entries all come from the same Resource.
	if len(entries[0].Resource) > 0 {
		resource := rls.Resource()
		resourceAtts := resource.Attributes()
		for k, v := range entries[0].Resource {
			resourceAtts.InsertString(k, v)
		}
	}

	rls.InstrumentationLibraryLogs().Resize(1)
	ills := rls.InstrumentationLibraryLogs().At(0)
	ills.Logs().Resize(len(entries))

	for i := 0; i < len(entries); i++ {
		ent := entries[i]
		convertInto(ent, ills.Logs().At(i))
	}

	return out
}

// convert converts one entry.Entry into pdata.LogRecord allocating it.
func convert(ent *entry.Entry) pdata.LogRecord {
	dest := pdata.NewLogRecord()
	convertInto(ent, dest)
	return dest
}

// convertInto converts entry.Entry into provided pdata.LogRecord.
func convertInto(ent *entry.Entry, dest pdata.LogRecord) {
	dest.SetTimestamp(pdata.TimestampFromTime(ent.Timestamp))

	sevText, sevNum := convertSeverity(ent.Severity)
	dest.SetSeverityText(sevText)
	dest.SetSeverityNumber(sevNum)

	if len(ent.Attributes) > 0 {
		attributes := dest.Attributes()
		for k, v := range ent.Attributes {
			attributes.InsertString(k, v)
		}
	}

	insertToAttributeVal(ent.Body, dest.Body())
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
	attMap.InitEmptyWithCapacity(len(obsMap))
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
	arr := arrVal.ArrayVal()
	for _, v := range obsArr {
		attVal := pdata.NewAttributeValueNull()
		insertToAttributeVal(v, attVal)
		arr.Append(attVal)
	}
	return arrVal
}

func convertSeverity(s entry.Severity) (string, pdata.SeverityNumber) {
	switch {

	// Handle standard severity levels
	case s == entry.Catastrophe:
		return "Fatal", pdata.SeverityNumberFATAL4
	case s == entry.Emergency:
		return "Error", pdata.SeverityNumberFATAL
	case s == entry.Alert:
		return "Error", pdata.SeverityNumberERROR3
	case s == entry.Critical:
		return "Error", pdata.SeverityNumberERROR2
	case s == entry.Error:
		return "Error", pdata.SeverityNumberERROR
	case s == entry.Warning:
		return "Info", pdata.SeverityNumberINFO4
	case s == entry.Notice:
		return "Info", pdata.SeverityNumberINFO3
	case s == entry.Info:
		return "Info", pdata.SeverityNumberINFO
	case s == entry.Debug:
		return "Debug", pdata.SeverityNumberDEBUG
	case s == entry.Trace:
		return "Trace", pdata.SeverityNumberTRACE2

	// Handle custom severity levels
	case s > entry.Emergency:
		return "Fatal", pdata.SeverityNumberFATAL2
	case s > entry.Alert:
		return "Error", pdata.SeverityNumberERROR4
	case s > entry.Critical:
		return "Error", pdata.SeverityNumberERROR3
	case s > entry.Error:
		return "Error", pdata.SeverityNumberERROR2
	case s > entry.Warning:
		return "Info", pdata.SeverityNumberINFO4
	case s > entry.Notice:
		return "Info", pdata.SeverityNumberINFO3
	case s > entry.Info:
		return "Info", pdata.SeverityNumberINFO2
	case s > entry.Debug:
		return "Debug", pdata.SeverityNumberDEBUG2
	case s > entry.Trace:
		return "Trace", pdata.SeverityNumberTRACE3
	case s > entry.Default:
		return "Trace", pdata.SeverityNumberTRACE

	default:
		return "Undefined", pdata.SeverityNumberUNDEFINED
	}
}
