// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Converter converts a batch of entry.Entry into plog.Logs aggregating translated
// entries into logs coming from the same Resource.
//
// The diagram below illustrates the internal communication inside the Converter:
//
//	          ┌─────────────────────────────────┐
//	          │ Batch()                         │
//	┌─────────┤  Ingests batches of log entries │
//	│         │  and sends them onto workerChan │
//	│         └─────────────────────────────────┘
//	│
//	│ ┌───────────────────────────────────────────────────┐
//	├─► workerLoop()                                      │
//	│ │ ┌─────────────────────────────────────────────────┴─┐
//	├─┼─► workerLoop()                                      │
//	│ │ │ ┌─────────────────────────────────────────────────┴─┐
//	└─┼─┼─► workerLoop()                                      │
//	  └─┤ │   consumes sent log entries from workerChan,      │
//	    │ │   translates received entries to plog.LogRecords, │
//	    └─┤   and sends them on flushChan                     │
//	      └─────────────────────────┬─────────────────────────┘
//	                                │
//	                                ▼
//	    ┌─────────────────────────────────────────────────────┐
//	    │ flushLoop()                                         │
//	    │   receives log records from flushChan and sends     │
//	    │   them onto pLogsChan which is consumed by          │
//	    │   downstream consumers via OutChannel()             │
//	    └─────────────────────────────────────────────────────┘
type Converter struct {
	// pLogsChan is a channel on which aggregated logs will be sent to.
	pLogsChan chan plog.Logs

	stopOnce sync.Once
	stopChan chan struct{}

	// workerChan is an internal communication channel that gets the log
	// entries from Batch() calls and it receives the data in workerLoop().
	workerChan chan []*entry.Entry
	// workerCount configures the amount of workers started.
	workerCount int

	// flushChan is an internal channel used for transporting batched plog.Logs.
	flushChan chan plog.Logs

	// wg is a WaitGroup that makes sure that we wait for spun up goroutines exit
	// when Stop() is called.
	wg sync.WaitGroup

	logger *zap.Logger
}

type converterOption interface {
	apply(*Converter)
}

func withWorkerCount(workerCount int) converterOption {
	return workerCountOption{workerCount}
}

type workerCountOption struct {
	workerCount int
}

func (o workerCountOption) apply(c *Converter) {
	c.workerCount = o.workerCount
}

func NewConverter(logger *zap.Logger, opts ...converterOption) *Converter {
	c := &Converter{
		workerChan:  make(chan []*entry.Entry),
		workerCount: int(math.Max(1, float64(runtime.NumCPU()/4))),
		pLogsChan:   make(chan plog.Logs),
		stopChan:    make(chan struct{}),
		flushChan:   make(chan plog.Logs),
		logger:      logger,
	}
	for _, opt := range opts {
		opt.apply(c)
	}
	return c
}

func (c *Converter) Start() {
	c.logger.Debug("Starting log converter", zap.Int("worker_count", c.workerCount))

	c.wg.Add(c.workerCount)
	for i := 0; i < c.workerCount; i++ {
		go c.workerLoop()
	}

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
func (c *Converter) OutChannel() <-chan plog.Logs {
	return c.pLogsChan
}

// workerLoop is responsible for obtaining log entries from Batch() calls,
// converting them to plog.LogRecords batched by Resource, and sending them
// on flushChan.
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

			resourceHashToIdx := make(map[uint64]int)

			pLogs := plog.NewLogs()
			var sl plog.ScopeLogs
			for _, e := range entries {
				resourceID := HashResource(e.Resource)
				resourceIdx, ok := resourceHashToIdx[resourceID]
				if !ok {
					resourceHashToIdx[resourceID] = pLogs.ResourceLogs().Len()
					rl := pLogs.ResourceLogs().AppendEmpty()
					upsertToMap(e.Resource, rl.Resource().Attributes())
					sl = rl.ScopeLogs().AppendEmpty()
				} else {
					sl = pLogs.ResourceLogs().At(resourceIdx).ScopeLogs().At(0)
				}
				convertInto(e, sl.LogRecords().AppendEmpty())
			}

			// Send plogs directly to flushChan
			select {
			case c.flushChan <- pLogs:
			case <-c.stopChan:
			}
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

// flush flushes provided plog.Logs entries onto a channel.
func (c *Converter) flush(ctx context.Context, pLogs plog.Logs) error {
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

// convert converts one entry.Entry into plog.LogRecord allocating it.
func convert(ent *entry.Entry) plog.LogRecord {
	dest := plog.NewLogRecord()
	convertInto(ent, dest)
	return dest
}

// convertInto converts entry.Entry into provided plog.LogRecord.
func convertInto(ent *entry.Entry, dest plog.LogRecord) {
	if !ent.Timestamp.IsZero() {
		dest.SetTimestamp(pcommon.NewTimestampFromTime(ent.Timestamp))
	}
	dest.SetObservedTimestamp(pcommon.NewTimestampFromTime(ent.ObservedTimestamp))
	dest.SetSeverityNumber(sevMap[ent.Severity])
	if ent.SeverityText == "" {
		dest.SetSeverityText(defaultSevTextMap[ent.Severity])
	} else {
		dest.SetSeverityText(ent.SeverityText)
	}

	upsertToMap(ent.Attributes, dest.Attributes())

	if ent.Body != nil {
		upsertToAttributeVal(ent.Body, dest.Body())
	}

	if ent.TraceID != nil {
		var buffer [16]byte
		copy(buffer[0:16], ent.TraceID)
		dest.SetTraceID(buffer)
	}
	if ent.SpanID != nil {
		var buffer [8]byte
		copy(buffer[0:8], ent.SpanID)
		dest.SetSpanID(buffer)
	}
	if ent.TraceFlags != nil && len(ent.TraceFlags) > 0 {
		// The 8 least significant bits are the trace flags as defined in W3C Trace
		// Context specification. Don't override the 24 reserved bits.
		flags := uint32(ent.TraceFlags[0])
		dest.SetFlags(plog.LogRecordFlags(flags))
	}
}

func upsertToAttributeVal(value any, dest pcommon.Value) {
	switch t := value.(type) {
	case bool:
		dest.SetBool(t)
	case string:
		dest.SetStr(t)
	case []string:
		upsertStringsToSlice(t, dest.SetEmptySlice())
	case []byte:
		dest.SetEmptyBytes().FromRaw(t)
	case int64:
		dest.SetInt(t)
	case int32:
		dest.SetInt(int64(t))
	case int16:
		dest.SetInt(int64(t))
	case int8:
		dest.SetInt(int64(t))
	case int:
		dest.SetInt(int64(t))
	case uint64:
		dest.SetInt(int64(t))
	case uint32:
		dest.SetInt(int64(t))
	case uint16:
		dest.SetInt(int64(t))
	case uint8:
		dest.SetInt(int64(t))
	case uint:
		dest.SetInt(int64(t))
	case float64:
		dest.SetDouble(t)
	case float32:
		dest.SetDouble(float64(t))
	case map[string]any:
		upsertToMap(t, dest.SetEmptyMap())
	case []any:
		upsertToSlice(t, dest.SetEmptySlice())
	default:
		dest.SetStr(fmt.Sprintf("%v", t))
	}
}

func upsertToMap(obsMap map[string]any, dest pcommon.Map) {
	dest.EnsureCapacity(len(obsMap))
	for k, v := range obsMap {
		upsertToAttributeVal(v, dest.PutEmpty(k))
	}
}

func upsertToSlice(obsArr []any, dest pcommon.Slice) {
	dest.EnsureCapacity(len(obsArr))
	for _, v := range obsArr {
		upsertToAttributeVal(v, dest.AppendEmpty())
	}
}

func upsertStringsToSlice(obsArr []string, dest pcommon.Slice) {
	dest.EnsureCapacity(len(obsArr))
	for _, v := range obsArr {
		dest.AppendEmpty().SetStr(v)
	}
}

var sevMap = map[entry.Severity]plog.SeverityNumber{
	entry.Default: plog.SeverityNumberUnspecified,
	entry.Trace:   plog.SeverityNumberTrace,
	entry.Trace2:  plog.SeverityNumberTrace2,
	entry.Trace3:  plog.SeverityNumberTrace3,
	entry.Trace4:  plog.SeverityNumberTrace4,
	entry.Debug:   plog.SeverityNumberDebug,
	entry.Debug2:  plog.SeverityNumberDebug2,
	entry.Debug3:  plog.SeverityNumberDebug3,
	entry.Debug4:  plog.SeverityNumberDebug4,
	entry.Info:    plog.SeverityNumberInfo,
	entry.Info2:   plog.SeverityNumberInfo2,
	entry.Info3:   plog.SeverityNumberInfo3,
	entry.Info4:   plog.SeverityNumberInfo4,
	entry.Warn:    plog.SeverityNumberWarn,
	entry.Warn2:   plog.SeverityNumberWarn2,
	entry.Warn3:   plog.SeverityNumberWarn3,
	entry.Warn4:   plog.SeverityNumberWarn4,
	entry.Error:   plog.SeverityNumberError,
	entry.Error2:  plog.SeverityNumberError2,
	entry.Error3:  plog.SeverityNumberError3,
	entry.Error4:  plog.SeverityNumberError4,
	entry.Fatal:   plog.SeverityNumberFatal,
	entry.Fatal2:  plog.SeverityNumberFatal2,
	entry.Fatal3:  plog.SeverityNumberFatal3,
	entry.Fatal4:  plog.SeverityNumberFatal4,
}

var defaultSevTextMap = map[entry.Severity]string{
	entry.Default: "",
	entry.Trace:   "TRACE",
	entry.Trace2:  "TRACE2",
	entry.Trace3:  "TRACE3",
	entry.Trace4:  "TRACE4",
	entry.Debug:   "DEBUG",
	entry.Debug2:  "DEBUG2",
	entry.Debug3:  "DEBUG3",
	entry.Debug4:  "DEBUG4",
	entry.Info:    "INFO",
	entry.Info2:   "INFO2",
	entry.Info3:   "INFO3",
	entry.Info4:   "INFO4",
	entry.Warn:    "WARN",
	entry.Warn2:   "WARN2",
	entry.Warn3:   "WARN3",
	entry.Warn4:   "WARN4",
	entry.Error:   "ERROR",
	entry.Error2:  "ERROR2",
	entry.Error3:  "ERROR3",
	entry.Error4:  "ERROR4",
	entry.Fatal:   "FATAL",
	entry.Fatal2:  "FATAL2",
	entry.Fatal3:  "FATAL3",
	entry.Fatal4:  "FATAL4",
}

// pairSep is chosen to be an invalid byte for a utf-8 sequence
// making it very unlikely to be present in the resource maps keys or values
var pairSep = []byte{0xfe}

// emptyResourceID is the ID returned by HashResource when it is passed an empty resource.
// This specific number is chosen as it is the starting offset of xxHash.
const emptyResourceID uint64 = 17241709254077376921

type hashWriter struct {
	h        *xxhash.Digest
	keySlice []string
}

func newHashWriter() *hashWriter {
	return &hashWriter{
		h:        xxhash.New(),
		keySlice: make([]string, 0),
	}
}

var hashWriterPool = &sync.Pool{
	New: func() any { return newHashWriter() },
}

// HashResource will hash an entry.Entry.Resource
func HashResource(resource map[string]any) uint64 {
	if len(resource) == 0 {
		return emptyResourceID
	}

	hw := hashWriterPool.Get().(*hashWriter)
	defer hashWriterPool.Put(hw)
	hw.h.Reset()
	hw.keySlice = hw.keySlice[:0]

	for k := range resource {
		hw.keySlice = append(hw.keySlice, k)
	}

	if len(hw.keySlice) > 1 {
		// In order for this to be deterministic, we need to sort the map. Using range, like above,
		// has no guarantee about order.
		sort.Strings(hw.keySlice)
	}

	for _, k := range hw.keySlice {
		hw.h.Write([]byte(k)) //nolint:errcheck
		hw.h.Write(pairSep)   //nolint:errcheck

		switch t := resource[k].(type) {
		case string:
			hw.h.Write([]byte(t)) //nolint:errcheck
		case []byte:
			hw.h.Write(t) //nolint:errcheck
		case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			binary.Write(hw.h, binary.BigEndian, t) // nolint - nothing to do about it
		default:
			b, _ := json.Marshal(t)
			hw.h.Write(b) //nolint:errcheck
		}

		hw.h.Write(pairSep) //nolint:errcheck
	}

	return hw.h.Sum64()
}
